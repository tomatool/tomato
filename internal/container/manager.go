package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/runlog"
)

// The Manager itself: what a run owns, and how the containers it creates are
// named and labelled so a later run can recognise them.

// Manager handles the lifecycle of test containers
type Manager struct {
	configs     map[string]config.Container
	containers  map[string]testcontainers.Container
	order       []string // startup order based on dependencies
	mu          sync.RWMutex
	runCtx      *runlog.RunContext
	logFiles    map[string]*os.File
	network     *testcontainers.DockerNetwork
	networkName string

	// scope distinguishes one project's containers from another's on a
	// shared Docker host; runID distinguishes one run from the next.
	scope        string
	runID        string
	settings     config.ContainerSettings
	networkReady bool
}

// NewManager creates a new container manager.
func NewManager(configs map[string]config.Container) (*Manager, error) {
	return NewManagerFor(configs, "", config.ContainerSettings{})
}

// NewManagerFor creates a manager scoped to a config file. The scope goes into
// every container and network name, so two projects on one Docker host never
// collide and anyone reading `docker ps` can tell whose containers these are.
func NewManagerFor(configs map[string]config.Container, configPath string, settings config.ContainerSettings) (*Manager, error) {
	m := &Manager{
		configs:    configs,
		containers: make(map[string]testcontainers.Container),
		logFiles:   make(map[string]*os.File),
		scope:      scopeOf(configPath),
		settings:   settings,
	}
	m.networkName = m.nameFor("net")

	// testcontainers' reaper kills everything this session labelled once the
	// process exits, which would defeat reuse before the next run starts.
	if settings.Reuse {
		if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
			return nil, fmt.Errorf("disabling the testcontainers reaper for reuse: %w", err)
		}
	}

	// Calculate startup order based on dependencies
	order, err := m.calculateStartOrder()
	if err != nil {
		return nil, fmt.Errorf("calculating start order: %w", err)
	}
	m.order = order

	return m, nil
}

// scopeOf is a short stable id for a project: the config file's absolute path
// hashed, so the same project reuses one scope and two projects never share.
func scopeOf(configPath string) string {
	if configPath == "" {
		return uuid.New().String()[:8]
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		abs = configPath
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])[:8]
}

// nameFor builds the Docker name for one of this run's objects.
//
// Without reuse the run id is in the name, so every run gets its own
// containers and two runs can go at once. With reuse it is left out, so a
// rerun finds what the last run left behind — which is the whole point, and
// also why reuse means a project can only have one run at a time.
func (m *Manager) nameFor(name string) string {
	if m.settings.Reuse || m.runID == "" {
		return fmt.Sprintf("tomato-%s-%s", m.scope, name)
	}
	return fmt.Sprintf("tomato-%s-%s-%s", m.scope, m.runID, name)
}

// labels mark everything tomato creates, so leftovers can be found and
// cleaned up by anything that speaks Docker, not just tomato.
func (m *Manager) labels(role string) map[string]string {
	l := map[string]string{
		"tomato.managed": "true",
		"tomato.scope":   m.scope,
		"tomato.role":    role,
	}
	if m.runID != "" {
		l["tomato.run"] = m.runID
	}
	if m.settings.Reuse {
		l["tomato.reuse"] = "true"
	}
	return l
}

// SetRunContext sets the run context for logging. It is called before the
// containers start, so the run id reaches their names.
func (m *Manager) SetRunContext(ctx *runlog.RunContext) {
	m.runCtx = ctx
	if ctx != nil {
		m.runID = ctx.ID
		m.networkName = m.nameFor("net")
	}
}

// RegisterContainer adds an externally managed container to the manager
// This allows resources to reference containers started by apprunner
func (m *Manager) RegisterContainer(name string, container testcontainers.Container) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.containers[name] = container
}
