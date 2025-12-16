package apprunner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
)

// Tests for NewRunner

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name    string
		config  config.AppConfig
		wantPort int
	}{
		{
			name:     "empty config",
			config:   config.AppConfig{},
			wantPort: 0,
		},
		{
			name: "with port",
			config: config.AppConfig{
				Port: 8080,
			},
			wantPort: 8080,
		},
		{
			name: "with command",
			config: config.AppConfig{
				Command: "./app serve",
				Port:    3000,
			},
			wantPort: 3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.config, nil)

			if runner == nil {
				t.Fatal("expected runner, got nil")
			}
			if runner.appPort != tt.wantPort {
				t.Errorf("expected port %d, got %d", tt.wantPort, runner.appPort)
			}
			if runner.appHost != "localhost" {
				t.Errorf("expected host localhost, got %s", runner.appHost)
			}
			if !runner.showLogs {
				t.Error("expected showLogs to be true by default")
			}
		})
	}
}

// Tests for SetShowLogs

func TestSetShowLogs(t *testing.T) {
	runner := NewRunner(config.AppConfig{}, nil)

	if !runner.showLogs {
		t.Error("expected showLogs to be true initially")
	}

	runner.SetShowLogs(false)
	if runner.showLogs {
		t.Error("expected showLogs to be false after SetShowLogs(false)")
	}

	runner.SetShowLogs(true)
	if !runner.showLogs {
		t.Error("expected showLogs to be true after SetShowLogs(true)")
	}
}

// Tests for GetBaseURL

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		expected string
	}{
		{
			name:     "port 8080",
			port:     8080,
			expected: "http://localhost:8080",
		},
		{
			name:     "port 3000",
			port:     3000,
			expected: "http://localhost:3000",
		},
		{
			name:     "port 0",
			port:     0,
			expected: "http://localhost:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(config.AppConfig{Port: tt.port}, nil)
			url := runner.GetBaseURL()

			if url != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, url)
			}
		})
	}
}

// Tests for GetRecentLogs

func TestGetRecentLogs(t *testing.T) {
	tests := []struct {
		name     string
		logLines []string
		n        int
		expected []string
	}{
		{
			name:     "empty logs",
			logLines: []string{},
			n:        5,
			expected: nil,
		},
		{
			name:     "n is 0",
			logLines: []string{"line1", "line2"},
			n:        0,
			expected: nil,
		},
		{
			name:     "n is negative",
			logLines: []string{"line1", "line2"},
			n:        -1,
			expected: nil,
		},
		{
			name:     "n less than available",
			logLines: []string{"line1", "line2", "line3", "line4", "line5"},
			n:        3,
			expected: []string{"line3", "line4", "line5"},
		},
		{
			name:     "n equals available",
			logLines: []string{"line1", "line2", "line3"},
			n:        3,
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "n greater than available",
			logLines: []string{"line1", "line2"},
			n:        10,
			expected: []string{"line1", "line2"},
		},
		{
			name:     "n is 1",
			logLines: []string{"line1", "line2", "line3"},
			n:        1,
			expected: []string{"line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(config.AppConfig{}, nil)
			runner.logLines = tt.logLines

			result := runner.GetRecentLogs(tt.n)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

// Tests for waitReady

func TestWaitReadyNoConfig(t *testing.T) {
	// Test with no ready config and no port - should just wait 2 seconds
	runner := NewRunner(config.AppConfig{}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := runner.waitReady(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have waited approximately 2 seconds
	if elapsed < 1*time.Second || elapsed > 4*time.Second {
		t.Errorf("expected ~2s wait, got %v", elapsed)
	}
}

func TestWaitReadyUnknownType(t *testing.T) {
	runner := NewRunner(config.AppConfig{
		Ready: &config.ReadyCheck{
			Type: "unknown",
		},
	}, nil)

	ctx := context.Background()
	err := runner.waitReady(ctx)

	if err == nil {
		t.Error("expected error for unknown ready type")
	}
	if !contains(err.Error(), "unknown ready check type") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Tests for waitHTTP

func TestWaitHTTP(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Parse server URL to get host and port
	host, port := parseHostPort(t, server.URL)

	tests := []struct {
		name           string
		config         config.AppConfig
		timeout        time.Duration
		wantErr        bool
	}{
		{
			name: "successful health check",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Path:   "/health",
					Status: 200,
				},
			},
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name: "default path /health",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Status: 200,
				},
			},
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name: "default status 200",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type: "http",
					Path: "/health",
				},
			},
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name: "wrong status code",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Path:   "/health",
					Status: 201,
				},
			},
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name: "wrong path",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Path:   "/nonexistent",
					Status: 200,
				},
			},
			timeout: 1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.config, nil)
			runner.appHost = host

			ctx := context.Background()
			err := runner.waitHTTP(ctx, tt.timeout)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitHTTPContextCanceled(t *testing.T) {
	runner := NewRunner(config.AppConfig{
		Port: 59999, // Non-existent port
		Ready: &config.ReadyCheck{
			Type: "http",
			Path: "/health",
		},
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runner.waitHTTP(ctx, 30*time.Second)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Tests for waitTCP

func TestWaitTCP(t *testing.T) {
	// Create a test TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)

	tests := []struct {
		name    string
		host    string
		port    int
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "successful connection",
			host:    "127.0.0.1",
			port:    addr.Port,
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "connection refused",
			host:    "127.0.0.1",
			port:    59998, // Non-existent port
			timeout: 1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(config.AppConfig{}, nil)

			ctx := context.Background()
			err := runner.waitTCP(ctx, tt.host, tt.port, tt.timeout)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitTCPContextCanceled(t *testing.T) {
	runner := NewRunner(config.AppConfig{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runner.waitTCP(ctx, "127.0.0.1", 59997, 30*time.Second)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Tests for waitExec

func TestWaitExec(t *testing.T) {
	tests := []struct {
		name    string
		command string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "successful command",
			command: "true",
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "failing command",
			command: "false",
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name:    "echo command",
			command: "echo hello",
			timeout: 5 * time.Second,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(config.AppConfig{
				Ready: &config.ReadyCheck{
					Type:    "exec",
					Command: tt.command,
				},
			}, nil)

			ctx := context.Background()
			err := runner.waitExec(ctx, tt.timeout)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitExecContextCanceled(t *testing.T) {
	runner := NewRunner(config.AppConfig{
		Ready: &config.ReadyCheck{
			Type:    "exec",
			Command: "sleep 10",
		},
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runner.waitExec(ctx, 30*time.Second)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Tests for VerifyHealthy

func TestVerifyHealthy(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)

	tests := []struct {
		name    string
		config  config.AppConfig
		host    string
		wantErr bool
	}{
		{
			name:    "no port configured",
			config:  config.AppConfig{},
			wantErr: false,
		},
		{
			name: "http health check success",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Path:   "/health",
					Status: 200,
				},
			},
			host:    host,
			wantErr: false,
		},
		{
			name: "http health check wrong status",
			config: config.AppConfig{
				Port: port,
				Ready: &config.ReadyCheck{
					Type:   "http",
					Path:   "/health",
					Status: 201,
				},
			},
			host:    host,
			wantErr: true,
		},
		{
			name: "tcp health check success",
			config: config.AppConfig{
				Port: port,
			},
			host:    host,
			wantErr: false,
		},
		{
			name: "tcp health check failure",
			config: config.AppConfig{
				Port: 59996,
			},
			host:    "127.0.0.1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.config, nil)
			if tt.host != "" {
				runner.appHost = tt.host
			}

			ctx := context.Background()
			err := runner.VerifyHealthy(ctx)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Tests for Stop

func TestStop(t *testing.T) {
	t.Run("stop without process", func(t *testing.T) {
		runner := NewRunner(config.AppConfig{}, nil)

		// Should not panic or error
		err := runner.Stop()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("stop can be called multiple times", func(t *testing.T) {
		runner := NewRunner(config.AppConfig{}, nil)

		// Multiple stops should not panic
		runner.Stop()
		runner.Stop()
		runner.Stop()
	})
}

// Tests for buildEnv

func TestBuildEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected []string
	}{
		{
			name:     "empty env",
			env:      map[string]string{},
			expected: []string{},
		},
		{
			name: "simple env vars",
			env: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{"KEY1=value1", "KEY2=value2"},
		},
		{
			name: "env with no templates",
			env: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
			expected: []string{"DB_HOST=localhost", "DB_PORT=5432"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(config.AppConfig{
				Env: tt.env,
			}, nil)

			ctx := context.Background()
			result, err := runner.buildEnv(ctx)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check that all expected vars are present (order may vary)
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in result, got %v", expected, result)
				}
			}
		})
	}
}

// Tests for streamLogs

func TestStreamLogs(t *testing.T) {
	runner := NewRunner(config.AppConfig{}, nil)
	runner.showLogs = false // Don't print to stdout during tests

	// Create a pipe
	pr, pw := newPipe(t)

	// Start streaming in background
	done := make(chan struct{})
	go func() {
		runner.streamLogs(pr, "test")
		close(done)
	}()

	// Write some lines
	lines := []string{"line1", "line2", "line3"}
	for _, line := range lines {
		pw.Write([]byte(line + "\n"))
	}

	// Close the writer and wait for streaming to finish
	pw.Close()
	<-done

	// Check that lines were captured
	result := runner.GetRecentLogs(10)
	if len(result) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(result))
	}
}

func TestStreamLogsMaxLines(t *testing.T) {
	runner := NewRunner(config.AppConfig{}, nil)
	runner.showLogs = false

	pr, pw := newPipe(t)

	done := make(chan struct{})
	go func() {
		runner.streamLogs(pr, "test")
		close(done)
	}()

	// Write more than 100 lines
	for i := 0; i < 150; i++ {
		pw.Write([]byte(fmt.Sprintf("line%d\n", i)))
	}

	pw.Close()
	<-done

	// Should only keep last 100 lines
	result := runner.GetRecentLogs(200)
	if len(result) != 100 {
		t.Errorf("expected 100 lines max, got %d", len(result))
	}

	// First line should be line50 (0-49 were trimmed)
	if result[0] != "line50" {
		t.Errorf("expected first line to be 'line50', got %q", result[0])
	}
}

// Helper functions

func parseHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	// rawURL is like "http://127.0.0.1:12345"
	var host string
	var port int
	_, err := fmt.Sscanf(rawURL, "http://%s", &host)
	if err != nil {
		// Try parsing differently
		host = "127.0.0.1"
	}

	// Extract host and port
	addr := rawURL[len("http://"):]
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse URL %s: %v", rawURL, err)
	}
	host = h
	fmt.Sscanf(p, "%d", &port)
	return host, port
}

func newPipe(t *testing.T) (*readCloser, *writeCloser) {
	t.Helper()
	pr, pw, err := newTestPipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	return pr, pw
}

// Simple pipe implementation for testing
type readCloser struct {
	pr *pipeReader
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	return r.pr.Read(p)
}

func (r *readCloser) Close() error {
	return r.pr.Close()
}

type writeCloser struct {
	pw *pipeWriter
}

func (w *writeCloser) Write(p []byte) (n int, err error) {
	return w.pw.Write(p)
}

func (w *writeCloser) Close() error {
	return w.pw.Close()
}

type pipeReader struct {
	ch     chan []byte
	buf    []byte
	closed bool
}

func (r *pipeReader) Read(p []byte) (n int, err error) {
	if len(r.buf) > 0 {
		n = copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	data, ok := <-r.ch
	if !ok {
		return 0, fmt.Errorf("EOF")
	}

	n = copy(p, data)
	if n < len(data) {
		r.buf = data[n:]
	}
	return n, nil
}

func (r *pipeReader) Close() error {
	return nil
}

type pipeWriter struct {
	ch chan []byte
}

func (w *pipeWriter) Write(p []byte) (n int, err error) {
	data := make([]byte, len(p))
	copy(data, p)
	w.ch <- data
	return len(p), nil
}

func (w *pipeWriter) Close() error {
	close(w.ch)
	return nil
}

func newTestPipe() (*readCloser, *writeCloser, error) {
	ch := make(chan []byte, 100)
	return &readCloser{pr: &pipeReader{ch: ch}}, &writeCloser{pw: &pipeWriter{ch: ch}}, nil
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
