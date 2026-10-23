package runlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RunContext holds information about the current test run
type RunContext struct {
	ID        string    // Short unique identifier (8 chars)
	Timestamp time.Time // When the run started
	Dir       string    // Full path to the run directory
}

// New creates a new run context and initializes the run directory
func New() (*RunContext, error) {
	now := time.Now()
	shortID := uuid.New().String()[:8]

	// Format: .tomato/runs/2025-01-15_143052_a1b2c3d4/
	dirName := fmt.Sprintf("%s_%s", now.Format("2006-01-02_150405"), shortID)
	runDir := filepath.Join(".tomato", "runs", dirName)

	// Create the directory
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("creating run directory: %w", err)
	}

	return &RunContext{
		ID:        shortID,
		Timestamp: now,
		Dir:       runDir,
	}, nil
}

// LogPath returns the full path for a log file
func (r *RunContext) LogPath(name string) string {
	return filepath.Join(r.Dir, name+".log")
}

// CreateLogFile creates a log file and returns the file handle
func (r *RunContext) CreateLogFile(name string) (*os.File, error) {
	path := r.LogPath(name)
	return os.Create(path)
}

// WriteLog writes content to a log file
func (r *RunContext) WriteLog(name string, content []byte) error {
	path := r.LogPath(name)
	return os.WriteFile(path, content, 0644)
}

// AppendLog appends content to a log file
func (r *RunContext) AppendLog(name string, content []byte) error {
	path := r.LogPath(name)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

// ListRuns returns all run directories sorted by most recent first
func ListRuns() ([]RunInfo, error) {
	runsDir := filepath.Join(".tomato", "runs")

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunInfo{}, nil
		}
		return nil, err
	}

	var runs []RunInfo
	for i := len(entries) - 1; i >= 0; i-- { // Reverse order (newest first)
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		runDir := filepath.Join(runsDir, entry.Name())
		logs, _ := listLogs(runDir)

		runs = append(runs, RunInfo{
			Name:      entry.Name(),
			Dir:       runDir,
			Timestamp: info.ModTime(),
			Logs:      logs,
		})
	}

	return runs, nil
}

// RunInfo contains information about a stored run
type RunInfo struct {
	Name      string    `json:"name"`
	Dir       string    `json:"dir"`
	Timestamp time.Time `json:"timestamp"`
	Logs      []LogFile `json:"logs"`
}

// LogFile represents a log file in a run directory
type LogFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// AbsPath is where the file actually is, for a reader who wants to open
	// it outside tomato. Path stays relative because it identifies the log.
	AbsPath string `json:"absPath,omitempty"`
	Size    int64  `json:"size"`
}

func listLogs(runDir string) ([]LogFile, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}

	var logs []LogFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		logs = append(logs, LogFile{
			Name:    entry.Name()[:len(entry.Name())-4], // Remove .log extension
			Path:    filepath.Join(runDir, entry.Name()),
			AbsPath: absolute(filepath.Join(runDir, entry.Name())),
			Size:    info.Size(),
		})
	}

	return logs, nil
}

// GetLogContent reads the content of a log file
func GetLogContent(runName, logName string) (string, error) {
	path := filepath.Join(".tomato", "runs", runName, logName+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// absolute resolves path against the working directory, falling back to the
// path as given when that is not possible.
func absolute(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// EventsLog is the file in a run directory that keeps the run's events, for
// reports and replays after the fact.
const EventsLog = "events"

// Find is the run directory a run is known by: its id (the last part of the
// directory name), its directory name, or "" for the most recent run. A name
// with a path separator in it is refused, so a request cannot reach outside
// .tomato/runs.
func Find(run string) (RunInfo, error) {
	if strings.ContainsAny(run, `/\`) || run == "." || run == ".." {
		return RunInfo{}, fmt.Errorf("no run %q", run)
	}
	runs, err := ListRuns()
	if err != nil {
		return RunInfo{}, err
	}
	for _, r := range runs {
		if run == "" || r.Name == run || strings.HasSuffix(r.Name, "_"+run) {
			return r, nil
		}
	}
	if run == "" {
		return RunInfo{}, fmt.Errorf("no runs in .tomato/runs yet")
	}
	return RunInfo{}, fmt.Errorf("no run %q in .tomato/runs", run)
}

// Started is when a run began, read from its directory name.
func (r RunInfo) Started() time.Time {
	parts := strings.SplitN(r.Name, "_", 3)
	if len(parts) < 2 {
		return r.Timestamp
	}
	t, err := time.ParseInLocation("2006-01-02_150405", parts[0]+"_"+parts[1], time.Local)
	if err != nil {
		return r.Timestamp
	}
	return t
}

// ID is the run's id, the last part of its directory name.
func (r RunInfo) ID() string {
	return r.Name[strings.LastIndex(r.Name, "_")+1:]
}
