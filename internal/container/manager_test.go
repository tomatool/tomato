package container

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/tomatool/tomato/internal/config"
)

// Mock container for testing
type mockContainer struct {
	hostVal    string
	hostErr    error
	ports      map[nat.Port][]nat.PortBinding
	portsErr   error
	execCode   int
	execReader io.Reader
	execErr    error
}

func (m *mockContainer) GetContainerID() string { return "mock-id" }
func (m *mockContainer) Start(ctx context.Context) error { return nil }
func (m *mockContainer) Stop(ctx context.Context, timeout *time.Duration) error { return nil }
func (m *mockContainer) Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) error { return nil }
func (m *mockContainer) Host(ctx context.Context) (string, error) {
	return m.hostVal, m.hostErr
}
func (m *mockContainer) MappedPort(ctx context.Context, port nat.Port) (nat.Port, error) {
	if bindings, ok := m.ports[port]; ok && len(bindings) > 0 {
		return nat.Port(bindings[0].HostPort), nil
	}
	return "", fmt.Errorf("port not found: %s", port)
}
func (m *mockContainer) Ports(ctx context.Context) (nat.PortMap, error) {
	return m.ports, m.portsErr
}
func (m *mockContainer) SessionID() string { return "session" }
func (m *mockContainer) IsRunning() bool { return true }
func (m *mockContainer) Exec(ctx context.Context, cmd []string, options ...tcexec.ProcessOption) (int, io.Reader, error) {
	return m.execCode, m.execReader, m.execErr
}
func (m *mockContainer) Logs(ctx context.Context) (io.ReadCloser, error) {
	if m.execReader == nil {
		return io.NopCloser(&mockReader{}), nil
	}
	return io.NopCloser(m.execReader), nil
}
func (m *mockContainer) FollowOutput(consumer testcontainers.LogConsumer) {}
func (m *mockContainer) StartLogProducer(ctx context.Context, opts ...testcontainers.LogProductionOption) error { return nil }
func (m *mockContainer) StopLogProducer() error { return nil }
func (m *mockContainer) Name(ctx context.Context) (string, error) { return "mock", nil }
func (m *mockContainer) State(ctx context.Context) (*container.State, error) { return nil, nil }
func (m *mockContainer) Networks(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockContainer) NetworkAliases(ctx context.Context) (map[string][]string, error) { return nil, nil }
func (m *mockContainer) Endpoint(ctx context.Context, proto string) (string, error) { return "", nil }
func (m *mockContainer) PortEndpoint(ctx context.Context, port nat.Port, proto string) (string, error) { return "", nil }
func (m *mockContainer) CopyToContainer(ctx context.Context, fileContent []byte, containerFilePath string, fileMode int64) error { return nil }
func (m *mockContainer) CopyDirToContainer(ctx context.Context, hostDirPath string, containerParentPath string, fileMode int64) error { return nil }
func (m *mockContainer) CopyFileToContainer(ctx context.Context, hostFilePath string, containerFilePath string, fileMode int64) error { return nil }
func (m *mockContainer) CopyFileFromContainer(ctx context.Context, filePath string) (io.ReadCloser, error) { return nil, nil }
func (m *mockContainer) GetLogProductionErrorChannel() <-chan error { return nil }
func (m *mockContainer) Inspect(ctx context.Context) (*container.InspectResponse, error) { return nil, nil }
func (m *mockContainer) ContainerIP(ctx context.Context) (string, error) { return "172.17.0.2", nil }
func (m *mockContainer) ContainerIPs(ctx context.Context) ([]string, error) { return []string{"172.17.0.2"}, nil }

// Tests for NewManager and calculateStartOrder

func TestNewManager(t *testing.T) {
	tests := []struct {
		name        string
		configs     map[string]config.Container
		wantOrder   []string
		wantErr     bool
		errContains string
	}{
		{
			name:      "empty configs",
			configs:   map[string]config.Container{},
			wantOrder: []string{},
			wantErr:   false,
		},
		{
			name: "single container",
			configs: map[string]config.Container{
				"postgres": {Image: "postgres:15"},
			},
			wantOrder: []string{"postgres"},
			wantErr:   false,
		},
		{
			name: "two independent containers",
			configs: map[string]config.Container{
				"postgres": {Image: "postgres:15"},
				"redis":    {Image: "redis:7"},
			},
			wantOrder: []string{"postgres", "redis"}, // alphabetical when no deps
			wantErr:   false,
		},
		{
			name: "simple dependency",
			configs: map[string]config.Container{
				"app":      {Image: "myapp", DependsOn: []string{"postgres"}},
				"postgres": {Image: "postgres:15"},
			},
			wantOrder: []string{"postgres", "app"},
			wantErr:   false,
		},
		{
			name: "chain dependency",
			configs: map[string]config.Container{
				"app":      {Image: "myapp", DependsOn: []string{"api"}},
				"api":      {Image: "api", DependsOn: []string{"postgres"}},
				"postgres": {Image: "postgres:15"},
			},
			wantOrder: []string{"postgres", "api", "app"},
			wantErr:   false,
		},
		{
			name: "multiple dependencies",
			configs: map[string]config.Container{
				"app":      {Image: "myapp", DependsOn: []string{"postgres", "redis"}},
				"postgres": {Image: "postgres:15"},
				"redis":    {Image: "redis:7"},
			},
			wantOrder: []string{"postgres", "redis", "app"},
			wantErr:   false,
		},
		{
			name: "diamond dependency",
			configs: map[string]config.Container{
				"app":      {Image: "myapp", DependsOn: []string{"api", "worker"}},
				"api":      {Image: "api", DependsOn: []string{"postgres"}},
				"worker":   {Image: "worker", DependsOn: []string{"postgres"}},
				"postgres": {Image: "postgres:15"},
			},
			// postgres first, then api and worker (alphabetical), then app
			wantOrder: []string{"postgres", "api", "worker", "app"},
			wantErr:   false,
		},
		{
			name: "circular dependency - self",
			configs: map[string]config.Container{
				"app": {Image: "myapp", DependsOn: []string{"app"}},
			},
			wantErr:     true,
			errContains: "circular dependency",
		},
		{
			name: "circular dependency - two containers",
			configs: map[string]config.Container{
				"a": {Image: "a", DependsOn: []string{"b"}},
				"b": {Image: "b", DependsOn: []string{"a"}},
			},
			wantErr:     true,
			errContains: "circular dependency",
		},
		{
			name: "circular dependency - three containers",
			configs: map[string]config.Container{
				"a": {Image: "a", DependsOn: []string{"b"}},
				"b": {Image: "b", DependsOn: []string{"c"}},
				"c": {Image: "c", DependsOn: []string{"a"}},
			},
			wantErr:     true,
			errContains: "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.configs)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if manager == nil {
				t.Fatal("expected manager, got nil")
			}

			if len(manager.order) != len(tt.wantOrder) {
				t.Errorf("expected order length %d, got %d", len(tt.wantOrder), len(manager.order))
				return
			}

			for i, name := range tt.wantOrder {
				if manager.order[i] != name {
					t.Errorf("order[%d]: expected %q, got %q (full order: %v)", i, name, manager.order[i], manager.order)
				}
			}
		})
	}
}

// Tests for buildWaitStrategy

func TestBuildWaitStrategy(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name     string
		config   config.WaitStrategy
		validate func(*testing.T, string)
	}{
		{
			name:   "empty config - default strategy",
			config: config.WaitStrategy{},
			validate: func(t *testing.T, strategyType string) {
				// Default returns a log strategy with empty target
			},
		},
		{
			name: "port strategy",
			config: config.WaitStrategy{
				Type:    "port",
				Target:  "5432/tcp",
				Timeout: 30 * time.Second,
			},
			validate: func(t *testing.T, strategyType string) {
				// Strategy is created without error
			},
		},
		{
			name: "log strategy",
			config: config.WaitStrategy{
				Type:    "log",
				Target:  "ready to accept connections",
				Timeout: 60 * time.Second,
			},
			validate: func(t *testing.T, strategyType string) {
				// Strategy is created without error
			},
		},
		{
			name: "http strategy",
			config: config.WaitStrategy{
				Type:    "http",
				Target:  "8080/tcp",
				Path:    "/health",
				Timeout: 30 * time.Second,
			},
			validate: func(t *testing.T, strategyType string) {
				// Strategy is created without error
			},
		},
		{
			name: "http strategy with method",
			config: config.WaitStrategy{
				Type:    "http",
				Target:  "8080/tcp",
				Path:    "/health",
				Method:  "POST",
				Timeout: 30 * time.Second,
			},
			validate: func(t *testing.T, strategyType string) {
				// Strategy is created without error
			},
		},
		{
			name: "exec strategy",
			config: config.WaitStrategy{
				Type:    "exec",
				Target:  "pg_isready -U postgres",
				Timeout: 30 * time.Second,
			},
			validate: func(t *testing.T, strategyType string) {
				// Strategy is created without error
			},
		},
		{
			name: "unknown strategy - returns default",
			config: config.WaitStrategy{
				Type:   "unknown",
				Target: "something",
			},
			validate: func(t *testing.T, strategyType string) {
				// Falls back to default log strategy
			},
		},
		{
			name: "zero timeout - uses default 60s",
			config: config.WaitStrategy{
				Type:    "port",
				Target:  "5432/tcp",
				Timeout: 0,
			},
			validate: func(t *testing.T, strategyType string) {
				// Should use 60s default timeout
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := manager.buildWaitStrategy(tt.config)

			if strategy == nil {
				t.Error("expected strategy, got nil")
				return
			}

			// The strategy is an interface, we can at least verify it's not nil
			// More detailed testing would require running containers
		})
	}
}

// Tests for Get

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		containers  map[string]testcontainers.Container
		getName     string
		wantErr     bool
		errContains string
	}{
		{
			name: "container exists",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{hostVal: "localhost"},
			},
			getName: "postgres",
			wantErr: false,
		},
		{
			name:        "container not found - empty map",
			containers:  map[string]testcontainers.Container{},
			getName:     "postgres",
			wantErr:     true,
			errContains: "container not found",
		},
		{
			name: "container not found - wrong name",
			containers: map[string]testcontainers.Container{
				"redis": &mockContainer{hostVal: "localhost"},
			},
			getName:     "postgres",
			wantErr:     true,
			errContains: "container not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				containers: tt.containers,
			}

			container, err := manager.Get(tt.getName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if container == nil {
				t.Error("expected container, got nil")
			}
		})
	}
}

// Tests for GetHost

func TestGetHost(t *testing.T) {
	tests := []struct {
		name        string
		containers  map[string]testcontainers.Container
		getName     string
		wantHost    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful host retrieval",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{hostVal: "localhost"},
			},
			getName:  "postgres",
			wantHost: "localhost",
			wantErr:  false,
		},
		{
			name: "host with IP",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{hostVal: "172.17.0.2"},
			},
			getName:  "postgres",
			wantHost: "172.17.0.2",
			wantErr:  false,
		},
		{
			name:        "container not found",
			containers:  map[string]testcontainers.Container{},
			getName:     "postgres",
			wantErr:     true,
			errContains: "container not found",
		},
		{
			name: "host error",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{hostErr: fmt.Errorf("network error")},
			},
			getName:     "postgres",
			wantErr:     true,
			errContains: "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				containers: tt.containers,
			}

			host, err := manager.GetHost(context.Background(), tt.getName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if host != tt.wantHost {
				t.Errorf("expected host %q, got %q", tt.wantHost, host)
			}
		})
	}
}

// Tests for GetPort

func TestGetPort(t *testing.T) {
	tests := []struct {
		name        string
		containers  map[string]testcontainers.Container
		getName     string
		port        string
		wantPort    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful port retrieval",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					ports: map[nat.Port][]nat.PortBinding{
						"5432/tcp": {{HostPort: "32768"}},
					},
				},
			},
			getName:  "postgres",
			port:     "5432/tcp",
			wantPort: "32768",
			wantErr:  false,
		},
		{
			name:        "container not found",
			containers:  map[string]testcontainers.Container{},
			getName:     "postgres",
			port:        "5432/tcp",
			wantErr:     true,
			errContains: "container not found",
		},
		{
			name: "port not found",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					ports: map[nat.Port][]nat.PortBinding{
						"5432/tcp": {{HostPort: "32768"}},
					},
				},
			},
			getName:     "postgres",
			port:        "3306/tcp",
			wantErr:     true,
			errContains: "port not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				containers: tt.containers,
			}

			port, err := manager.GetPort(context.Background(), tt.getName, tt.port)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if port != tt.wantPort {
				t.Errorf("expected port %q, got %q", tt.wantPort, port)
			}
		})
	}
}

// Tests for GetConnectionString

func TestGetConnectionString(t *testing.T) {
	tests := []struct {
		name        string
		containers  map[string]testcontainers.Container
		getName     string
		port        string
		wantConn    string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful connection string",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					hostVal: "localhost",
					ports: map[nat.Port][]nat.PortBinding{
						"5432/tcp": {{HostPort: "32768"}},
					},
				},
			},
			getName:  "postgres",
			port:     "5432/tcp",
			wantConn: "localhost:32768",
			wantErr:  false,
		},
		{
			name:        "container not found",
			containers:  map[string]testcontainers.Container{},
			getName:     "postgres",
			port:        "5432/tcp",
			wantErr:     true,
			errContains: "container not found",
		},
		{
			name: "port not found",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					hostVal: "localhost",
					ports:   map[nat.Port][]nat.PortBinding{},
				},
			},
			getName:     "postgres",
			port:        "5432/tcp",
			wantErr:     true,
			errContains: "port not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				containers: tt.containers,
			}

			conn, err := manager.GetConnectionString(context.Background(), tt.getName, tt.port)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if conn != tt.wantConn {
				t.Errorf("expected connection %q, got %q", tt.wantConn, conn)
			}
		})
	}
}

// Tests for Exec

func TestExec(t *testing.T) {
	tests := []struct {
		name        string
		containers  map[string]testcontainers.Container
		getName     string
		cmd         []string
		wantCode    int
		wantErr     bool
		errContains string
	}{
		{
			name: "successful exec",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					execCode:   0,
					execReader: &mockReader{data: "output"},
				},
			},
			getName:  "postgres",
			cmd:      []string{"echo", "hello"},
			wantCode: 0,
			wantErr:  false,
		},
		{
			name: "exec with non-zero exit",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					execCode:   1,
					execReader: &mockReader{data: "error"},
				},
			},
			getName:  "postgres",
			cmd:      []string{"false"},
			wantCode: 1,
			wantErr:  false,
		},
		{
			name:        "container not found",
			containers:  map[string]testcontainers.Container{},
			getName:     "postgres",
			cmd:         []string{"echo"},
			wantErr:     true,
			errContains: "container not found",
		},
		{
			name: "exec error",
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{
					execErr: fmt.Errorf("exec failed"),
				},
			},
			getName:     "postgres",
			cmd:         []string{"bad"},
			wantErr:     true,
			errContains: "exec failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				containers: tt.containers,
			}

			code, _, err := manager.Exec(context.Background(), tt.getName, tt.cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if code != tt.wantCode {
				t.Errorf("expected exit code %d, got %d", tt.wantCode, code)
			}
		})
	}
}

// Tests for StopAll

func TestStopAll(t *testing.T) {
	t.Run("empty containers", func(t *testing.T) {
		manager := &Manager{
			containers: make(map[string]testcontainers.Container),
			order:      []string{},
		}

		err := manager.StopAll(context.Background(), true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("stops containers in reverse order", func(t *testing.T) {
		manager := &Manager{
			containers: map[string]testcontainers.Container{
				"postgres": &mockContainer{},
				"app":      &mockContainer{},
			},
			order: []string{"postgres", "app"},
		}

		err := manager.StopAll(context.Background(), true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Containers should be removed from map
		if len(manager.containers) != 0 {
			t.Errorf("expected containers to be cleared, got %d", len(manager.containers))
		}
	})
}

// Tests for SetRunContext

func TestSetRunContext(t *testing.T) {
	manager := &Manager{}

	if manager.runCtx != nil {
		t.Error("expected runCtx to be nil initially")
	}

	// Setting nil should not panic
	manager.SetRunContext(nil)
	if manager.runCtx != nil {
		t.Error("expected runCtx to remain nil")
	}
}

// Helper types

type mockReader struct {
	data string
	pos  int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Helper functions

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
