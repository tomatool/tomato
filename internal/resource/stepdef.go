package resource

import (
	"reflect"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
)

// StepDef represents a structured step definition with metadata
type StepDef struct {
	// Group is the category within a handler (e.g., "Request Setup", "Response Assertions")
	Group string `json:"group,omitempty"`

	// Pattern is the regex pattern for matching Gherkin steps
	// Use {resource} as placeholder for the resource name
	Pattern string `json:"pattern"`

	// Description explains what this step does
	Description string `json:"description"`

	// Example shows how to use this step in a feature file
	Example string `json:"example,omitempty"`

	// Deprecated, when set, says what to use instead. The step keeps working
	// (see docs/stability.md: at least one minor release) but logs a warning
	// the first time it runs.
	Deprecated string `json:"deprecated,omitempty"`

	// Handler is the function that implements the step
	Handler interface{} `json:"-"`
}

// warnedDeprecated remembers which deprecated patterns already warned, so a
// step used in every scenario warns once per run, not once per use.
var warnedDeprecated sync.Map

// stepHandler returns the function to register with godog: a wrapper with
// the handler's signature that warns first when the step is deprecated, and
// adds the network faults that were active to the error of a step that
// failed, so a failure is never read without knowing the network was bad.
func (s StepDef) stepHandler(pattern string) interface{} {
	fn := reflect.ValueOf(s.Handler)
	if s.Handler == nil || fn.Kind() != reflect.Func {
		return s.Handler
	}
	t := fn.Type()
	returnsError := t.NumOut() > 0 && t.Out(t.NumOut()-1) == errorType
	if s.Deprecated == "" && !returnsError {
		return s.Handler
	}
	return reflect.MakeFunc(t, func(args []reflect.Value) []reflect.Value {
		if s.Deprecated != "" {
			if _, seen := warnedDeprecated.LoadOrStore(pattern, true); !seen {
				log.Warn().Str("step", pattern).Msgf("deprecated step: %s", s.Deprecated)
			}
		}
		var out []reflect.Value
		if t.IsVariadic() {
			out = fn.CallSlice(args)
		} else {
			out = fn.Call(args)
		}
		if returnsError {
			last := out[len(out)-1]
			if err, ok := last.Interface().(error); ok && err != nil {
				if wrapped := WithNetworkFaults(err); wrapped != err {
					out[len(out)-1] = reflect.ValueOf(&wrapped).Elem()
				}
			}
		}
		return out
	}).Interface()
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// StepCategory groups related steps together
type StepCategory struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Steps       []StepDef `json:"steps"`
}

// StepProvider is implemented by handlers that provide structured step definitions
type StepProvider interface {
	// Steps returns the structured step definitions for this handler
	Steps() StepCategory
}

// RegisterStepsToGodog registers steps from a StepCategory to godog, replacing {resource} placeholder
func RegisterStepsToGodog(ctx *godog.ScenarioContext, resourceName string, category StepCategory) {
	for _, step := range category.Steps {
		pattern := strings.ReplaceAll(step.Pattern, "{resource}", resourceName)
		ctx.Step(pattern, step.stepHandler(pattern))
	}
}

// DummyConfig returns a minimal config.Resource for documentation generation
func DummyConfig() config.Resource {
	return config.Resource{
		Type:    "dummy",
		Options: make(map[string]any),
	}
}
