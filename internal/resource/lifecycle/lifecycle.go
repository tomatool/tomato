// Package lifecycle provides tomato's `lifecycle` resource: stopping and
// starting the application under test and the containers it depends on, in
// the middle of a scenario.
//
// Production fails this way: the service restarts while clients are
// connected, a database goes down and comes back, a broker hangs. A
// scenario stops or pauses something, drives the app, and checks it
// recovers. Every scenario starts with everything back up: Reset restarts
// what a scenario stopped, before any other resource resets.
package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
)

// containerControl is what Lifecycle needs from the container manager.
type containerControl interface {
	StopContainer(ctx context.Context, name string) error
	StartContainer(ctx context.Context, name string) error
	PauseContainer(ctx context.Context, name string) error
	UnpauseContainer(ctx context.Context, name string) error
}

// Lifecycle stops and starts the app and containers.
type Lifecycle struct {
	name      string
	container containerControl
	app       resource.AppControl

	// What the scenario changed, for Reset to put back.
	appStopped bool
	overrides  map[string]string
	stopped    map[string]bool
	paused     map[string]bool
}

func New(name string, cfg config.Resource, cm *container.Manager) (*Lifecycle, error) {
	l := &Lifecycle{
		name:    name,
		stopped: make(map[string]bool),
		paused:  make(map[string]bool),
	}
	if cm != nil {
		l.container = cm
	}
	return l, nil
}

func (l *Lifecycle) Name() string                          { return l.name }
func (l *Lifecycle) Init(ctx context.Context) error        { return nil }
func (l *Lifecycle) Ready(ctx context.Context) error       { return nil }
func (l *Lifecycle) Cleanup(ctx context.Context) error     { return nil }
func (l *Lifecycle) SetAppControl(app resource.AppControl) { l.app = app }

// ResetsFirst: the containers this brings back are what other resources reset.
func (l *Lifecycle) ResetsFirst() bool { return true }

// Reset brings back everything the scenario stopped or paused, and the app
// with its configured environment.
func (l *Lifecycle) Reset(ctx context.Context) error {
	var errs []string
	for _, name := range sortedKeys(l.paused) {
		if err := l.container.UnpauseContainer(ctx, name); err != nil {
			errs = append(errs, err.Error())
		}
	}
	for _, name := range sortedKeys(l.stopped) {
		if err := l.container.StartContainer(ctx, name); err != nil {
			errs = append(errs, err.Error())
		}
	}
	l.paused = make(map[string]bool)
	l.stopped = make(map[string]bool)

	if l.app != nil && (l.appStopped || l.overrides != nil) {
		if l.app.Running() {
			if err := l.app.StopApp(); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if err := l.app.StartApp(ctx, nil); err != nil {
			errs = append(errs, err.Error())
		}
	}
	l.appStopped = false
	l.overrides = nil

	if len(errs) > 0 {
		return fmt.Errorf("bringing back what the last scenario stopped: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (l *Lifecycle) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, l.name, l.Steps())
}

func (l *Lifecycle) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "Lifecycle",
		Description: "Steps for stopping and starting the app and its containers mid-scenario",
		Steps: []resource.StepDef{
			{
				Group:       "App",
				Pattern:     `^"{resource}" restarts the app$`,
				Description: "Stop the app and start it again, waiting until it is ready, as a deploy or a crash does",
				Example:     `"{resource}" restarts the app`,
				Handler:     l.restartApp,
			},
			{
				Group:       "App",
				Pattern:     `^"{resource}" restarts the app with env:$`,
				Description: "Restart the app with these environment variables changed, for the rest of the scenario, e.g. a boundary a few seconds away",
				Example:     "\"{resource}\" restarts the app with env:\n  | name             | value |\n  | SESSION_BOUNDARY | 10s   |",
				Handler:     l.restartAppWithEnv,
			},
			{
				Group:       "App",
				Pattern:     `^"{resource}" stops the app$`,
				Description: "Stop the app, as when it crashes",
				Example:     `"{resource}" stops the app`,
				Handler:     l.stopApp,
			},
			{
				Group:       "App",
				Pattern:     `^"{resource}" starts the app$`,
				Description: "Start the stopped app and wait until it is ready",
				Example:     `"{resource}" starts the app`,
				Handler:     l.startApp,
			},
			{
				Group:       "Containers",
				Pattern:     `^"{resource}" stops container "([^"]*)"$`,
				Description: "Stop a container, as when the service it runs goes down. It keeps its state and its host ports",
				Example:     `"{resource}" stops container "postgres"`,
				Handler:     l.stopContainer,
			},
			{
				Group:       "Containers",
				Pattern:     `^"{resource}" starts container "([^"]*)"$`,
				Description: "Start a stopped container on the same address and wait until it is ready",
				Example:     `"{resource}" starts container "postgres"`,
				Handler:     l.startContainer,
			},
			{
				Group:       "Containers",
				Pattern:     `^"{resource}" restarts container "([^"]*)"$`,
				Description: "Stop a container and start it again, as a failover does",
				Example:     `"{resource}" restarts container "postgres"`,
				Handler:     l.restartContainer,
			},
			{
				Group:       "Containers",
				Pattern:     `^"{resource}" pauses container "([^"]*)"$`,
				Description: "Freeze a container: connections stay open and nothing answers, as with a hung service or a network partition",
				Example:     `"{resource}" pauses container "rabbitmq"`,
				Handler:     l.pauseContainer,
			},
			{
				Group:       "Containers",
				Pattern:     `^"{resource}" unpauses container "([^"]*)"$`,
				Description: "Resume a paused container",
				Example:     `"{resource}" unpauses container "rabbitmq"`,
				Handler:     l.unpauseContainer,
			},
		},
	}
}

func (l *Lifecycle) requireApp() error {
	if l.app == nil {
		return fmt.Errorf("no app to stop or start: configure app in tomato.yml")
	}
	return nil
}

func (l *Lifecycle) restartApp() error {
	return l.restart(l.overrides)
}

func (l *Lifecycle) restartAppWithEnv(table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("the table needs a header row (name | value) and at least one variable")
	}
	overrides := make(map[string]string)
	for k, v := range l.overrides {
		overrides[k] = v
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) < 2 {
			return fmt.Errorf("each row needs a name and a value")
		}
		overrides[row.Cells[0].Value] = resource.ReplaceVariables(row.Cells[1].Value)
	}
	return l.restart(overrides)
}

func (l *Lifecycle) restart(overrides map[string]string) error {
	if err := l.requireApp(); err != nil {
		return err
	}
	if l.app.Running() {
		if err := l.app.StopApp(); err != nil {
			return err
		}
	}
	l.overrides = overrides
	l.appStopped = true // until it is up again
	if err := l.app.StartApp(context.Background(), overrides); err != nil {
		return err
	}
	l.appStopped = false
	return nil
}

func (l *Lifecycle) stopApp() error {
	if err := l.requireApp(); err != nil {
		return err
	}
	if !l.app.Running() {
		return fmt.Errorf("the app is not running")
	}
	l.appStopped = true
	return l.app.StopApp()
}

func (l *Lifecycle) startApp() error {
	if err := l.requireApp(); err != nil {
		return err
	}
	if l.app.Running() {
		return fmt.Errorf("the app is already running")
	}
	if err := l.app.StartApp(context.Background(), l.overrides); err != nil {
		return err
	}
	l.appStopped = false
	return nil
}

func (l *Lifecycle) requireContainers() error {
	if l.container == nil {
		return fmt.Errorf("no containers: configure containers in tomato.yml")
	}
	return nil
}

func (l *Lifecycle) stopContainer(name string) error {
	if err := l.requireContainers(); err != nil {
		return err
	}
	if l.stopped[name] {
		return fmt.Errorf("container %s is already stopped", name)
	}
	if err := l.container.StopContainer(context.Background(), name); err != nil {
		return err
	}
	l.stopped[name] = true
	return nil
}

func (l *Lifecycle) startContainer(name string) error {
	if !l.stopped[name] {
		return fmt.Errorf("container %s is not stopped", name)
	}
	if err := l.container.StartContainer(context.Background(), name); err != nil {
		return err
	}
	delete(l.stopped, name)
	return nil
}

func (l *Lifecycle) restartContainer(name string) error {
	if err := l.stopContainer(name); err != nil {
		return err
	}
	return l.startContainer(name)
}

func (l *Lifecycle) pauseContainer(name string) error {
	if err := l.requireContainers(); err != nil {
		return err
	}
	if l.paused[name] {
		return fmt.Errorf("container %s is already paused", name)
	}
	if err := l.container.PauseContainer(context.Background(), name); err != nil {
		return err
	}
	l.paused[name] = true
	return nil
}

func (l *Lifecycle) unpauseContainer(name string) error {
	if !l.paused[name] {
		return fmt.Errorf("container %s is not paused", name)
	}
	if err := l.container.UnpauseContainer(context.Background(), name); err != nil {
		return err
	}
	delete(l.paused, name)
	return nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var (
	_ resource.Handler       = (*Lifecycle)(nil)
	_ resource.AppControlled = (*Lifecycle)(nil)
	_ resource.ResetsFirst   = (*Lifecycle)(nil)
)
