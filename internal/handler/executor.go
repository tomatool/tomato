package handler

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
)

// Executor runs a single step by its text, without godog or a feature file.
// It is what `tomato serve` uses to let code in another language (a Kotlin
// test, say) drive tomato's resources: same step definitions, same handler
// instances, same argument conversion as a Gherkin run.
type Executor struct {
	steps []boundStep
}

type boundStep struct {
	resource string
	def      StepDef
	re       *regexp.Regexp
}

// StepCall is one step to execute: the step text as it would appear in a
// feature file (without Given/When/Then), plus an optional docstring or table.
type StepCall struct {
	Text      string     `json:"step"`
	DocString *string    `json:"docString,omitempty"`
	Table     [][]string `json:"table,omitempty"`
}

// BoundStepInfo describes a step as bound to a concrete resource name.
type BoundStepInfo struct {
	Resource    string `json:"resource"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
}

// NewExecutor binds every step of every handler in the registry to its
// resource name.
func NewExecutor(r *Registry) (*Executor, error) {
	e := &Executor{}
	for _, name := range r.Names() {
		h, _ := r.Get(name)
		provider, ok := h.(StepProvider)
		if !ok {
			continue
		}
		for _, def := range provider.Steps().Steps {
			pattern := FormatStepPattern(def.Pattern, name)
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("resource %q step %q: %w", name, def.Pattern, err)
			}
			e.steps = append(e.steps, boundStep{resource: name, def: def, re: re})
		}
	}
	return e, nil
}

// Steps lists every bound step, sorted by resource then pattern.
func (e *Executor) Steps() []BoundStepInfo {
	out := make([]BoundStepInfo, 0, len(e.steps))
	for _, s := range e.steps {
		out = append(out, BoundStepInfo{
			Resource:    s.resource,
			Pattern:     s.re.String(),
			Description: s.def.Description,
			Example:     FormatStepExample(s.def.Example, s.resource),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Resource < out[j].Resource })
	return out
}

// Run executes one step. The error is the step's failure, exactly as it
// would be reported in a Gherkin run.
func (e *Executor) Run(call StepCall) error {
	text := strings.TrimSpace(call.Text)
	var match *boundStep
	var groups []string
	for i := range e.steps {
		s := &e.steps[i]
		if m := s.re.FindStringSubmatch(text); m != nil {
			if match != nil {
				return fmt.Errorf("step %q is ambiguous: matches %q and %q", text, match.re, s.re)
			}
			match, groups = s, m[1:]
		}
	}
	if match == nil {
		return fmt.Errorf("undefined step: %q", text)
	}
	return invoke(match.def.stepHandler(match.re.String()), groups, call)
}

// invoke calls a step handler with regex captures converted to its parameter
// types, followed by the docstring or table when it takes one.
func invoke(fn interface{}, groups []string, call StepCall) error {
	v := reflect.ValueOf(fn)
	t := v.Type()
	args := make([]reflect.Value, 0, t.NumIn())
	g := 0
	for i := 0; i < t.NumIn(); i++ {
		p := t.In(i)
		switch {
		case p == reflect.TypeOf((*godog.DocString)(nil)):
			if call.DocString == nil {
				return fmt.Errorf("step needs a docString")
			}
			args = append(args, reflect.ValueOf(&godog.DocString{Content: *call.DocString}))
		case p == reflect.TypeOf((*godog.Table)(nil)):
			if call.Table == nil {
				return fmt.Errorf("step needs a table")
			}
			args = append(args, reflect.ValueOf(toTable(call.Table)))
		default:
			if g >= len(groups) {
				return fmt.Errorf("step handler takes more arguments than the pattern captures")
			}
			val, err := convertArg(groups[g], p)
			if err != nil {
				return err
			}
			args = append(args, val)
			g++
		}
	}
	out := v.Call(args)
	if len(out) == 1 && !out[0].IsNil() {
		return out[0].Interface().(error)
	}
	return nil
}

func convertArg(s string, t reflect.Type) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(s).Convert(t), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%q is not an integer", s)
		}
		return reflect.ValueOf(n).Convert(t), nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%q is not a number", s)
		}
		return reflect.ValueOf(f).Convert(t), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("%q is not a boolean", s)
		}
		return reflect.ValueOf(b), nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return reflect.ValueOf([]byte(s)), nil
		}
	}
	return reflect.Value{}, fmt.Errorf("unsupported step argument type %s", t)
}

func toTable(rows [][]string) *godog.Table {
	table := &godog.Table{}
	for _, row := range rows {
		r := &messages.PickleTableRow{}
		for _, cell := range row {
			r.Cells = append(r.Cells, &messages.PickleTableCell{Value: cell})
		}
		table.Rows = append(table.Rows, r)
	}
	return table
}
