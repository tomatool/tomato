package command

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource/browser"
	"github.com/tomatool/tomato/internal/runlog"
)

// The screenshots the browser resource writes, served to the UI: the list
// behind the inspector's Screenshots tab, and the image behind each row and
// behind the button on the step that took it.

const screenshotsAPI = "/api/screenshots"

// ScreenshotJSON is one image in a screenshots directory.
type ScreenshotJSON struct {
	// Name is the file name without .png: what `saves screenshot "name"` was
	// given, or failed-<scenario>-<resource> for the page when a step failed.
	Name string `json:"name"`
	// Path is the file as the browser resource wrote it and as a step_end
	// event names it: relative to the working directory, unless the
	// screenshots option is absolute. It is what the UI matches on.
	Path string `json:"path"`
	// Dir is the screenshots directory it sits in, as configured.
	Dir string `json:"dir"`
	// AbsPath is where the file is on disk, for a reader who wants it
	// outside tomato; a page served over http cannot link to it.
	AbsPath string `json:"absPath"`
	// URL is where this server serves the image.
	URL     string    `json:"url"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// configuredScreenshotDirs is where the suite's browsers are told to write
// screenshots, resolved the way the resource resolves its own: the
// screenshots option of each browser in tomato.yml, and the default beside
// the run's logs — which .tomato/screenshots stands in for when there is no
// run. The default is always included: tomato.yml may not load while it is
// being edited, and a run made before an option was set left its pictures
// there.
func (s *UIServer) configuredScreenshotDirs() []string {
	var dirs []string
	if cfg, err := config.Load(s.configPath); err == nil {
		for _, name := range sortedKeys(cfg.Resources) {
			if r := cfg.Resources[name]; r.Type == "browser" {
				dirs = append(dirs, browser.ScreenshotsOption(r))
			}
		}
	}
	return uniqueDirs(append(dirs, browser.DefaultScreenshotDir))
}

// screenshotDirs is every directory a screenshot of this suite can be in:
// the configured ones, and each past run's own under .tomato/runs.
func (s *UIServer) screenshotDirs() []string {
	dirs := s.configuredScreenshotDirs()
	if runs, err := runlog.ListRuns(); err == nil {
		for _, run := range runs {
			dirs = append(dirs, browser.RunScreenshotDir(run.Dir))
		}
	}
	return uniqueDirs(dirs)
}

// uniqueDirs cleans the paths and drops blanks and repeats, keeping order.
func uniqueDirs(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range in {
		if d == "" {
			continue
		}
		d = filepath.Clean(d)
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

// listScreenshots is every PNG directly inside a screenshots directory,
// newest first. The browser resource writes nothing nested, so nothing
// nested is looked at. Never nil: the JSON is a list even when empty.
func (s *UIServer) listScreenshots() []ScreenshotJSON {
	out := []ScreenshotJSON{}
	for _, dir := range s.screenshotDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isPNG(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			out = append(out, ScreenshotJSON{
				Name:    strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
				Path:    path,
				Dir:     dir,
				AbsPath: absolutePath(path),
				URL:     screenshotURL(path, info.ModTime()),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ModTime.After(out[j].ModTime)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func isPNG(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".png")
}

// screenshotURL is where handleScreenshots serves the file at path. Every
// segment is escaped on its own so the slashes stay, and an absolute path
// loses its leading slash — the mux would fold a double slash away — which
// screenshotFile reads back. The modification time rides along so a browser
// does not show a cached picture after a step took a new one by the same name.
func screenshotURL(path string, mod time.Time) string {
	parts := strings.Split(strings.TrimPrefix(filepath.ToSlash(path), "/"), "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return screenshotsAPI + "/" + strings.Join(parts, "/") + "?t=" + strconv.FormatInt(mod.UnixMilli(), 10)
}

// handleScreenshots answers GET /api/screenshots with the list and
// GET /api/screenshots/{path} with one image. Only a PNG directly inside one
// of the screenshot directories is served; a path that climbs out with ..,
// a file anywhere else, or a log beside a screenshot is refused.
func (s *UIServer) handleScreenshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, screenshotsAPI), "/")
	if rest == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.listScreenshots())
		return
	}

	path, ok := s.screenshotFile(rest)
	if !ok {
		http.Error(w, "not a screenshot", http.StatusNotFound)
		return
	}
	if !isPNG(path) {
		http.Error(w, "not a PNG", http.StatusForbidden)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not a screenshot", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not a screenshot", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// screenshotFile resolves a requested path to a file that sits directly in
// one of the screenshot directories, reading it relative to the working
// directory first and as an absolute path second. Both sides are resolved
// through symlinks before they are compared, so neither a .. segment nor a
// link planted in the directory reaches anything outside it. false when the
// path is in no screenshot directory.
func (s *UIServer) screenshotFile(rel string) (string, bool) {
	rel = filepath.FromSlash(rel)
	candidates := []string{rel, string(filepath.Separator) + rel}
	for _, dir := range s.screenshotDirs() {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		realDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			continue // the directory does not exist yet
		}
		for _, cand := range candidates {
			abs, err := filepath.Abs(cand)
			if err != nil {
				continue
			}
			real, err := filepath.EvalSymlinks(abs)
			if err != nil || filepath.Dir(real) != realDir {
				continue
			}
			return real, true
		}
	}
	return "", false
}

// watchScreenshotDirs watches the configured screenshot directories that
// exist, and .tomato so one created later is picked up by watchLoop's rule
// for new directories. A run's own directory under .tomato/runs is not
// watched — on kqueue every file in a watched directory costs a descriptor,
// and a run writes many logs — so a run started outside the UI shows there
// when the tab is opened or the next run ends. Adding a path twice is
// harmless, which is why this runs again when a run starts.
func (s *UIServer) watchScreenshotDirs() {
	if s.watcher == nil {
		return
	}
	for _, p := range append([]string{".tomato"}, s.configuredScreenshotDirs()...) {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			s.watcher.Add(p)
		}
	}
}

func (s *UIServer) broadcastScreenshotsUpdate() {
	s.broadcast(WSMessage{Type: "screenshots_update", Screenshots: s.listScreenshots()})
}
