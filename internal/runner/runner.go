package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/feature"
	_ "github.com/tomatool/tomato/internal/formatter" // Register tomato formatter
	"github.com/tomatool/tomato/internal/profile"
	"github.com/tomatool/tomato/internal/registry"
	"github.com/tomatool/tomato/internal/resource"
)

// Options configures runner behavior
type Options struct {
	Watch  bool
	Format string // Override output format (e.g., "tomato" for structured events)
	// Profiler, when set, measures each scenario (see internal/profile).
	Profiler ScenarioProfiler
}

// ScenarioProfiler opens a window per scenario and closes it when it ends.
type ScenarioProfiler interface {
	ScenarioStarted(name string)
	ScenarioEnded(name string) *profile.Summary
}

// Runner executes behavioral tests
type Runner struct {
	config    *config.Config
	container ContainerExecutor
	handlers  HandlerRegistry
	opts      Options
	// scenarioPaths is "file.feature:12" per selected scenario, empty when
	// no filter is set.
	scenarioPaths []string
	godogTags     string
}

// NewWithRegistry creates a runner over a registry the caller already built,
// for when some resources had to be initialized before the app started.
func NewWithRegistry(cfg *config.Config, cm *container.Manager, registry *registry.Registry, opts Options) (*Runner, error) {
	return newRunner(cfg, cm, registry, opts)
}

// newRunner is the internal constructor that allows dependency injection for testing
func newRunner(cfg *config.Config, container ContainerExecutor, handlers HandlerRegistry, opts Options) (*Runner, error) {
	r := &Runner{
		config:    cfg,
		container: container,
		handlers:  handlers,
		opts:      opts,
	}

	tags, err := toGodogTags(cfg.Features.Tags)
	if err != nil {
		return nil, err
	}
	r.godogTags = tags

	// A scenario filter selects the scenarios up front rather than skipping
	// the others as they start. Skipping made godog run and report every
	// scenario, and a scenario skipped in a Before hook counts as passed —
	// so filtering to one scenario reported the whole suite green, and a
	// filter matching nothing reported a passing run that tested nothing.
	if cfg.Features.Scenario != "" {
		regex, err := regexp.Compile(cfg.Features.Scenario)
		if err != nil {
			return nil, fmt.Errorf("invalid scenario filter regex: %w", err)
		}
		matches, err := feature.Select(cfg.Features.Paths, regex)
		if err != nil {
			return nil, err
		}
		r.scenarioPaths = make([]string, len(matches))
		for i, m := range matches {
			r.scenarioPaths[i] = m.Path
		}
		log.Info().Str("pattern", cfg.Features.Scenario).
			Int("scenarios", len(matches)).Msg("scenario filter active")
	}

	return r, nil
}

// featurePaths is what godog is told to run: the whole configured paths, or
// just the scenarios a filter selected, each as "file.feature:line".
func (r *Runner) featurePaths() []string {
	if len(r.scenarioPaths) > 0 {
		return r.scenarioPaths
	}
	return r.config.Features.Paths
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

	format := resolveFormat(r.config.Settings.Output, r.opts.Format)
	if err := ensureReportDirs(format); err != nil {
		return err
	}

	opts := &godog.Options{
		Format:        format,
		Paths:         r.featurePaths(),
		Tags:          r.godogTags,
		StopOnFailure: r.config.Settings.FailFast,
		Strict:        true,
		// One at a time, always: every scenario resets every resource, and
		// the resources are shared. config rejects settings.parallel > 1.
		Concurrency: 1,
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
	r.setupScenarioHooks(ctx)
	r.handlers.RegisterSteps(ctx)
}

// setupScenarioHooks sets up before/after hooks on the scenario context
// This internal method accepts an interface for testability
func (r *Runner) setupScenarioHooks(ctx ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		// Every scenario starts from the same state. There is no way to turn
		// this off: a scenario that needs a starting point builds it in a
		// Background, which godog runs after this hook.
		log.Debug().Str("scenario", sc.Name).Msg("resetting state")
		if err := r.handlers.ResetAll(ctx); err != nil {
			return ctx, fmt.Errorf("reset failed: %w", err)
		}
		resource.ResetGlobalVariables()

		if err := r.runHooks(ctx, r.config.Hooks.BeforeScenario); err != nil {
			return ctx, fmt.Errorf("before_scenario hooks failed: %w", err)
		}

		// The scenario's profile starts after the resets and hooks, so
		// tomato's own work is not counted to it.
		if r.opts.Profiler != nil {
			r.opts.Profiler.ScenarioStarted(sc.Name)
		}
		return ctx, nil
	})

	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if r.opts.Profiler != nil {
			r.opts.Profiler.ScenarioEnded(sc.Name)
		}
		if hookErr := r.runHooks(ctx, r.config.Hooks.AfterScenario); hookErr != nil {
			log.Warn().Err(hookErr).Msg("after_scenario hooks failed")
		}
		return ctx, nil
	})
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
		if sqlHandler, ok := h.(resource.SQLExecutor); ok {
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
		if sqlHandler, ok := h.(resource.SQLExecutor); ok {
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

// resolveFormat combines the configured output with a --format override. The
// override replaces what goes to stdout, but file outputs from the config
// (`junit:reports/tomato.xml`) are kept: the GitHub Action forces
// `--format tomato` to build its PR comment, and that must not silently drop
// the JUnit report the same workflow uploads.
func resolveFormat(configured, override string) string {
	if override == "" {
		return configured
	}
	formats := []string{override}
	for _, part := range strings.Split(configured, ",") {
		part = strings.TrimSpace(part)
		if _, path, ok := strings.Cut(part, ":"); ok && path != "" {
			formats = append(formats, part)
		}
	}
	return strings.Join(formats, ",")
}

// ensureReportDirs creates the parent directory of every file output in a
// godog format string such as "pretty,junit:reports/tomato.xml". godog opens
// those files itself and, when the directory is missing, only prints an error
// and writes no report — CI then uploads nothing and nobody notices.
func ensureReportDirs(format string) error {
	for _, part := range strings.Split(format, ",") {
		_, path, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || path == "" {
			continue
		}
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating report directory for %s: %w", path, err)
			}
		}
	}
	return nil
}
