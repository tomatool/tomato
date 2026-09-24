package apprunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/runlog"
)

// fakeContainer stands in for a started testcontainer. Only the methods the
// runner calls are implemented; anything else panics via the nil interface.
type fakeContainer struct {
	testcontainers.Container
	host       string
	mapped     map[string]int
	logs       string
	logsErr    error
	termErr    error
	terminated bool
}

func (f *fakeContainer) Host(context.Context) (string, error) { return f.host, nil }

func (f *fakeContainer) MappedPort(_ context.Context, p nat.Port) (nat.Port, error) {
	if port, ok := f.mapped[string(p)]; ok {
		return nat.Port(fmt.Sprintf("%d/tcp", port)), nil
	}
	return "", fmt.Errorf("port %s not mapped", p)
}

func (f *fakeContainer) Logs(context.Context) (io.ReadCloser, error) {
	if f.logsErr != nil {
		return nil, f.logsErr
	}
	return io.NopCloser(strings.NewReader(f.logs)), nil
}

func (f *fakeContainer) Terminate(context.Context, ...testcontainers.TerminateOption) error {
	f.terminated = true
	return f.termErr
}

// managerWith returns a container manager holding fake containers by name.
func managerWith(t *testing.T, containers map[string]*fakeContainer) *container.Manager {
	t.Helper()
	cm, err := container.NewManager(map[string]config.Container{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for name, c := range containers {
		cm.RegisterContainer(name, c)
	}
	return cm
}

// freePort returns a port nothing listens on.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// listen starts a TCP listener that accepts and closes connections.
func listen(t *testing.T) (int, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return l.Addr().(*net.TCPAddr).Port, func() { l.Close() }
}

func serverPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

func quietRunner(cfg config.AppConfig, cm *container.Manager) *Runner {
	r := NewRunner(cfg, cm)
	r.SetShowLogs(false)
	return r
}

func TestNewRunnerModes(t *testing.T) {
	if got := NewRunner(config.AppConfig{Command: "./app"}, nil).GetMode(); got != ModeCommand {
		t.Errorf("command config: mode = %s, want %s", got, ModeCommand)
	}
	if got := NewRunner(config.AppConfig{Image: "app:latest"}, nil).GetMode(); got != ModeContainer {
		t.Errorf("image config: mode = %s, want %s", got, ModeContainer)
	}
	build := &config.AppBuild{Dockerfile: "Dockerfile"}
	if got := NewRunner(config.AppConfig{Build: build}, nil).GetMode(); got != ModeContainer {
		t.Errorf("build config: mode = %s, want %s", got, ModeContainer)
	}
	r := NewRunner(config.AppConfig{}, nil)
	if r.resources == nil || !r.showLogs {
		t.Error("NewRunner should initialise resources and show logs by default")
	}
}

func TestSetContainerMode(t *testing.T) {
	r := NewRunner(config.AppConfig{Command: "./app"}, nil)
	if err := r.SetContainerMode(true); err == nil {
		t.Error("container mode without image/build should fail")
	}
	if err := r.SetContainerMode(false); err != nil || r.GetMode() != ModeCommand {
		t.Errorf("disabling container mode should be a no-op, got %v / %s", err, r.GetMode())
	}

	r = NewRunner(config.AppConfig{Command: "./app", Image: "app:latest"}, nil)
	r.mode = ModeCommand
	if err := r.SetContainerMode(true); err != nil || r.GetMode() != ModeContainer {
		t.Errorf("container mode with an image: %v / %s", err, r.GetMode())
	}
}

func TestSetShowLogsAndResources(t *testing.T) {
	r := NewRunner(config.AppConfig{}, nil)
	r.SetShowLogs(false)
	if r.showLogs {
		t.Error("SetShowLogs(false) should disable log display")
	}
	res := map[string]config.Resource{"mock": {Type: "http-server"}}
	r.SetResources(res)
	if _, ok := r.resources["mock"]; !ok {
		t.Error("SetResources should store the resources")
	}
}

func TestSetRunContext(t *testing.T) {
	r := quietRunner(config.AppConfig{}, nil)
	r.SetRunContext(nil)
	if r.logFile != nil {
		t.Error("nil run context should not create a log file")
	}

	dir := t.TempDir()
	r.SetRunContext(&runlog.RunContext{Dir: dir})
	if r.logFile == nil {
		t.Fatal("run context should create an app log file")
	}
	r.streamCommandLogs(strings.NewReader("hello\n"), "stdout")
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if r.logFile != nil {
		t.Error("Stop should close the log file")
	}
	data, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[stdout] hello") {
		t.Errorf("log file = %q, want the streamed line", data)
	}

	// A run directory that doesn't exist only warns.
	r = quietRunner(config.AppConfig{}, nil)
	r.SetRunContext(&runlog.RunContext{Dir: filepath.Join(dir, "missing", "deeper")})
	if r.logFile != nil {
		t.Error("an unwritable run dir should leave logFile nil")
	}
}

func TestStartUnknownMode(t *testing.T) {
	r := quietRunner(config.AppConfig{}, nil)
	r.mode = Mode("bogus")
	if err := r.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "unknown mode") {
		t.Errorf("unknown mode should fail, got %v", err)
	}
}

func TestStartCommandRequiresCommand(t *testing.T) {
	r := quietRunner(config.AppConfig{}, nil)
	if err := r.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Errorf("missing command should fail, got %v", err)
	}
	r = quietRunner(config.AppConfig{Command: "   "}, nil)
	if err := r.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "empty command") {
		t.Errorf("blank command should fail, got %v", err)
	}
}

func TestStartCommandMissingBinary(t *testing.T) {
	r := quietRunner(config.AppConfig{Command: "/definitely/not/a/binary"}, nil)
	if err := r.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "starting app") {
		t.Errorf("missing binary should fail to start, got %v", err)
	}
}

func TestStartCommandStreamsOutputAndEnv(t *testing.T) {
	if _, err := exec.LookPath("env"); err != nil {
		t.Skip("env not available")
	}
	workdir := t.TempDir()
	r := quietRunner(config.AppConfig{
		Command: "env",
		WorkDir: workdir,
		Wait:    time.Millisecond,
		Env:     map[string]string{"TOMATO_TEST_VALUE": "from-tomato"},
	}, nil)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Stop()

	if r.cmd.Dir != workdir {
		t.Errorf("workdir = %q, want %q", r.cmd.Dir, workdir)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range r.GetRecentLogs(100) {
			if line == "TOMATO_TEST_VALUE=from-tomato" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("app output never showed the configured env; logs: %v", r.GetRecentLogs(100))
}

func TestStartCommandReadyTimeoutStopsApp(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	r := quietRunner(config.AppConfig{
		Command: "sleep 30",
		Port:    freePort(t),
		Ready:   &config.ReadyCheck{Type: "tcp", Timeout: 50 * time.Millisecond},
	}, nil)
	err := r.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "app not ready") {
		t.Fatalf("expected a readiness timeout, got %v", err)
	}
	if r.cmd != nil {
		t.Error("the app process should be stopped when it never becomes ready")
	}
}

func TestStartCommandSucceedsAndStops(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	r := quietRunner(config.AppConfig{Command: "sleep 30"}, nil)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if r.GetHost() != "localhost" || r.GetPort() != 0 {
		t.Errorf("host/port = %s/%d", r.GetHost(), r.GetPort())
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if r.cmd != nil {
		t.Error("Stop should clear the process")
	}
	// Stopping twice is safe.
	if err := r.Stop(); err != nil {
		t.Errorf("second Stop: %v", err)
	}
}

func TestStopCommandAlreadyExited(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	r := quietRunner(config.AppConfig{}, nil)
	r.cmd = exec.Command("true")
	if err := r.cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := r.stopCommand(); err == nil {
		t.Error("stopping a process that already exited should report the kill failure")
	}
}

func TestBuildEnvForCommand(t *testing.T) {
	cm := managerWith(t, map[string]*fakeContainer{
		"postgres": {host: "localhost", mapped: map[string]int{"5432/tcp": 55432}},
	})
	r := quietRunner(config.AppConfig{Env: map[string]string{
		"DB_HOST":     "{{.postgres.host}}",
		"DB_PORT":     "{{.postgres.port.5432}}",
		"DB_PORT_TCP": "{{ .postgres.port.5432/tcp }}",
		"DB_URL":      "postgres://u@{{.postgres.host}}:{{.postgres.port.5432}}/db",
		"NO_PORT":     "{{.postgres.port}}",
		"UNMAPPED":    "{{.postgres.port.6379}}",
		"UNKNOWN":     "{{.redis.port.6379}}",
		"MOCK":        "{{.mock.url}}/v1",
		"MOCK_NOPORT": "{{.mock-noport.url}}",
		"OTHER":       "{{.api.url}}",
		"MISSING":     "{{.nope.url}}",
		"PLAIN":       "value",
	}}, cm)
	r.SetResources(map[string]config.Resource{
		"mock":        {Type: "http-server", Options: map[string]any{"port": 9999}},
		"mock-noport": {Type: "http-server", Options: map[string]any{}},
		"api":         {Type: "http"},
	})

	env := r.buildEnvForCommand()
	want := map[string]string{
		"DB_HOST":     "localhost",
		"DB_PORT":     "55432",
		"DB_PORT_TCP": "55432",
		"DB_URL":      "postgres://u@localhost:55432/db",
		"NO_PORT":     "{{.postgres.port}}",
		"UNMAPPED":    "{{.postgres.port.6379}}",
		"UNKNOWN":     "{{.redis.port.6379}}",
		"MOCK":        "http://localhost:9999/v1",
		"MOCK_NOPORT": "{{.mock-noport.url}}",
		"OTHER":       "{{.api.url}}",
		"MISSING":     "{{.nope.url}}",
		"PLAIN":       "value",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

func TestBuildEnvForDocker(t *testing.T) {
	r := quietRunner(config.AppConfig{Env: map[string]string{
		"DB_HOST":     "{{.postgres.host}}",
		"DB_PORT":     "{{.postgres.port.5432}}",
		"DB_PORT_TCP": "{{.postgres.port.5432/tcp}}",
		"NO_PORT":     "{{.postgres.port}}",
		"MOCK":        "{{.mock.url}}",
		"MOCK_NOPORT": "{{.mock-noport.url}}",
		"OTHER":       "{{.api.url}}",
		"MISSING":     "{{.nope.url}}",
	}}, nil)
	r.SetResources(map[string]config.Resource{
		"mock":        {Type: "http-server", Options: map[string]any{"port": 9999}},
		"mock-noport": {Type: "http-server"},
		"api":         {Type: "http"},
	})

	env := r.buildEnvForDocker()
	want := map[string]string{
		"DB_HOST":     "postgres",
		"DB_PORT":     "5432",
		"DB_PORT_TCP": "5432",
		"NO_PORT":     "{{.postgres.port}}",
		"MOCK":        "http://host.docker.internal:9999",
		"MOCK_NOPORT": "{{.mock-noport.url}}",
		"OTHER":       "{{.api.url}}",
		"MISSING":     "{{.nope.url}}",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

func TestWaitForReadyNoCheck(t *testing.T) {
	r := quietRunner(config.AppConfig{}, nil)
	if err := r.waitForReady(context.Background()); err != nil {
		t.Errorf("no ready check or port should pass immediately, got %v", err)
	}
}

func TestWaitForReadyHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/ready":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	port := serverPort(t, srv)

	cases := []struct {
		name    string
		ready   *config.ReadyCheck
		wantErr bool
	}{
		{"default path and status", &config.ReadyCheck{Type: "http"}, false},
		{"custom path and status", &config.ReadyCheck{Type: "http", Path: "/ready", Status: 204}, false},
		{"never ready", &config.ReadyCheck{Type: "http", Path: "/down", Timeout: 50 * time.Millisecond}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := quietRunner(config.AppConfig{Port: port, Ready: c.ready}, nil)
			r.cmdHost, r.cmdPort = "127.0.0.1", port
			err := r.waitForReady(context.Background())
			if (err != nil) != c.wantErr {
				t.Errorf("waitForReady() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestWaitForReadyTCP(t *testing.T) {
	port, stop := listen(t)
	defer stop()

	r := quietRunner(config.AppConfig{Port: port}, nil)
	r.cmdHost, r.cmdPort = "127.0.0.1", port
	if err := r.waitForReady(context.Background()); err != nil {
		t.Errorf("listening port should be ready, got %v", err)
	}

	closed := freePort(t)
	r = quietRunner(config.AppConfig{Port: closed, Ready: &config.ReadyCheck{Type: "tcp", Timeout: 50 * time.Millisecond}}, nil)
	r.cmdHost, r.cmdPort = "127.0.0.1", closed
	if err := r.waitForReady(context.Background()); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("closed port should time out, got %v", err)
	}
}

func TestWaitForReadyContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := quietRunner(config.AppConfig{Port: freePort(t)}, nil)
	r.cmdHost = "127.0.0.1"
	if err := r.waitForReady(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context should stop waiting, got %v", err)
	}
}

func TestBuildWaitStrategy(t *testing.T) {
	cases := []config.AppConfig{
		{},
		{Port: 8080},
		{Port: 8080, Ready: &config.ReadyCheck{Type: "http"}},
		{Port: 8080, Ready: &config.ReadyCheck{Type: "http", Path: "/ready", Status: 204, Timeout: time.Second}},
		{Port: 8080, Ready: &config.ReadyCheck{Type: "tcp"}},
		{Ready: &config.ReadyCheck{Type: "exec", Command: "true"}},
		{Port: 8080, Ready: &config.ReadyCheck{Type: "other"}},
	}
	for i, cfg := range cases {
		if s := quietRunner(cfg, nil).buildWaitStrategy(); s == nil {
			t.Errorf("case %d: nil wait strategy", i)
		}
	}
}

func TestStartContainerNeedsImageOrBuild(t *testing.T) {
	cm := managerWith(t, nil)
	r := quietRunner(config.AppConfig{Port: 8080}, cm)
	if err := r.startContainer(context.Background()); err == nil || !strings.Contains(err.Error(), "requires 'image' or 'build'") {
		t.Errorf("container mode without image/build should fail, got %v", err)
	}
}

func TestCaptureContainerLogs(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeContainer{logs: "line one\n\nline two\n"}
	r := quietRunner(config.AppConfig{Image: "app"}, nil)
	r.SetRunContext(&runlog.RunContext{Dir: dir})
	r.appContainer = fc
	r.captureContainerLogs(context.Background())

	got := r.GetRecentLogs(10)
	if len(got) != 2 || got[0] != "line one" || got[1] != "line two" {
		t.Errorf("captured logs = %v", got)
	}
	r.Stop()
	data, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	if !strings.Contains(string(data), "line two") {
		t.Errorf("container logs should be written to the log file, got %q", data)
	}

	// No container, a logs error, a stopped runner and a canceled context all return quietly.
	quietRunner(config.AppConfig{}, nil).captureContainerLogs(context.Background())

	r = quietRunner(config.AppConfig{Image: "app"}, nil)
	r.appContainer = &fakeContainer{logsErr: errors.New("boom")}
	r.captureContainerLogs(context.Background())

	r = quietRunner(config.AppConfig{Image: "app"}, nil)
	r.appContainer = &fakeContainer{logs: "ignored\n"}
	r.Stop()
	r.appContainer = &fakeContainer{logs: "ignored\n"}
	r.captureContainerLogs(context.Background())
	if len(r.GetRecentLogs(10)) != 0 {
		t.Error("a stopped runner should not capture logs")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = quietRunner(config.AppConfig{Image: "app"}, nil)
	r.appContainer = &fakeContainer{logs: "ignored\n"}
	r.captureContainerLogs(ctx)
}

func TestStopContainer(t *testing.T) {
	fc := &fakeContainer{}
	r := quietRunner(config.AppConfig{Image: "app"}, nil)
	r.appContainer = fc
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !fc.terminated || r.appContainer != nil {
		t.Error("Stop should terminate and clear the container")
	}

	r = quietRunner(config.AppConfig{Image: "app"}, nil)
	r.appContainer = &fakeContainer{termErr: errors.New("stuck")}
	if err := r.Stop(); err == nil {
		t.Error("a failed terminate should be returned")
	}

	r = quietRunner(config.AppConfig{Image: "app"}, nil)
	if err := r.Stop(); err != nil {
		t.Errorf("stopping without a container: %v", err)
	}

	r = quietRunner(config.AppConfig{}, nil)
	r.mode = Mode("bogus")
	if err := r.Stop(); err != nil {
		t.Errorf("unknown mode Stop should be a no-op, got %v", err)
	}
}

func TestHostPortAccessors(t *testing.T) {
	r := quietRunner(config.AppConfig{Command: "./app", Port: 8080}, nil)
	r.cmdHost, r.cmdPort = "localhost", 8080
	if got := r.GetBaseURL(); got != "http://localhost:8080" {
		t.Errorf("command base URL = %q", got)
	}
	if h, p := r.GetHostPort(); h != "localhost" || p != 8080 {
		t.Errorf("command host/port = %s/%d", h, p)
	}
	if r.GetContainer() != nil {
		t.Error("command mode has no container")
	}
	if r.GetInternalPort() != 8080 {
		t.Errorf("internal port = %d", r.GetInternalPort())
	}

	fc := &fakeContainer{}
	r = quietRunner(config.AppConfig{Image: "app", Port: 8080}, nil)
	r.appContainer, r.appHost, r.appPort = fc, "127.0.0.1", 32768
	if r.GetHost() != "127.0.0.1" || r.GetPort() != 32768 {
		t.Errorf("container host/port = %s/%d", r.GetHost(), r.GetPort())
	}
	if got := r.GetBaseURL(); got != "http://127.0.0.1:32768" {
		t.Errorf("container base URL = %q", got)
	}
	if h, p := r.GetHostPort(); h != "127.0.0.1" || p != 32768 {
		t.Errorf("container host/port = %s/%d", h, p)
	}
	if r.GetContainer() != fc {
		t.Error("GetContainer should return the app container")
	}
}

func TestGetRecentLogs(t *testing.T) {
	r := quietRunner(config.AppConfig{}, nil)
	if got := r.GetRecentLogs(5); got != nil {
		t.Errorf("no logs yet: %v", got)
	}
	r.logLines = []string{"a", "b", "c"}
	if got := r.GetRecentLogs(0); got != nil {
		t.Errorf("n=0: %v", got)
	}
	if got := r.GetRecentLogs(2); strings.Join(got, ",") != "b,c" {
		t.Errorf("last 2 = %v", got)
	}
	if got := r.GetRecentLogs(10); strings.Join(got, ",") != "a,b,c" {
		t.Errorf("more than available = %v", got)
	}
}

func TestStreamCommandLogs(t *testing.T) {
	r := NewRunner(config.AppConfig{}, nil)
	r.SetShowLogs(true) // exercise the display branch too
	var b strings.Builder
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&b, "line %d\n\n", i)
	}
	r.streamCommandLogs(strings.NewReader(b.String()), "stdout")
	got := r.GetRecentLogs(1000)
	if len(got) != 100 {
		t.Fatalf("kept %d lines, want the last 100", len(got))
	}
	if got[0] != "line 50" || got[99] != "line 149" {
		t.Errorf("kept lines %q..%q", got[0], got[99])
	}

	r = quietRunner(config.AppConfig{}, nil)
	r.Stop()
	r.streamCommandLogs(strings.NewReader("ignored\n"), "stdout")
	if len(r.GetRecentLogs(10)) != 0 {
		t.Error("a stopped runner should not record logs")
	}
}

func TestVerifyHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	port := serverPort(t, srv)
	closed := freePort(t)

	cases := []struct {
		name    string
		port    int
		ready   *config.ReadyCheck
		wantErr bool
	}{
		{"no port", 0, nil, false},
		{"http ok", port, &config.ReadyCheck{Type: "http"}, false},
		{"http wrong status", port, &config.ReadyCheck{Type: "http", Path: "/broken"}, true},
		{"http unreachable", closed, &config.ReadyCheck{Type: "http", Status: 200}, true},
		{"tcp ok", port, nil, false},
		{"tcp unreachable", closed, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := quietRunner(config.AppConfig{Port: c.port, Ready: c.ready}, nil)
			r.cmdHost, r.cmdPort = "127.0.0.1", c.port
			err := r.VerifyHealthy(context.Background())
			if (err != nil) != c.wantErr {
				t.Errorf("VerifyHealthy() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}
