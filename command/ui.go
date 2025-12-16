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

//go:embed ui_assets/*
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
			Name:    "port",
			Value:   0,
			Usage:   "Port to run the UI server (default: random available port)",
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
	Table     [][]string `json:"table,omitempty"`
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
	// Run status fields
	Scenario string `json:"scenario,omitempty"`
	Status   string `json:"status,omitempty"` // "running", "passed", "failed"
	Output   string `json:"output,omitempty"`
	// Debug logs fields
	Runs  []runlog.RunInfo `json:"runs,omitempty"`
	RunID string           `json:"runId,omitempty"`
}

// TomatoEvent represents a structured event from the tomato formatter
type TomatoEvent struct {
	Type     string `json:"type"`
	Feature  string `json:"feature,omitempty"`
	Scenario string `json:"scenario,omitempty"`
	Step     string `json:"step,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	File     string `json:"file,omitempty"`
	Total    int    `json:"total,omitempty"`
	Passed   int    `json:"passed,omitempty"`
	Failed   int    `json:"failed,omitempty"`
	Skipped  int    `json:"skipped,omitempty"`
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
	assetsFS, err := fs.Sub(uiAssets, "ui_assets")
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

	msg := WSMessage{Type: "update"}
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

func (s *UIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(s.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path":    s.configPath,
		"content": string(content),
	})
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
					s.broadcastRunStatus("scenario_failed", event.Scenario, "failed", event.Error)
					featureResults[event.Feature] = "failed"
				} else if event.Status == "passed" {
					s.broadcastRunStatus("scenario_passed", event.Scenario, "passed", "")
					if featureResults[event.Feature] != "failed" {
						featureResults[event.Feature] = "passed"
					}
				}

			case "step_end":
				// Could broadcast step status if needed for real-time step tracking
				if event.Status == "failed" {
					// Broadcast the error immediately for UI display
					s.broadcastRunStatus("step_failed", event.Scenario, "failed", event.Error)
				}

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

// ansiToHTML converts ANSI color codes to HTML spans
func ansiToHTML(str string) string {
	// First escape HTML
	str = strings.ReplaceAll(str, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")

	// ANSI color map (foreground colors)
	colors := map[string]string{
		"30": "#6c7086", // black (gray)
		"31": "#f38ba8", // red
		"32": "#a6e3a1", // green
		"33": "#f9e2af", // yellow
		"34": "#89b4fa", // blue
		"35": "#cba6f7", // magenta
		"36": "#94e2d5", // cyan
		"37": "#cdd6f4", // white
		"90": "#6c7086", // bright black
		"91": "#f38ba8", // bright red
		"92": "#a6e3a1", // bright green
		"93": "#f9e2af", // bright yellow
		"94": "#89b4fa", // bright blue
		"95": "#cba6f7", // bright magenta
		"96": "#94e2d5", // bright cyan
		"97": "#cdd6f4", // bright white
	}

	// Replace ANSI codes with spans
	// Handle ESC[XXm and [XXm formats
	ansiPattern := regexp.MustCompile(`(?:\x1b)?\[(\d+)(?:;(\d+))?m`)

	result := ansiPattern.ReplaceAllStringFunc(str, func(match string) string {
		parts := ansiPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}

		code := parts[1]
		if code == "0" || code == "00" {
			return "</span>"
		}

		// Check for bold (1) prefix
		if len(parts) > 2 && parts[2] != "" {
			code = parts[2] // Use the color code after bold
		}

		if color, ok := colors[code]; ok {
			return fmt.Sprintf(`<span style="color:%s">`, color)
		}
		return ""
	})

	// Also handle plain [XXm without ESC
	bracketPattern := regexp.MustCompile(`\[(\d+)(?:;(\d+))?m`)
	result = bracketPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := bracketPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}

		code := parts[1]
		if code == "0" || code == "00" {
			return "</span>"
		}

		if len(parts) > 2 && parts[2] != "" {
			code = parts[2]
		}

		if color, ok := colors[code]; ok {
			return fmt.Sprintf(`<span style="color:%s">`, color)
		}
		return ""
	})

	return result
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
	msg := WSMessage{
		Type:     msgType,
		Scenario: scenario,
		Status:   status,
		Output:   output,
	}

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
	msg := WSMessage{Type: "init", Features: features, Runs: runs}
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

func (s *UIServer) loadFeatures() ([]FeatureJSON, error) {
	var features []FeatureJSON

	for _, path := range s.featurePaths {
		files, err := findFeatureFiles(path)
		if err != nil {
			continue
		}

		for _, file := range files {
			f, err := parseFeatureFileJSON(file)
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

func parseFeatureFileJSON(filePath string) (*FeatureJSON, error) {
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
		if child.Scenario != nil {
			sc := child.Scenario
			sd := ScenarioJSON{
				Name:        sc.Name,
				Description: strings.TrimSpace(sc.Description),
				Tags:        extractTagsJSON(sc.Tags),
				IsOutline:   len(sc.Examples) > 0,
			}

			for _, step := range sc.Steps {
				st := StepJSON{
					Keyword: step.Keyword,
					Text:    step.Text,
				}

				if step.DocString != nil {
					st.DocString = step.DocString.Content
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

				sd.Steps = append(sd.Steps, st)
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

			fd.Scenarios = append(fd.Scenarios, sd)
		}
	}

	return fd, nil
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
