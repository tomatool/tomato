package runner

import (
	"context"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/handler"
)

// HandlerRegistry abstracts handler.Registry for testing
type HandlerRegistry interface {
	WaitReady(ctx context.Context) error
	ResetAll(ctx context.Context) error
	RegisterSteps(ctx *godog.ScenarioContext)
	Get(name string) (handler.Handler, error)
}

// ContainerExecutor abstracts container execution for testing
type ContainerExecutor interface {
	Exec(ctx context.Context, name string, cmd []string) (int, string, error)
}

// RegistryFactory creates handler registries
type RegistryFactory func(configs map[string]any) (HandlerRegistry, error)

// ScenarioContext abstracts godog.ScenarioContext for testing
type ScenarioContext interface {
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
	Step(expr interface{}, stepFunc interface{})
}
