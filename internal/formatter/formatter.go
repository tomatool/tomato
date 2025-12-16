package formatter

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/formatters"
	messages "github.com/cucumber/messages/go/v21"
)

// Event types for structured output
const (
	EventFeatureStart  = "feature_start"
	EventFeatureEnd    = "feature_end"
	EventScenarioStart = "scenario_start"
	EventScenarioEnd   = "scenario_end"
	EventStepEnd       = "step_end"
	EventSummary       = "summary"
)

// Event represents a structured test event
type Event struct {
	Type     string `json:"type"`
	Feature  string `json:"feature,omitempty"`
	Scenario string `json:"scenario,omitempty"`
	Step     string `json:"step,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	File     string `json:"file,omitempty"`

	// Summary fields
	Total   int `json:"total,omitempty"`
	Passed  int `json:"passed,omitempty"`
	Failed  int `json:"failed,omitempty"`
	Skipped int `json:"skipped,omitempty"`
}

// TomatoFormatter outputs structured JSON events for UI parsing
type TomatoFormatter struct {
	out io.Writer

	// Track current context
	currentFeature     string
	currentFeatureFile string
	currentScenario    string
	currentScenarioErr string

	// Track scenario status
	scenarioHadFailure bool

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
	}
}

func (f *TomatoFormatter) emit(event Event) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(f.out, "TOMATO_EVENT:%s\n", string(data))
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

	f.emit(Event{
		Type:     EventScenarioEnd,
		Feature:  f.currentFeature,
		Scenario: f.currentScenario,
		Status:   status,
		Error:    f.currentScenarioErr,
	})

	// Update counters
	if f.scenarioHadFailure {
		f.scenarioFailed++
	} else {
		f.scenarioPassed++
	}

	f.currentScenario = ""
}

// Defined is called when a step definition is found
func (f *TomatoFormatter) Defined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
}

// Passed is called when a step passes
func (f *TomatoFormatter) Passed(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsPassed++
	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "passed",
	})
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

	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "failed",
		Error:    errMsg,
	})
}

// Skipped is called when a step is skipped
func (f *TomatoFormatter) Skipped(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsSkipped++
	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "skipped",
	})
}

// Undefined is called when a step has no matching definition
func (f *TomatoFormatter) Undefined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsFailed++
	f.scenarioHadFailure = true
	f.currentScenarioErr = fmt.Sprintf("step undefined: %s", step.Text)

	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "undefined",
		Error:    "step undefined",
	})
}

// Pending is called when a step is pending
func (f *TomatoFormatter) Pending(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	f.stepsSkipped++
	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "pending",
	})
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

	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "ambiguous",
		Error:    errMsg,
	})
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
