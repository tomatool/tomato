package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test helper to create temp config files
func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tomato.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	return path
}

// Tests for Load function

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "minimal valid config",
			content: `
version: 2
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Version != 2 {
					t.Errorf("expected version 2, got %d", cfg.Version)
				}
			},
		},
		{
			name: "full config with all sections",
			content: `
version: 2
settings:
  timeout: 10m
  parallel: 4
  fail_fast: true
  output: json
  reset:
    level: feature
    on_failure: keep
containers:
  postgres:
    image: postgres:15
    ports:
      - "5432/tcp"
    env:
      POSTGRES_PASSWORD: test
    wait_for:
      type: port
      target: "5432/tcp"
resources:
  db:
    type: postgres
    container: postgres
    database: test
features:
  paths:
    - ./features
    - ./integration
  tags: "@smoke"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Settings.Timeout != 10*time.Minute {
					t.Errorf("expected timeout 10m, got %v", cfg.Settings.Timeout)
				}
				if cfg.Settings.Parallel != 4 {
					t.Errorf("expected parallel 4, got %d", cfg.Settings.Parallel)
				}
				if !cfg.Settings.FailFast {
					t.Error("expected fail_fast true")
				}
				if cfg.Settings.Output != "json" {
					t.Errorf("expected output json, got %s", cfg.Settings.Output)
				}
				if cfg.Settings.Reset.Level != "feature" {
					t.Errorf("expected reset level feature, got %s", cfg.Settings.Reset.Level)
				}
				if cfg.Settings.Reset.OnFailure != "keep" {
					t.Errorf("expected on_failure keep, got %s", cfg.Settings.Reset.OnFailure)
				}
				if _, ok := cfg.Containers["postgres"]; !ok {
					t.Error("expected postgres container")
				}
				if _, ok := cfg.Resources["db"]; !ok {
					t.Error("expected db resource")
				}
				if len(cfg.Features.Paths) != 2 {
					t.Errorf("expected 2 feature paths, got %d", len(cfg.Features.Paths))
				}
				if cfg.Features.Tags != "@smoke" {
					t.Errorf("expected tags @smoke, got %s", cfg.Features.Tags)
				}
			},
		},
		{
			name: "config with hooks",
			content: `
version: 2
hooks:
  before_all:
    - sql: "DELETE FROM users"
      resource: db
  after_all:
    - exec: "echo done"
      container: app
  before_scenario:
    - sql_file: "seed.sql"
      resource: db
  after_scenario:
    - shell: "cleanup.sh"
      container: app
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Hooks.BeforeAll) != 1 {
					t.Errorf("expected 1 before_all hook, got %d", len(cfg.Hooks.BeforeAll))
				}
				if cfg.Hooks.BeforeAll[0].SQL != "DELETE FROM users" {
					t.Errorf("expected SQL hook, got %+v", cfg.Hooks.BeforeAll[0])
				}
				if len(cfg.Hooks.AfterAll) != 1 {
					t.Errorf("expected 1 after_all hook, got %d", len(cfg.Hooks.AfterAll))
				}
				if len(cfg.Hooks.BeforeScenario) != 1 {
					t.Errorf("expected 1 before_scenario hook, got %d", len(cfg.Hooks.BeforeScenario))
				}
				if len(cfg.Hooks.AfterScenario) != 1 {
					t.Errorf("expected 1 after_scenario hook, got %d", len(cfg.Hooks.AfterScenario))
				}
			},
		},
		{
			name: "config with app section - build",
			content: `
version: 2
app:
  build:
    dockerfile: Dockerfile
    context: .
  port: 8080
  ready:
    type: http
    path: /health
    status: 200
  wait: 5s
  env:
    DB_HOST: localhost
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.App.Build == nil {
					t.Error("expected app.build to be set")
					return
				}
				if cfg.App.Build.Dockerfile != "Dockerfile" {
					t.Errorf("expected dockerfile Dockerfile, got %s", cfg.App.Build.Dockerfile)
				}
				if cfg.App.Port != 8080 {
					t.Errorf("expected port 8080, got %d", cfg.App.Port)
				}
				if cfg.App.Ready == nil {
					t.Error("expected ready check")
					return
				}
				if cfg.App.Ready.Type != "http" {
					t.Errorf("expected ready type http, got %s", cfg.App.Ready.Type)
				}
				if cfg.App.Wait != 5*time.Second {
					t.Errorf("expected wait 5s, got %v", cfg.App.Wait)
				}
			},
		},
		{
			name: "config with app section - command",
			content: `
version: 2
app:
  command: "./myapp serve"
  workdir: /app
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.App.Command != "./myapp serve" {
					t.Errorf("expected command './myapp serve', got %s", cfg.App.Command)
				}
				if cfg.App.WorkDir != "/app" {
					t.Errorf("expected workdir /app, got %s", cfg.App.WorkDir)
				}
			},
		},
		{
			name:        "invalid YAML",
			content:     `version: [invalid`,
			wantErr:     true,
			errContains: "parsing config",
		},
		{
			name: "unsupported version",
			content: `
version: 1
`,
			wantErr:     true,
			errContains: "unsupported config version",
		},
		{
			name: "invalid reset level",
			content: `
version: 2
settings:
  reset:
    level: invalid
`,
			wantErr:     true,
			errContains: "invalid reset level",
		},
		{
			name: "resource references unknown container",
			content: `
version: 2
resources:
  db:
    type: postgres
    container: nonexistent
`,
			wantErr:     true,
			errContains: "references unknown container",
		},
		{
			name: "container depends on unknown container",
			content: `
version: 2
containers:
  app:
    image: myapp
    depends_on:
      - nonexistent
`,
			wantErr:     true,
			errContains: "depends on unknown container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTempConfig(t, tt.content)

			cfg, err := Load(path)

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

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/tomato.yml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !contains(err.Error(), "reading config") {
		t.Errorf("expected 'reading config' error, got %v", err)
	}
}

func TestLoadWithEnvVarExpansion(t *testing.T) {
	// Set environment variable
	os.Setenv("TEST_DB_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_DB_PASSWORD")

	content := `
version: 2
containers:
  postgres:
    image: postgres:15
    env:
      POSTGRES_PASSWORD: $TEST_DB_PASSWORD
`
	path := createTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Containers["postgres"].Env["POSTGRES_PASSWORD"] != "secret123" {
		t.Errorf("expected env var expansion, got %s", cfg.Containers["postgres"].Env["POSTGRES_PASSWORD"])
	}
}

// Tests for applyDefaults

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		validate func(*testing.T, *Config)
	}{
		{
			name:   "empty config gets all defaults",
			config: Config{},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Version != 2 {
					t.Errorf("expected default version 2, got %d", cfg.Version)
				}
				if cfg.Settings.Timeout != 5*time.Minute {
					t.Errorf("expected default timeout 5m, got %v", cfg.Settings.Timeout)
				}
				if cfg.Settings.Parallel != 1 {
					t.Errorf("expected default parallel 1, got %d", cfg.Settings.Parallel)
				}
				if cfg.Settings.Output != "pretty" {
					t.Errorf("expected default output pretty, got %s", cfg.Settings.Output)
				}
				if cfg.Settings.Reset.Level != "scenario" {
					t.Errorf("expected default reset level scenario, got %s", cfg.Settings.Reset.Level)
				}
				if cfg.Settings.Reset.OnFailure != "reset" {
					t.Errorf("expected default on_failure reset, got %s", cfg.Settings.Reset.OnFailure)
				}
				if len(cfg.Features.Paths) != 1 || cfg.Features.Paths[0] != "./features" {
					t.Errorf("expected default features path ./features, got %v", cfg.Features.Paths)
				}
			},
		},
		{
			name: "existing values are preserved",
			config: Config{
				Version: 2,
				Settings: Settings{
					Timeout:  10 * time.Minute,
					Parallel: 8,
					Output:   "json",
					Reset: ResetSettings{
						Level:     "feature",
						OnFailure: "keep",
					},
				},
				Features: Features{
					Paths: []string{"./tests"},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Settings.Timeout != 10*time.Minute {
					t.Errorf("timeout should be preserved, got %v", cfg.Settings.Timeout)
				}
				if cfg.Settings.Parallel != 8 {
					t.Errorf("parallel should be preserved, got %d", cfg.Settings.Parallel)
				}
				if cfg.Settings.Output != "json" {
					t.Errorf("output should be preserved, got %s", cfg.Settings.Output)
				}
				if cfg.Settings.Reset.Level != "feature" {
					t.Errorf("reset level should be preserved, got %s", cfg.Settings.Reset.Level)
				}
				if cfg.Features.Paths[0] != "./tests" {
					t.Errorf("features path should be preserved, got %v", cfg.Features.Paths)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.applyDefaults()
			tt.validate(t, &cfg)
		})
	}
}

// Tests for validate

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid reset level - feature",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "feature"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid reset level - run",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "run"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid reset level - none",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "none"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			config: Config{
				Version: 1,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
			},
			wantErr:     true,
			errContains: "unsupported config version",
		},
		{
			name: "invalid reset level",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "invalid"},
				},
			},
			wantErr:     true,
			errContains: "invalid reset level",
		},
		{
			name: "resource references valid container",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
				Containers: map[string]Container{
					"postgres": {Image: "postgres:15"},
				},
				Resources: map[string]Resource{
					"db": {Type: "postgres", Container: "postgres"},
				},
			},
			wantErr: false,
		},
		{
			name: "resource references unknown container",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
				Resources: map[string]Resource{
					"db": {Type: "postgres", Container: "nonexistent"},
				},
			},
			wantErr:     true,
			errContains: "references unknown container",
		},
		{
			name: "resource with empty container is valid",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
				Resources: map[string]Resource{
					"api": {Type: "http", BaseURL: "http://localhost:8080"},
				},
			},
			wantErr: false,
		},
		{
			name: "container dependencies valid",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
				Containers: map[string]Container{
					"postgres": {Image: "postgres:15"},
					"app":      {Image: "myapp", DependsOn: []string{"postgres"}},
				},
			},
			wantErr: false,
		},
		{
			name: "container depends on unknown",
			config: Config{
				Version: 2,
				Settings: Settings{
					Reset: ResetSettings{Level: "scenario"},
				},
				Containers: map[string]Container{
					"app": {Image: "myapp", DependsOn: []string{"nonexistent"}},
				},
			},
			wantErr:     true,
			errContains: "depends on unknown container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

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

// Tests for AppConfig.IsConfigured

func TestAppConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name       string
		appConfig  AppConfig
		configured bool
	}{
		{
			name:       "empty config - not configured",
			appConfig:  AppConfig{},
			configured: false,
		},
		{
			name: "with build - configured",
			appConfig: AppConfig{
				Build: &AppBuild{Dockerfile: "Dockerfile"},
			},
			configured: true,
		},
		{
			name: "with command - configured",
			appConfig: AppConfig{
				Command: "./app serve",
			},
			configured: true,
		},
		{
			name: "with both build and command - configured",
			appConfig: AppConfig{
				Build:   &AppBuild{Dockerfile: "Dockerfile"},
				Command: "./app serve",
			},
			configured: true,
		},
		{
			name: "with only port - not configured",
			appConfig: AppConfig{
				Port: 8080,
			},
			configured: false,
		},
		{
			name: "with only env - not configured",
			appConfig: AppConfig{
				Env: map[string]string{"KEY": "value"},
			},
			configured: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appConfig.IsConfigured()
			if result != tt.configured {
				t.Errorf("expected IsConfigured()=%v, got %v", tt.configured, result)
			}
		})
	}
}

// Tests for container configuration

func TestContainerConfig(t *testing.T) {
	content := `
version: 2
containers:
  postgres:
    image: postgres:15
    build:
      context: ./docker
      dockerfile: Dockerfile.postgres
    env:
      POSTGRES_PASSWORD: test
      POSTGRES_DB: testdb
    ports:
      - "5432/tcp"
    volumes:
      - "./data:/var/lib/postgresql/data"
    depends_on:
      - redis
    wait_for:
      type: log
      target: "ready to accept connections"
      timeout: 30s
    reset:
      strategy: truncate
      tables:
        - users
        - orders
      exclude:
        - migrations
  redis:
    image: redis:7
    ports:
      - "6379/tcp"
    wait_for:
      type: port
      target: "6379/tcp"
`
	path := createTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check postgres container
	pg := cfg.Containers["postgres"]
	if pg.Image != "postgres:15" {
		t.Errorf("expected image postgres:15, got %s", pg.Image)
	}
	if pg.Build == nil {
		t.Error("expected build config")
	} else {
		if pg.Build.Context != "./docker" {
			t.Errorf("expected build context ./docker, got %s", pg.Build.Context)
		}
		if pg.Build.Dockerfile != "Dockerfile.postgres" {
			t.Errorf("expected dockerfile Dockerfile.postgres, got %s", pg.Build.Dockerfile)
		}
	}
	if pg.Env["POSTGRES_PASSWORD"] != "test" {
		t.Errorf("expected env POSTGRES_PASSWORD=test, got %s", pg.Env["POSTGRES_PASSWORD"])
	}
	if len(pg.Ports) != 1 || pg.Ports[0] != "5432/tcp" {
		t.Errorf("expected ports [5432/tcp], got %v", pg.Ports)
	}
	if len(pg.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(pg.Volumes))
	}
	if len(pg.DependsOn) != 1 || pg.DependsOn[0] != "redis" {
		t.Errorf("expected depends_on [redis], got %v", pg.DependsOn)
	}
	if pg.WaitFor.Type != "log" {
		t.Errorf("expected wait_for type log, got %s", pg.WaitFor.Type)
	}
	if pg.WaitFor.Timeout != 30*time.Second {
		t.Errorf("expected wait_for timeout 30s, got %v", pg.WaitFor.Timeout)
	}
	if pg.Reset.Strategy != "truncate" {
		t.Errorf("expected reset strategy truncate, got %s", pg.Reset.Strategy)
	}
	if len(pg.Reset.Tables) != 2 {
		t.Errorf("expected 2 reset tables, got %d", len(pg.Reset.Tables))
	}
	if len(pg.Reset.Exclude) != 1 {
		t.Errorf("expected 1 exclude table, got %d", len(pg.Reset.Exclude))
	}
}

// Tests for resource configuration

func TestResourceConfig(t *testing.T) {
	content := `
version: 2
containers:
  postgres:
    image: postgres:15
  rabbitmq:
    image: rabbitmq:3
resources:
  db:
    type: postgres
    container: postgres
    database: testdb
    reset: false
    options:
      tables:
        - users
        - orders
  api:
    type: http
    base_url: http://localhost:8080
  queue:
    type: rabbitmq
    container: rabbitmq
    options:
      queue: events
  kafka:
    type: kafka
    brokers:
      - localhost:9092
    consumer_group: test-group
  ws:
    type: websocket
    url: ws://localhost:8080/ws
`
	path := createTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check postgres resource
	db := cfg.Resources["db"]
	if db.Type != "postgres" {
		t.Errorf("expected type postgres, got %s", db.Type)
	}
	if db.Container != "postgres" {
		t.Errorf("expected container postgres, got %s", db.Container)
	}
	if db.Database != "testdb" {
		t.Errorf("expected database testdb, got %s", db.Database)
	}
	if db.Reset == nil || *db.Reset != false {
		t.Errorf("expected reset false, got %v", db.Reset)
	}

	// Check http resource
	api := cfg.Resources["api"]
	if api.Type != "http" {
		t.Errorf("expected type http, got %s", api.Type)
	}
	if api.BaseURL != "http://localhost:8080" {
		t.Errorf("expected base_url http://localhost:8080, got %s", api.BaseURL)
	}

	// Check kafka resource
	kafka := cfg.Resources["kafka"]
	if len(kafka.Brokers) != 1 {
		t.Errorf("expected 1 broker, got %d", len(kafka.Brokers))
	}
	if kafka.ConsumerGroup != "test-group" {
		t.Errorf("expected consumer_group test-group, got %s", kafka.ConsumerGroup)
	}

	// Check websocket resource
	ws := cfg.Resources["ws"]
	if ws.URL != "ws://localhost:8080/ws" {
		t.Errorf("expected url ws://localhost:8080/ws, got %s", ws.URL)
	}
}

// Tests for wait strategy configuration

func TestWaitStrategyConfig(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, WaitStrategy)
	}{
		{
			name: "port wait strategy",
			content: `
version: 2
containers:
  app:
    image: myapp
    wait_for:
      type: port
      target: "8080/tcp"
      timeout: 60s
`,
			validate: func(t *testing.T, ws WaitStrategy) {
				if ws.Type != "port" {
					t.Errorf("expected type port, got %s", ws.Type)
				}
				if ws.Target != "8080/tcp" {
					t.Errorf("expected target 8080/tcp, got %s", ws.Target)
				}
				if ws.Timeout != 60*time.Second {
					t.Errorf("expected timeout 60s, got %v", ws.Timeout)
				}
			},
		},
		{
			name: "log wait strategy",
			content: `
version: 2
containers:
  app:
    image: myapp
    wait_for:
      type: log
      target: "Server started"
`,
			validate: func(t *testing.T, ws WaitStrategy) {
				if ws.Type != "log" {
					t.Errorf("expected type log, got %s", ws.Type)
				}
				if ws.Target != "Server started" {
					t.Errorf("expected target 'Server started', got %s", ws.Target)
				}
			},
		},
		{
			name: "http wait strategy",
			content: `
version: 2
containers:
  app:
    image: myapp
    wait_for:
      type: http
      target: "8080/tcp"
      path: /health
      method: GET
`,
			validate: func(t *testing.T, ws WaitStrategy) {
				if ws.Type != "http" {
					t.Errorf("expected type http, got %s", ws.Type)
				}
				if ws.Path != "/health" {
					t.Errorf("expected path /health, got %s", ws.Path)
				}
				if ws.Method != "GET" {
					t.Errorf("expected method GET, got %s", ws.Method)
				}
			},
		},
		{
			name: "exec wait strategy",
			content: `
version: 2
containers:
  app:
    image: myapp
    wait_for:
      type: exec
      target: "pg_isready -U postgres"
`,
			validate: func(t *testing.T, ws WaitStrategy) {
				if ws.Type != "exec" {
					t.Errorf("expected type exec, got %s", ws.Type)
				}
				if ws.Target != "pg_isready -U postgres" {
					t.Errorf("expected target 'pg_isready -U postgres', got %s", ws.Target)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTempConfig(t, tt.content)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, cfg.Containers["app"].WaitFor)
		})
	}
}

// Tests for ready check configuration

func TestReadyCheckConfig(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *ReadyCheck)
	}{
		{
			name: "http ready check",
			content: `
version: 2
app:
  command: ./app
  ready:
    type: http
    path: /health
    status: 200
    timeout: 30s
`,
			validate: func(t *testing.T, rc *ReadyCheck) {
				if rc.Type != "http" {
					t.Errorf("expected type http, got %s", rc.Type)
				}
				if rc.Path != "/health" {
					t.Errorf("expected path /health, got %s", rc.Path)
				}
				if rc.Status != 200 {
					t.Errorf("expected status 200, got %d", rc.Status)
				}
				if rc.Timeout != 30*time.Second {
					t.Errorf("expected timeout 30s, got %v", rc.Timeout)
				}
			},
		},
		{
			name: "exec ready check",
			content: `
version: 2
app:
  command: ./app
  ready:
    type: exec
    command: "curl -f http://localhost:8080/health"
`,
			validate: func(t *testing.T, rc *ReadyCheck) {
				if rc.Type != "exec" {
					t.Errorf("expected type exec, got %s", rc.Type)
				}
				if rc.Command != "curl -f http://localhost:8080/health" {
					t.Errorf("expected command, got %s", rc.Command)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTempConfig(t, tt.content)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.App.Ready == nil {
				t.Fatal("expected ready check to be set")
			}
			tt.validate(t, cfg.App.Ready)
		})
	}
}

// Helper function

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
