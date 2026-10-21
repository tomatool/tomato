package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func testResources() map[string]config.Resource {
	return map[string]config.Resource{
		"app-client": {Type: "http-client"},
		"app-db":     {Type: "postgres"},
		"bad":        {Type: "no-such-type"},
	}
}

func TestStepMatcherFindsResourceAndDefinition(t *testing.T) {
	m := newStepMatcher(testResources())

	got, ok := m.match(`"app-client" sends "GET" to "/users/1"`)
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Resource != "app-client" || got.Type != "http-client" {
		t.Errorf("got resource %q type %q", got.Resource, got.Type)
	}
	if got.Def.Group == "" || got.Def.Description == "" {
		t.Errorf("definition has no group or description: %+v", got.Def)
	}

	if _, ok := m.match(`"app-db" table "users" is empty`); !ok {
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
    Given "app-db" table "users" is empty

  Scenario: Get a user
    When "app-client" sends "GET" to "/users/1"
    Then "app-client" response status is "200"
    And "app-db" table "users" is empty
    And something tomato has no step for

  Scenario Outline: Status
    When "app-client" sends "GET" to "<path>"
    Then "app-client" response status is "<status>"

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

	if len(f.Background) != 1 || f.Background[0].Resource != "app-db" || f.Background[0].Phase != "given" {
		t.Errorf("background not parsed: %+v", f.Background)
	}

	steps := f.Scenarios[0].Steps
	wantPhase := []string{"when", "then", "then", "then"}
	wantResource := []string{"app-client", "app-client", "app-db", ""}
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
		if st.Resource != "app-client" {
			t.Errorf("outline step %q not matched with example values", st.Text)
		}
	}
}

func TestParseFeatureWithoutConfigLeavesStepsUnannotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.feature")
	if err := os.WriteFile(path, []byte("Feature: A\n  Scenario: S\n    When \"app-client\" sends \"GET\" to \"/\"\n"), 0o644); err != nil {
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
		// A client defaults to the app only when there is one to run.
		App: config.AppConfig{Command: "go run ./app", Port: 8080},
		Resources: map[string]config.Resource{
			"app-client":      {Type: "http-client", BaseURL: "http://localhost:8080"},
			"app-grpc-client": {Type: "grpc", Address: "localhost:9090"},
			"app-db":          {Type: "postgres"},
			"prices-stub":     {Type: "websocket-server", Options: map[string]any{"port": 9997}},
			"prices-client":   {Type: "websocket-client", URL: "ws://localhost:9997/"},
		},
	}
	got := map[string]string{}
	for _, r := range topology(cfg).Resources {
		got[r.Name] = r.Target
	}
	want := map[string]string{"app-client": "app", "app-grpc-client": "app", "app-db": "", "prices-stub": "", "prices-client": "prices-stub"}
	for name, target := range want {
		if got[name] != target {
			t.Errorf("%s: target %q, want %q", name, got[name], target)
		}
	}
}
