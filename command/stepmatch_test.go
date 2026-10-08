package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func testResources() map[string]config.Resource {
	return map[string]config.Resource{
		"api": {Type: "http-client"},
		"db":  {Type: "postgres"},
		"bad": {Type: "no-such-type"},
	}
}

func TestStepMatcherFindsResourceAndDefinition(t *testing.T) {
	m := newStepMatcher(testResources())

	got, ok := m.match(`"api" sends "GET" to "/users/1"`)
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Resource != "api" || got.Type != "http-client" {
		t.Errorf("got resource %q type %q", got.Resource, got.Type)
	}
	if got.Def.Group == "" || got.Def.Description == "" {
		t.Errorf("definition has no group or description: %+v", got.Def)
	}

	if _, ok := m.match(`"db" table "users" is empty`); !ok {
		t.Error("expected the postgres step to match")
	}
	if _, ok := m.match(`"nope" sends "GET" to "/"`); ok {
		t.Error("a step naming an unknown resource must not match")
	}
}

func TestParseFeatureAnnotatesSteps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.feature")
	feature := `Feature: Orders
  Background:
    Given "db" table "users" is empty

  Scenario: Get a user
    When "api" sends "GET" to "/users/1"
    Then "api" response status is "200"
    And "db" table "users" is empty
    And something tomato has no step for

  Scenario Outline: Status
    When "api" sends "GET" to "<path>"
    Then "api" response status is "<status>"

    Examples:
      | path | status |
      | /a   | 200    |
`
	if err := os.WriteFile(path, []byte(feature), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := parseFeatureFileJSON(path, newStepMatcher(testResources()))
	if err != nil {
		t.Fatal(err)
	}

	if len(f.Background) != 1 || f.Background[0].Resource != "db" || f.Background[0].Phase != "given" {
		t.Errorf("background not parsed: %+v", f.Background)
	}

	steps := f.Scenarios[0].Steps
	wantPhase := []string{"when", "then", "then", "then"}
	wantResource := []string{"api", "api", "db", ""}
	for i, st := range steps {
		if st.Phase != wantPhase[i] {
			t.Errorf("step %d phase %q, want %q", i, st.Phase, wantPhase[i])
		}
		if st.Resource != wantResource[i] {
			t.Errorf("step %d resource %q, want %q", i, st.Resource, wantResource[i])
		}
	}
	if !steps[3].Unmatched || steps[0].Unmatched {
		t.Errorf("unmatched flags wrong: %+v", steps)
	}

	for _, st := range f.Scenarios[1].Steps {
		if st.Resource != "api" {
			t.Errorf("outline step %q not matched with example values", st.Text)
		}
	}
}

func TestParseFeatureWithoutConfigLeavesStepsUnannotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.feature")
	if err := os.WriteFile(path, []byte("Feature: A\n  Scenario: S\n    When \"api\" sends \"GET\" to \"/\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := parseFeatureFileJSON(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	st := f.Scenarios[0].Steps[0]
	if st.Resource != "" || st.Unmatched {
		t.Errorf("expected no annotation without a config: %+v", st)
	}
}

func TestTopologyResolvesWhatClientsCall(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Port: 8080},
		Resources: map[string]config.Resource{
			"api":           {Type: "http-client", BaseURL: "http://localhost:8080"},
			"grpc":          {Type: "grpc", Address: "localhost:9090"},
			"db":            {Type: "postgres"},
			"wsmock":        {Type: "websocket-server", Options: map[string]any{"port": 9997}},
			"wsmock-client": {Type: "websocket-client", URL: "ws://localhost:9997/"},
		},
	}
	got := map[string]string{}
	for _, r := range topology(cfg).Resources {
		got[r.Name] = r.Target
	}
	want := map[string]string{"api": "app", "grpc": "app", "db": "", "wsmock": "", "wsmock-client": "wsmock"}
	for name, target := range want {
		if got[name] != target {
			t.Errorf("%s: target %q, want %q", name, got[name], target)
		}
	}
}
