package formatter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
)

// events parses every TOMATO_EVENT line the formatter wrote.
func events(t *testing.T, out *bytes.Buffer) []Event {
	t.Helper()
	var evs []Event
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "TOMATO_EVENT:") {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "TOMATO_EVENT:")), &ev); err != nil {
			t.Fatalf("bad event line %q: %v", line, err)
		}
		evs = append(evs, ev)
	}
	return evs
}

func TestStepEndCarriesTheStepsScreenshots(t *testing.T) {
	var out bytes.Buffer
	f := TomatoFormatterFunc("suite", &out).(*TomatoFormatter)
	pickle := &messages.Pickle{Name: "A screenshot of the page", Steps: []*messages.PickleStep{
		{Id: "s1", Text: `"alice" opens "/ui/master"`},
		{Id: "s2", Text: `"alice" saves screenshot "master"`},
		{Id: "s3", Text: `"alice" sees "Order cancelled"`},
	}}
	f.Pickle(pickle)

	f.Defined(pickle, pickle.Steps[0], nil)
	f.Passed(pickle, pickle.Steps[0], nil)

	// The step's handler writes the file and records it before godog reports
	// the step as passed.
	f.Defined(pickle, pickle.Steps[1], nil)
	resource.RecordScreenshot(pickle.Name, resource.Screenshot{Resource: "alice", Path: ".tomato/screenshots/master.png"})
	f.Passed(pickle, pickle.Steps[1], nil)

	// A failure's screenshot comes from the after-step hook, which godog runs
	// before Failed; two people with open pages leave two.
	f.Defined(pickle, pickle.Steps[2], nil)
	resource.RecordScreenshot(pickle.Name, resource.Screenshot{Resource: "alice", Path: ".tomato/screenshots/failed-A-screenshot-of-the-page-alice.png"})
	resource.RecordScreenshot(pickle.Name, resource.Screenshot{Resource: "bob", Path: ".tomato/screenshots/failed-A-screenshot-of-the-page-bob.png"})
	f.Failed(pickle, pickle.Steps[2], nil, errAssertion("does not see"))

	var ends []Event
	for _, ev := range events(t, &out) {
		if ev.Type == EventStepEnd {
			ends = append(ends, ev)
		}
	}
	if len(ends) != 3 {
		t.Fatalf("got %d step_end events, want 3", len(ends))
	}
	if ends[0].Screenshots != nil {
		t.Errorf("a step that took no screenshot reported %v", ends[0].Screenshots)
	}
	if len(ends[1].Screenshots) != 1 || ends[1].Screenshots[0].Resource != "alice" || ends[1].Screenshots[0].Path != ".tomato/screenshots/master.png" {
		t.Errorf("the saving step reported %v", ends[1].Screenshots)
	}
	if ends[1].Status != "passed" || *ends[1].StepIndex != 1 {
		t.Errorf("the saving step: status %q index %v", ends[1].Status, ends[1].StepIndex)
	}
	if len(ends[2].Screenshots) != 2 || ends[2].Screenshots[0].Resource != "alice" || ends[2].Screenshots[1].Resource != "bob" {
		t.Errorf("the failed step reported %v", ends[2].Screenshots)
	}
	if ends[2].Status != "failed" || ends[2].Error != "does not see" {
		t.Errorf("the failed step: status %q error %q", ends[2].Status, ends[2].Error)
	}
	// The raw line says `screenshots`, which is the name the docs give the field.
	if !strings.Contains(out.String(), `"screenshots":[{"resource":"alice","path":".tomato/screenshots/master.png"}]`) {
		t.Errorf("screenshots not serialised as documented:\n%s", out.String())
	}
}

func TestStepEndOmitsScreenshotsOfAnotherScenario(t *testing.T) {
	var out bytes.Buffer
	f := TomatoFormatterFunc("suite", &out).(*TomatoFormatter)
	resource.RecordScreenshot("Other", resource.Screenshot{Resource: "alice", Path: "other.png"})
	t.Cleanup(func() { resource.TakeScreenshots("Other") })

	pickle := &messages.Pickle{Name: "This", Steps: []*messages.PickleStep{{Id: "s1", Text: "x"}}}
	f.Pickle(pickle)
	f.Defined(pickle, pickle.Steps[0], nil)
	f.Passed(pickle, pickle.Steps[0], nil)

	for _, ev := range events(t, &out) {
		if ev.Type == EventStepEnd && ev.Screenshots != nil {
			t.Errorf("step_end of %q carried another scenario's screenshots: %v", ev.Scenario, ev.Screenshots)
		}
	}
	if strings.Contains(out.String(), `"screenshots"`) {
		t.Errorf("an empty screenshots field was serialised:\n%s", out.String())
	}
}

type errAssertion string

func (e errAssertion) Error() string { return string(e) }

// failStep runs one scenario whose only step fails with err and returns the
// step_end event the formatter emitted.
func failStep(t *testing.T, err error) (Event, string) {
	t.Helper()
	var out bytes.Buffer
	f := TomatoFormatterFunc("suite", &out).(*TomatoFormatter)
	step := &messages.PickleStep{Id: "s1", Text: `"shell" stdout is:`}
	pickle := &messages.Pickle{Name: "Fails", Steps: []*messages.PickleStep{step}}
	f.Pickle(pickle)
	f.Defined(pickle, step, nil)
	f.Failed(pickle, step, nil, err)

	for _, line := range strings.Split(out.String(), "\n") {
		data, ok := strings.CutPrefix(line, "TOMATO_EVENT:")
		if !ok {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("bad event %q: %v", data, err)
		}
		if ev.Type == EventStepEnd {
			return ev, data
		}
	}
	t.Fatalf("no step_end in %q", out.String())
	return Event{}, ""
}

func TestStepEndCarriesTheMismatch(t *testing.T) {
	m := &jsonmatch.Mismatch{What: "stdout", Expected: `{"a":1}`, Actual: `{"a":2}`, Mode: jsonmatch.ModeText, NoColon: true}
	ev, raw := failStep(t, fmt.Errorf("step: %w", m))

	if ev.Error != "step: "+m.Error() {
		t.Errorf("error = %q, want the wrapped text unchanged", ev.Error)
	}
	want := Mismatch{What: "stdout", Expected: `{"a":1}`, Actual: `{"a":2}`, Mode: "text"}
	if ev.Mismatch == nil || *ev.Mismatch != want {
		t.Fatalf("mismatch = %+v, want %+v", ev.Mismatch, want)
	}
	for _, key := range []string{`"mismatch":{`, `"what":"stdout"`, `"mode":"text"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("event %s lacks %s", raw, key)
		}
	}
}

func TestStepEndOmitsMismatchOtherwise(t *testing.T) {
	big := strings.Repeat("x", maxMismatchBytes)
	for name, err := range map[string]error{
		"plain error": errors.New("stdout does not contain \"hello\""),
		"too big":     &jsonmatch.Mismatch{What: "body", Expected: big, Actual: "y", Mode: jsonmatch.ModeText},
	} {
		ev, raw := failStep(t, err)
		if ev.Mismatch != nil || strings.Contains(raw, `"mismatch"`) {
			t.Errorf("%s: event carries a mismatch: %s", name, raw)
		}
		if ev.Error != err.Error() {
			t.Errorf("%s: error = %q", name, ev.Error)
		}
	}
}

func TestEmitRunError(t *testing.T) {
	var buf bytes.Buffer
	EmitRunError(&buf, "app", errors.New("failed to start application: app exited with exit status 1"), []string{`{"level":"fatal","message":"no config"}`})

	line := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(line, "TOMATO_EVENT:") {
		t.Fatalf("not an event line: %q", line)
	}
	var ev Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "TOMATO_EVENT:")), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != EventRunError || ev.Phase != "app" || ev.Status != "failed" {
		t.Errorf("event = %+v", ev)
	}
	if !strings.Contains(ev.Error, "exit status 1") {
		t.Errorf("error = %q", ev.Error)
	}
	if len(ev.Logs) != 1 || !strings.Contains(ev.Logs[0], "no config") {
		t.Errorf("logs = %q", ev.Logs)
	}
}

// The run coming up is reported the same way its scenarios are, so a viewer
// has something to show before the first one.
func TestEmit_WritesTheRunComingUp(t *testing.T) {
	var out bytes.Buffer
	Emit(&out, Event{Type: EventPhase, Phase: "containers"})
	Emit(&out, Event{Type: EventContainer, Name: "kafka", Status: "ready", DurationMs: Millis(2300 * time.Millisecond)})
	Emit(&out, Event{Type: EventApp, Name: "orderly", Status: "listening", URL: "http://localhost:8099", DurationMs: Millis(time.Second)})
	evs := events(t, &out)
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3:\n%s", len(evs), out.String())
	}
	if evs[0].Type != EventPhase || evs[0].Phase != "containers" {
		t.Errorf("phase event %+v", evs[0])
	}
	if evs[1].Type != EventContainer || evs[1].Name != "kafka" || evs[1].Status != "ready" || evs[1].DurationMs == nil || *evs[1].DurationMs != 2300 {
		t.Errorf("container event %+v", evs[1])
	}
	if evs[2].Type != EventApp || evs[2].Name != "orderly" || evs[2].URL != "http://localhost:8099" || *evs[2].DurationMs != 1000 {
		t.Errorf("app event %+v", evs[2])
	}
	if strings.Contains(out.String(), `"scenario"`) {
		t.Errorf("empty fields should be left out: %s", out.String())
	}
}
