package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/formatters"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/profile"
	"github.com/tomatool/tomato/internal/resource"
)

// Event types for structured output
const (
	EventFeatureStart  = "feature_start"
	EventFeatureEnd    = "feature_end"
	EventScenarioStart = "scenario_start"
	EventScenarioEnd   = "scenario_end"
	EventStepStart     = "step_start"
	EventStepEnd       = "step_end"
	EventSummary       = "summary"
	// EventRunError is a run that failed outside any scenario: a container
	// that didn't start, a resource that couldn't connect, an app that never
	// became ready. Without it a report sees no scenarios and nothing failed.
	EventRunError = "run_error"
	// EventPhase is the run entering a phase: containers, resources, app or
	// tests. EventContainer is a dependency starting, ready or failed, and
	// EventApp the app starting, listening and ready, each with how long it
	// took, so whoever watches the run sees it come up before the first
	// scenario rather than a silent minute.
	EventPhase     = "phase"
	EventContainer = "container"
	EventApp       = "app"
)

// Event represents a structured test event
type Event struct {
	Type string `json:"type"`
	// At is when the event happened, in UTC.
	At       string `json:"at,omitempty"`
	Feature  string `json:"feature,omitempty"`
	Scenario string `json:"scenario,omitempty"`
	Step     string `json:"step,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	File     string `json:"file,omitempty"`

	// StepIndex is the step's 0-based position in its scenario. The web UI
	// keys per-step status on it, so it must be the pickle order, not a
	// counter — a background step shifts every index after it.
	StepIndex *int `json:"stepIndex,omitempty"`
	// DurationMs is how long the step or scenario took.
	DurationMs *int64 `json:"durationMs,omitempty"`
	// Mismatch is both sides of a failed comparison, on a step_end whose
	// error is (or wraps) a jsonmatch.Mismatch, so the web UI can draw a
	// diff instead of parsing Error back apart.
	Mismatch *Mismatch `json:"mismatch,omitempty"`

	// Screenshots are the images a step left behind, on its step_end: what
	// the browser resource wrote for `saves screenshot`, and the page when
	// the step failed. The web UI opens them from the step, so it never has
	// to guess which file a step produced from the file's name.
	Screenshots []resource.Screenshot `json:"screenshots,omitempty"`

	// Network is what network resources did while the step ran, and the
	// faults that were on when it ended, so a report can show which steps
	// ran on a bad network. On a scenario_end, the events after its last step.
	Network *Network `json:"network,omitempty"`

	// Phase is where a run_error happened: containers, resources or app; on
	// a phase event, the phase the run is entering.
	Phase string `json:"phase,omitempty"`
	// Name is the container a container event is about.
	Name string `json:"name,omitempty"`
	// URL is where the app answers, on an app event once it listens.
	URL string `json:"url,omitempty"`
	// Logs are the app's last log lines, on a run_error in the app phase.
	Logs []string `json:"logs,omitempty"`
	// Samples are the app's and its containers' readings since the last
	// event that carried them, on a step_end; Profile is what a scenario
	// cost, on its scenario_end. Only when the run is profiled.
	Samples []profile.Sample `json:"samples,omitempty"`
	Profile *profile.Summary `json:"profile,omitempty"`

	// Summary fields
	Total   int `json:"total,omitempty"`
	Passed  int `json:"passed,omitempty"`
	Failed  int `json:"failed,omitempty"`
	Skipped int `json:"skipped,omitempty"`
}

// Network is the step_end event's "network" object.
type Network struct {
	Events []resource.NetworkEvent `json:"events,omitempty"`
	// Active has a line per fault that was on when the step ended.
	Active []string `json:"active,omitempty"`
}

// networkOf reports the scenario's network events and active faults, nil
// when there is nothing to say.
func networkOf(scenario string, withActive bool) *Network {
	n := &Network{Events: resource.TakeNetworkEvents(scenario)}
	if withActive {
		n.Active = resource.NetworkFaults()
	}
	if len(n.Events) == 0 && len(n.Active) == 0 {
		return nil
	}
	return n
}

func takeSamples() []profile.Sample {
	if p := profile.Active(); p != nil {
		return p.TakeSamples()
	}
	return nil
}

func takeProfile(scenario string) *profile.Summary {
	if p := profile.Active(); p != nil {
		return p.TakeSummary(scenario)
	}
	return nil
}

// Mismatch is the step_end event's "mismatch" object.
type Mismatch struct {
	What     string `json:"what"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	// Mode is "text", "json" or "subset" (jsonmatch.Mode).
	Mode string `json:"mode"`
}

// maxMismatchBytes bounds the expected and actual values a step_end carries.
// The error text already holds both, and `tomato ui` reads events a line at
// a time with a 1 MiB limit; above this the UI falls back to the error text,
// which it shows as plain text at that size anyway.
const maxMismatchBytes = 256 << 10

// mismatchOf returns the event's mismatch object for err, or nil when err is
// not a comparison or its values are too big to repeat.
func mismatchOf(err error) *Mismatch {
	var m *jsonmatch.Mismatch
	if !errors.As(err, &m) || len(m.Expected)+len(m.Actual) > maxMismatchBytes {
		return nil
	}
	return &Mismatch{What: m.What, Expected: m.Expected, Actual: m.Actual, Mode: string(m.Mode)}
}

// TomatoFormatter outputs structured JSON events for UI parsing
type TomatoFormatter struct {
	out io.Writer
	// rec is the run's recorder, when this formatter claimed it (Record).
	rec io.Writer

	// Track current context
	currentFeature     string
	currentFeatureFile string
	currentScenario    string
	currentScenarioErr string

	// Track scenario status
	scenarioHadFailure bool

	// Timing and step position. stepIndex is resolved from the pickle so it
	// matches what the UI parsed out of the .feature file.
	scenarioStart time.Time
	stepStart     time.Time
	stepIndexByID map[string]int

	// Counters
	scenarioTotal   int
	scenarioPassed  int
	scenarioFailed  int
	scenarioSkipped int
	stepsPassed     int
	stepsFailed     int
	stepsSkipped    int
}

func init() {
	godog.Format("tomato", "Tomato structured JSON formatter", TomatoFormatterFunc)
}

// TomatoFormatterFunc creates a new TomatoFormatter
func TomatoFormatterFunc(suite string, out io.Writer) formatters.Formatter {
	return &TomatoFormatter{
		out: out,
		rec: claimRecorder(),
	}
}

// The recorder is where a run's events are kept, whatever it prints: the
// events.log in its run directory, which reports and replays are built from
// after the fact. Exactly one TomatoFormatter writes the scenarios to it, the
// first one godog creates, because the profile's samples are drained by
// whichever formatter takes them; Emit writes the rest of the run to it.
var (
	recMu      sync.Mutex
	recorder   io.Writer
	recClaimed bool
)

// Record sends the run's events to w from now on, until the returned stop
// is called.
func Record(w io.Writer) (stop func()) {
	recMu.Lock()
	recorder, recClaimed = w, false
	recMu.Unlock()
	return func() {
		recMu.Lock()
		recorder, recClaimed = nil, false
		recMu.Unlock()
	}
}

// claimRecorder hands the recorder to the first formatter that asks.
func claimRecorder() io.Writer {
	recMu.Lock()
	defer recMu.Unlock()
	if recorder == nil || recClaimed {
		return nil
	}
	recClaimed = true
	return recorder
}

func currentRecorder() io.Writer {
	recMu.Lock()
	defer recMu.Unlock()
	return recorder
}

// EmitRunError writes a run_error event: the run failed in phase before or
// outside the scenarios, with err, and logs the app wrote last when there are any.
func EmitRunError(out io.Writer, phase string, err error, logs []string) {
	Emit(out, Event{Type: EventRunError, Phase: phase, Status: "failed", Error: err.Error(), Logs: logs})
}

// Emit writes one event, for what happens outside godog: the phases of a run
// before its scenarios, and the containers and app coming up in them. It goes
// to out, when there is one, and to the run's recorder.
func Emit(out io.Writer, event Event) {
	(&TomatoFormatter{out: out, rec: currentRecorder()}).emit(event)
}

// Millis is a duration as an event carries it.
func Millis(d time.Duration) *int64 {
	ms := d.Milliseconds()
	return &ms
}

func (f *TomatoFormatter) emit(event Event) {
	if event.At == "" {
		event.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, _ := json.Marshal(event)
	line := fmt.Sprintf("TOMATO_EVENT:%s\n", data)
	if f.out != nil {
		io.WriteString(f.out, line)
	}
	if f.rec != nil && f.rec != f.out {
		io.WriteString(f.rec, line)
	}
}

func intp(i int) *int { return &i }

func msp(d time.Duration) *int64 { return Millis(d) }

// stepIdx returns the step's 0-based position within its pickle.
func (f *TomatoFormatter) stepIdx(pickle *messages.Pickle, step *messages.PickleStep) *int {
	if pickle == nil || step == nil {
		return nil
	}
	if f.stepIndexByID == nil {
		f.stepIndexByID = make(map[string]int)
	}
	if i, ok := f.stepIndexByID[step.Id]; ok {
		return intp(i)
	}
	for i, s := range pickle.Steps {
		f.stepIndexByID[s.Id] = i
	}
	if i, ok := f.stepIndexByID[step.Id]; ok {
		return intp(i)
	}
	return nil
}

// stepElapsed is the time since Defined fired for the current step. It falls
// back to zero rather than a bogus duration when Defined never ran (an
// undefined step is reported without being matched first).
func (f *TomatoFormatter) stepElapsed() *int64 {
	if f.stepStart.IsZero() {
		return msp(0)
	}
	d := time.Since(f.stepStart)
	f.stepStart = time.Time{}
	return msp(d)
}

// stepEnd emits one step_end with the position, duration and the screenshots
// the step left behind filled in. A failed step's screenshot is taken in an
// after-step hook, which godog runs before it reports the step, so it is
// recorded by the time this runs.
func (f *TomatoFormatter) stepEnd(pickle *messages.Pickle, step *messages.PickleStep, status, errMsg string, mm *Mismatch) {
	f.emit(Event{
		Type:        EventStepEnd,
		Feature:     f.currentFeature,
		Scenario:    pickle.Name,
		Step:        step.Text,
		Status:      status,
		Error:       errMsg,
		StepIndex:   f.stepIdx(pickle, step),
		DurationMs:  f.stepElapsed(),
		Screenshots: resource.TakeScreenshots(pickle.Name),
		Mismatch:    mm,
		Network:     networkOf(pickle.Name, true),
		Samples:     takeSamples(),
	})
}

// TestRunStarted is called when the test run starts
func (f *TomatoFormatter) TestRunStarted() {}

// Feature is called when a feature file is parsed
func (f *TomatoFormatter) Feature(doc *messages.GherkinDocument, uri string, content []byte) {
	// Emit end of previous feature if any
	if f.currentFeature != "" {
		f.emit(Event{
			Type:    EventFeatureEnd,
			Feature: f.currentFeature,
		})
	}

	if doc.Feature != nil {
		f.currentFeature = doc.Feature.Name
		f.currentFeatureFile = uri
		f.emit(Event{
			Type:    EventFeatureStart,
			Feature: doc.Feature.Name,
			File:    uri,
		})
	}
}

// Pickle is called when a scenario is about to run
func (f *TomatoFormatter) Pickle(pickle *messages.Pickle) {
	// Emit end of previous scenario if any
	f.emitScenarioEndIfNeeded()

	f.currentScenario = pickle.Name
	f.currentScenarioErr = ""
	f.scenarioHadFailure = false
	f.scenarioTotal++
	f.scenarioStart = time.Now()
	f.stepStart = time.Time{}
	f.stepIndexByID = nil

	f.emit(Event{
		Type:     EventScenarioStart,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		File:     f.currentFeatureFile,
	})
}

func (f *TomatoFormatter) emitScenarioEndIfNeeded() {
	if f.currentScenario == "" {
		return
	}

	status := "passed"
	if f.scenarioHadFailure {
		status = "failed"
	}

	var dur *int64
	if !f.scenarioStart.IsZero() {
		dur = msp(time.Since(f.scenarioStart))
	}

	f.emit(Event{
		Type:       EventScenarioEnd,
		Feature:    f.currentFeature,
		Scenario:   f.currentScenario,
		Status:     status,
		Error:      f.currentScenarioErr,
		DurationMs: dur,
		Network:    networkOf(f.currentScenario, false),
		Samples:    takeSamples(),
		Profile:    takeProfile(f.currentScenario),
	})

	// Update counters
	if f.scenarioHadFailure {
		f.scenarioFailed++
	} else {
		f.scenarioPassed++
	}

	f.currentScenario = ""
}

// Defined is called when a step definition is found, which is the last hook
// before the step runs — so it is where the step clock starts.
func (f *TomatoFormatter) Defined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepStart = time.Now()
	f.emit(Event{
		Type:      EventStepStart,
		Feature:   f.currentFeature,
		Scenario:  pickle.Name,
		Step:      step.Text,
		Status:    "running",
		StepIndex: f.stepIdx(pickle, step),
	})
}

// Passed is called when a step passes
func (f *TomatoFormatter) Passed(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsPassed++
	f.stepEnd(pickle, step, "passed", "", nil)
}

// Failed is called when a step fails
func (f *TomatoFormatter) Failed(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition, err error) {
	f.stepsFailed++
	f.scenarioHadFailure = true

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
		f.currentScenarioErr = errMsg
	}

	f.stepEnd(pickle, step, "failed", errMsg, mismatchOf(err))
}

// Skipped is called when a step is skipped
func (f *TomatoFormatter) Skipped(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsSkipped++
	f.stepEnd(pickle, step, "skipped", "", nil)
}

// Undefined is called when a step has no matching definition
func (f *TomatoFormatter) Undefined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsFailed++
	f.scenarioHadFailure = true
	f.currentScenarioErr = fmt.Sprintf("step undefined: %s", step.Text)

	f.stepEnd(pickle, step, "undefined", "step undefined", nil)
}

// Pending is called when a step is pending
func (f *TomatoFormatter) Pending(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsSkipped++
	f.stepEnd(pickle, step, "pending", "", nil)
}

// Ambiguous is called when a step matches multiple definitions
func (f *TomatoFormatter) Ambiguous(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition, err error) {
	f.stepsFailed++
	f.scenarioHadFailure = true

	errMsg := "ambiguous step"
	if err != nil {
		errMsg = err.Error()
		f.currentScenarioErr = errMsg
	}

	f.stepEnd(pickle, step, "ambiguous", errMsg, nil)
}

// Summary is called after all tests complete
func (f *TomatoFormatter) Summary() {
	// Emit end of last scenario
	f.emitScenarioEndIfNeeded()

	// Emit end of last feature
	if f.currentFeature != "" {
		f.emit(Event{
			Type:    EventFeatureEnd,
			Feature: f.currentFeature,
		})
	}

	// Emit summary
	f.emit(Event{
		Type:    EventSummary,
		Total:   f.scenarioTotal,
		Passed:  f.scenarioPassed,
		Failed:  f.scenarioFailed,
		Skipped: f.scenarioSkipped,
	})

	// Also print human-readable summary
	fmt.Fprintln(f.out)
	fmt.Fprintf(f.out, "%d scenarios (%d passed", f.scenarioTotal, f.scenarioPassed)
	if f.scenarioFailed > 0 {
		fmt.Fprintf(f.out, ", %d failed", f.scenarioFailed)
	}
	if f.scenarioSkipped > 0 {
		fmt.Fprintf(f.out, ", %d skipped", f.scenarioSkipped)
	}
	fmt.Fprintln(f.out, ")")

	totalSteps := f.stepsPassed + f.stepsFailed + f.stepsSkipped
	fmt.Fprintf(f.out, "%d steps (%d passed", totalSteps, f.stepsPassed)
	if f.stepsFailed > 0 {
		fmt.Fprintf(f.out, ", %d failed", f.stepsFailed)
	}
	if f.stepsSkipped > 0 {
		fmt.Fprintf(f.out, ", %d skipped", f.stepsSkipped)
	}
	fmt.Fprintln(f.out, ")")
}
