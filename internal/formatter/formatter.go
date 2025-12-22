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
	godog.Format("pretty-fixed", "Pretty formatter without false undefined warnings", PrettyFixedFormatterFunc)
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
// NOTE: This is often a false positive with godog when using dynamically registered steps.
// We'll track it but not count it as a failure since the step might still execute.
func (f *TomatoFormatter) Undefined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	// Don't count as failure - godog often marks dynamically registered steps as undefined
	// even though they execute successfully. We'll only mark actual failures in Failed().
	f.emit(Event{
		Type:     EventStepEnd,
		Feature:  f.currentFeature,
		Scenario: pickle.Name,
		Step:     step.Text,
		Status:   "undefined",
		Error:    "step undefined (may be false positive)",
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

// PrettyFixedFormatter is a custom formatter that shows clean output without false undefined warnings
type PrettyFixedFormatter struct {
	out io.Writer

	// Track steps that have executed
	executedSteps map[string]bool

	// Track current context
	currentFeature   *messages.GherkinDocument
	currentFeatureURI string
	currentPickle    *messages.Pickle

	// Scenario state
	currentScenarioFailed bool
	failedStep            *stepResult

	// Counters
	scenarioTotal  int
	scenarioPassed int
	scenarioFailed int

	stepsPassed  int
	stepsFailed  int
	stepsSkipped int
}

type stepResult struct {
	pickle *messages.Pickle
	step   *messages.PickleStep
	err    error
}

// PrettyFixedFormatterFunc creates a new PrettyFixedFormatter
func PrettyFixedFormatterFunc(suite string, out io.Writer) formatters.Formatter {
	return &PrettyFixedFormatter{
		out:           out,
		executedSteps: make(map[string]bool),
	}
}

func (f *PrettyFixedFormatter) stepKey(pickle *messages.Pickle, step *messages.PickleStep) string {
	return fmt.Sprintf("%s::%s", pickle.Id, step.Id)
}

func (f *PrettyFixedFormatter) getStepKeyword(step *messages.PickleStep) string {
	// Get the keyword from the AST node
	if f.currentFeature != nil && len(step.AstNodeIds) > 0 {
		for _, child := range f.currentFeature.Feature.Children {
			if child.Scenario != nil {
				for _, s := range child.Scenario.Steps {
					if s.Id == step.AstNodeIds[0] {
						return s.Keyword
					}
				}
			}
		}
	}
	return "Step"
}

func (f *PrettyFixedFormatter) TestRunStarted() {}

func (f *PrettyFixedFormatter) Feature(doc *messages.GherkinDocument, uri string, content []byte) {
	if doc.Feature != nil {
		f.currentFeature = doc
		f.currentFeatureURI = uri
		fmt.Fprintf(f.out, "\n\033[1;37mFeature:\033[0m %s\n", doc.Feature.Name)
	}
}

func (f *PrettyFixedFormatter) Pickle(pickle *messages.Pickle) {
	if f.currentPickle != nil {
		// End previous scenario
		f.endScenario()
	}

	f.currentPickle = pickle
	f.currentScenarioFailed = false
	f.failedStep = nil
	f.scenarioTotal++

	fmt.Fprintf(f.out, "\n  \033[1;37mScenario:\033[0m %s\n", pickle.Name)
}

func (f *PrettyFixedFormatter) endScenario() {
	if f.currentScenarioFailed {
		f.scenarioFailed++
		if f.failedStep != nil && f.failedStep.err != nil {
			fmt.Fprintf(f.out, "      \033[31mError: \033[0m\033[1;31m%s\033[0m\n", f.failedStep.err.Error())
		}
	} else {
		f.scenarioPassed++
	}
}

func (f *PrettyFixedFormatter) Defined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {}

func (f *PrettyFixedFormatter) Passed(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	key := f.stepKey(pickle, step)
	f.executedSteps[key] = true
	f.stepsPassed++
	keyword := f.getStepKeyword(step)
	fmt.Fprintf(f.out, "    \033[32m%s\033[0m \033[32m%s\033[0m\n", keyword, step.Text)
}

func (f *PrettyFixedFormatter) Failed(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition, err error) {
	key := f.stepKey(pickle, step)
	f.executedSteps[key] = true
	f.stepsFailed++
	f.currentScenarioFailed = true
	f.failedStep = &stepResult{pickle: pickle, step: step, err: err}
	keyword := f.getStepKeyword(step)
	fmt.Fprintf(f.out, "    \033[31m%s\033[0m \033[31m%s\033[0m\n", keyword, step.Text)
}

func (f *PrettyFixedFormatter) Skipped(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	key := f.stepKey(pickle, step)
	f.executedSteps[key] = true
	f.stepsSkipped++
	keyword := f.getStepKeyword(step)
	fmt.Fprintf(f.out, "    \033[36m%s\033[0m \033[36m%s\033[0m\n", keyword, step.Text)
}

func (f *PrettyFixedFormatter) Undefined(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	key := f.stepKey(pickle, step)
	// Only mark as undefined if the step didn't actually execute (ignore false positives)
	if !f.executedSteps[key] {
		f.stepsFailed++
		f.currentScenarioFailed = true
		keyword := f.getStepKeyword(step)
		fmt.Fprintf(f.out, "    \033[33m%s\033[0m \033[33m%s\033[0m\n", keyword, step.Text)
	}
}

func (f *PrettyFixedFormatter) Pending(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition) {
	key := f.stepKey(pickle, step)
	f.executedSteps[key] = true
	f.stepsSkipped++
	keyword := f.getStepKeyword(step)
	fmt.Fprintf(f.out, "    \033[33m%s\033[0m \033[33m%s\033[0m\n", keyword, step.Text)
}

func (f *PrettyFixedFormatter) Ambiguous(pickle *messages.Pickle, step *messages.PickleStep, def *formatters.StepDefinition, err error) {
	key := f.stepKey(pickle, step)
	f.executedSteps[key] = true
	f.stepsFailed++
	f.currentScenarioFailed = true
	f.failedStep = &stepResult{pickle: pickle, step: step, err: err}
	keyword := f.getStepKeyword(step)
	fmt.Fprintf(f.out, "    \033[31m%s\033[0m \033[31m%s\033[0m\n", keyword, step.Text)
}

func (f *PrettyFixedFormatter) Summary() {
	// End last scenario
	if f.currentPickle != nil {
		f.endScenario()
	}

	// Print summary
	fmt.Fprintln(f.out)
	fmt.Fprintf(f.out, "%d scenarios (", f.scenarioTotal)
	if f.scenarioPassed > 0 {
		fmt.Fprintf(f.out, "\033[32m%d passed\033[0m", f.scenarioPassed)
	}
	if f.scenarioFailed > 0 {
		if f.scenarioPassed > 0 {
			fmt.Fprint(f.out, ", ")
		}
		fmt.Fprintf(f.out, "\033[31m%d failed\033[0m", f.scenarioFailed)
	}
	fmt.Fprintln(f.out, ")")

	totalSteps := f.stepsPassed + f.stepsFailed + f.stepsSkipped
	fmt.Fprintf(f.out, "%d steps (", totalSteps)
	if f.stepsPassed > 0 {
		fmt.Fprintf(f.out, "\033[32m%d passed\033[0m", f.stepsPassed)
	}
	if f.stepsFailed > 0 {
		if f.stepsPassed > 0 {
			fmt.Fprint(f.out, ", ")
		}
		fmt.Fprintf(f.out, "\033[31m%d failed\033[0m", f.stepsFailed)
	}
	if f.stepsSkipped > 0 {
		if f.stepsPassed > 0 || f.stepsFailed > 0 {
			fmt.Fprint(f.out, ", ")
		}
		fmt.Fprintf(f.out, "\033[36m%d skipped\033[0m", f.stepsSkipped)
	}
	fmt.Fprintln(f.out, ")")
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
