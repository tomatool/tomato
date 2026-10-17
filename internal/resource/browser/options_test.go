package browser

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
	"github.com/tomatool/tomato/internal/runlog"
)

func newFor(t *testing.T, cfg config.Resource) *Browser {
	t.Helper()
	r, err := New("alice", cfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func TestNewDefaults(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser"})
	if r.width != 1280 || r.height != 720 {
		t.Errorf("viewport = %dx%d, want 1280x720", r.width, r.height)
	}
	if r.timeout != 5*time.Second {
		t.Errorf("timeout = %s, want 5s", r.timeout)
	}
	if r.baseURL != "" || r.locale != "" || r.screenshots != "" {
		t.Errorf("unexpected defaults: %+v", r)
	}
	if r.Name() != "alice" {
		t.Errorf("Name() = %q", r.Name())
	}
}

func TestNewReadsOptions(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser", Options: map[string]any{
		"base_url":    "http://127.0.0.1:8080/",
		"viewport":    "1440x900",
		"locale":      "de-DE",
		"screenshots": "reports/screenshots",
		"timeout":     "2s",
	}})
	if r.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q: a loopback IP is spelled localhost and the trailing slash dropped", r.baseURL)
	}
	if r.width != 1440 || r.height != 900 {
		t.Errorf("viewport = %dx%d", r.width, r.height)
	}
	if r.locale != "de-DE" || r.screenshots != "reports/screenshots" || r.timeout != 2*time.Second {
		t.Errorf("options not read: %+v", r)
	}
}

func TestNewTopLevelBaseURL(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser", BaseURL: "http://app:3000"})
	if r.baseURL != "http://app:3000" {
		t.Errorf("baseURL = %q", r.baseURL)
	}
	// The option wins over the field, as it is the one the docs show.
	r = newFor(t, config.Resource{Type: "browser", BaseURL: "http://app:3000", Options: map[string]any{"base_url": "http://web:80"}})
	if r.baseURL != "http://web:80" {
		t.Errorf("baseURL = %q", r.baseURL)
	}
}

func TestNewRejectsBadOptions(t *testing.T) {
	cases := map[string]map[string]any{
		"is not an absolute URL":   {"base_url": "/master"},
		"is not WIDTHxHEIGHT":      {"viewport": "large"},
		"has a zero side":          {"viewport": "0x900"},
		"is not a duration":        {"timeout": "soon"},
		"is not a duration like":   {"timeout": "-1s"},
		"not an absolute URL like": {"base_url": "localhost"},
	}
	for want, options := range cases {
		_, err := New("alice", config.Resource{Type: "browser", Options: options}, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("options %v: error = %v, want it to say %q", options, err, want)
		}
	}
}

func TestParseViewportAcceptsSpaces(t *testing.T) {
	w, h, err := parseViewport(" 800 x 600 ")
	if err != nil || w != 800 || h != 600 {
		t.Errorf("parseViewport = %d, %d, %v", w, h, err)
	}
}

func TestResolve(t *testing.T) {
	resource.ResetGlobalVariables()
	resource.SetVariable("clause", "7.2")
	t.Cleanup(resource.ResetGlobalVariables)

	r := newFor(t, config.Resource{Type: "browser", Options: map[string]any{"base_url": "http://localhost:8080"}})
	cases := map[string]string{
		"/master":                        "http://localhost:8080/master",
		"master":                         "http://localhost:8080/master",
		"/master/{{clause}}?tab=history": "http://localhost:8080/master/7.2?tab=history",
		"http://127.0.0.1:9999/stub":     "http://localhost:9999/stub",
		"http://[::1]/x":                 "http://localhost/x",
		"http://frontend:3000/":          "http://frontend:3000/",
		"data:text/html,<h1>Hi</h1>":     "data:text/html,<h1>Hi</h1>",
		"about:blank":                    "about:blank",
	}
	for in, want := range cases {
		got, err := r.resolve(in)
		if err != nil || got != want {
			t.Errorf("resolve(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	bare := newFor(t, config.Resource{Type: "browser"})
	if _, err := bare.resolve("/master"); err == nil || !strings.Contains(err.Error(), "no base_url") {
		t.Errorf("a path without base_url: error = %v", err)
	}
	if _, err := bare.resolve("http://a b"); err == nil {
		t.Error("an invalid URL resolved")
	}
}

func TestParseKeys(t *testing.T) {
	cases := []struct {
		in   string
		key  rune
		mods []chromedp.Modifier
	}{
		{"Enter", []rune(kb.Enter)[0], nil},
		{"enter", []rune(kb.Enter)[0], nil},
		{"Return", []rune(kb.Enter)[0], nil},
		{"Escape", []rune(kb.Escape)[0], nil},
		{"esc", []rune(kb.Escape)[0], nil},
		{"Tab", []rune(kb.Tab)[0], nil},
		{"ArrowDown", []rune(kb.ArrowDown)[0], nil},
		{"up", []rune(kb.ArrowUp)[0], nil},
		{"space", ' ', nil},
		{"a", 'a', nil},
		{"+", '+', nil},
		{"ctrl+z", 'z', []chromedp.Modifier{chromedp.ModifierCtrl}},
		{"Ctrl+Shift+Z", 'Z', []chromedp.Modifier{chromedp.ModifierCtrl, chromedp.ModifierShift}},
		{"shift+Tab", []rune(kb.Tab)[0], []chromedp.Modifier{chromedp.ModifierShift}},
		{"cmd+a", 'a', []chromedp.Modifier{chromedp.ModifierMeta}},
		{"alt+ArrowLeft", []rune(kb.ArrowLeft)[0], []chromedp.Modifier{chromedp.ModifierAlt}},
		{"ctrl++", '+', []chromedp.Modifier{chromedp.ModifierCtrl}},
	}
	for _, c := range cases {
		key, mods, err := parseKeys(c.in)
		if err != nil {
			t.Errorf("parseKeys(%q): %v", c.in, err)
			continue
		}
		if key != c.key || len(mods) != len(c.mods) {
			t.Errorf("parseKeys(%q) = %q %v, want %q %v", c.in, key, mods, c.key, c.mods)
			continue
		}
		for i := range mods {
			if mods[i] != c.mods[i] {
				t.Errorf("parseKeys(%q) modifiers = %v, want %v", c.in, mods, c.mods)
			}
		}
	}

	for in, want := range map[string]string{
		"hyper+z":   "unknown modifier",
		"ctrl+Nope": "unknown key",
		"Banana":    "unknown key",
	} {
		if _, _, err := parseKeys(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseKeys(%q): error = %v, want %q", in, err, want)
		}
	}
}

func TestFileName(t *testing.T) {
	cases := map[string]string{
		"after-apply":                       "after-apply",
		"failed Clause 7.2: is shown alice": "failed-Clause-7.2-is-shown-alice",
		"../../etc/passwd":                  "etc-passwd",
		"   ":                               "screenshot",
		strings.Repeat("x", 200):            strings.Repeat("x", 120),
	}
	for in, want := range cases {
		if got := fileName(in); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScreenshotDir(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser", Options: map[string]any{"screenshots": "reports/shots"}})
	if got := r.screenshotDir(); got != "reports/shots" {
		t.Errorf("with the option: %q", got)
	}

	r = newFor(t, config.Resource{Type: "browser"})
	if got := r.screenshotDir(); got != filepath.Join(".tomato", "screenshots") {
		t.Errorf("without a run: %q", got)
	}

	cm, err := container.NewManager(nil)
	if err != nil {
		t.Fatal(err)
	}
	cm.SetRunContext(&runlog.RunContext{ID: "abc", Dir: filepath.Join(".tomato", "runs", "x")})
	r, err = New("alice", config.Resource{Type: "browser"}, cm)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.screenshotDir(); got != filepath.Join(".tomato", "runs", "x", "screenshots") {
		t.Errorf("in a run: %q", got)
	}
}

func TestInitNeedsAContainer(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser"})
	err := r.Init(context.Background())
	if err == nil || !strings.Contains(err.Error(), "preset: chromium") {
		t.Errorf("Init without a container: %v", err)
	}
	if err := r.Ready(context.Background()); err == nil {
		t.Error("Ready passed without a connection")
	}
	if _, err := r.page(); err == nil {
		t.Error("a page opened without a connection")
	}
	// Reset and Cleanup do nothing harmful before Init.
	if err := r.Reset(context.Background()); err != nil {
		t.Error(err)
	}
	if err := r.Cleanup(context.Background()); err != nil {
		t.Error(err)
	}
}

func TestInitNeedsTheContainerToBeRunning(t *testing.T) {
	cm, err := container.NewManager(nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New("alice", config.Resource{Type: "browser", Container: "chrome"}, cm)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Init(context.Background()); err == nil || !strings.Contains(err.Error(), "container host") {
		t.Errorf("Init against a container that is not running: %v", err)
	}
}

func TestIPFor(t *testing.T) {
	ctx := context.Background()
	for in, want := range map[string]string{
		"localhost":            "localhost",
		"10.0.0.7":             "10.0.0.7",
		"no-such-host.invalid": "no-such-host.invalid",
	} {
		if got := ipFor(ctx, in); got != want {
			t.Errorf("ipFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStepsAreDocumented(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser"})
	cat := r.Steps()
	if len(cat.Steps) == 0 {
		t.Fatal("no steps")
	}
	seen := map[string]bool{}
	for _, s := range cat.Steps {
		if s.Description == "" || s.Example == "" || s.Group == "" || s.Handler == nil {
			t.Errorf("step %q lacks a description, example, group or handler", s.Pattern)
		}
		if seen[s.Pattern] {
			t.Errorf("step %q is defined twice", s.Pattern)
		}
		seen[s.Pattern] = true
		re := regexp.MustCompile(strings.ReplaceAll(s.Pattern, "{resource}", `[^"]+`))
		if !re.MatchString(s.Example) {
			t.Errorf("example %q does not match its own pattern %q", s.Example, s.Pattern)
		}
	}
}

func TestScreenshotDirRulesAreExported(t *testing.T) {
	// `tomato ui` lists the directories the suite's browsers write to by the
	// same rules New applies, so the two cannot drift apart.
	if got := ScreenshotsOption(config.Resource{Options: map[string]any{"screenshots": "reports/shots"}}); got != "reports/shots" {
		t.Errorf("ScreenshotsOption = %q", got)
	}
	if got := ScreenshotsOption(config.Resource{Options: map[string]any{"screenshots": 7}}); got != "" {
		t.Errorf("ScreenshotsOption of a non-string = %q, want the default", got)
	}
	if got := ScreenshotsOption(config.Resource{}); got != "" {
		t.Errorf("ScreenshotsOption without options = %q", got)
	}
	if got := RunScreenshotDir(filepath.Join(".tomato", "runs", "x")); got != filepath.Join(".tomato", "runs", "x", "screenshots") {
		t.Errorf("RunScreenshotDir = %q", got)
	}
	if DefaultScreenshotDir != filepath.Join(".tomato", "screenshots") {
		t.Errorf("DefaultScreenshotDir = %q", DefaultScreenshotDir)
	}
}
