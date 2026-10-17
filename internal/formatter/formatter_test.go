package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	messages "github.com/cucumber/messages/go/v21"

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
