package command

import (
	"reflect"
	"testing"
)

func ms(n int64) *int64 { return &n }

// The /ws feed is the runner's events in the UI's words: the run coming up
// before its scenarios, the features, the scenarios and their steps, and
// how the run ended.
func TestWSMessagesFor(t *testing.T) {
	idx := 2
	for _, tc := range []struct {
		name  string
		event TomatoEvent
		want  []WSMessage
	}{
		{"a phase", TomatoEvent{Type: "phase", Phase: "containers"}, []WSMessage{{Type: "run_phase", Phase: "containers"}}},
		{"a container ready", TomatoEvent{Type: "container", Name: "kafka", Status: "ready", DurationMs: ms(2300)},
			[]WSMessage{{Type: "container_status", Name: "kafka", Status: "ready", DurationMs: ms(2300)}}},
		{"the app listening", TomatoEvent{Type: "app", Name: "orderly", Status: "listening", URL: "http://localhost:8099", DurationMs: ms(900)},
			[]WSMessage{{Type: "app_status", Name: "orderly", Status: "listening", URL: "http://localhost:8099", DurationMs: ms(900)}}},
		{"a feature", TomatoEvent{Type: "feature_start", Feature: "The web shop", File: "features/web-shop.feature"},
			[]WSMessage{{Type: "feature_started", Feature: "The web shop", File: "features/web-shop.feature"}}},
		{"a feature's end", TomatoEvent{Type: "feature_end", Feature: "The web shop"}, []WSMessage{{Type: "feature_finished", Feature: "The web shop"}}},
		{"a scenario", TomatoEvent{Type: "scenario_start", Feature: "F", Scenario: "S", File: "f.feature"},
			[]WSMessage{{Type: "scenario_running", Feature: "F", Scenario: "S", File: "f.feature", Status: "running"}}},
		{"a scenario passing", TomatoEvent{Type: "scenario_end", Feature: "F", Scenario: "S", Status: "passed", DurationMs: ms(40)},
			[]WSMessage{{Type: "scenario_passed", Feature: "F", Scenario: "S", Status: "passed", DurationMs: ms(40)}}},
		{"a scenario failing", TomatoEvent{Type: "scenario_end", Feature: "F", Scenario: "S", Status: "failed", Error: "boom", DurationMs: ms(40)},
			[]WSMessage{{Type: "scenario_failed", Feature: "F", Scenario: "S", Status: "failed", Output: "boom", DurationMs: ms(40)}}},
		{"a scenario skipped", TomatoEvent{Type: "scenario_end", Scenario: "S", Status: "skipped"}, nil},
		{"a step starting", TomatoEvent{Type: "step_start", Scenario: "S", StepIndex: &idx},
			[]WSMessage{{Type: "step_started", Scenario: "S", Status: "running", StepIndex: &idx}}},
		{"a step ending", TomatoEvent{Type: "step_end", Scenario: "S", Status: "failed", Error: "no", StepIndex: &idx, DurationMs: ms(3)},
			[]WSMessage{{Type: "step_finished", Scenario: "S", Status: "failed", Output: "no", StepIndex: &idx, DurationMs: ms(3)}}},
		{"the run failing to start", TomatoEvent{Type: "run_error", Phase: "app", Status: "failed", Error: "port in use", Logs: []string{"last line"}},
			[]WSMessage{{Type: "run_error", Phase: "app", Status: "failed", Output: "port in use", Logs: []string{"last line"}}}},
		{"the summary", TomatoEvent{Type: "summary", Total: 20, Passed: 14, Failed: 6},
			[]WSMessage{{Type: "run_summary", Total: 20, Passed: 14, Failed: 6}}},
		{"something new", TomatoEvent{Type: "weather", Status: "sunny"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := wsMessagesFor(tc.event); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// A past run is replayed from its log as the feed a live run sends, so the
// same viewer shows both.
func TestReplayMessages_RebuildsTheFeedFromTheLog(t *testing.T) {
	log := "\n🍅 Tomato\n  📋 run: 216b61f6\n\nStarting dependencies...\n" +
		`TOMATO_EVENT:{"type":"phase","phase":"containers"}` + "\n" +
		`TOMATO_EVENT:{"type":"container","name":"kafka","status":"ready","durationMs":2300}` + "\n" +
		"  ✓ kafka 2.3s\n" +
		`TOMATO_EVENT:{"type":"feature_start","feature":"F","file":"f.feature"}` + "\n" +
		`TOMATO_EVENT:{"type":"scenario_start","feature":"F","scenario":"S","file":"f.feature"}` + "\n" +
		`TOMATO_EVENT:{"type":"step_end","scenario":"S","status":"passed","stepIndex":0,"durationMs":1}` + "\n" +
		`TOMATO_EVENT:{"type":"scenario_end","feature":"F","scenario":"S","status":"failed","durationMs":5}` + "\n" +
		`TOMATO_EVENT:{"type":"summary","total":1,"failed":1}` + "\n" +
		"not an event: TOMATO_EVENT:{\n"
	got := replayMessages(log)
	types := make([]string, len(got))
	for i, m := range got {
		types[i] = m.Type
	}
	want := []string{"run_started", "run_created", "run_phase", "container_status", "feature_started", "scenario_running", "step_finished", "scenario_failed", "run_summary", "run_finished"}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("replayed %v\nwant %v", types, want)
	}
	if got[1].RunID != "216b61f6" {
		t.Errorf("run id %q, want the banner's", got[1].RunID)
	}
	if last := got[len(got)-1]; last.Status != "failed" {
		t.Errorf("a run with a failed scenario finished %q, want failed", last.Status)
	}
	if last := replayMessages("TOMATO_EVENT:{\"type\":\"summary\",\"total\":1,\"passed\":1}\n"); last[len(last)-1].Status != "passed" {
		t.Errorf("a run with every scenario passed finished %q, want passed", last[len(last)-1].Status)
	}
	// A log that stops before the summary is a run that was killed, or one
	// still going: it did not pass.
	cut := replayMessages("TOMATO_EVENT:{\"type\":\"scenario_start\",\"scenario\":\"S\"}\n")
	if cut[len(cut)-1].Status != "interrupted" {
		t.Errorf("a log without a summary finished %q, want interrupted", cut[len(cut)-1].Status)
	}
}

func TestRunIDOf(t *testing.T) {
	for line, want := range map[string]string{
		"  📋 run: 216b61f6":            "216b61f6",
		"📋 run: 216b61f6":              "216b61f6",
		"2026/10/23 05:02 app running": "",
		"  📋 run: not-an-id-at-all":    "",
	} {
		if got := runIDOf(line); got != want {
			t.Errorf("runIDOf(%q) = %q, want %q", line, got, want)
		}
	}
}
