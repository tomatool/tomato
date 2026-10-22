package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	Timeout    time.Duration     `yaml:"timeout"`
	Parallel   int               `yaml:"parallel"`
	FailFast   bool              `yaml:"fail_fast"`
	Output     string            `yaml:"output"`
	Containers ContainerSettings `yaml:"containers"`
}

// ContainerSettings tunes how tomato runs the containers it starts.
type ContainerSettings struct {
	// Reuse keeps containers between runs instead of starting fresh ones.
	// The names then leave out the run id, so a rerun finds the containers
	// the last run left behind. Resets still run before every scenario, so
	// scenarios stay isolated; what reuse costs is that a container an
	// earlier run corrupted is no longer thrown away, and two runs of the
	// same project can no longer run at the same time.
	Reuse bool `yaml:"reuse"`
}

type Container struct {
	// Preset expands the entry into a ready configuration (see presets.go);
	// the other fields then only override what the preset fills in.
	Preset string `yaml:"preset,omitempty"`
	// Auth selects how a preset authenticates clients (kafka: plaintext or
	// aws_msk_iam).
	Auth string `yaml:"auth,omitempty"`
	// Cluster runs the redis preset in cluster mode: one node owning every
	// slot, announcing the host port clients reach it on.
	Cluster bool `yaml:"cluster,omitempty"`
	// TLS serves the redis preset over TLS only, with a certificate from a CA
	// made for the run (see CAFile).
	TLS       bool              `yaml:"tls,omitempty"`
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
	// Entrypoint replaces the image's entrypoint; command becomes its
	// arguments. Presets fill it in; tomato.yml cannot set it.
	Entrypoint []string `yaml:"-"`
	// HostAccessPorts are ports on the host the container reaches as
	// host.testcontainers.internal:<port>, through a tunnel that works with
	// any Docker, local or remote. Presets fill them in; tomato.yml cannot
	// set them.
	HostAccessPorts []int `yaml:"-"`
	// CA is the PEM of the CA a preset made for the run, which tomato writes
	// to CAFile on the host when the container starts: what
	// {{.<name>.ca_file}} names for the app, and what tomato's own resources
	// trust. Presets fill them in; tomato.yml cannot set them.
	CA     []byte `yaml:"-"`
	CAFile string `yaml:"-"`
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
	if err := checkRemovedKeys(data); err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, explainYAMLError(data, err)
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
	resettingDocsURL  = "https://tomatool.github.io/tomato/configuration/#resetting-state"
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
		// This runs before the full decode, so it is where a syntax error
		// surfaces first — and where it has to be explained.
		return explainYAMLError(data, err)
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

// checkRemovedKeys refuses the reset opt-outs that v2.3.0 removed:
// `settings.reset`, a resource's `reset`, and `reset_strategy: none`.
//
// The loader does not decode strictly, so without this a `reset: false`
// written for v2.2.1 loaded without a word — and the resource it protected
// was wiped before every scenario. An upgrade may change what a config
// means, but not silently.
func checkRemovedKeys(data []byte) error {
	// Node values, not pointers: yaml.v3 will not decode a scalar such as
	// `reset: false` into a *yaml.Node inside a map of structs. A key that
	// is absent leaves the Node's Kind zero.
	var probe struct {
		Settings struct {
			Reset yaml.Node `yaml:"reset"`
		} `yaml:"settings"`
		Resources map[string]struct {
			Reset   yaml.Node `yaml:"reset"`
			Options struct {
				ResetStrategy string `yaml:"reset_strategy"`
			} `yaml:"options"`
		} `yaml:"resources"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		// Not this probe's job: the full decode reports it, with its line.
		return nil
	}

	const why = "every resource is reset before every scenario, and since v2.3.0 there is no way " +
		"to turn that off; a scenario that needs a starting state builds it in a Background. " +
		"Remove the key, see " + resettingDocsURL

	if probe.Settings.Reset.Kind != 0 {
		return fmt.Errorf("settings.reset was removed: %s", why)
	}

	names := make([]string, 0, len(probe.Resources))
	for name := range probe.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := probe.Resources[name]
		if r.Reset.Kind != 0 {
			return fmt.Errorf("resource %q: `reset` was removed: %s", name, why)
		}
		if r.Options.ResetStrategy == "none" {
			return fmt.Errorf("resource %q: `reset_strategy: none` was removed: %s", name, why)
		}
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

	// Scenarios run one at a time. tomato resets every resource before every
	// scenario, and the resources are shared — so with two scenarios in
	// flight, one scenario's reset truncates the tables the other is halfway
	// through asserting on. Isolation is the guarantee tomato exists to make,
	// and it cannot make it concurrently against one database.
	if c.Settings.Parallel > 1 {
		return fmt.Errorf(
			"settings.parallel is %d, but scenarios have to run one at a time: "+
				"every scenario resets every resource, and the resources are shared, "+
				"so a concurrent scenario's reset would wipe state another scenario "+
				"is still using. Remove the setting, or set it to 1",
			c.Settings.Parallel)
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
		if res.Type == "network" && res.Container == "" {
			return fmt.Errorf("resource %q (network) needs container: the dependency the app reaches through it", name)
		}
		if res.Type == "network" && res.Container != "" && c.App.IsConfigured() && res.Container == c.App.GetName() {
			return fmt.Errorf("resource %q (network) stands between the app and a dependency; container %q is the app", name, res.Container)
		}
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

var (
	yamlErrLine = regexp.MustCompile(`line (\d+):`)
	// A mapping entry whose value opens with a Go template. Specifically
	// "{{", not any "{": `resources: {}` and `env: {A: b}` are valid YAML
	// flow mappings, and matching those turned an unrelated indentation
	// error into confidently wrong advice.
	bareTemplate = regexp.MustCompile(`^(\s*)([A-Za-z0-9_.-]+):\s+(\{\{.*)$`)
)

// explainYAMLError turns go-yaml's report into one someone can act on.
//
// "yaml: line 8: did not find expected key" names neither what is wrong nor
// where. In a tomato.yml the cause is usually the same: a value starting with
// "{{" that is not quoted. YAML reads "{" as the start of a flow mapping, so
//
//	KAFKA_BROKERS: {{.kafka.host}}:{{.kafka.port.9092}}
//
// is a syntax error, while DATABASE_URL: postgres://{{...}} is fine because it
// starts with a letter. A tomato.yml is full of these templates, so it is
// worth naming rather than leaving to the reader.
//
// YAML reports where the mapping began, not where it broke, so the search for
// the offending line starts at the reported line and runs forward.
func explainYAMLError(data []byte, err error) error {
	lines := strings.Split(string(data), "\n")

	from := 0
	if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n >= 1 && n <= len(lines) {
			from = n - 1
		}
	}

	for i := from; i < len(lines) && i < from+40; i++ {
		m := bareTemplate.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		indent, key, value := m[1], m[2], strings.TrimSpace(m[3])
		return fmt.Errorf(
			"parsing config file: %v\n\n  %d | %s\n\n"+
				"A value starting with \"{\" is read as the start of a YAML mapping, so a bare\n"+
				"template is a syntax error. Quote it:\n\n  %s%s: %q",
			err, i+1, lines[i], indent, key, value)
	}

	if from > 0 {
		return fmt.Errorf("parsing config file: %v\n\n  %d | %s", err, from+1, lines[from])
	}
	return fmt.Errorf("parsing config file: %w", err)
}
