package runner

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/handler"
)

// Options configures runner behavior
type Options struct {
	NoReset bool
	Watch   bool
}

// Runner executes behavioral tests
type Runner struct {
	config    *config.Config
	container *container.Manager
	handlers  *handler.Registry
	opts      Options
}

// New creates a new test runner
func New(cfg *config.Config, cm *container.Manager, opts Options) (*Runner, error) {
	registry, err := handler.NewRegistry(cfg.Resources, cm)
	if err != nil {
		return nil, fmt.Errorf("initializing handlers: %w", err)
	}

	return &Runner{
		config:    cfg,
		container: cm,
		handlers:  registry,
		opts:      opts,
	}, nil
}

// Run executes all tests
func (r *Runner) Run(ctx context.Context) error {
	log.Debug().Msg("waiting for handlers to be ready")
	if err := r.handlers.WaitReady(ctx); err != nil {
		return fmt.Errorf("handlers not ready: %w", err)
	}

	if err := r.runHooks(ctx, r.config.Hooks.BeforeAll); err != nil {
		return fmt.Errorf("before_all hooks failed: %w", err)
	}

	opts := &godog.Options{
		Format:        r.config.Settings.Output,
		Paths:         r.config.Features.Paths,
		Tags:          r.config.Features.Tags,
		StopOnFailure: r.config.Settings.FailFast,
		Strict:        true,
		Concurrency:   r.config.Settings.Parallel,
	}

	suite := godog.TestSuite{
		Name:                "tomato",
		ScenarioInitializer: r.initializeScenario,
		Options:             opts,
	}

	status := suite.Run()

	if err := r.runHooks(ctx, r.config.Hooks.AfterAll); err != nil {
		log.Warn().Err(err).Msg("after_all hooks failed")
	}

	if status != 0 {
		return fmt.Errorf("tests failed with status %d", status)
	}

	return nil
}

func (r *Runner) initializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if !r.opts.NoReset {
			log.Debug().Str("scenario", sc.Name).Msg("resetting state")
			if err := r.handlers.ResetAll(ctx); err != nil {
				return ctx, fmt.Errorf("reset failed: %w", err)
			}
			// Reset captured variables between scenarios
			handler.ResetGlobalVariables()
		}

		if err := r.runHooks(ctx, r.config.Hooks.BeforeScenario); err != nil {
			return ctx, fmt.Errorf("before_scenario hooks failed: %w", err)
		}

		return ctx, nil
	})

	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if hookErr := r.runHooks(ctx, r.config.Hooks.AfterScenario); hookErr != nil {
			log.Warn().Err(hookErr).Msg("after_scenario hooks failed")
		}
		return ctx, nil
	})

	r.handlers.RegisterSteps(ctx)
}

func (r *Runner) runHooks(ctx context.Context, hooks []config.Hook) error {
	for _, hook := range hooks {
		if err := r.executeHook(ctx, hook); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) executeHook(ctx context.Context, hook config.Hook) error {
	switch {
	case hook.SQL != "":
		h, err := r.handlers.Get(hook.Resource)
		if err != nil {
			return fmt.Errorf("handler %s not found: %w", hook.Resource, err)
		}
		if sqlHandler, ok := h.(handler.SQLExecutor); ok {
			if _, err := sqlHandler.ExecSQL(ctx, hook.SQL); err != nil {
				return fmt.Errorf("executing SQL: %w", err)
			}
		} else {
			return fmt.Errorf("handler %s does not support SQL", hook.Resource)
		}

	case hook.SQLFile != "":
		h, err := r.handlers.Get(hook.Resource)
		if err != nil {
			return fmt.Errorf("handler %s not found: %w", hook.Resource, err)
		}
		if sqlHandler, ok := h.(handler.SQLExecutor); ok {
			if err := sqlHandler.ExecSQLFile(ctx, hook.SQLFile); err != nil {
				return fmt.Errorf("executing SQL file: %w", err)
			}
		} else {
			return fmt.Errorf("handler %s does not support SQL", hook.Resource)
		}

	case hook.Exec != "":
		if _, _, err := r.container.Exec(ctx, hook.Container, []string{"sh", "-c", hook.Exec}); err != nil {
			return fmt.Errorf("executing command in %s: %w", hook.Container, err)
		}

	case hook.Shell != "":
		if _, _, err := r.container.Exec(ctx, hook.Container, []string{"sh", "-c", hook.Shell}); err != nil {
			return fmt.Errorf("executing shell in %s: %w", hook.Container, err)
		}
	}

	return nil
}
