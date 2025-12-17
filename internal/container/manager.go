package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/runlog"
)

// ErrDockerNotRunning is returned when Docker daemon is not available
var ErrDockerNotRunning = fmt.Errorf("docker is not running")

// CheckDockerAvailable verifies that Docker daemon is running and accessible
func CheckDockerAvailable() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return &DockerNotRunningError{}
	}
	return nil
}

// DockerNotRunningError provides helpful instructions for starting Docker
type DockerNotRunningError struct{}

func (e *DockerNotRunningError) Error() string {
	var instructions string

	switch runtime.GOOS {
	case "darwin":
		instructions = `Docker is not running. To fix this:

  1. Open Docker Desktop application
  2. Wait for Docker to start (whale icon in menu bar stops animating)
  3. Run tomato again

  Or start Docker from terminal:
    open -a Docker`

	case "linux":
		instructions = `Docker is not running. To fix this:

  1. Start Docker daemon:
       sudo systemctl start docker

  2. Make sure your user is in the docker group:
       sudo usermod -aG docker $USER
       (log out and back in after this)

  3. Run tomato again`

	case "windows":
		instructions = `Docker is not running. To fix this:

  1. Open Docker Desktop application
  2. Wait for Docker to start
  3. Run tomato again`

	default:
		instructions = `Docker is not running. Please start Docker and try again.`
	}

	return instructions
}

// Manager handles the lifecycle of test containers
type Manager struct {
	configs     map[string]config.Container
	containers  map[string]testcontainers.Container
	order       []string // startup order based on dependencies
	mu          sync.RWMutex
	runCtx      *runlog.RunContext
	logFiles    map[string]*os.File
	network     *testcontainers.DockerNetwork
	networkName string
}

// NewManager creates a new container manager
func NewManager(configs map[string]config.Container) (*Manager, error) {
	m := &Manager{
		configs:     configs,
		containers:  make(map[string]testcontainers.Container),
		logFiles:    make(map[string]*os.File),
		networkName: fmt.Sprintf("tomato-%s", uuid.New().String()[:8]),
	}

	// Calculate startup order based on dependencies
	order, err := m.calculateStartOrder()
	if err != nil {
		return nil, fmt.Errorf("calculating start order: %w", err)
	}
	m.order = order

	return m, nil
}

// SetRunContext sets the run context for logging
func (m *Manager) SetRunContext(ctx *runlog.RunContext) {
	m.runCtx = ctx
}

// CreateNetwork creates the shared Docker network for all containers
func (m *Manager) CreateNetwork(ctx context.Context) error {
	if m.network != nil {
		return nil // Already created
	}

	log.Debug().Str("network", m.networkName).Msg("creating docker network")

	net, err := network.New(ctx, network.WithCheckDuplicate(), network.WithDriver("bridge"))
	if err != nil {
		return fmt.Errorf("creating network: %w", err)
	}

	m.network = net
	m.networkName = net.Name

	log.Debug().Str("network", m.networkName).Msg("docker network created")
	return nil
}

// GetNetworkName returns the shared network name
func (m *Manager) GetNetworkName() string {
	return m.networkName
}

// GetInternalAddress returns the internal Docker DNS address for a container
// This is used by containers to communicate with each other within the network
func (m *Manager) GetInternalAddress(name, port string) string {
	// Strip /tcp or /udp suffix if present
	if idx := strings.Index(port, "/"); idx > 0 {
		port = port[:idx]
	}
	return fmt.Sprintf("%s:%s", name, port)
}

// RegisterContainer adds an externally managed container to the manager
// This allows resources to reference containers started by apprunner
func (m *Manager) RegisterContainer(name string, container testcontainers.Container) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.containers[name] = container
}

// calculateStartOrder returns containers in dependency order using topological sort
func (m *Manager) calculateStartOrder() ([]string, error) {
	// Build dependency graph
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for name := range m.configs {
		inDegree[name] = 0
	}

	for name, cfg := range m.configs {
		for _, dep := range cfg.DependsOn {
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic order for containers with no deps

	var order []string
	for len(queue) > 0 {
		// Pop from queue
		name := queue[0]
		queue = queue[1:]
		order = append(order, name)

		// Reduce in-degree of dependents
		for _, dep := range dependents[name] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(m.configs) {
		return nil, fmt.Errorf("circular dependency detected in container configuration")
	}

	return order, nil
}

// StartAll starts all containers in dependency order
func (m *Manager) StartAll(ctx context.Context) error {
	// Create the shared network first
	if err := m.CreateNetwork(ctx); err != nil {
		return fmt.Errorf("creating network: %w", err)
	}

	for _, name := range m.order {
		if err := m.Start(ctx, name); err != nil {
			return fmt.Errorf("starting container %s: %w", name, err)
		}
	}
	return nil
}

// Start starts a single container
func (m *Manager) Start(ctx context.Context, name string) error {
	cfg, ok := m.configs[name]
	if !ok {
		return fmt.Errorf("unknown container: %s", name)
	}

	log.Debug().Str("container", name).Str("image", cfg.Image).Msg("starting container")
	startTime := time.Now()

	req := testcontainers.ContainerRequest{
		Image:      cfg.Image,
		Env:        cfg.Env,
		WaitingFor: m.buildWaitStrategy(cfg.WaitFor),
	}

	// Add exposed ports
	for _, port := range cfg.Ports {
		req.ExposedPorts = append(req.ExposedPorts, port)
	}

	// Attach to shared network with DNS alias
	if m.network != nil {
		req.Networks = []string{m.networkName}
		req.NetworkAliases = map[string][]string{
			m.networkName: {name}, // Container accessible via its config name
		}
	}

	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	m.mu.Lock()
	m.containers[name] = container
	m.mu.Unlock()

	log.Debug().
		Str("container", name).
		Dur("duration", time.Since(startTime)).
		Msg("container ready")

	// Start capturing container logs if run context is set
	if m.runCtx != nil {
		go m.captureContainerLogs(ctx, name, container)
	}

	return nil
}

// captureContainerLogs streams container logs to a file
func (m *Manager) captureContainerLogs(ctx context.Context, name string, container testcontainers.Container) {
	logFile, err := m.runCtx.CreateLogFile("container-" + name)
	if err != nil {
		log.Warn().Err(err).Str("container", name).Msg("failed to create container log file")
		return
	}

	m.mu.Lock()
	m.logFiles[name] = logFile
	m.mu.Unlock()

	// Get container logs
	logs, err := container.Logs(ctx)
	if err != nil {
		log.Warn().Err(err).Str("container", name).Msg("failed to get container logs")
		return
	}

	// Stream logs to file
	go func() {
		defer logs.Close()
		io.Copy(logFile, logs)
	}()
}

// buildWaitStrategy converts config wait strategy to testcontainers wait strategy
func (m *Manager) buildWaitStrategy(ws config.WaitStrategy) wait.Strategy {
	timeout := ws.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	switch ws.Type {
	case "port":
		return wait.ForListeningPort(nat.Port(ws.Target)).WithStartupTimeout(timeout)
	case "log":
		return wait.ForLog(ws.Target).WithStartupTimeout(timeout)
	case "http":
		strategy := wait.ForHTTP(ws.Path).WithPort(nat.Port(ws.Target)).WithStartupTimeout(timeout)
		if ws.Method != "" {
			strategy = strategy.WithMethod(ws.Method)
		}
		return strategy
	case "exec":
		return wait.ForExec([]string{"sh", "-c", ws.Target}).WithStartupTimeout(timeout)
	default:
		// Default: wait for container to be running
		return wait.ForLog("").WithStartupTimeout(timeout)
	}
}

// Get returns a running container by name
func (m *Manager) Get(name string) (testcontainers.Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, ok := m.containers[name]
	if !ok {
		return nil, fmt.Errorf("container not found: %s", name)
	}
	return container, nil
}

// GetHost returns the host address for a container
func (m *Manager) GetHost(ctx context.Context, name string) (string, error) {
	container, err := m.Get(name)
	if err != nil {
		return "", err
	}
	return container.Host(ctx)
}

// GetPort returns the mapped port for a container
func (m *Manager) GetPort(ctx context.Context, name, port string) (string, error) {
	container, err := m.Get(name)
	if err != nil {
		return "", err
	}
	mappedPort, err := container.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return "", err
	}
	return mappedPort.Port(), nil
}

// GetMappedPort returns the mapped host port as an integer (used by apprunner for command mode)
func (m *Manager) GetMappedPort(name, port string) int {
	ctx := context.Background()
	container, err := m.Get(name)
	if err != nil {
		return 0
	}
	mappedPort, err := container.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return 0
	}
	return mappedPort.Int()
}

// GetConnectionString builds a connection string for a container
func (m *Manager) GetConnectionString(ctx context.Context, name, port string) (string, error) {
	host, err := m.GetHost(ctx, name)
	if err != nil {
		return "", err
	}
	mappedPort, err := m.GetPort(ctx, name, port)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", host, mappedPort), nil
}

// StopAll stops all containers
func (m *Manager) StopAll(ctx context.Context, removeVolumes bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop in reverse order
	for i := len(m.order) - 1; i >= 0; i-- {
		name := m.order[i]
		if container, ok := m.containers[name]; ok {
			log.Debug().Str("container", name).Msg("stopping container")
			if err := container.Terminate(ctx); err != nil {
				log.Warn().Err(err).Str("container", name).Msg("failed to stop container")
			}
			delete(m.containers, name)
		}
	}

	return nil
}

// Cleanup stops all containers and cleans up resources
func (m *Manager) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close all log files
	m.mu.Lock()
	for _, f := range m.logFiles {
		f.Close()
	}
	m.logFiles = make(map[string]*os.File)
	m.mu.Unlock()

	if err := m.StopAll(ctx, true); err != nil {
		log.Warn().Err(err).Msg("cleanup error")
	}

	// Remove the shared network
	if m.network != nil {
		log.Debug().Str("network", m.networkName).Msg("removing docker network")
		if err := m.network.Remove(ctx); err != nil {
			log.Warn().Err(err).Str("network", m.networkName).Msg("failed to remove network")
		}
		m.network = nil
	}
}

// PrintConnectionInfo prints connection information for all containers
func (m *Manager) PrintConnectionInfo() {
	ctx := context.Background()

	fmt.Println("\nContainer connection info:")
	fmt.Println("─────────────────────────────────────────")

	for _, name := range m.order {
		container, err := m.Get(name)
		if err != nil {
			continue
		}

		host, _ := container.Host(ctx)
		ports, _ := container.Ports(ctx)

		fmt.Printf("  %s:\n", name)
		for containerPort, hostBindings := range ports {
			if len(hostBindings) > 0 {
				fmt.Printf("    %s → %s:%s\n", containerPort.Port(), host, hostBindings[0].HostPort)
			}
		}
	}
	fmt.Println()
}

// Exec executes a command in a container
func (m *Manager) Exec(ctx context.Context, name string, cmd []string) (int, string, error) {
	container, err := m.Get(name)
	if err != nil {
		return 0, "", err
	}

	exitCode, reader, err := container.Exec(ctx, cmd)
	if err != nil {
		return 0, "", err
	}

	// Read output
	buf := make([]byte, 4096)
	n, _ := reader.Read(buf)
	output := string(buf[:n])

	return exitCode, output, nil
}
