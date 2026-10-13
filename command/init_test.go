package command

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/gherkin/go/v26"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/registry"
	"gopkg.in/yaml.v3"
)

// `tomato init` is the first command a new user runs, and nothing checked what
// it writes. It drifted badly: the config carried `settings.reset.level`,
// removed when reset became unconditional and silently ignored by the loader
// ever since, and every step in the generated feature used wording no
// resource registers any more — five steps, five undefined.
//
// These tests tie the generator to the things it has to stay in step with:
// the config struct and the live step definitions. Rename a step or drop a
// config field and they fail here, rather than in a new user's first run.

func allSelections() []initModel {
	var models []initModel
	all := map[string]bool{}
	for _, d := range availableResources {
		all[d.key] = true
	}

	// Every runner, including the empty one. The app block — and the env
	// vars inside it — is only written when a runner is chosen, so leaving
	// runnerType empty skipped it entirely. That is how a generated config
	// with an unquoted `{{.kafka.host}}` env var, which YAML cannot parse,
	// got past these tests.
	runners := []initModel{
		{},
		{runnerType: "command", customCommand: "go run ./cmd/app"},
		{runnerType: "docker", dockerfile: "Dockerfile"},
		// values that need quoting to survive YAML
		{runnerType: "command", customCommand: `sh -c "echo {hi}: there"`},
	}

	for _, r := range runners {
		for _, d := range availableResources {
			m := r
			m.selected = map[string]bool{d.key: true}
			models = append(models, m)
		}
		m := r
		m.selected = all
		models = append(models, m)
		m2 := r
		m2.selected = map[string]bool{}
		models = append(models, m2)
	}
	return models
}

func TestInit_GeneratedConfigHasNoUnknownFields(t *testing.T) {
	for _, m := range allSelections() {
		body := generateTomatoConfig(m)

		// KnownFields is the point: the loader ignores unknown keys, so a
		// field removed from the struct stays in the generated config for
		// years without anything noticing.
		dec := yaml.NewDecoder(strings.NewReader(body))
		dec.KnownFields(true)
		var into struct {
			Version   int            `yaml:"version"`
			Settings  map[string]any `yaml:"settings"`
			App       map[string]any `yaml:"app"`
			Container map[string]any `yaml:"containers"`
			Resources map[string]any `yaml:"resources"`
			Features  map[string]any `yaml:"features"`
			Hooks     map[string]any `yaml:"hooks"`
		}
		if err := dec.Decode(&into); err != nil {
			t.Fatalf("selected %v: generated config does not parse: %v\n%s", keys(m), err, body)
		}

		for _, gone := range []string{"reset", "parallel"} {
			if _, found := into.Settings[gone]; found {
				t.Errorf("selected %v: generated config still writes settings.%s, "+
					"which tomato no longer supports", keys(m), gone)
			}
		}
		if into.Version != 2 {
			t.Errorf("selected %v: generated version is %d, want 2", keys(m), into.Version)
		}
	}
}

func TestInit_GeneratedSuiteIsValid(t *testing.T) {
	// the same patterns `tomato validate` checks feature files against
	var patterns []*regexp.Regexp
	for _, cat := range registry.AllStepCategories() {
		for _, step := range cat.Steps {
			p := strings.ReplaceAll(step.Pattern, "{resource}", `[^"]+`)
			if re, err := regexp.Compile("(?i)" + p); err == nil {
				patterns = append(patterns, re)
			}
		}
	}
	if len(patterns) == 0 {
		t.Fatal("no step patterns loaded; this test would pass vacuously")
	}

	for _, m := range allSelections() {
		body := generateExampleFeatureFile(m)
		doc, err := gherkin.ParseGherkinDocument(strings.NewReader(body),
			(&messages.Incrementing{}).NewId)
		if err != nil {
			t.Fatalf("selected %v: generated feature does not parse: %v\n%s", keys(m), err, body)
		}

		steps := 0
		for _, child := range doc.Feature.Children {
			if child.Scenario == nil {
				continue
			}
			for _, step := range child.Scenario.Steps {
				steps++
				text := strings.TrimSpace(step.Text)
				matched := false
				for _, re := range patterns {
					if re.MatchString(text) {
						matched = true
						break
					}
				}
				if !matched {
					t.Errorf("selected %v: generated step is undefined: %q\n"+
						"  no resource registers a step matching it; run `tomato steps` for the real wording",
						keys(m), text)
				}
			}
		}
		if steps == 0 {
			t.Errorf("selected %v: generated feature has no steps", keys(m))
		}
	}
}

// A step can match a pattern and still be broken at runtime: the patterns use
// [^"]+ for the resource name, so "api" matches even when the config declares
// no resource called that. This catches the generator naming a resource one
// thing in the config and another in the feature.
func TestInit_GeneratedStepsNameResourcesThatExist(t *testing.T) {
	for _, m := range allSelections() {
		var cfg struct {
			Resources map[string]any `yaml:"resources"`
		}
		if err := yaml.Unmarshal([]byte(generateTomatoConfig(m)), &cfg); err != nil {
			t.Fatalf("selected %v: %v", keys(m), err)
		}

		body := generateExampleFeatureFile(m)
		doc, err := gherkin.ParseGherkinDocument(strings.NewReader(body),
			(&messages.Incrementing{}).NewId)
		if err != nil {
			t.Fatalf("selected %v: %v", keys(m), err)
		}

		// the resource is the first quoted token of a tomato step
		first := regexp.MustCompile(`^"([^"]+)"`)
		for _, child := range doc.Feature.Children {
			if child.Scenario == nil {
				continue
			}
			for _, step := range child.Scenario.Steps {
				mm := first.FindStringSubmatch(strings.TrimSpace(step.Text))
				if mm == nil {
					continue
				}
				if _, ok := cfg.Resources[mm[1]]; !ok {
					t.Errorf("selected %v: step names resource %q, which the generated config does not declare (it has %v)\n  step: %s",
						keys(m), mm[1], resourceNames(cfg.Resources), strings.TrimSpace(step.Text))
				}
			}
		}
	}
}

func resourceNames(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keys(m initModel) []string {
	var out []string
	for k, v := range m.selected {
		if v {
			out = append(out, k)
		}
	}
	if out == nil {
		return []string{"(none)"}
	}
	return out
}

// The three tests above check the wording of what `tomato init` writes. This
// one checks that the suite is green, which is what a first `tomato run` has
// to be: a template that matched every step pattern still advertised Kafka on
// a port nothing listened on, seeded a table nothing created, and asserted a
// request nothing sent. It needs Docker and a built binary, so it runs only
// when TOMATO_BIN names one; `make coverage` and `make integration-test` do.
func TestInit_GeneratedSuitePasses(t *testing.T) {
	bin := os.Getenv("TOMATO_BIN")
	if bin == "" {
		t.Skip("set TOMATO_BIN to the absolute path of a tomato binary to run the generated suite under Docker")
	}
	if !filepath.IsAbs(bin) {
		t.Fatalf("TOMATO_BIN must be an absolute path (go test runs in the package directory), got %q", bin)
	}

	// every dependency, no app: the project a first run is most likely to be
	m := initModel{selected: map[string]bool{}}
	for _, d := range availableResources {
		m.selected[d.key] = true
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	feature := generateExampleFeatureFile(m)
	for name, body := range map[string]string{
		"tomato.yml":               generateTomatoConfig(m),
		"features/example.feature": feature,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "run")
	cmd.Dir = dir
	raw, err := cmd.CombinedOutput()
	out := regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`).ReplaceAllString(string(raw), "")
	if err != nil {
		t.Fatalf("the project tomato init writes does not pass its own suite: %v\n%s", err, out)
	}

	want := scenarioCount(t, feature)
	summary := regexp.MustCompile(`(?m)^(\d+) scenarios \((\d+) passed\)$`).FindStringSubmatch(out)
	if summary == nil || summary[1] != strconv.Itoa(want) || summary[2] != strconv.Itoa(want) {
		t.Fatalf("expected %d scenarios, all passed; the run printed:\n%s", want, out)
	}
}

func scenarioCount(t *testing.T, feature string) int {
	t.Helper()
	doc, err := gherkin.ParseGherkinDocument(strings.NewReader(feature), (&messages.Incrementing{}).NewId)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, child := range doc.Feature.Children {
		if child.Scenario != nil {
			n++
		}
	}
	return n
}
