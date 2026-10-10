package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the tomato.yml configuration
type Config struct {
	Version int `yaml:"version"`
	// VersionDeclared is false when the file omits `version` and it was
	// defaulted; `tomato validate` warns about that.
	VersionDeclared bool                 `yaml:"-"`
	Settings        Settings             `yaml:"settings"`
	App             AppConfig            `yaml:"app"`
	Containers      map[string]Container `yaml:"containers"`
	Resources       map[string]Resource  `yaml:"resources"`
	Hooks           Hooks                `yaml:"hooks"`
	Features        Features             `yaml:"features"`
}

// AppConfig defines how to run the application under test
type AppConfig struct {
	// Command mode (default): run app as local process
	Command string `yaml:"command,omitempty"`
	WorkDir string `yaml:"workdir,omitempty"`

	// Container mode: use pre-built Docker image
	Image string `yaml:"image,omitempty"`
	// Container mode: build from Dockerfile
	Build *AppBuild `yaml:"build,omitempty"`

	// Container name/alias for DNS resolution (default: "app")
	Name string `yaml:"name,omitempty"`
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

// IsConfigured returns true if the app section has any configuration
func (a *AppConfig) IsConfigured() bool {
	return a.Command != "" || a.Image != "" || a.Build != nil
}

// UseContainer returns true if the app should run in a container (image or build specified)
func (a *AppConfig) UseContainer() bool {
	return a.Image != "" || a.Build != nil
}

// GetName returns the container name/alias (defaults to "app")
func (a *AppConfig) GetName() string {
	if a.Name != "" {
		return a.Name
	}
	return "app"
}

type Settings struct {
	Timeout  time.Duration `yaml:"timeout"`
	Parallel int           `yaml:"parallel"`
	FailFast bool          `yaml:"fail_fast"`
	Output   string        `yaml:"output"`
}

type Container struct {
	// Preset expands the entry into a ready configuration (see presets.go);
	// the other fields then only override what the preset fills in.
	Preset string `yaml:"preset,omitempty"`
	// Auth selects how a preset authenticates clients (kafka: plaintext or
	// aws_msk_iam).
	Auth      string            `yaml:"auth,omitempty"`
	Image     string            `yaml:"image"`
	Build     *BuildConfig      `yaml:"build,omitempty"`
	Command   StringList        `yaml:"command,omitempty"`
	Env       map[string]string `yaml:"env"`
	Ports     []string          `yaml:"ports"`
	Volumes   []string          `yaml:"volumes"`
	DependsOn []string          `yaml:"depends_on"`
	WaitFor   WaitStrategy      `yaml:"wait_for"`
	Reset     ContainerReset    `yaml:"reset"`
	// Files are copied into the container after it is created and before it
	// starts. Presets fill them in; tomato.yml cannot set them.
	Files []ContainerFile `yaml:"-"`
}

// ContainerFile is a file tomato puts into a container before it starts.
type ContainerFile struct {
	Path    string // absolute path in the container
	Content []byte
	Mode    int64
}

// StringList accepts either a YAML sequence or a single scalar string, so both
// shell form (command: redis-server --appendonly yes) and exec form
// (command: ["server", "/data"]) work.
type StringList []string

func (s *StringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var str string
		if err := value.Decode(&str); err != nil {
			return err
		}
		*s = strings.Fields(str)
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*s = list
		return nil
	default:
		return fmt.Errorf("line %d: expected a string or a list of strings", value.Line)
	}
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
	Strategy string `yaml:"strategy"`
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
	// AllowDestructiveReset acknowledges that this resource is not one tomato
	// starts, and that tomato will still wipe it before every scenario. It is
	// required on a stateful resource with no container, so that pointing a
	// suite at a real database is a deliberate act rather than a typo.
	AllowDestructiveReset bool `yaml:"allow_destructive_reset,omitempty"`
	// Database specific
	Database string `yaml:"database,omitempty"`
	// HTTP specific
	BaseURL string `yaml:"base_url,omitempty"`
	// Queue/Kafka specific
	Brokers       []string `yaml:"brokers,omitempty"`
	ConsumerGroup string   `yaml:"consumer_group,omitempty"`
	// WebSocket specific
	URL string `yaml:"url,omitempty"`
	// gRPC specific — a dial target (host:port), not a URL. Omit it to dial
	// a managed container instead.
	Address string `yaml:"address,omitempty"`
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
	Paths    []string `yaml:"paths"`
	Tags     string   `yaml:"tags"`
	Scenario string   `yaml:"-"` // CLI only, not from config file
}

// destructiveTypes lose data when tomato resets them: it truncates tables,
// flushes keys, recreates topics, purges buckets. The other types only hold
// state in tomato's own memory.
var destructiveTypes = map[string]bool{
	"postgres": true, "postgresql": true,
	"cassandra": true, "scylladb": true,
	"redis": true, "kafka": true, "rabbitmq": true,
	"s3": true, "minio": true,
}

// IsDestructive reports whether resetting this resource destroys data.
func (r Resource) IsDestructive() bool {
	return destructiveTypes[strings.ToLower(r.Type)]
}

// Unmanaged reports whether the resource points at something tomato did not
// start. tomato resets what it is given either way, so an unmanaged stateful
// resource is a real system about to be wiped.
func (r Resource) Unmanaged() bool {
	return r.Container == ""
}

// DangerousResources are the stateful resources tomato does not own. Every
// one of them is wiped before every scenario.
func (c *Config) DangerousResources() []string {
	var out []string
	for name, res := range c.Resources {
		if res.IsDestructive() && res.Unmanaged() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Load reads and parses the tomato.yml configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	if err := checkSchemaVersion(data); err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.expandPresets(); err != nil {
		return nil, fmt.Errorf("expanding presets: %w", err)
	}
	cfg.resolvePaths(filepath.Dir(path))

	// Apply defaults
	cfg.applyDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// SupportedVersion is the only tomato.yml schema version this release reads.
const SupportedVersion = 2

// migrationDocsURL explains the v1 → v2 differences; versioningDocsURL explains
// how schema versions relate to tomato releases.
const (
	migrationDocsURL  = "https://tomatool.github.io/tomato/stability/#migrating-from-v1"
	versioningDocsURL = "https://tomatool.github.io/tomato/stability/#versioning"
)

// checkSchemaVersion rejects configs written for another schema before they are
// decoded into the v2 structs. Decoding a v1 file directly fails deep inside
// the YAML decoder ("cannot unmarshal !!seq into map[string]config.Resource"),
// which tells the user nothing about what is actually wrong.
func checkSchemaVersion(data []byte) error {
	var probe struct {
		Version   *int      `yaml:"version"`
		Resources yaml.Node `yaml:"resources"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if probe.Version != nil && *probe.Version != SupportedVersion {
		docs := versioningDocsURL
		if *probe.Version < SupportedVersion {
			docs = migrationDocsURL
		}
		return fmt.Errorf("unsupported config version: %d (this tomato reads version %d); see %s",
			*probe.Version, SupportedVersion, docs)
	}

	// v1 had no version field and declared resources as a list of
	// {name, type, options}; v2 uses a map keyed by resource name.
	if probe.Version == nil && probe.Resources.Kind == yaml.SequenceNode {
		return fmt.Errorf("this looks like a tomato v1 config (resources is a list); "+
			"tomato v2 needs `version: 2` and resources keyed by name, see %s", migrationDocsURL)
	}

	return nil
}

func (c *Config) applyDefaults() {
	c.VersionDeclared = c.Version != 0
	if c.Version == 0 {
		c.Version = SupportedVersion
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
	if len(c.Features.Paths) == 0 {
		c.Features.Paths = []string{"./features"}
	}
}

func (c *Config) validate() error {
	if c.Version != SupportedVersion {
		return fmt.Errorf("unsupported config version: %d (expected %d)", c.Version, SupportedVersion)
	}

	// Validate app config - only one mode allowed
	modes := 0
	if c.App.Command != "" {
		modes++
	}
	if c.App.Image != "" {
		modes++
	}
	if c.App.Build != nil {
		modes++
	}
	if modes > 1 {
		return fmt.Errorf("app config can only have one of: 'command', 'image', or 'build'")
	}

	// A stateful resource with no container is a system tomato did not start,
	// and tomato wipes what it is given before every scenario. Make that a
	// decision someone wrote down rather than a typo in a host name.
	for _, name := range c.DangerousResources() {
		if !c.Resources[name].AllowDestructiveReset {
			return fmt.Errorf(
				"resource %q (%s) has no container, so it points at a system tomato does not manage \u2014 "+
					"and tomato wipes every resource before every scenario, which would destroy its data. "+
					"Give it `container:` to let tomato run it, or set `allow_destructive_reset: true` on it "+
					"to confirm that wiping it is intended",
				name, c.Resources[name].Type)
		}
	}

	// Validate resource references
	for name, res := range c.Resources {
		if res.Container != "" {
			// Check if it references a configured container
			if _, ok := c.Containers[res.Container]; !ok {
				// Also allow referencing the app container by its name
				if !c.App.IsConfigured() || res.Container != c.App.GetName() {
					return fmt.Errorf("resource %q references unknown container %q", name, res.Container)
				}
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
