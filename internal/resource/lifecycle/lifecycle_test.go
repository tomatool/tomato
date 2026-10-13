package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

// fakeApp records what the resource does to the app.
type fakeApp struct {
	running bool
	starts  []map[string]string
	stops   int
	failOn  string
}

func (a *fakeApp) StopApp() error {
	if a.failOn == "stop" {
		return errors.New("stop failed")
	}
	a.stops++
	a.running = false
	return nil
}

func (a *fakeApp) StartApp(ctx context.Context, env map[string]string) error {
	if a.failOn == "start" {
		return errors.New("start failed")
	}
	a.starts = append(a.starts, env)
	a.running = true
	return nil
}

func (a *fakeApp) Running() bool { return a.running }

// fakeContainers records container calls.
type fakeContainers struct {
	calls  []string
	failOn string
}

func (c *fakeContainers) do(op, name string) error {
	if c.failOn == op {
		return errors.New(op + " failed")
	}
	c.calls = append(c.calls, op+" "+name)
	return nil
}
func (c *fakeContainers) StopContainer(_ context.Context, n string) error  { return c.do("stop", n) }
func (c *fakeContainers) StartContainer(_ context.Context, n string) error { return c.do("start", n) }
func (c *fakeContainers) PauseContainer(_ context.Context, n string) error { return c.do("pause", n) }
func (c *fakeContainers) UnpauseContainer(_ context.Context, n string) error {
	return c.do("unpause", n)
}

func newTest(t *testing.T) (*Lifecycle, *fakeApp, *fakeContainers) {
	t.Helper()
	l, err := New("lifecycle", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := &fakeApp{running: true}
	cs := &fakeContainers{}
	l.SetAppControl(app)
	l.container = cs
	return l, app, cs
}

func table(rows ...[]string) *godog.Table {
	t := &godog.Table{}
	for _, r := range rows {
		row := &messages.PickleTableRow{}
		for _, v := range r {
			row.Cells = append(row.Cells, &messages.PickleTableCell{Value: v})
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func TestLifecycle_RestartWithEnvAndResetRestoresIt(t *testing.T) {
	l, app, _ := newTest(t)
	resource.SetVariable("boundary", "10s")

	if err := l.restartAppWithEnv(table([]string{"name", "value"}, []string{"SESSION_BOUNDARY", "{{boundary}}"})); err != nil {
		t.Fatal(err)
	}
	if app.stops != 1 || len(app.starts) != 1 || app.starts[0]["SESSION_BOUNDARY"] != "10s" {
		t.Fatalf("restart with env: %+v", app)
	}
	// A plain restart keeps the scenario's overrides; another adds to them.
	if err := l.restartApp(); err != nil {
		t.Fatal(err)
	}
	if app.starts[1]["SESSION_BOUNDARY"] != "10s" {
		t.Errorf("a restart dropped the overrides: %v", app.starts[1])
	}
	if err := l.restartAppWithEnv(table([]string{"name", "value"}, []string{"FLAG", "on"})); err != nil {
		t.Fatal(err)
	}
	if got := app.starts[2]; got["SESSION_BOUNDARY"] != "10s" || got["FLAG"] != "on" {
		t.Errorf("overrides accumulate: %v", got)
	}

	// The next scenario gets the configured environment back.
	if err := l.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if last := app.starts[len(app.starts)-1]; last != nil {
		t.Errorf("reset restarted with %v, want the configured env", last)
	}
	// And a reset with nothing changed leaves the app alone.
	n := len(app.starts)
	l.Reset(context.Background())
	if len(app.starts) != n {
		t.Error("reset restarted an app nothing had changed")
	}
}

func TestLifecycle_StopAndStartApp(t *testing.T) {
	l, app, _ := newTest(t)
	if err := l.startApp(); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("start while running: %v", err)
	}
	if err := l.stopApp(); err != nil {
		t.Fatal(err)
	}
	if err := l.stopApp(); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("stop while stopped: %v", err)
	}
	if err := l.startApp(); err != nil || !app.running {
		t.Fatalf("start: %v", err)
	}

	// A scenario that ends with the app down gets it back.
	l.stopApp()
	if err := l.Reset(context.Background()); err != nil || !app.running {
		t.Fatalf("reset should start the stopped app: %v", err)
	}
}

func TestLifecycle_AppErrors(t *testing.T) {
	l, err := New("lifecycle", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, step := range map[string]func() error{
		"restart": l.restartApp,
		"stop":    l.stopApp,
		"start":   l.startApp,
		"env":     func() error { return l.restartAppWithEnv(table([]string{"name", "value"}, []string{"A", "1"})) },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "no app") {
			t.Errorf("%s without an app: %v", name, err)
		}
	}
	for name, step := range map[string]func() error{
		"stop container":  func() error { return l.stopContainer("db") },
		"pause container": func() error { return l.pauseContainer("db") },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "no containers") {
			t.Errorf("%s without containers: %v", name, err)
		}
	}

	l, app, _ := newTest(t)
	if err := l.restartAppWithEnv(table([]string{"name", "value"})); err == nil {
		t.Error("a table without rows")
	}
	if err := l.restartAppWithEnv(table([]string{"name", "value"}, []string{"only-name"})); err == nil {
		t.Error("a row without a value")
	}
	app.failOn = "stop"
	if err := l.restartApp(); err == nil {
		t.Error("a failed stop fails the restart")
	}
	app.failOn = "start"
	app.running = false
	if err := l.restartApp(); err == nil {
		t.Error("a failed start fails the restart")
	}
	if err := l.Reset(context.Background()); err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Errorf("reset reports what it could not bring back: %v", err)
	}
}

func TestLifecycle_Containers(t *testing.T) {
	l, _, cs := newTest(t)

	if err := l.startContainer("db"); err == nil || !strings.Contains(err.Error(), "not stopped") {
		t.Errorf("start a running container: %v", err)
	}
	if err := l.unpauseContainer("mq"); err == nil || !strings.Contains(err.Error(), "not paused") {
		t.Errorf("unpause a running container: %v", err)
	}
	if err := l.restartContainer("db"); err != nil {
		t.Fatal(err)
	}
	if err := l.stopContainer("db"); err != nil {
		t.Fatal(err)
	}
	if err := l.stopContainer("db"); err == nil || !strings.Contains(err.Error(), "already stopped") {
		t.Errorf("stop twice: %v", err)
	}
	if err := l.pauseContainer("mq"); err != nil {
		t.Fatal(err)
	}
	if err := l.pauseContainer("mq"); err == nil || !strings.Contains(err.Error(), "already paused") {
		t.Errorf("pause twice: %v", err)
	}

	// Reset brings back what the scenario left down, paused ones first.
	if err := l.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop db", "start db", "stop db", "pause mq", "unpause mq", "start db"}
	if strings.Join(cs.calls, ",") != strings.Join(want, ",") {
		t.Errorf("calls:\n got %v\nwant %v", cs.calls, want)
	}
	if len(l.stopped) != 0 || len(l.paused) != 0 {
		t.Error("reset must forget what it brought back")
	}

	// A container that will not come back fails the reset.
	l.stopContainer("db")
	cs.failOn = "start"
	if err := l.Reset(context.Background()); err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Errorf("reset error: %v", err)
	}
	cs.failOn = "stop"
	if err := l.stopContainer("x"); err == nil {
		t.Error("a failed stop")
	}
	cs.failOn = "pause"
	if err := l.pauseContainer("x"); err == nil {
		t.Error("a failed pause")
	}
	cs.failOn = ""
	l.pauseContainer("y")
	cs.failOn = "unpause"
	if err := l.unpauseContainer("y"); err == nil {
		t.Error("a failed unpause")
	}
	l.stopped["z"] = true
	cs.failOn = "start"
	if err := l.startContainer("z"); err == nil {
		t.Error("a failed start")
	}
}

func TestLifecycle_Contract(t *testing.T) {
	l, _, _ := newTest(t)
	if l.Name() != "lifecycle" || !l.ResetsFirst() {
		t.Error("name and reset order")
	}
	ctx := context.Background()
	if l.Init(ctx) != nil || l.Ready(ctx) != nil || l.Cleanup(ctx) != nil {
		t.Error("lifecycle hooks")
	}
	for _, s := range l.Steps().Steps {
		if s.Handler == nil || s.Description == "" || !strings.Contains(s.Example, "{resource}") {
			t.Errorf("step %q", s.Pattern)
		}
	}
}
