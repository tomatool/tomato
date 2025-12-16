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
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// Runner manages the application under test
type Runner struct {
	config    config.AppConfig
	container *container.Manager
	cmd       *exec.Cmd
	appHost   string
	appPort   int

	// Log streaming
	showLogs   bool
	logLines   []string
	logMu      sync.Mutex
	stopLogs   chan struct{}
}

// NewRunner creates a new app runner
func NewRunner(cfg config.AppConfig, cm *container.Manager) *Runner {
	return &Runner{
		config:    cfg,
		container: cm,
		appHost:   "localhost",
		appPort:   cfg.Port,
		showLogs:  true,
		stopLogs:  make(chan struct{}),
	}
}

// SetShowLogs enables or disables log streaming
func (r *Runner) SetShowLogs(show bool) {
	r.showLogs = show
}

// Start starts the application with injected environment variables
func (r *Runner) Start(ctx context.Context) error {
	if r.config.Build != nil {
		return r.startDocker(ctx)
	}
	return r.startCommand(ctx)
}

// startCommand runs the app using a shell command
func (r *Runner) startCommand(ctx context.Context) error {
	log.Debug().Str("command", r.config.Command).Msg("starting app with command")

	// Build environment with container connections
	env, err := r.buildEnv(ctx)
	if err != nil {
		return fmt.Errorf("building environment: %w", err)
	}

	// Create command
	r.cmd = exec.CommandContext(ctx, "sh", "-c", r.config.Command)
	r.cmd.Env = append(os.Environ(), env...)

	if r.config.WorkDir != "" {
		r.cmd.Dir = r.config.WorkDir
	}

	// Set up pipes for streaming output
	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start the process
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("starting app: %w", err)
	}

	log.Debug().Int("pid", r.cmd.Process.Pid).Msg("app process started")

	// Stream logs in background
	go r.streamLogs(stdout, "stdout")
	go r.streamLogs(stderr, "stderr")

	// Wait for app to be ready
	if err := r.waitReady(ctx); err != nil {
		r.Stop()
		return err
	}

	return nil
}

// streamLogs reads from a pipe and displays logs
func (r *Runner) streamLogs(pipe io.ReadCloser, name string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		select {
		case <-r.stopLogs:
			return
		default:
		}

		line := scanner.Text()

		// Store log line
		r.logMu.Lock()
		r.logLines = append(r.logLines, line)
		// Keep only last 100 lines
		if len(r.logLines) > 100 {
			r.logLines = r.logLines[1:]
		}
		r.logMu.Unlock()

		// Display if enabled
		if r.showLogs {
			fmt.Printf("    │ %s\n", line)
		}
	}
}

// startDocker runs the app in a Docker container
func (r *Runner) startDocker(ctx context.Context) error {
	log.Debug().Str("dockerfile", r.config.Build.Dockerfile).Msg("starting app with docker")

	// Build environment with container connections
	env, err := r.buildEnv(ctx)
	if err != nil {
		return fmt.Errorf("building environment: %w", err)
	}

	// Build docker run command
	args := []string{"build", "-f", r.config.Build.Dockerfile}

	buildCtx := "."
	if r.config.Build.Context != "" {
		buildCtx = r.config.Build.Context
	}
	args = append(args, "-t", "tomato-app:test", buildCtx)

	// Build the image
	buildCmd := exec.CommandContext(ctx, "docker", args...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("building docker image: %w", err)
	}

	// Run the container
	runArgs := []string{"run", "-d", "--rm", "--name", "tomato-app"}

	// Add port mapping
	if r.config.Port > 0 {
		runArgs = append(runArgs, "-p", fmt.Sprintf("%d:%d", r.config.Port, r.config.Port))
	}

	// Add environment variables
	for _, e := range env {
		runArgs = append(runArgs, "-e", e)
	}

	// Connect to the same network as other containers (if using testcontainers network)
	runArgs = append(runArgs, "--network", "host")
	runArgs = append(runArgs, "tomato-app:test")

	runCmd := exec.CommandContext(ctx, "docker", runArgs...)
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("starting docker container: %w", err)
	}

	// Wait for app to be ready
	if err := r.waitReady(ctx); err != nil {
		r.Stop()
		return err
	}

	return nil
}

// buildEnv creates environment variables with container connection info
func (r *Runner) buildEnv(ctx context.Context) ([]string, error) {
	var env []string

	// Get connection info for all containers
	connInfo := make(map[string]map[string]string)

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

			// Cache container info
			if _, ok := connInfo[containerName]; !ok {
				connInfo[containerName] = make(map[string]string)

				host, err := r.container.GetHost(ctx, containerName)
				if err != nil {
					log.Warn().Str("container", containerName).Err(err).Msg("failed to get host")
					return match
				}
				connInfo[containerName]["host"] = host
			}

			switch infoType {
			case "host":
				return connInfo[containerName]["host"]
			case "port":
				if port == "" {
					return match // need port number
				}
				// Normalize port format
				if !strings.Contains(port, "/") {
					port = port + "/tcp"
				}
				mappedPort, err := r.container.GetPort(ctx, containerName, port)
				if err != nil {
					log.Warn().Str("container", containerName).Str("port", port).Err(err).Msg("failed to get port")
					return match
				}
				return mappedPort
			}
			return match
		})

		env = append(env, fmt.Sprintf("%s=%s", key, resolved))
	}

	return env, nil
}

// waitReady waits for the app to be ready
func (r *Runner) waitReady(ctx context.Context) error {
	if r.config.Ready == nil {
		// Default: wait for TCP port if configured
		if r.config.Port > 0 {
			return r.waitTCP(ctx, r.appHost, r.config.Port, 30*time.Second)
		}
		// No ready check configured, assume ready after short delay
		time.Sleep(2 * time.Second)
		return nil
	}

	timeout := r.config.Ready.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	switch r.config.Ready.Type {
	case "http":
		return r.waitHTTP(ctx, timeout)
	case "tcp":
		return r.waitTCP(ctx, r.appHost, r.config.Port, timeout)
	case "exec":
		return r.waitExec(ctx, timeout)
	default:
		return fmt.Errorf("unknown ready check type: %s", r.config.Ready.Type)
	}
}

func (r *Runner) waitHTTP(ctx context.Context, timeout time.Duration) error {
	path := r.config.Ready.Path
	if path == "" {
		path = "/health"
	}

	url := fmt.Sprintf("http://%s:%d%s", r.appHost, r.config.Port, path)
	expectedStatus := r.config.Ready.Status
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == expectedStatus {
				log.Debug().Str("url", url).Int("status", resp.StatusCode).Msg("app ready")
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("app not ready after %s (waiting for HTTP %d at %s)", timeout, expectedStatus, url)
}

func (r *Runner) waitTCP(ctx context.Context, host string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			log.Debug().Str("addr", addr).Msg("app ready (TCP)")
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("app not ready after %s (waiting for TCP %s)", timeout, addr)
}

func (r *Runner) waitExec(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", r.config.Ready.Command)
		if err := cmd.Run(); err == nil {
			log.Debug().Str("command", r.config.Ready.Command).Msg("app ready (exec)")
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("app not ready after %s (exec check failed)", timeout)
}

// Stop stops the application
func (r *Runner) Stop() error {
	if r.config.Build != nil {
		// Stop docker container
		cmd := exec.Command("docker", "stop", "tomato-app")
		return cmd.Run()
	}

	if r.cmd != nil && r.cmd.Process != nil {
		log.Debug().Int("pid", r.cmd.Process.Pid).Msg("stopping app")
		return r.cmd.Process.Kill()
	}

	return nil
}

// GetBaseURL returns the base URL for the running app
func (r *Runner) GetBaseURL() string {
	return fmt.Sprintf("http://%s:%d", r.appHost, r.appPort)
}
