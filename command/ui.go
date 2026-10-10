package command

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"

	gherkin "github.com/cucumber/gherkin/go/v26"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/runlog"
	"github.com/urfave/cli/v2"
)

// The UI is a Vite project under ui/; `pnpm build` writes the bundle here and
// this bakes it into the binary. The bundle is committed so `go build` works
// without Node — `make ui` (or CI) regenerates it when the sources change.
//
//go:embed all:ui_assets/dist
var uiAssets embed.FS

var uiCommand = &cli.Command{
	Name:  "ui",
	Usage: "Interactive web UI to browse and visualize test cases",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "tomato.yml",
			Usage:   "Path to configuration file",
		},
		&cli.StringSliceFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "Path to feature files (can be specified multiple times)",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 0,
			Usage: "Port to run the UI server (default: random available port)",
		},
		&cli.BoolFlag{
			Name:  "no-browser",
			Usage: "Don't open the browser automatically",
		},
	},
	Action: runWebUI,
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// UIServer handles the web UI
type UIServer struct {
	featurePaths []string
	clients      map[*websocket.Conn]bool
	clientsMux   sync.RWMutex
	watcher      *fsnotify.Watcher
	configPath   string
	isRunning    bool
	runningMux   sync.Mutex
	runningCmd   *exec.Cmd
}

// Feature data structures for JSON
type FeatureJSON struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	FilePath    string         `json:"filePath"`
	Background  []StepJSON     `json:"background,omitempty"`
	Scenarios   []ScenarioJSON `json:"scenarios"`
}

type ScenarioJSON struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Steps       []StepJSON    `json:"steps"`
	IsOutline   bool          `json:"isOutline,omitempty"`
	Examples    []ExampleJSON `json:"examples,omitempty"`
}

type StepJSON struct {
	Keyword   string     `json:"keyword"`
	Text      string     `json:"text"`
	DocString string     `json:"docString,omitempty"`
	DocLang   string     `json:"docStringLang,omitempty"`
	Table     [][]string `json:"table,omitempty"`
	// Phase is given, when or then; And and But take the one before them.
	Phase string `json:"phase,omitempty"`
	// The resource and step definition the step runs, when tomato.yml was
	// read. Unmatched is set when it was read and no step definition matches.
	Resource     string `json:"resource,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	Group        string `json:"group,omitempty"`
	Description  string `json:"description,omitempty"`
	Unmatched    bool   `json:"unmatched,omitempty"`
}

type ExampleJSON struct {
	Name string     `json:"name,omitempty"`
	Tags []string   `json:"tags,omitempty"`
	Rows [][]string `json:"rows"`
}

type WSMessage struct {
	Type         string        `json:"type"`
	Features     []FeatureJSON `json:"features,omitempty"`
	ChangedFiles []string      `json:"changedFiles,omitempty"`
	Error        string        `json:"error,omitempty"`
	// Topology is the app and the resources tomato.yml defines, for the flow
	// view; nil when tomato.yml can't be read.
	Topology *TopologyJSON `json:"topology,omitempty"`
	// Run status fields
	Scenario   string `json:"scenario,omitempty"`
	Status     string `json:"status,omitempty"` // "running", "passed", "failed"
	Output     string `json:"output,omitempty"`
	StepIndex  *int   `json:"stepIndex,omitempty"`
	DurationMs *int64 `json:"durationMs,omitempty"`
	// Debug logs fields
	Runs  []runlog.RunInfo `json:"runs,omitempty"`
	RunID string           `json:"runId,omitempty"`
}

// TomatoEvent represents a structured event from the tomato formatter
type TomatoEvent struct {
	Type       string `json:"type"`
	Feature    string `json:"feature,omitempty"`
	Scenario   string `json:"scenario,omitempty"`
	Step       string `json:"step,omitempty"`
	Status     string `json:"status,omitempty"`
	Error      string `json:"error,omitempty"`
	File       string `json:"file,omitempty"`
	StepIndex  *int   `json:"stepIndex,omitempty"`
	DurationMs *int64 `json:"durationMs,omitempty"`
	Total      int    `json:"total,omitempty"`
	Passed     int    `json:"passed,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	Skipped    int    `json:"skipped,omitempty"`
}

func runWebUI(c *cli.Context) error {
	var featurePaths []string

	// Get paths from flags or config
	if paths := c.StringSlice("path"); len(paths) > 0 {
		featurePaths = paths
	} else {
		cfg, err := config.Load(c.String("config"))
		if err != nil {
			featurePaths = []string{"./features"}
		} else {
			featurePaths = cfg.Features.Paths
			if len(featurePaths) == 0 {
				featurePaths = []string{"./features"}
			}
		}
	}

	// Create server
	server := &UIServer{
		featurePaths: featurePaths,
		clients:      make(map[*websocket.Conn]bool),
		configPath:   c.String("config"),
	}

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()
	server.watcher = watcher

	// Watch feature directories
	for _, path := range featurePaths {
		if err := server.watchPath(path); err != nil {
			fmt.Printf("Warning: couldn't watch %s: %v\n", path, err)
		}
	}

	// Start watching for changes
	go server.watchLoop()

	// Setup HTTP handlers
	mux := http.NewServeMux()

	// Serve embedded assets
	assetsFS, err := fs.Sub(uiAssets, "ui_assets/dist")
	if err != nil {
		return fmt.Errorf("failed to setup assets: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(assetsFS)))

	// API endpoints
	mux.HandleFunc("/api/features", server.handleFeatures)
	mux.HandleFunc("/api/config", server.handleConfig)
	mux.HandleFunc("/api/run", server.handleRun)
	mux.HandleFunc("/api/stop", server.handleStop)
	mux.HandleFunc("/api/runs", server.handleRuns)
	mux.HandleFunc("/api/runs/", server.handleRunLog)
	mux.HandleFunc("/ws", server.handleWebSocket)

	// Find available port
	port := c.Int("port")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d", addr.Port)

	fmt.Printf("%s Tomato UI running at %s\n", successStyle.Render(""), url)
	fmt.Printf("%s Watching for changes in: %s\n", helpStyle.Render(""), strings.Join(featurePaths, ", "))
	fmt.Printf("%s Press Ctrl+C to stop\n\n", helpStyle.Render(""))

	// Open browser
	if !c.Bool("no-browser") {
		go openBrowser(url)
	}

	return http.Serve(listener, mux)
}

func (s *UIServer) watchPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return s.watcher.Add(p)
			}
			return nil
		})
	}

	return s.watcher.Add(filepath.Dir(path))
}

func (s *UIServer) watchLoop() {
	// Debounce timer to batch rapid changes
	var debounceTimer *time.Timer
	var debounceMux sync.Mutex
	changedFiles := make(map[string]bool)

	triggerUpdate := func(filePath string) {
		debounceMux.Lock()
		defer debounceMux.Unlock()

		changedFiles[filePath] = true

		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
			debounceMux.Lock()
			files := make([]string, 0, len(changedFiles))
			for f := range changedFiles {
				files = append(files, f)
			}
			changedFiles = make(map[string]bool)
			debounceMux.Unlock()

			s.broadcastUpdate(files)
		})
	}

	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Handle new directories - add them to watcher
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					s.watcher.Add(event.Name)
				}
			}

			// Handle feature file changes
			if strings.HasSuffix(event.Name, ".feature") {
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					triggerUpdate(event.Name)
				}
			}

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (s *UIServer) broadcastUpdate(changedFiles []string) {
	features, err := s.loadFeatures()

	msg := WSMessage{Type: "update", Topology: s.loadTopology()}
	if err != nil {
		msg.Type = "error"
		msg.Error = err.Error()
	} else {
		msg.Features = features
		msg.ChangedFiles = changedFiles
	}

	data, _ := json.Marshal(msg)

	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *UIServer) handleFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.loadFeatures()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
}

// ConfigJSON is what the config screen renders. It carries the raw file for
// the YAML tab plus a flattened view of the parts the structured tab shows, so
// the browser never has to understand tomato's config schema.
type ConfigJSON struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
	Valid    bool   `json:"valid"`
	Version  string `json:"version,omitempty"`
	Watching string `json:"watching,omitempty"`

	Settings   *SettingsJSON     `json:"settings,omitempty"`
	App        *AppJSON          `json:"app,omitempty"`
	Containers []ContainerJSON   `json:"containers,omitempty"`
	Resources  []CfgResourceJSON `json:"resources,omitempty"`
	Hooks      *HooksJSON        `json:"hooks,omitempty"`
	Features   *FeaturesJSON     `json:"features,omitempty"`
	HookCount  int               `json:"hookCount"`
}

type SettingsJSON struct {
	Timeout  string `json:"timeout,omitempty"`
	Parallel int    `json:"parallel,omitempty"`
	FailFast bool   `json:"failFast"`
	Output   string `json:"output,omitempty"`
}

type AppJSON struct {
	Configured bool         `json:"configured"`
	Name       string       `json:"name,omitempty"`
	Command    string       `json:"command,omitempty"`
	Image      string       `json:"image,omitempty"`
	Port       int          `json:"port,omitempty"`
	Ready      string       `json:"ready,omitempty"`
	Wait       string       `json:"wait,omitempty"`
	Env        []AppEnvJSON `json:"env,omitempty"`
}

// AppEnvJSON shows a template next to what it resolved to, which is the only
// place a reader can see why an app got the port it did.
type AppEnvJSON struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Resolved string `json:"resolved,omitempty"`
}

type ContainerJSON struct {
	Name    string   `json:"name"`
	Image   string   `json:"image,omitempty"`
	Preset  string   `json:"preset,omitempty"`
	Ports   []string `json:"ports,omitempty"`
	WaitFor string   `json:"waitFor,omitempty"`
}

type CfgResourceJSON struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	ConnectsTo string `json:"connectsTo,omitempty"`
	Options    string `json:"options,omitempty"`
}

type HooksJSON struct {
	BeforeAll      []string `json:"beforeAll,omitempty"`
	AfterAll       []string `json:"afterAll,omitempty"`
	BeforeScenario []string `json:"beforeScenario,omitempty"`
	AfterScenario  []string `json:"afterScenario,omitempty"`
}

type FeaturesJSON struct {
	Paths []string `json:"paths,omitempty"`
	Tags  string   `json:"tags,omitempty"`
}

func (s *UIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	out := ConfigJSON{
		Path:     s.configPath,
		Watching: strings.Join(s.featurePaths, ", "),
	}

	if raw, err := os.ReadFile(s.configPath); err == nil {
		out.Content = string(raw)
	} else {
		out.Error = err.Error()
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		if out.Error == "" {
			out.Error = err.Error()
		}
	} else {
		out.Valid = true
		out.Version = fmt.Sprintf("%d", cfg.Version)
		out.Settings = &SettingsJSON{
			Timeout:  durStr(cfg.Settings.Timeout),
			Parallel: cfg.Settings.Parallel,
			FailFast: cfg.Settings.FailFast,
			Output:   cfg.Settings.Output,
		}
		out.App = appJSON(&cfg.App)
		out.Containers = containersJSON(cfg.Containers)
		out.Resources = resourcesJSON(cfg.Resources)
		out.Hooks, out.HookCount = hooksJSON(cfg.Hooks)
		out.Features = &FeaturesJSON{Paths: cfg.Features.Paths, Tags: cfg.Features.Tags}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func durStr(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

func appJSON(a *config.AppConfig) *AppJSON {
	if !a.IsConfigured() {
		return &AppJSON{Configured: false}
	}
	out := &AppJSON{
		Configured: true,
		Name:       a.GetName(),
		Command:    a.Command,
		Image:      a.Image,
		Port:       a.Port,
		Wait:       durStr(a.Wait),
	}
	if a.Build != nil && out.Image == "" {
		out.Image = "build: " + a.Build.Dockerfile
	}
	if a.Ready != nil {
		ready := a.Ready.Type
		if a.Ready.Path != "" {
			ready += " " + a.Ready.Path
		}
		if a.Ready.Status != 0 {
			ready += fmt.Sprintf(" → %d", a.Ready.Status)
		}
		if a.Ready.Command != "" {
			ready += " " + a.Ready.Command
		}
		if a.Ready.Timeout != 0 {
			ready += " · timeout " + a.Ready.Timeout.String()
		}
		out.Ready = ready
	}
	for _, k := range sortedKeys(a.Env) {
		out.Env = append(out.Env, AppEnvJSON{Key: k, Value: a.Env[k]})
	}
	return out
}

func containersJSON(cs map[string]config.Container) []ContainerJSON {
	out := make([]ContainerJSON, 0, len(cs))
	for _, name := range sortedKeys(cs) {
		c := cs[name]
		out = append(out, ContainerJSON{
			Name:    name,
			Image:   c.Image,
			Preset:  c.Preset,
			Ports:   c.Ports,
			WaitFor: waitStr(c.WaitFor),
		})
	}
	return out
}

func waitStr(w config.WaitStrategy) string {
	if w.Type == "" {
		return ""
	}
	out := w.Type
	switch {
	case w.Target != "":
		out += " " + w.Target
	case w.Path != "":
		out += " " + w.Path
	case w.Port != 0:
		out += fmt.Sprintf(" %d", w.Port)
	}
	return out
}

func resourcesJSON(rs map[string]config.Resource) []CfgResourceJSON {
	out := make([]CfgResourceJSON, 0, len(rs))
	for _, name := range sortedKeys(rs) {
		r := rs[name]
		out = append(out, CfgResourceJSON{
			Name:       name,
			Type:       r.Type,
			ConnectsTo: connectsTo(r),
			Options:    optionsStr(r),
		})
	}
	return out
}

func connectsTo(r config.Resource) string {
	switch {
	case r.BaseURL != "":
		return r.BaseURL
	case r.URL != "":
		return r.URL
	case r.Address != "":
		return r.Address
	case r.Container != "":
		return "container " + r.Container
	case len(r.Brokers) > 0:
		return strings.Join(r.Brokers, ", ")
	}
	return ""
}

func optionsStr(r config.Resource) string {
	var parts []string
	if r.Database != "" {
		parts = append(parts, "database "+r.Database)
	}
	if r.ConsumerGroup != "" {
		parts = append(parts, "group "+r.ConsumerGroup)
	}
	for _, k := range sortedKeys(r.Options) {
		parts = append(parts, fmt.Sprintf("%s %v", k, r.Options[k]))
	}
	return strings.Join(parts, " · ")
}

func hooksJSON(h config.Hooks) (*HooksJSON, int) {
	out := &HooksJSON{
		BeforeAll:      hookList(h.BeforeAll),
		AfterAll:       hookList(h.AfterAll),
		BeforeScenario: hookList(h.BeforeScenario),
		AfterScenario:  hookList(h.AfterScenario),
	}
	n := len(out.BeforeAll) + len(out.AfterAll) + len(out.BeforeScenario) + len(out.AfterScenario)
	return out, n
}

func hookList(hs []config.Hook) []string {
	var out []string
	for _, h := range hs {
		var s string
		switch {
		case h.SQLFile != "":
			s = "sql_file " + h.SQLFile
		case h.SQL != "":
			s = "sql " + h.SQL
		case h.Shell != "":
			s = "shell " + h.Shell
		case h.Exec != "":
			s = "exec " + h.Exec
		default:
			continue
		}
		if h.Resource != "" {
			s += " on " + h.Resource
		}
		if h.Container != "" {
			s += " in " + h.Container
		}
		out = append(out, s)
	}
	return out
}

// sortedKeys keeps the config screen stable between reloads; Go map order
// would otherwise reshuffle the rows on every fetch.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *UIServer) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runningMux.Lock()
	if s.isRunning {
		s.runningMux.Unlock()
		http.Error(w, "Tests already running", http.StatusConflict)
		return
	}
	s.isRunning = true
	s.runningMux.Unlock()

	// Get optional scenario filter from query
	scenario := r.URL.Query().Get("scenario")

	go s.runTests(scenario)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *UIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runningMux.Lock()
	if !s.isRunning || s.runningCmd == nil {
		s.runningMux.Unlock()
		http.Error(w, "No tests running", http.StatusConflict)
		return
	}
	cmd := s.runningCmd
	s.runningMux.Unlock()

	// Kill the process
	if cmd.Process != nil {
		cmd.Process.Kill()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *UIServer) handleRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := runlog.ListRuns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

func (s *UIServer) handleRunLog(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/runs/{runName}/logs/{logName}
	path := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	parts := strings.Split(path, "/logs/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	runName := parts[0]
	logName := parts[1]

	content, err := runlog.GetLogContent(runName, logName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(content))
}

func (s *UIServer) runTests(scenarioFilter string) {
	defer func() {
		s.runningMux.Lock()
		s.isRunning = false
		s.runningCmd = nil
		s.runningMux.Unlock()
	}()

	// Broadcast run started
	s.broadcastRunStatus("run_started", "", "", "")

	// Build command with quiet flag and tomato format for structured events
	args := []string{"run", "-c", s.configPath, "--quiet", "--format", "tomato"}
	if scenarioFilter != "" {
		args = append(args, "--scenario", scenarioFilter)
	}

	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		s.broadcastRunStatus("run_error", "", "failed", err.Error())
		return
	}

	cmd := exec.Command(execPath, args...)

	// Store the command for stop functionality
	s.runningMux.Lock()
	s.runningCmd = cmd
	s.runningMux.Unlock()

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.broadcastRunStatus("run_error", "", "failed", err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.broadcastRunStatus("run_error", "", "failed", err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		s.broadcastRunStatus("run_error", "", "failed", err.Error())
		return
	}

	// Channel to receive lines from both stdout and stderr
	lines := make(chan string, 100)
	var wg sync.WaitGroup

	// Read stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	// Read stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	// Close lines channel when both readers are done
	go func() {
		wg.Wait()
		close(lines)
	}()

	const tomatoEventPrefix = "TOMATO_EVENT:"
	featureResults := make(map[string]string) // feature -> passed/failed

	for line := range lines {
		cleanLine := stripAnsi(line)
		trimmedLine := strings.TrimSpace(cleanLine)

		// Detect run ID from output (format: "📋 run: <id>")
		if strings.Contains(cleanLine, "run:") {
			parts := strings.Split(cleanLine, "run:")
			if len(parts) > 1 {
				runID := strings.TrimSpace(parts[1])
				if len(runID) == 8 { // UUID short format
					s.broadcastNewRun(runID)
					go s.broadcastRunsUpdate()
				}
			}
		}

		// Parse TOMATO_EVENT lines for structured status updates
		if strings.HasPrefix(trimmedLine, tomatoEventPrefix) {
			jsonData := strings.TrimPrefix(trimmedLine, tomatoEventPrefix)
			var event TomatoEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				continue
			}

			switch event.Type {
			case "scenario_start":
				s.broadcastRunStatus("scenario_running", event.Scenario, "running", "")

			case "scenario_end":
				if event.Status == "failed" {
					s.broadcast(WSMessage{
						Type:       "scenario_failed",
						Scenario:   event.Scenario,
						Status:     "failed",
						Output:     event.Error,
						DurationMs: event.DurationMs,
					})
					featureResults[event.Feature] = "failed"
				} else if event.Status == "passed" {
					s.broadcast(WSMessage{
						Type:       "scenario_passed",
						Scenario:   event.Scenario,
						Status:     "passed",
						DurationMs: event.DurationMs,
					})
					if featureResults[event.Feature] != "failed" {
						featureResults[event.Feature] = "passed"
					}
				}

			case "step_start":
				// The UI marks the running step, so it needs the position as
				// well as the name — step text repeats across scenarios.
				s.broadcast(WSMessage{
					Type:      "step_started",
					Scenario:  event.Scenario,
					Status:    "running",
					StepIndex: event.StepIndex,
				})

			case "step_end":
				s.broadcast(WSMessage{
					Type:       "step_finished",
					Scenario:   event.Scenario,
					Status:     event.Status,
					Output:     event.Error,
					StepIndex:  event.StepIndex,
					DurationMs: event.DurationMs,
				})

			case "feature_end":
				status := featureResults[event.Feature]
				if status == "" {
					status = "passed"
				}
				s.broadcastRunStatus("feature_finished", event.Feature, status, "")

			case "summary":
				// Summary is handled by run_finished
			}
			continue
		}

		// Broadcast non-event output lines (human-readable output)
		// Skip internal log lines but show test output
		if isGherkinOutput(trimmedLine) && len(cleanLine) < 500 {
			htmlLine := ansiToHTML(line)
			s.broadcastRunStatus("run_output", "", "", htmlLine)
		}
	}

	// Wait for command to finish
	err = cmd.Wait()

	if err != nil {
		s.broadcastRunStatus("run_finished", "", "failed", err.Error())
	} else {
		s.broadcastRunStatus("run_finished", "", "passed", "")
	}

	// Broadcast updated runs list after run completes
	s.broadcastRunsUpdate()
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(str string) string {
	// Standard ANSI escape sequences
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	result := re.ReplaceAllString(str, "")

	// Also remove bracket-only codes like [31m, [0m, [1;30m etc. (without ESC prefix)
	bracketAnsi := regexp.MustCompile(`\[(\d+;)*\d*m`)
	result = bracketAnsi.ReplaceAllString(result, "")

	return result
}

// ansiClass maps an SGR parameter to one of the design system's console
// classes. The stylesheet owns the actual colours, so the server never emits a
// hex value — switching the palette is a CSS change, not a Go change.
var ansiClass = map[string]string{
	"1": "c-bold", "2": "c-dim",
	"30": "c-dim", "90": "c-dim",
	"31": "c-red", "91": "c-red",
	"32": "c-green", "92": "c-green",
	"33": "c-yellow", "93": "c-yellow",
	"34": "c-blue", "94": "c-blue",
	"35": "c-magenta", "95": "c-magenta",
	"36": "c-cyan", "96": "c-cyan",
}

var ansiSeq = regexp.MustCompile(`(?:\x1b)?\[([0-9;]*)m`)

// ansiToHTML escapes a line and converts its ANSI colour codes into spans
// carrying .c-* classes.
func ansiToHTML(str string) string {
	var out strings.Builder
	open := 0
	last := 0

	for _, loc := range ansiSeq.FindAllStringSubmatchIndex(str, -1) {
		out.WriteString(htmlEscape(str[last:loc[0]]))
		last = loc[1]

		params := str[loc[2]:loc[3]]
		var classes []string
		reset := params == "" || params == "0" || params == "00"
		if !reset {
			for _, code := range strings.Split(params, ";") {
				if code == "0" || code == "00" {
					reset = true
					break
				}
				if c, ok := ansiClass[code]; ok {
					classes = append(classes, c)
				}
			}
		}

		if reset {
			for ; open > 0; open-- {
				out.WriteString("</span>")
			}
			continue
		}
		if len(classes) > 0 {
			out.WriteString(`<span class="` + strings.Join(classes, " ") + `">`)
			open++
		}
	}

	out.WriteString(htmlEscape(str[last:]))
	for ; open > 0; open-- {
		out.WriteString("</span>")
	}
	return out.String()
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// isGherkinOutput checks if a line is relevant Gherkin test output
func isGherkinOutput(line string) bool {
	// Gherkin keywords and test output markers
	gherkinPrefixes := []string{
		"Feature:", "Scenario:", "Scenario Outline:", "Background:",
		"Given ", "When ", "Then ", "And ", "But ", "Examples:",
		"---", "===", "passed", "failed", "skipped", "undefined",
		"scenarios", "steps", "Error", "✓", "✗",
		"filtering scenarios", "skipping scenario", "🍅", "⚡",
	}

	for _, prefix := range gherkinPrefixes {
		if strings.Contains(line, prefix) {
			return true
		}
	}

	// Also show lines that start with common test output patterns
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// Filter out log lines (typically have timestamps or log levels)
	if strings.Contains(line, "level=") ||
		strings.Contains(line, "\"level\":") ||
		strings.Contains(line, "time=") ||
		strings.Contains(line, "\"time\":") ||
		strings.Contains(line, "msg=") ||
		strings.Contains(line, "\"msg\":") {
		return false
	}

	return true
}

func (s *UIServer) broadcastRunStatus(msgType, scenario, status, output string) {
	s.broadcast(WSMessage{
		Type:     msgType,
		Scenario: scenario,
		Status:   status,
		Output:   output,
	})
}

// broadcast fans one message out to every connected client.
func (s *UIServer) broadcast(msg WSMessage) {
	data, _ := json.Marshal(msg)

	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *UIServer) broadcastRunsUpdate() {
	runs, err := runlog.ListRuns()
	if err != nil {
		return
	}

	msg := WSMessage{
		Type: "runs_update",
		Runs: runs,
	}

	data, _ := json.Marshal(msg)

	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *UIServer) broadcastNewRun(runID string) {
	msg := WSMessage{
		Type:  "run_created",
		RunID: runID,
	}

	data, _ := json.Marshal(msg)

	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *UIServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	s.clientsMux.Lock()
	s.clients[conn] = true
	s.clientsMux.Unlock()

	defer func() {
		s.clientsMux.Lock()
		delete(s.clients, conn)
		s.clientsMux.Unlock()
	}()

	// Send initial data
	features, _ := s.loadFeatures()
	runs, _ := runlog.ListRuns()
	msg := WSMessage{Type: "init", Features: features, Runs: runs, Topology: s.loadTopology()}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *UIServer) loadTopology() *TopologyJSON {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return nil
	}
	t := topology(cfg)
	return &t
}

func (s *UIServer) loadFeatures() ([]FeatureJSON, error) {
	var features []FeatureJSON

	// Without a readable tomato.yml the steps are shown unannotated.
	var matcher *stepMatcher
	if cfg, err := config.Load(s.configPath); err == nil {
		matcher = newStepMatcher(cfg.Resources)
	}

	for _, path := range s.featurePaths {
		files, err := findFeatureFiles(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			f, err := parseFeatureFileJSON(file, matcher)
			if err != nil {
				continue
			}
			if f != nil {
				features = append(features, *f)
			}
		}
	}

	return features, nil
}

func findFeatureFiles(root string) ([]string, error) {
	var files []string

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if strings.HasSuffix(root, ".feature") {
			return []string{root}, nil
		}
		return nil, nil
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".feature") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func parseFeatureFileJSON(filePath string, matcher *stepMatcher) (*FeatureJSON, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	reader := strings.NewReader(string(content))
	idCounter := 0
	newID := func() string {
		idCounter++
		return fmt.Sprintf("%d", idCounter)
	}
	gherkinDoc, err := gherkin.ParseGherkinDocument(reader, newID)
	if err != nil {
		return nil, err
	}

	if gherkinDoc.Feature == nil {
		return nil, nil
	}

	feature := gherkinDoc.Feature

	fd := &FeatureJSON{
		Name:        feature.Name,
		Description: strings.TrimSpace(feature.Description),
		FilePath:    filePath,
		Tags:        extractTagsJSON(feature.Tags),
	}

	for _, child := range feature.Children {
		switch {
		case child.Background != nil:
			fd.Background = stepsJSON(child.Background.Steps, nil, matcher)
		case child.Scenario != nil:
			fd.Scenarios = append(fd.Scenarios, scenarioJSON(child.Scenario, matcher))
		case child.Rule != nil:
			// A Rule nests its own background and scenarios. They used to be
			// dropped, so a feature written with Rule blocks looked empty.
			for _, rc := range child.Rule.Children {
				switch {
				case rc.Background != nil:
					fd.Background = append(fd.Background, stepsJSON(rc.Background.Steps, nil, matcher)...)
				case rc.Scenario != nil:
					fd.Scenarios = append(fd.Scenarios, scenarioJSON(rc.Scenario, matcher))
				}
			}
		}
	}

	return fd, nil
}

// stepsJSON converts steps for the UI. An outline's steps are matched with
// example's values in place of their <placeholders>; matcher may be nil.
func scenarioJSON(sc *messages.Scenario, matcher *stepMatcher) ScenarioJSON {
	sd := ScenarioJSON{
		Name:        sc.Name,
		Description: strings.TrimSpace(sc.Description),
		Tags:        extractTagsJSON(sc.Tags),
		IsOutline:   len(sc.Examples) > 0,
	}

	for _, ex := range sc.Examples {
		ed := ExampleJSON{
			Name: ex.Name,
			Tags: extractTagsJSON(ex.Tags),
		}
		if ex.TableHeader != nil {
			var header []string
			for _, cell := range ex.TableHeader.Cells {
				header = append(header, cell.Value)
			}
			ed.Rows = append(ed.Rows, header)
		}
		for _, row := range ex.TableBody {
			var cells []string
			for _, cell := range row.Cells {
				cells = append(cells, cell.Value)
			}
			ed.Rows = append(ed.Rows, cells)
		}
		sd.Examples = append(sd.Examples, ed)
	}

	sd.Steps = stepsJSON(sc.Steps, firstExampleRow(sd.Examples), matcher)
	return sd
}

func stepsJSON(steps []*messages.Step, example map[string]string, matcher *stepMatcher) []StepJSON {
	var out []StepJSON
	phase := ""
	for _, step := range steps {
		st := StepJSON{
			Keyword: step.Keyword,
			Text:    step.Text,
		}

		switch strings.ToLower(strings.TrimSpace(step.Keyword)) {
		case "given":
			phase = "given"
		case "when":
			phase = "when"
		case "then":
			phase = "then"
		}
		st.Phase = phase

		if step.DocString != nil {
			st.DocString = step.DocString.Content
			st.DocLang = step.DocString.MediaType
		}

		if step.DataTable != nil {
			for _, row := range step.DataTable.Rows {
				var cells []string
				for _, cell := range row.Cells {
					cells = append(cells, cell.Value)
				}
				st.Table = append(st.Table, cells)
			}
		}

		if matcher != nil {
			text := step.Text
			for k, v := range example {
				text = strings.ReplaceAll(text, "<"+k+">", v)
			}
			if m, ok := matcher.match(text); ok {
				st.Resource = m.Resource
				st.ResourceType = m.Type
				st.Group = m.Def.Group
				st.Description = m.Def.Description
			} else {
				st.Unmatched = true
			}
		}

		out = append(out, st)
	}
	return out
}

// firstExampleRow is an outline's first row of examples by column name, or
// nil when it has none.
func firstExampleRow(examples []ExampleJSON) map[string]string {
	for _, ex := range examples {
		if len(ex.Rows) < 2 {
			continue
		}
		row := map[string]string{}
		for i, name := range ex.Rows[0] {
			if i < len(ex.Rows[1]) {
				row[name] = ex.Rows[1][i]
			}
		}
		return row
	}
	return nil
}

func extractTagsJSON(tags []*messages.Tag) []string {
	var result []string
	for _, t := range tags {
		result = append(result, t.Name)
	}
	return result
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}

	exec.Command(cmd, args...).Start()
}
