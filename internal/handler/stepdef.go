package handler

import (
	"strings"

	"github.com/cucumber/godog"
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

	// Handler is the function that implements the step
	Handler interface{} `json:"-"`
}

// StepCategory groups related steps together
type StepCategory struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Steps       []StepDef `json:"steps"`
}

// StepRegistry holds all registered step definitions from all handlers
type StepRegistry struct {
	categories []StepCategory
}

// NewStepRegistry creates a new step registry
func NewStepRegistry() *StepRegistry {
	return &StepRegistry{
		categories: make([]StepCategory, 0),
	}
}

// AddCategory adds a category of steps to the registry
func (r *StepRegistry) AddCategory(category StepCategory) {
	r.categories = append(r.categories, category)
}

// Categories returns all registered categories
func (r *StepRegistry) Categories() []StepCategory {
	return r.categories
}

// AllSteps returns all steps across all categories
func (r *StepRegistry) AllSteps() []StepDef {
	var all []StepDef
	for _, cat := range r.categories {
		all = append(all, cat.Steps...)
	}
	return all
}

// RegisterToGodog registers all steps with a godog scenario context
func (r *StepRegistry) RegisterToGodog(ctx *godog.ScenarioContext) {
	for _, cat := range r.categories {
		for _, step := range cat.Steps {
			ctx.Step(step.Pattern, step.Handler)
		}
	}
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
		ctx.Step(pattern, step.Handler)
	}
}

// FormatStepPattern replaces {resource} placeholder with the actual resource name
func FormatStepPattern(pattern, resourceName string) string {
	return strings.ReplaceAll(pattern, "{resource}", resourceName)
}

// FormatStepExample replaces {resource} placeholder with the actual resource name
func FormatStepExample(example, resourceName string) string {
	return strings.ReplaceAll(example, "{resource}", resourceName)
}

// DummyConfig returns a minimal config.Resource for documentation generation
func DummyConfig() config.Resource {
	return config.Resource{
		Type:    "dummy",
		Options: make(map[string]any),
	}
}
