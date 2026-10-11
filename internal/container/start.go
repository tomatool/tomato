package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	mobynetwork "github.com/moby/moby/api/types/network"
	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tomatool/tomato/internal/config"
)

// Starting containers: the order dependencies imply, the request built for
// each one, and how its image is named when tomato builds it.

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
	// Nothing to start: don't touch Docker at all. A shell- or HTTP-only
	// project must run on machines (and CI jobs) without a Docker daemon.
	if len(m.order) == 0 {
		return nil
	}

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

	// Resolve environment variable templates (e.g., {{.zookeeper.host}})
	resolvedEnv := m.resolveEnvTemplates(cfg.Env)

	req := testcontainers.ContainerRequest{
		Image:      cfg.Image,
		Cmd:        cfg.Command,
		Env:        resolvedEnv,
		WaitingFor: m.buildWaitStrategy(cfg.WaitFor),
		// Named rather than left to Docker's random words, so `docker ps`
		// says which project and which run a container belongs to.
		Name:   m.nameFor(name),
		Labels: m.labels("container"),
	}

	// Build the image when the entry has a build section and no image. The
	// context path was made absolute against the config file when it loaded.
	if cfg.Build != nil && cfg.Image == "" {
		dockerfile := cfg.Build.Dockerfile
		if dockerfile == "" {
			dockerfile = "Dockerfile"
		}
		repo, tag := buildImageName(name, cfg.Build.Context, dockerfile)
		req.FromDockerfile = testcontainers.FromDockerfile{
			Context:    cfg.Build.Context,
			Dockerfile: dockerfile,
			// A stable name, and the image kept: a rerun builds from cache into
			// the same image instead of leaving one more behind.
			Repo:      repo,
			Tag:       tag,
			KeepImage: true,
		}
	}

	// Files the config puts into the container before it starts, such as the
	// kafka preset's AWS_MSK_IAM plugin.
	req.Files = containerFiles(cfg.Files)

	// Follow the container's output from the moment it starts, so a container
	// that fails its wait strategy still leaves its logs behind.
	if m.runCtx != nil {
		if consumer := m.openLogConsumer(name); consumer != nil {
			req.LogConsumerCfg = &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{consumer},
			}
		}
	}

	// Parse ports - support both dynamic (9092/tcp) and fixed (9092:9092) mapping
	fixedPorts := make(mobynetwork.PortMap)
	for _, portSpec := range cfg.Ports {
		if strings.Contains(portSpec, ":") {
			// Fixed port mapping: "hostPort:containerPort" or "hostPort:containerPort/tcp"
			parts := strings.SplitN(portSpec, ":", 2)
			hostPort := parts[0]
			containerPort := parts[1]

			// Ensure container port has protocol suffix
			if !strings.Contains(containerPort, "/") {
				containerPort = containerPort + "/tcp"
			}

			req.ExposedPorts = append(req.ExposedPorts, containerPort)
			parsedPort, err := mobynetwork.ParsePort(containerPort)
			if err != nil {
				return fmt.Errorf("container %s: invalid port %q: %w", name, portSpec, err)
			}
			fixedPorts[parsedPort] = []mobynetwork.PortBinding{
				{HostIP: netip.IPv4Unspecified(), HostPort: hostPort},
			}
		} else {
			// Dynamic port mapping: "9092/tcp" or "9092"
			port := portSpec
			if !strings.Contains(port, "/") {
				port = port + "/tcp"
			}
			req.ExposedPorts = append(req.ExposedPorts, port)
		}
	}

	// Fixed port bindings and volumes ("source:target[:mode]"; bind sources were
	// made absolute when the config loaded, a name is a Docker volume)
	binds := append([]string(nil), cfg.Volumes...)
	if len(fixedPorts) > 0 || len(binds) > 0 {
		req.HostConfigModifier = func(hc *container.HostConfig) {
			if len(fixedPorts) > 0 {
				if hc.PortBindings == nil {
					hc.PortBindings = make(mobynetwork.PortMap)
				}
				for port, bindings := range fixedPorts {
					hc.PortBindings[port] = bindings
				}
			}
			hc.Binds = append(hc.Binds, binds...)
		}
	}

	// Attach to shared network with DNS alias
	if m.networkReady {
		req.Networks = []string{m.networkName}
		req.NetworkAliases = map[string][]string{
			m.networkName: {name}, // Container accessible via its config name
		}
	}

	// Start container
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            m.settings.Reuse,
	})
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	m.mu.Lock()
	m.containers[name] = ctr
	m.mu.Unlock()

	log.Debug().
		Str("container", name).
		Dur("duration", time.Since(startTime)).
		Msg("container ready")

	return nil
}

// buildImageName names the image built for a container: tomato-<name>, tagged
// with a hash of what it is built from, so the reruns of a project reuse one
// image, and containers of the same name in two projects do not collide.
func buildImageName(name, buildContext, dockerfile string) (repo, tag string) {
	repo = strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		}
		return '-'
	}, name), "-")
	if repo == "" {
		repo = "build"
	}
	sum := sha256.Sum256([]byte(buildContext + "\x00" + dockerfile))
	return "tomato-" + repo, hex.EncodeToString(sum[:6])
}

// containerFiles copies files into a container through the Docker API, which
// works with a remote Docker host, where a bind mount of a local file would not.
func containerFiles(files []config.ContainerFile) []testcontainers.ContainerFile {
	var out []testcontainers.ContainerFile
	for _, f := range files {
		out = append(out, testcontainers.ContainerFile{
			Reader:            bytes.NewReader(f.Content),
			ContainerFilePath: f.Path,
			FileMode:          f.Mode,
		})
	}
	return out
}

// buildWaitStrategy converts config wait strategy to testcontainers wait strategy
func (m *Manager) buildWaitStrategy(ws config.WaitStrategy) wait.Strategy {
	timeout := ws.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	switch ws.Type {
	case "port":
		return wait.ForListeningPort(ws.Target).WithStartupTimeout(timeout)
	case "log":
		return wait.ForLog(ws.Target).WithStartupTimeout(timeout)
	case "http":
		strategy := wait.ForHTTP(ws.Path).WithPort(ws.Target).WithStartupTimeout(timeout)
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
