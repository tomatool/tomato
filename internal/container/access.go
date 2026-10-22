package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/testcontainers/testcontainers-go"
)

// What the resources ask the manager for once containers are up.

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
	mappedPort, err := container.MappedPort(ctx, port)
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
	mappedPort, err := container.MappedPort(ctx, port)
	if err != nil {
		return 0
	}
	return int(mappedPort.Num())
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

// ExposedPorts returns the container ports a configured container exposes, as
// "5432/tcp", whether they are mapped to a fixed host port or a dynamic one.
func (m *Manager) ExposedPorts(name string) []string {
	m.mu.RLock()
	cfg, ok := m.configs[name]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	out := make([]string, 0, len(cfg.Ports))
	for _, spec := range cfg.Ports {
		port := spec
		if i := strings.LastIndex(spec, ":"); i >= 0 {
			port = spec[i+1:]
		}
		if !strings.Contains(port, "/") {
			port += "/tcp"
		}
		out = append(out, port)
	}
	return out
}

// CAFile is where the CA a preset made for container name is on the host, or
// "" when it has none.
func (m *Manager) CAFile(name string) string {
	if m == nil {
		return ""
	}
	return m.configs[name].CAFile
}
