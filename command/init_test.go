package command

import (
	"regexp"
	"strings"
	"testing"

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
	// each dependency on its own, then all of them together
	all := map[string]bool{}
	for _, d := range availableResources {
		models = append(models, initModel{selected: map[string]bool{d.key: true}})
		all[d.key] = true
	}
	models = append(models, initModel{selected: all})
	models = append(models, initModel{selected: map[string]bool{}}) // nothing selected
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
