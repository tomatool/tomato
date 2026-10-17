package command

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const runName = "2025-01-01_000000_abcd1234"

// screenshotProject lays out a project in a temporary working directory:
// two browsers, alice writing to shots/ and bob to the defaults, a picture
// in each place a screenshot can be, and files that must never be listed or
// served from beside them.
func screenshotProject(t *testing.T) *UIServer {
	t.Helper()
	t.Chdir(t.TempDir())
	now := time.Now()
	write := func(rel string, body []byte, mod time.Time) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(rel, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(rel, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	write("tomato.yml", []byte(`version: 2
containers:
  chrome:
    preset: chromium
resources:
  alice:
    type: browser
    container: chrome
    options:
      screenshots: shots
  bob:
    type: browser
    container: chrome
`), now)
	png := []byte("\x89PNG\r\n\x1a\nfake")
	write("shots/a.png", png, now.Add(-3*time.Minute))
	write(filepath.Join(".tomato", "screenshots", "b.png"), append(png, '!'), now.Add(-2*time.Minute))
	write(filepath.Join(".tomato", "runs", runName, "screenshots", "c.png"), append(png, '!', '!'), now.Add(-time.Minute))
	// Not screenshots: a note beside one, a PNG in no screenshots directory,
	// a nested one, and a run's log.
	write("shots/notes.txt", []byte("not an image"), now)
	write("stray.png", png, now)
	write(filepath.Join("shots", "nested", "d.png"), png, now)
	write(filepath.Join(".tomato", "runs", runName, "app.log"), []byte("log"), now)
	return &UIServer{configPath: "tomato.yml", clients: make(map[*websocket.Conn]*sync.Mutex)}
}

func getScreenshot(s *UIServer, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handleScreenshots(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestScreenshotDirsFollowEachBrowser(t *testing.T) {
	s := screenshotProject(t)
	want := []string{
		"shots",
		filepath.Join(".tomato", "screenshots"),
		filepath.Join(".tomato", "runs", runName, "screenshots"),
	}
	if got := s.screenshotDirs(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("screenshotDirs = %v, want %v", got, want)
	}
	// Without a readable tomato.yml the defaults still stand, so the tab
	// does not empty while the file is being edited.
	if err := os.Remove("tomato.yml"); err != nil {
		t.Fatal(err)
	}
	if got := s.screenshotDirs(); strings.Join(got, ",") != strings.Join(want[1:], ",") {
		t.Errorf("screenshotDirs without a config = %v, want %v", got, want[1:])
	}
}

func TestScreenshotsListEveryDirectoryNewestFirst(t *testing.T) {
	s := screenshotProject(t)
	rec := getScreenshot(s, http.MethodGet, "/api/screenshots")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("GET /api/screenshots: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got []ScreenshotJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range got {
		names = append(names, g.Name)
	}
	if strings.Join(names, ",") != "c,b,a" {
		t.Fatalf("listed %v, want c,b,a — every directory, newest first, nothing nested or stray", names)
	}
	if got[0].ModTime.Before(got[1].ModTime) || got[1].ModTime.Before(got[2].ModTime) {
		t.Errorf("not newest first: %v", got)
	}
	a := got[2]
	if a.Path != filepath.Join("shots", "a.png") || a.Dir != "shots" || a.Size != 12 {
		t.Errorf("a = %+v", a)
	}
	if !strings.HasPrefix(a.URL, "/api/screenshots/shots/a.png?t=") {
		t.Errorf("a.URL = %q", a.URL)
	}
	if !filepath.IsAbs(a.AbsPath) || filepath.Base(a.AbsPath) != "a.png" {
		t.Errorf("a.AbsPath = %q", a.AbsPath)
	}
	if got[0].Dir != filepath.Join(".tomato", "runs", runName, "screenshots") {
		t.Errorf("c.Dir = %q", got[0].Dir)
	}
}

func TestScreenshotsListIsAListEvenWhenEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	s := &UIServer{configPath: "tomato.yml"}
	rec := getScreenshot(s, http.MethodGet, "/api/screenshots")
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %q, want []", body)
	}
}

func TestScreenshotIsServedAsPNG(t *testing.T) {
	s := screenshotProject(t)
	abs := absolutePath(filepath.Join("shots", "a.png"))
	for _, target := range []string{
		"/api/screenshots/shots/a.png",
		"/api/screenshots/shots/a.png?t=1700000000000",
		// An absolute screenshots directory is addressed without its leading
		// slash, since the mux folds a double slash away.
		"/api/screenshots/" + strings.TrimPrefix(filepath.ToSlash(abs), "/"),
	} {
		rec := getScreenshot(s, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: %d %s", target, rec.Code, rec.Body.String())
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Errorf("GET %s: Content-Type %q", target, ct)
		}
		if !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG")) || rec.Body.Len() != 12 {
			t.Errorf("GET %s: body %q", target, rec.Body.String())
		}
	}
	// The URL the list hands out works as given.
	list := s.listScreenshots()
	rec := getScreenshot(s, http.MethodGet, list[0].URL)
	if rec.Code != http.StatusOK || rec.Body.Len() != 14 {
		t.Errorf("GET %s (the newest, c.png): %d, %d bytes", list[0].URL, rec.Code, rec.Body.Len())
	}
}

func TestScreenshotRefusesPathsOutsideTheDirectories(t *testing.T) {
	s := screenshotProject(t)
	// A link planted beside the screenshots must not reach out either.
	if err := os.Symlink(filepath.Join("..", "tomato.yml"), filepath.Join("shots", "evil.png")); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		"/api/screenshots/shots/../tomato.yml",
		"/api/screenshots/shots/../stray.png",
		"/api/screenshots/../../../../etc/passwd",
		"/api/screenshots/stray.png",                                // a PNG, in no screenshots directory
		"/api/screenshots/shots/nested/d.png",                       // nothing nested is a screenshot
		"/api/screenshots/shots/evil.png",                           // a symlink out of the directory
		"/api/screenshots/.tomato/runs/" + runName + "/app.log",     // a run's log, not in its screenshots
		"/api/screenshots/.tomato/runs/" + runName + "/screenshots", // the directory itself
		"/api/screenshots/shots/missing.png",
	} {
		if rec := getScreenshot(s, http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", target, rec.Code)
		}
	}
}

func TestScreenshotRefusesWhatIsNotAPNG(t *testing.T) {
	s := screenshotProject(t)
	if rec := getScreenshot(s, http.MethodGet, "/api/screenshots/shots/notes.txt"); rec.Code != http.StatusForbidden {
		t.Errorf("a text file beside the screenshots: %d, want 403", rec.Code)
	}
	if rec := getScreenshot(s, http.MethodPost, "/api/screenshots/shots/a.png"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", rec.Code)
	}
}

func TestScreenshotURLEscapesEachSegment(t *testing.T) {
	mod := time.UnixMilli(1700000000000)
	cases := map[string]string{
		filepath.Join("shots", "with space#1.png"): "/api/screenshots/shots/with%20space%231.png?t=1700000000000",
		"/abs/dir/x.png": "/api/screenshots/abs/dir/x.png?t=1700000000000",
	}
	for in, want := range cases {
		if got := screenshotURL(in, mod); got != want {
			t.Errorf("screenshotURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInitCarriesTheScreenshots(t *testing.T) {
	s := screenshotProject(t)
	srv := httptest.NewServer(http.HandlerFunc(s.handleWebSocket))
	defer srv.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Type        string           `json:"type"`
		Screenshots []ScreenshotJSON `json:"screenshots"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "init" || len(msg.Screenshots) != 3 {
		t.Errorf("init carried %d screenshots (type %q), want 3", len(msg.Screenshots), msg.Type)
	}
}
