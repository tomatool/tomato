package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
)

func TestAnsiToHTMLEmitsClassesNotColours(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\x1b[31mfail\x1b[0m", `<span class="c-red">fail</span>`},
		{"\x1b[32mok\x1b[0m", `<span class="c-green">ok</span>`},
		{"[33mwarn[0m", `<span class="c-yellow">warn</span>`},
		{"plain", "plain"},
		// An unterminated sequence must still close, or its colour bleeds into
		// every console line appended after it.
		{"\x1b[31moops", `<span class="c-red">oops</span>`},
	}
	for _, c := range cases {
		if got := ansiToHTML(c.in); got != c.want {
			t.Errorf("ansiToHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAnsiToHTMLEscapesMarkup(t *testing.T) {
	got := ansiToHTML(`<script>alert("x")</script> & more`)
	if strings.Contains(got, "<script>") {
		t.Fatalf("raw markup survived escaping: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("expected escaped output, got %q", got)
	}
}

func TestAnsiToHTMLNeverEmitsInlineColour(t *testing.T) {
	// The design system owns the palette; the server must not hardcode hex, or
	// changing a colour means changing Go.
	got := ansiToHTML("\x1b[31ma\x1b[32mb\x1b[0m")
	if strings.Contains(got, "style=") || strings.Contains(got, "#") {
		t.Errorf("inline colour leaked into console output: %q", got)
	}
}

func writeTempFeature(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.feature")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFeatureReadsRuleChildren(t *testing.T) {
	// A Rule nests its own background and scenarios. They used to be dropped,
	// so a feature written with Rule blocks rendered as empty.
	path := writeTempFeature(t, `Feature: F
  Rule: First rule
    Background:
      Given shared setup
    Scenario: Inside the rule
      Then it works

  Rule: Second rule
    Scenario: Also inside a rule
      Then it also works
`)
	f, err := parseFeatureFileJSON(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Scenarios) != 2 {
		t.Fatalf("got %d scenarios, want both rules' scenarios", len(f.Scenarios))
	}
	if len(f.Background) != 1 {
		t.Errorf("got %d background steps, want the rule's background", len(f.Background))
	}
}

func TestParseFeatureStillReadsTopLevelChildren(t *testing.T) {
	path := writeTempFeature(t, `Feature: F
  Background:
    Given setup
  Scenario: Plain
    Then it works
`)
	f, err := parseFeatureFileJSON(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Background) != 1 || len(f.Scenarios) != 1 {
		t.Fatalf("background=%d scenarios=%d, want 1 and 1", len(f.Background), len(f.Scenarios))
	}
}

func TestParseFeatureRecordsDocStringLanguage(t *testing.T) {
	path := writeTempFeature(t, "Feature: F\n"+
		"  Scenario: S\n"+
		"    When it posts:\n"+
		"      \"\"\"json\n"+
		"      {\"a\": 1}\n"+
		"      \"\"\"\n")
	f, err := parseFeatureFileJSON(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	step := f.Scenarios[0].Steps[0]
	if step.DocLang != "json" {
		t.Errorf("docstring language = %q, want json", step.DocLang)
	}
	if !strings.Contains(step.DocString, `"a": 1`) {
		t.Errorf("docstring content = %q", step.DocString)
	}
}

func TestConfigSummaryHelpers(t *testing.T) {
	if got := resetStr(config.ResetSettings{Level: "scenario", OnFailure: "reset"}); got != "per scenario · on failure: reset" {
		t.Errorf("resetStr = %q", got)
	}
	if got := resetStr(config.ResetSettings{}); got != "" {
		t.Errorf("resetStr(zero) = %q, want empty", got)
	}
	if got := durStr(0); got != "" {
		t.Errorf("durStr(0) = %q, want empty", got)
	}
	if got := durStr(90 * time.Second); got != "1m30s" {
		t.Errorf("durStr = %q", got)
	}
	if got := waitStr(config.WaitStrategy{Type: "port", Port: 5432}); got != "port 5432" {
		t.Errorf("waitStr = %q", got)
	}
	if got := waitStr(config.WaitStrategy{}); got != "" {
		t.Errorf("waitStr(zero) = %q, want empty", got)
	}
}

func TestConnectsToPrefersTheExplicitTarget(t *testing.T) {
	cases := map[string]config.Resource{
		"http://localhost:8080": {BaseURL: "http://localhost:8080", Container: "ignored"},
		"ws://localhost/ws":     {URL: "ws://localhost/ws"},
		"localhost:9090":        {Address: "localhost:9090"},
		"container postgres":    {Container: "postgres"},
		"":                      {},
	}
	for want, r := range cases {
		if got := connectsTo(r); got != want {
			t.Errorf("connectsTo(%+v) = %q, want %q", r, got, want)
		}
	}
}

func TestHookListDescribesEachHook(t *testing.T) {
	got := hookList([]config.Hook{
		{SQLFile: "./schema.sql", Resource: "db"},
		{Shell: "./cleanup.sh"},
		{}, // nothing set: skipped rather than rendered as a blank row
	})
	want := []string{"sql_file ./schema.sql on db", "shell ./cleanup.sh"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hook %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSortedKeysIsStable(t *testing.T) {
	// Go map order would otherwise reshuffle the config screen on every fetch.
	got := sortedKeys(map[string]int{"z": 1, "a": 1, "m": 1})
	want := []string{"a", "m", "z"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortedKeys = %v, want %v", got, want)
		}
	}
}

func TestAppJSONUnconfigured(t *testing.T) {
	if appJSON(&config.AppConfig{}).Configured {
		t.Error("empty app reported as configured")
	}
}

func TestAppJSONDescribesReadyCheck(t *testing.T) {
	a := appJSON(&config.AppConfig{
		Command: "go run .",
		Port:    8080,
		Ready:   &config.ReadyCheck{Type: "http", Path: "/health", Status: 200, Timeout: 30 * time.Second},
		Env:     map[string]string{"B": "2", "A": "1"},
	})
	if !a.Configured {
		t.Fatal("app not reported as configured")
	}
	if a.Ready != "http /health → 200 · timeout 30s" {
		t.Errorf("ready = %q", a.Ready)
	}
	if len(a.Env) != 2 || a.Env[0].Key != "A" {
		t.Errorf("env not sorted: %+v", a.Env)
	}
}
