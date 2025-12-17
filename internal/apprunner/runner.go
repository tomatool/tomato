package apprunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/runlog"
)

// Mode determines how the app is run
type Mode string

const (
	ModeCommand   Mode = "command"   // Run as local process
	ModeContainer Mode = "container" // Run in Docker container
)

// Runner manages the application under test
type Runner struct {
	config    config.AppConfig
	container *container.Manager
	mode      Mode

	// Command mode fields
	cmd     *exec.Cmd
	cmdHost string // Host for test runner (usually localhost)
	cmdPort int    // Port from config

	// Container mode fields
	appContainer testcontainers.Container
	appHost      string // Host for test runner (mapped port)
	appPort      int    // Mapped port on host

	// Log streaming
	showLogs     bool
	logLines     []string
	logMu        sync.Mutex
	stopLogs     chan struct{}
	stopLogsOnce sync.Once

	// Run context for logging
	runCtx  *runlog.RunContext
	logFile *os.File
}

// NewRunner creates a new app runner
func NewRunner(cfg config.AppConfig, cm *container.Manager) *Runner {
	// Determine mode based on config
	mode := ModeCommand
	if cfg.UseContainer() {
		mode = ModeContainer
	}

	return &Runner{
		config:    cfg,
		container: cm,
		mode:      mode,
		showLogs:  true,
		stopLogs:  make(chan struct{}),
	}
}

// SetContainerMode forces container mode (for --container flag)
func (r *Runner) SetContainerMode(enabled bool) error {
	if enabled {
		if !r.config.UseContainer() {
			return fmt.Errorf("container mode requires 'image' or 'build' in app config")
		}
		r.mode = ModeContainer
	}
	return nil
}

// GetMode returns the current running mode
func (r *Runner) GetMode() Mode {
	return r.mode
}

// SetShowLogs enables or disables log streaming
func (r *Runner) SetShowLogs(show bool) {
	r.showLogs = show
}

// SetRunContext sets the run context for logging
func (r *Runner) SetRunContext(ctx *runlog.RunContext) {
	r.runCtx = ctx
	if ctx != nil {
		f, err := ctx.CreateLogFile("app")
		if err != nil {
			log.Warn().Err(err).Msg("failed to create app log file")
		} else {
			r.logFile = f
		}
	}
}

// Start starts the application
func (r *Runner) Start(ctx context.Context) error {
	switch r.mode {
	case ModeCommand:
		return r.startCommand(ctx)
	case ModeContainer:
		return r.startContainer(ctx)
	default:
		return fmt.Errorf("unknown mode: %s", r.mode)
	}
}

// startCommand starts the app as a local process
func (r *Runner) startCommand(ctx context.Context) error {
	if r.config.Command == "" {
		return fmt.Errorf("app command is required for command mode")
	}

	// Build environment with mapped host ports (for local process)
	env := r.buildEnvForCommand()

	// Parse command
	parts := strings.Fields(r.config.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	r.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)

	// Set working directory
	if r.config.WorkDir != "" {
		r.cmd.Dir = r.config.WorkDir
	}

	// Build environment
	r.cmd.Env = os.Environ()
	for k, v := range env {
		r.cmd.Env = append(r.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture output
	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start process
	log.Debug().Str("command", r.config.Command).Msg("starting app process")
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("starting app: %w", err)
	}

	// Stream logs
	go r.streamCommandLogs(stdout, "stdout")
	go r.streamCommandLogs(stderr, "stderr")

	// Set host/port for test runner
	r.cmdHost = "localhost"
	r.cmdPort = r.config.Port

	// Wait for ready
	if err := r.waitForReady(ctx); err != nil {
		r.Stop()
		return fmt.Errorf("app not ready: %w", err)
	}

	// Additional wait time
	if r.config.Wait > 0 {
		time.Sleep(r.config.Wait)
	}

	log.Debug().
		Str("command", r.config.Command).
		Str("host", r.cmdHost).
		Int("port", r.cmdPort).
		Msg("app process ready")

	return nil
}

// buildEnvForCommand creates environment variables with mapped host ports
// Templates like {{.postgres.host}} resolve to localhost and mapped ports
func (r *Runner) buildEnvForCommand() map[string]string {
	env := make(map[string]string)

	// Template pattern: {{.container_name.host}} or {{.container_name.port.5432}}
	templatePattern := regexp.MustCompile(`\{\{\s*\.(\w+)\.(host|port)(?:\.(\d+(?:/tcp)?))?\s*\}\}`)

	for key, value := range r.config.Env {
		resolved := templatePattern.ReplaceAllStringFunc(value, func(match string) string {
			matches := templatePattern.FindStringSubmatch(match)
			if len(matches) < 3 {
				return match
			}

			containerName := matches[1]
			infoType := matches[2]
			port := matches[3]

			switch infoType {
			case "host":
				// Return localhost for command mode
				return "localhost"
			case "port":
				// Return mapped host port
				if port == "" {
					return match
				}
				// Ensure port has /tcp suffix for lookup
				if !strings.Contains(port, "/") {
					port = port + "/tcp"
				}
				mappedPort := r.container.GetMappedPort(containerName, port)
				if mappedPort == 0 {
					log.Warn().Str("container", containerName).Str("port", port).Msg("could not find mapped port")
					return match
				}
				return fmt.Sprintf("%d", mappedPort)
			}
			return match
		})

		env[key] = resolved
	}

	return env
}

// streamCommandLogs reads from a pipe and stores/displays logs
func (r *Runner) streamCommandLogs(pipe io.Reader, source string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		select {
		case <-r.stopLogs:
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Store log line
		r.logMu.Lock()
		r.logLines = append(r.logLines, line)
		if len(r.logLines) > 100 {
			r.logLines = r.logLines[1:]
		}
		r.logMu.Unlock()

		// Write to log file
		if r.logFile != nil {
			r.logFile.WriteString(fmt.Sprintf("[%s] %s\n", source, line))
		}

		// Display if enabled
		if r.showLogs {
			fmt.Printf("    │ %s\n", line)
		}
	}
}

// waitForReady waits for the app to be ready (command mode)
func (r *Runner) waitForReady(ctx context.Context) error {
	if r.config.Ready == nil && r.config.Port == 0 {
		return nil // No ready check configured
	}

	timeout := 30 * time.Second
	if r.config.Ready != nil && r.config.Ready.Timeout > 0 {
		timeout = r.config.Ready.Timeout
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if r.config.Ready != nil && r.config.Ready.Type == "http" {
			path := r.config.Ready.Path
			if path == "" {
				path = "/health"
			}
			url := fmt.Sprintf("http://%s:%d%s", r.cmdHost, r.cmdPort, path)
			expectedStatus := r.config.Ready.Status
			if expectedStatus == 0 {
				expectedStatus = 200
			}

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == expectedStatus {
					return nil
				}
			}
		} else if r.config.Port > 0 {
			// Default: TCP port check
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", r.cmdHost, r.cmdPort), 2*time.Second)
			if err == nil {
				conn.Close()
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for app to be ready")
}

// startContainer starts the app as a testcontainer
func (r *Runner) startContainer(ctx context.Context) error {
	// Build environment with container DNS names (for internal Docker communication)
	env := r.buildEnvForDocker()

	// Get network from container manager
	networkName := r.container.GetNetworkName()

	var req testcontainers.ContainerRequest

	if r.config.Image != "" {
		// Use pre-built image
		log.Debug().Str("image", r.config.Image).Msg("starting app with image")
		req = testcontainers.ContainerRequest{
			Image: r.config.Image,
		}
	} else if r.config.Build != nil {
		// Build from Dockerfile
		log.Debug().Str("dockerfile", r.config.Build.Dockerfile).Msg("starting app with dockerfile")

		buildCtx := "."
		if r.config.Build.Context != "" {
			buildCtx = r.config.Build.Context
		}

		req = testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       buildCtx,
				Dockerfile:    r.config.Build.Dockerfile,
				PrintBuildLog: true,
			},
		}
	} else {
		return fmt.Errorf("container mode requires 'image' or 'build' in app config")
	}

	// Container name/alias
	appName := r.config.GetName()

	// Attach to shared network with DNS alias
	req.Networks = []string{networkName}
	req.NetworkAliases = map[string][]string{
		networkName: {appName},
	}

	// Set environment variables
	req.Env = env

	// Expose port
	if r.config.Port > 0 {
		req.ExposedPorts = []string{fmt.Sprintf("%d/tcp", r.config.Port)}
	}

	// Build wait strategy from config
	req.WaitingFor = r.buildWaitStrategy()

	// Start container
	startTime := time.Now()
	appContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("starting app container: %w", err)
	}

	r.appContainer = appContainer

	// Get mapped port for test runner to connect
	if r.config.Port > 0 {
		host, err := appContainer.Host(ctx)
		if err != nil {
			r.Stop()
			return fmt.Errorf("getting app host: %w", err)
		}
		r.appHost = host

		mappedPort, err := appContainer.MappedPort(ctx, nat.Port(fmt.Sprintf("%d/tcp", r.config.Port)))
		if err != nil {
			r.Stop()
			return fmt.Errorf("getting mapped port: %w", err)
		}
		r.appPort = mappedPort.Int()
	}

	log.Debug().
		Str("name", appName).
		Str("host", r.appHost).
		Int("port", r.appPort).
		Dur("duration", time.Since(startTime)).
		Msg("app container ready")

	// Start log capture
	go r.captureContainerLogs(ctx)

	// Apply additional wait time if configured
	if r.config.Wait > 0 {
		time.Sleep(r.config.Wait)
	}

	return nil
}

// buildWaitStrategy creates a testcontainers wait strategy from config
func (r *Runner) buildWaitStrategy() wait.Strategy {
	if r.config.Ready == nil {
		// Default: wait for TCP port if configured
		if r.config.Port > 0 {
			return wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", r.config.Port))).
				WithStartupTimeout(30 * time.Second)
		}
		// No ready check, just wait briefly
		return wait.ForLog("").WithStartupTimeout(5 * time.Second)
	}

	timeout := r.config.Ready.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	switch r.config.Ready.Type {
	case "http":
		path := r.config.Ready.Path
		if path == "" {
			path = "/health"
		}
		expectedStatus := r.config.Ready.Status
		if expectedStatus == 0 {
			expectedStatus = 200
		}
		return wait.ForHTTP(path).
			WithPort(nat.Port(fmt.Sprintf("%d/tcp", r.config.Port))).
			WithStatusCodeMatcher(func(status int) bool { return status == expectedStatus }).
			WithStartupTimeout(timeout)

	case "tcp":
		return wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", r.config.Port))).
			WithStartupTimeout(timeout)

	case "exec":
		return wait.ForExec([]string{"sh", "-c", r.config.Ready.Command}).
			WithStartupTimeout(timeout)

	default:
		return wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", r.config.Port))).
			WithStartupTimeout(timeout)
	}
}

// buildEnvForDocker creates environment variables with container DNS names
// Templates like {{.postgres.host}} resolve to container names (not mapped ports)
func (r *Runner) buildEnvForDocker() map[string]string {
	env := make(map[string]string)

	// Template pattern: {{.container_name.host}} or {{.container_name.port.5432}}
	templatePattern := regexp.MustCompile(`\{\{\s*\.(\w+)\.(host|port)(?:\.(\d+(?:/tcp)?))?\s*\}\}`)

	for key, value := range r.config.Env {
		resolved := templatePattern.ReplaceAllStringFunc(value, func(match string) string {
			matches := templatePattern.FindStringSubmatch(match)
			if len(matches) < 3 {
				return match
			}

			containerName := matches[1]
			infoType := matches[2]
			port := matches[3]

			switch infoType {
			case "host":
				// Return container DNS name (accessible within Docker network)
				return containerName
			case "port":
				// Return internal port (not mapped host port)
				if port == "" {
					return match // need port number
				}
				// Strip /tcp suffix if present
				if idx := strings.Index(port, "/"); idx > 0 {
					port = port[:idx]
				}
				return port
			}
			return match
		})

		env[key] = resolved
	}

	return env
}

// captureContainerLogs streams container logs to file and memory
func (r *Runner) captureContainerLogs(ctx context.Context) {
	if r.appContainer == nil {
		return
	}

	logs, err := r.appContainer.Logs(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get app container logs")
		return
	}
	defer logs.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-r.stopLogs:
			return
		case <-ctx.Done():
			return
		default:
		}

		n, err := logs.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Debug().Err(err).Msg("error reading app logs")
			}
			return
		}

		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}

				// Store log line
				r.logMu.Lock()
				r.logLines = append(r.logLines, line)
				// Keep only last 100 lines
				if len(r.logLines) > 100 {
					r.logLines = r.logLines[1:]
				}
				r.logMu.Unlock()

				// Write to log file if available
				if r.logFile != nil {
					r.logFile.WriteString(line + "\n")
				}

				// Display if enabled
				if r.showLogs {
					fmt.Printf("    │ %s\n", line)
				}
			}
		}
	}
}

// Stop stops the application
func (r *Runner) Stop() error {
	// Signal log streaming to stop (only once)
	r.stopLogsOnce.Do(func() {
		close(r.stopLogs)
	})

	// Close log file
	if r.logFile != nil {
		r.logFile.Close()
		r.logFile = nil
	}

	switch r.mode {
	case ModeCommand:
		return r.stopCommand()
	case ModeContainer:
		return r.stopContainer()
	}
	return nil
}

// stopCommand stops the local process
func (r *Runner) stopCommand() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	log.Debug().Int("pid", r.cmd.Process.Pid).Msg("stopping app process")

	// Try graceful shutdown first
	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Debug().Err(err).Msg("failed to send SIGTERM, trying SIGKILL")
		if err := r.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("killing app process: %w", err)
		}
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		_, err := r.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Force kill if graceful shutdown takes too long
		r.cmd.Process.Kill()
		<-done
	}

	r.cmd = nil
	return nil
}

// stopContainer stops the Docker container
func (r *Runner) stopContainer() error {
	if r.appContainer == nil {
		return nil
	}

	log.Debug().Msg("stopping app container")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.appContainer.Terminate(ctx); err != nil {
		log.Warn().Err(err).Msg("failed to terminate app container")
		return err
	}
	r.appContainer = nil
	return nil
}

// GetBaseURL returns the base URL for the running app (for test runner to connect)
func (r *Runner) GetBaseURL() string {
	host, port := r.GetHostPort()
	return fmt.Sprintf("http://%s:%d", host, port)
}

// GetHost returns the host address for the running app
func (r *Runner) GetHost() string {
	if r.mode == ModeContainer {
		return r.appHost
	}
	return r.cmdHost
}

// GetPort returns the port for the running app
func (r *Runner) GetPort() int {
	if r.mode == ModeContainer {
		return r.appPort
	}
	return r.cmdPort
}

// GetHostPort returns both host and port
func (r *Runner) GetHostPort() (string, int) {
	if r.mode == ModeContainer {
		return r.appHost, r.appPort
	}
	return r.cmdHost, r.cmdPort
}

// GetContainer returns the underlying testcontainer (nil in command mode)
func (r *Runner) GetContainer() testcontainers.Container {
	return r.appContainer
}

// GetInternalPort returns the internal container port (same as config port)
func (r *Runner) GetInternalPort() int {
	return r.config.Port
}

// GetRecentLogs returns the most recent log lines
func (r *Runner) GetRecentLogs(n int) []string {
	r.logMu.Lock()
	defer r.logMu.Unlock()

	if n <= 0 || len(r.logLines) == 0 {
		return nil
	}

	start := len(r.logLines) - n
	if start < 0 {
		start = 0
	}

	result := make([]string, len(r.logLines)-start)
	copy(result, r.logLines[start:])
	return result
}

// VerifyHealthy performs a single health check to verify the app is responding
func (r *Runner) VerifyHealthy(ctx context.Context) error {
	host, port := r.GetHostPort()
	if port == 0 {
		return nil // No port configured, skip check
	}

	// Try HTTP health check first if configured
	if r.config.Ready != nil && r.config.Ready.Type == "http" {
		path := r.config.Ready.Path
		if path == "" {
			path = "/health"
		}
		url := fmt.Sprintf("http://%s:%d%s", host, port, path)
		expectedStatus := r.config.Ready.Status
		if expectedStatus == 0 {
			expectedStatus = 200
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != expectedStatus {
			return fmt.Errorf("health check returned status %d, expected %d", resp.StatusCode, expectedStatus)
		}
		return nil
	}

	// Default: TCP port check
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("app not responding on %s: %w", addr, err)
	}
	conn.Close()
	return nil
}
