package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/handler"
)

// Mock implementations

type mockRegistry struct {
	waitReadyErr    error
	resetAllErr     error
	getHandler      handler.Handler
	getErr          error
	registerCalled  bool
	waitReadyCalled bool
	resetAllCalled  bool
}

func (m *mockRegistry) WaitReady(ctx context.Context) error {
	m.waitReadyCalled = true
	return m.waitReadyErr
}

func (m *mockRegistry) ResetAll(ctx context.Context) error {
	m.resetAllCalled = true
	return m.resetAllErr
}

func (m *mockRegistry) RegisterSteps(ctx *godog.ScenarioContext) {
	m.registerCalled = true
}

func (m *mockRegistry) Get(name string) (handler.Handler, error) {
	return m.getHandler, m.getErr
}

type mockContainerExecutor struct {
	execExitCode int
	execOutput   string
	execErr      error
	execCalls    []execCall
}

type execCall struct {
	name string
	cmd  []string
}

func (m *mockContainerExecutor) Exec(ctx context.Context, name string, cmd []string) (int, string, error) {
	m.execCalls = append(m.execCalls, execCall{name: name, cmd: cmd})
	return m.execExitCode, m.execOutput, m.execErr
}

// mockHandler is a basic handler that doesn't support SQL
type mockHandler struct {
	name       string
	initErr    error
	readyErr   error
	resetErr   error
	cleanupErr error
}

func (m *mockHandler) Name() string                             { return m.name }
func (m *mockHandler) Init(ctx context.Context) error           { return m.initErr }
func (m *mockHandler) Ready(ctx context.Context) error          { return m.readyErr }
func (m *mockHandler) Reset(ctx context.Context) error          { return m.resetErr }
func (m *mockHandler) RegisterSteps(ctx *godog.ScenarioContext) {}
func (m *mockHandler) Cleanup(ctx context.Context) error        { return m.cleanupErr }

// mockSQLHandler is a handler that supports SQL operations
type mockSQLHandler struct {
	mockHandler
	execSQLErr  error
	execSQLRows int64
	execFileErr error
}

func (m *mockSQLHandler) ExecSQL(ctx context.Context, query string) (int64, error) {
	return m.execSQLRows, m.execSQLErr
}

func (m *mockSQLHandler) ExecSQLFile(ctx context.Context, path string) error {
	return m.execFileErr
}

// Test helper functions

func newTestConfig() *config.Config {
	return &config.Config{
		Version: 2,
		Settings: config.Settings{
			Output:   "pretty",
			Parallel: 1,
		},
		Features: config.Features{
			Paths: []string{"./features"},
		},
		Resources: make(map[string]config.Resource),
		Hooks:     config.Hooks{},
	}
}

// Tests for newRunner

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		opts          Options
		wantErr       bool
		errContains   string
		checkRegex    bool
	}{
		{
			name:    "successful creation with defaults",
			config:  newTestConfig(),
			opts:    Options{},
			wantErr: false,
		},
		{
			name: "successful creation with valid scenario regex",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Features.Scenario = "^Test.*"
				return cfg
			}(),
			opts:       Options{},
			wantErr:    false,
			checkRegex: true,
		},
		{
			name: "invalid scenario regex",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Features.Scenario = "[invalid"
				return cfg
			}(),
			opts:        Options{},
			wantErr:     true,
			errContains: "invalid scenario filter regex",
		},
		{
			name: "with NoReset option",
			config: newTestConfig(),
			opts: Options{
				NoReset: true,
			},
			wantErr: false,
		},
		{
			name: "with Watch option",
			config: newTestConfig(),
			opts: Options{
				Watch: true,
			},
			wantErr: false,
		},
		{
			name: "with Format override",
			config: newTestConfig(),
			opts: Options{
				Format: "json",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &mockRegistry{}
			container := &mockContainerExecutor{}

			runner, err := newRunner(tt.config, container, registry, tt.opts)

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

			if runner == nil {
				t.Error("expected runner, got nil")
				return
			}

			if runner.config != tt.config {
				t.Error("config not set correctly")
			}

			if runner.opts != tt.opts {
				t.Error("options not set correctly")
			}

			if tt.checkRegex && runner.scenarioRegex == nil {
				t.Error("expected scenario regex to be set")
			}
		})
	}
}

// Tests for executeHook

func TestExecuteHook(t *testing.T) {
	tests := []struct {
		name        string
		hook        config.Hook
		registry    *mockRegistry
		container   *mockContainerExecutor
		wantErr     bool
		errContains string
	}{
		{
			name: "SQL hook success",
			hook: config.Hook{
				Resource: "db",
				SQL:      "SELECT 1",
			},
			registry: &mockRegistry{
				getHandler: &mockSQLHandler{},
			},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "SQL hook - handler not found",
			hook: config.Hook{
				Resource: "unknown",
				SQL:      "SELECT 1",
			},
			registry: &mockRegistry{
				getErr: errors.New("handler not found"),
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "handler unknown not found",
		},
		{
			name: "SQL hook - handler does not support SQL",
			hook: config.Hook{
				Resource: "http",
				SQL:      "SELECT 1",
			},
			registry: &mockRegistry{
				getHandler: &mockHandler{}, // mockHandler doesn't implement SQLExecutor
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "does not support SQL",
		},
		{
			name: "SQL hook - ExecSQL fails",
			hook: config.Hook{
				Resource: "db",
				SQL:      "INVALID SQL",
			},
			registry: &mockRegistry{
				getHandler: &mockSQLHandler{
					execSQLErr: errors.New("syntax error"),
				},
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "executing SQL",
		},
		{
			name: "SQLFile hook success",
			hook: config.Hook{
				Resource: "db",
				SQLFile:  "schema.sql",
			},
			registry: &mockRegistry{
				getHandler: &mockSQLHandler{},
			},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "SQLFile hook - handler not found",
			hook: config.Hook{
				Resource: "unknown",
				SQLFile:  "schema.sql",
			},
			registry: &mockRegistry{
				getErr: errors.New("handler not found"),
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "handler unknown not found",
		},
		{
			name: "SQLFile hook - handler does not support SQL",
			hook: config.Hook{
				Resource: "http",
				SQLFile:  "schema.sql",
			},
			registry: &mockRegistry{
				getHandler: &mockHandler{}, // mockHandler doesn't implement SQLExecutor
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "does not support SQL",
		},
		{
			name: "SQLFile hook - ExecSQLFile fails",
			hook: config.Hook{
				Resource: "db",
				SQLFile:  "missing.sql",
			},
			registry: &mockRegistry{
				getHandler: &mockSQLHandler{
					execFileErr: errors.New("file not found"),
				},
			},
			container:   &mockContainerExecutor{},
			wantErr:     true,
			errContains: "executing SQL file",
		},
		{
			name: "Exec hook success",
			hook: config.Hook{
				Container: "app",
				Exec:      "echo hello",
			},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "Exec hook - container exec fails",
			hook: config.Hook{
				Container: "app",
				Exec:      "exit 1",
			},
			registry: &mockRegistry{},
			container: &mockContainerExecutor{
				execErr: errors.New("command failed"),
			},
			wantErr:     true,
			errContains: "executing command in app",
		},
		{
			name: "Shell hook success",
			hook: config.Hook{
				Container: "app",
				Shell:     "ls -la",
			},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "Shell hook - container exec fails",
			hook: config.Hook{
				Container: "app",
				Shell:     "invalid command",
			},
			registry: &mockRegistry{},
			container: &mockContainerExecutor{
				execErr: errors.New("command failed"),
			},
			wantErr:     true,
			errContains: "executing shell in app",
		},
		{
			name:      "empty hook - no operation",
			hook:      config.Hook{},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &Runner{
				config:    newTestConfig(),
				handlers:  tt.registry,
				container: tt.container,
			}

			err := runner.executeHook(context.Background(), tt.hook)

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
			}
		})
	}
}

// Tests for runHooks

func TestRunHooks(t *testing.T) {
	tests := []struct {
		name        string
		hooks       []config.Hook
		registry    *mockRegistry
		container   *mockContainerExecutor
		wantErr     bool
		errContains string
	}{
		{
			name:      "empty hooks",
			hooks:     []config.Hook{},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "single hook success",
			hooks: []config.Hook{
				{Container: "app", Exec: "echo test"},
			},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "multiple hooks success",
			hooks: []config.Hook{
				{Container: "app", Exec: "echo first"},
				{Container: "app", Exec: "echo second"},
				{Container: "app", Shell: "echo third"},
			},
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
		{
			name: "first hook fails - stops execution",
			hooks: []config.Hook{
				{Container: "app", Exec: "fail"},
				{Container: "app", Exec: "echo second"},
			},
			registry: &mockRegistry{},
			container: &mockContainerExecutor{
				execErr: errors.New("command failed"),
			},
			wantErr:     true,
			errContains: "executing command",
		},
		{
			name: "mixed hook types",
			hooks: []config.Hook{
				{Resource: "db", SQL: "SELECT 1"},
				{Container: "app", Exec: "echo test"},
			},
			registry: &mockRegistry{
				getHandler: &mockSQLHandler{},
			},
			container: &mockContainerExecutor{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &Runner{
				config:    newTestConfig(),
				handlers:  tt.registry,
				container: tt.container,
			}

			err := runner.runHooks(context.Background(), tt.hooks)

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
			}
		})
	}
}

// Tests for Run method

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		registry    *mockRegistry
		container   *mockContainerExecutor
		opts        Options
		wantErr     bool
		errContains string
	}{
		{
			name:   "WaitReady fails",
			config: newTestConfig(),
			registry: &mockRegistry{
				waitReadyErr: errors.New("connection refused"),
			},
			container:   &mockContainerExecutor{},
			opts:        Options{},
			wantErr:     true,
			errContains: "handlers not ready",
		},
		{
			name: "before_all hooks fail",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Hooks.BeforeAll = []config.Hook{
					{Container: "app", Exec: "fail"},
				}
				return cfg
			}(),
			registry: &mockRegistry{},
			container: &mockContainerExecutor{
				execErr: errors.New("hook failed"),
			},
			opts:        Options{},
			wantErr:     true,
			errContains: "before_all hooks failed",
		},
		{
			name: "format override from options",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Settings.Output = "pretty"
				return cfg
			}(),
			registry:  &mockRegistry{},
			container: &mockContainerExecutor{},
			opts: Options{
				Format: "json",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &Runner{
				config:    tt.config,
				handlers:  tt.registry,
				container: tt.container,
				opts:      tt.opts,
			}

			err := runner.Run(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			// Note: successful Run() with no features will still pass
			// as godog returns 0 for empty test suites
			if err != nil && !contains(err.Error(), "tests failed") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Tests for Options struct

func TestOptions(t *testing.T) {
	tests := []struct {
		name   string
		opts   Options
		check  func(Options) bool
	}{
		{
			name: "default options",
			opts: Options{},
			check: func(o Options) bool {
				return !o.NoReset && !o.Watch && o.Format == ""
			},
		},
		{
			name: "NoReset enabled",
			opts: Options{NoReset: true},
			check: func(o Options) bool {
				return o.NoReset
			},
		},
		{
			name: "Watch enabled",
			opts: Options{Watch: true},
			check: func(o Options) bool {
				return o.Watch
			},
		},
		{
			name: "Format set",
			opts: Options{Format: "tomato"},
			check: func(o Options) bool {
				return o.Format == "tomato"
			},
		},
		{
			name: "all options enabled",
			opts: Options{NoReset: true, Watch: true, Format: "json"},
			check: func(o Options) bool {
				return o.NoReset && o.Watch && o.Format == "json"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.opts) {
				t.Error("options check failed")
			}
		})
	}
}

// Tests for exec/shell hook command construction

func TestExecHookCommandConstruction(t *testing.T) {
	tests := []struct {
		name            string
		hook            config.Hook
		expectedCmd     []string
		expectedCalls   int
	}{
		{
			name: "exec command wraps in sh -c",
			hook: config.Hook{
				Container: "app",
				Exec:      "echo hello",
			},
			expectedCmd:   []string{"sh", "-c", "echo hello"},
			expectedCalls: 1,
		},
		{
			name: "shell command wraps in sh -c",
			hook: config.Hook{
				Container: "app",
				Shell:     "ls -la && pwd",
			},
			expectedCmd:   []string{"sh", "-c", "ls -la && pwd"},
			expectedCalls: 1,
		},
		{
			name: "exec with complex command",
			hook: config.Hook{
				Container: "db",
				Exec:      "psql -c 'SELECT 1'",
			},
			expectedCmd:   []string{"sh", "-c", "psql -c 'SELECT 1'"},
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &mockContainerExecutor{}
			runner := &Runner{
				config:    newTestConfig(),
				handlers:  &mockRegistry{},
				container: container,
			}

			_ = runner.executeHook(context.Background(), tt.hook)

			if len(container.execCalls) != tt.expectedCalls {
				t.Errorf("expected %d exec calls, got %d", tt.expectedCalls, len(container.execCalls))
				return
			}

			if tt.expectedCalls > 0 {
				call := container.execCalls[0]
				if call.name != tt.hook.Container {
					t.Errorf("expected container %q, got %q", tt.hook.Container, call.name)
				}
				if !sliceEqual(call.cmd, tt.expectedCmd) {
					t.Errorf("expected cmd %v, got %v", tt.expectedCmd, call.cmd)
				}
			}
		})
	}
}

// Tests for scenario regex matching

func TestScenarioRegexMatching(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		scenarioName string
		shouldMatch  bool
	}{
		{
			name:         "prefix match",
			pattern:      "^Test.*",
			scenarioName: "Test user login",
			shouldMatch:  true,
		},
		{
			name:         "prefix no match",
			pattern:      "^Test.*",
			scenarioName: "User can login",
			shouldMatch:  false,
		},
		{
			name:         "contains match",
			pattern:      ".*login.*",
			scenarioName: "User can login successfully",
			shouldMatch:  true,
		},
		{
			name:         "exact match pattern",
			pattern:      "^User creates account$",
			scenarioName: "User creates account",
			shouldMatch:  true,
		},
		{
			name:         "case sensitive no match",
			pattern:      "^Test.*",
			scenarioName: "test user login",
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			cfg.Features.Scenario = tt.pattern

			runner, err := newRunner(cfg, &mockContainerExecutor{}, &mockRegistry{}, Options{})
			if err != nil {
				t.Fatalf("failed to create runner: %v", err)
			}

			matches := runner.scenarioRegex.MatchString(tt.scenarioName)
			if matches != tt.shouldMatch {
				t.Errorf("pattern %q on %q: expected match=%v, got match=%v",
					tt.pattern, tt.scenarioName, tt.shouldMatch, matches)
			}
		})
	}
}

// Tests for initializeScenario

func TestInitializeScenario(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		registry       *mockRegistry
		opts           Options
		scenarioName   string
		scenarioRegex  string
		expectSkip     bool
		expectReset    bool
		beforeHookErr  bool
	}{
		{
			name:         "scenario matches filter - executes normally",
			config:       newTestConfig(),
			registry:     &mockRegistry{},
			opts:         Options{},
			scenarioName: "Test login",
			scenarioRegex: "^Test.*",
			expectSkip:   false,
			expectReset:  true,
		},
		{
			name:         "scenario does not match filter - skipped",
			config:       newTestConfig(),
			registry:     &mockRegistry{},
			opts:         Options{},
			scenarioName: "User can login",
			scenarioRegex: "^Test.*",
			expectSkip:   true,
			expectReset:  false,
		},
		{
			name:         "no filter - executes normally",
			config:       newTestConfig(),
			registry:     &mockRegistry{},
			opts:         Options{},
			scenarioName: "Any scenario",
			scenarioRegex: "",
			expectSkip:   false,
			expectReset:  true,
		},
		{
			name:         "NoReset option - skips reset",
			config:       newTestConfig(),
			registry:     &mockRegistry{},
			opts:         Options{NoReset: true},
			scenarioName: "Test scenario",
			scenarioRegex: "",
			expectSkip:   false,
			expectReset:  false,
		},
		{
			name:   "reset fails",
			config: newTestConfig(),
			registry: &mockRegistry{
				resetAllErr: errors.New("reset failed"),
			},
			opts:         Options{},
			scenarioName: "Test scenario",
			scenarioRegex: "",
			expectSkip:   false,
			expectReset:  true,
		},
		{
			name: "before_scenario hook fails",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Hooks.BeforeScenario = []config.Hook{
					{Container: "app", Exec: "fail"},
				}
				return cfg
			}(),
			registry:      &mockRegistry{},
			opts:          Options{},
			scenarioName:  "Test scenario",
			scenarioRegex: "",
			expectSkip:    false,
			expectReset:   true,
			beforeHookErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up runner with scenario regex if specified
			if tt.scenarioRegex != "" {
				tt.config.Features.Scenario = tt.scenarioRegex
			}

			container := &mockContainerExecutor{}
			if tt.beforeHookErr {
				container.execErr = errors.New("hook failed")
			}

			runner, err := newRunner(tt.config, container, tt.registry, tt.opts)
			if err != nil {
				t.Fatalf("failed to create runner: %v", err)
			}

			// Create a capturing scenario context
			capturingCtx := &capturingScenarioContext{}
			runner.setupScenarioHooks(capturingCtx)

			// Verify hooks were registered
			if capturingCtx.beforeHook == nil {
				t.Error("before hook was not registered")
				return
			}

			// Create mock scenario
			sc := &godog.Scenario{Name: tt.scenarioName}

			// Invoke the before hook
			ctx := context.Background()
			_, err = capturingCtx.beforeHook(ctx, sc)

			// Check if scenario was skipped
			if tt.expectSkip {
				if err != godog.ErrSkip {
					t.Errorf("expected ErrSkip, got %v", err)
				}
			} else if tt.registry.resetAllErr != nil {
				if err == nil || !contains(err.Error(), "reset failed") {
					t.Errorf("expected reset error, got %v", err)
				}
			} else if tt.beforeHookErr {
				if err == nil || !contains(err.Error(), "before_scenario hooks failed") {
					t.Errorf("expected before_scenario hook error, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			// Check if reset was called
			if tt.expectReset && !tt.expectSkip {
				if !tt.registry.resetAllCalled {
					t.Error("expected ResetAll to be called")
				}
			}
		})
	}
}

// Tests for after scenario hook

func TestInitializeScenarioAfterHook(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		container *mockContainerExecutor
	}{
		{
			name:      "after hook success",
			config:    newTestConfig(),
			container: &mockContainerExecutor{},
		},
		{
			name: "after hook failure - logs warning but returns nil",
			config: func() *config.Config {
				cfg := newTestConfig()
				cfg.Hooks.AfterScenario = []config.Hook{
					{Container: "app", Exec: "cleanup"},
				}
				return cfg
			}(),
			container: &mockContainerExecutor{
				execErr: errors.New("cleanup failed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, _ := newRunner(tt.config, tt.container, &mockRegistry{}, Options{})

			capturingCtx := &capturingScenarioContext{}
			runner.setupScenarioHooks(capturingCtx)

			if capturingCtx.afterHook == nil {
				t.Error("after hook was not registered")
				return
			}

			// After hook should always return nil (warnings are logged)
			sc := &godog.Scenario{Name: "Test"}
			_, err := capturingCtx.afterHook(context.Background(), sc, nil)
			if err != nil {
				t.Errorf("after hook should return nil, got %v", err)
			}
		})
	}
}

// capturingScenarioContext captures registered hooks for testing
type capturingScenarioContext struct {
	beforeHook godog.BeforeScenarioHook
	afterHook  godog.AfterScenarioHook
}

func (c *capturingScenarioContext) Before(h godog.BeforeScenarioHook) {
	c.beforeHook = h
}

func (c *capturingScenarioContext) After(h godog.AfterScenarioHook) {
	c.afterHook = h
}

func (c *capturingScenarioContext) Step(expr interface{}, stepFunc interface{}) {}

// Tests for Run with more coverage

func TestRunWithAfterAllHooks(t *testing.T) {
	// Test that after_all hooks are called even when they fail (just logged)
	cfg := newTestConfig()
	cfg.Hooks.AfterAll = []config.Hook{
		{Container: "app", Exec: "cleanup"},
	}

	container := &mockContainerExecutor{
		execErr: errors.New("cleanup failed"),
	}

	runner := &Runner{
		config:    cfg,
		handlers:  &mockRegistry{},
		container: container,
		opts:      Options{},
	}

	// This should not error even though after_all hook fails
	// (after_all failures are just logged)
	_ = runner.Run(context.Background())

	// Verify the hook was attempted
	if len(container.execCalls) == 0 {
		t.Error("expected after_all hook to be attempted")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
