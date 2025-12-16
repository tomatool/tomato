package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the tomato.yml configuration
type Config struct {
	Version    int                  `yaml:"version"`
	Settings   Settings             `yaml:"settings"`
	App        AppConfig            `yaml:"app"`
	Containers map[string]Container `yaml:"containers"`
	Resources  map[string]Resource  `yaml:"resources"`
	Hooks      Hooks                `yaml:"hooks"`
	Features   Features             `yaml:"features"`
}

// AppConfig defines how to run the application under test
type AppConfig struct {
	// Build from Dockerfile
	Build *AppBuild `yaml:"build,omitempty"`
	// Or run with command
	Command string `yaml:"command,omitempty"`
	// Working directory for command
	WorkDir string `yaml:"workdir,omitempty"`
	// Port the app listens on
	Port int `yaml:"port,omitempty"`
	// Health check to verify app is ready
	Ready *ReadyCheck `yaml:"ready,omitempty"`
	// Time to wait after ready check passes (for app to fully stabilize)
	Wait time.Duration `yaml:"wait,omitempty"`
	// Environment variables (supports {{.container.host}}, {{.container.port}} templates)
	Env map[string]string `yaml:"env,omitempty"`
}

type AppBuild struct {
	Dockerfile string `yaml:"dockerfile"`
	Context    string `yaml:"context,omitempty"`
}

type ReadyCheck struct {
	// Type: http, tcp, exec
	Type string `yaml:"type"`
	// For HTTP: endpoint path
	Path string `yaml:"path,omitempty"`
	// For HTTP: expected status (default 200)
	Status int `yaml:"status,omitempty"`
	// Timeout for ready check
	Timeout time.Duration `yaml:"timeout,omitempty"`
	// For exec: command to run
	Command string `yaml:"command,omitempty"`
}

// IsConfigured returns true if the app section has build or command configured
func (a *AppConfig) IsConfigured() bool {
	return a.Build != nil || a.Command != ""
}

type Settings struct {
	Timeout  time.Duration `yaml:"timeout"`
	Parallel int           `yaml:"parallel"`
	FailFast bool          `yaml:"fail_fast"`
	Output   string        `yaml:"output"`
	Reset    ResetSettings `yaml:"reset"`
}

type ResetSettings struct {
	Level     string `yaml:"level"`      // scenario, feature, run, none
	OnFailure string `yaml:"on_failure"` // keep, reset
}

type Container struct {
	Image     string            `yaml:"image"`
	Build     *BuildConfig      `yaml:"build,omitempty"`
	Env       map[string]string `yaml:"env"`
	Ports     []string          `yaml:"ports"`
	Volumes   []string          `yaml:"volumes"`
	DependsOn []string          `yaml:"depends_on"`
	WaitFor   WaitStrategy      `yaml:"wait_for"`
	Reset     ContainerReset    `yaml:"reset"`
}

type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

type WaitStrategy struct {
	// Can be: port(8080), log("ready"), http("GET", "/health", 8080), exec("cmd")
	Type   string `yaml:"type"`
	Target string `yaml:"target"`
	// For HTTP
	Method string `yaml:"method,omitempty"`
	Path   string `yaml:"path,omitempty"`
	Port   int    `yaml:"port,omitempty"`
	// Timeout for wait strategy
	Timeout time.Duration `yaml:"timeout"`
}

type ContainerReset struct {
	Strategy string   `yaml:"strategy"`
	// Database specific
	Tables  []string `yaml:"tables,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
	// Queue specific
	Queues []string `yaml:"queues,omitempty"`
	// Kafka specific
	Topics      []string    `yaml:"topics,omitempty"`
	TopicConfig TopicConfig `yaml:"topic_config,omitempty"`
}

type TopicConfig struct {
	Partitions        int `yaml:"partitions"`
	ReplicationFactor int `yaml:"replication_factor"`
}

type Resource struct {
	Type      string         `yaml:"type"`
	Container string         `yaml:"container"`
	Options   map[string]any `yaml:"options"`
	// Reset configuration
	Reset *bool `yaml:"reset,omitempty"` // nil = use global setting, true = always reset, false = never reset
	// Database specific
	Database string `yaml:"database,omitempty"`
	// HTTP specific
	BaseURL string `yaml:"base_url,omitempty"`
	// Queue/Kafka specific
	Brokers       []string `yaml:"brokers,omitempty"`
	ConsumerGroup string   `yaml:"consumer_group,omitempty"`
	// WebSocket specific
	URL string `yaml:"url,omitempty"`
}

type Hooks struct {
	BeforeAll      []Hook `yaml:"before_all"`
	AfterAll       []Hook `yaml:"after_all"`
	BeforeScenario []Hook `yaml:"before_scenario"`
	AfterScenario  []Hook `yaml:"after_scenario"`
}

type Hook struct {
	SQL       string `yaml:"sql,omitempty"`
	SQLFile   string `yaml:"sql_file,omitempty"`
	Exec      string `yaml:"exec,omitempty"`
	Shell     string `yaml:"shell,omitempty"`
	Resource  string `yaml:"resource,omitempty"`
	Container string `yaml:"container,omitempty"`
}

type Features struct {
	Paths []string `yaml:"paths"`
	Tags  string   `yaml:"tags"`
}

// Load reads and parses the tomato.yml configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = 2
	}
	if c.Settings.Timeout == 0 {
		c.Settings.Timeout = 5 * time.Minute
	}
	if c.Settings.Parallel == 0 {
		c.Settings.Parallel = 1
	}
	if c.Settings.Output == "" {
		c.Settings.Output = "pretty"
	}
	if c.Settings.Reset.Level == "" {
		c.Settings.Reset.Level = "scenario"
	}
	if c.Settings.Reset.OnFailure == "" {
		c.Settings.Reset.OnFailure = "reset"
	}
	if len(c.Features.Paths) == 0 {
		c.Features.Paths = []string{"./features"}
	}
}

func (c *Config) validate() error {
	if c.Version != 2 {
		return fmt.Errorf("unsupported config version: %d (expected 2)", c.Version)
	}

	// Validate reset level
	validLevels := map[string]bool{"scenario": true, "feature": true, "run": true, "none": true}
	if !validLevels[c.Settings.Reset.Level] {
		return fmt.Errorf("invalid reset level: %s", c.Settings.Reset.Level)
	}

	// Validate resource references
	for name, res := range c.Resources {
		if res.Container != "" {
			if _, ok := c.Containers[res.Container]; !ok {
				return fmt.Errorf("resource %q references unknown container %q", name, res.Container)
			}
		}
	}

	// Validate container dependencies
	for name, cont := range c.Containers {
		for _, dep := range cont.DependsOn {
			if _, ok := c.Containers[dep]; !ok {
				return fmt.Errorf("container %q depends on unknown container %q", name, dep)
			}
		}
	}

	return nil
}
