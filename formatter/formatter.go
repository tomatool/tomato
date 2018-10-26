package formatter

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/errors"
)

func New(suite string, out io.Writer) godog.Formatter {
	return &tomatofmt{
		out:     out,
		started: time.Now(),
	}
}

var (
	red    = colors.Red
	redb   = colors.Bold(colors.Red)
	green  = colors.Green
	black  = colors.Black
	yellow = colors.Yellow
	cyan   = colors.Cyan
	cyanb  = colors.Bold(colors.Cyan)
	white  = colors.White
	whiteb = colors.Bold(colors.White)
)

type stepType int

const (
	passed stepType = iota
	failed
	skipped
	undefined
	pending
)

type feature struct {
	feature *gherkin.Feature
	path    string
}

type stepResult struct {
	typ     stepType
	feature *feature
	owner   interface{}
	step    *gherkin.Step
	def     *godog.StepDef
	err     error
}

func (f stepResult) line() string {
	return fmt.Sprintf("%s:%d", f.feature.path, f.step.Location.Line)
}

func (f stepResult) scenarioDesc() string {
	if sc, ok := f.owner.(*gherkin.Scenario); ok {
		return fmt.Sprintf("%s: %s", sc.Keyword, sc.Name)
	}

	if row, ok := f.owner.(*gherkin.TableRow); ok {
		for _, def := range f.feature.feature.ScenarioDefinitions {
			out, ok := def.(*gherkin.ScenarioOutline)
			if !ok {
				continue
			}

			for _, ex := range out.Examples {
				for _, rw := range ex.TableBody {
					if rw.Location.Line == row.Location.Line {
						return fmt.Sprintf("%s: %s", out.Keyword, out.Name)
					}
				}
			}
		}
	}
	return f.line() // was not expecting different owner
}

func (f stepResult) scenarioLine() string {
	if sc, ok := f.owner.(*gherkin.Scenario); ok {
		return fmt.Sprintf("%s:%d", f.feature.path, sc.Location.Line)
	}

	if row, ok := f.owner.(*gherkin.TableRow); ok {
		for _, def := range f.feature.feature.ScenarioDefinitions {
			out, ok := def.(*gherkin.ScenarioOutline)
			if !ok {
				continue
			}

			for _, ex := range out.Examples {
				for _, rw := range ex.TableBody {
					if rw.Location.Line == row.Location.Line {
						return fmt.Sprintf("%s:%d", f.feature.path, out.Location.Line)
					}
				}
			}
		}
	}
	return f.line() // was not expecting different owner
}

type tomatofmt struct {
	out    io.Writer
	owner  interface{}
	indent int

	started   time.Time
	features  []*feature
	failed    []*stepResult
	passed    []*stepResult
	skipped   []*stepResult
	undefined []*stepResult
	pending   []*stepResult
}

func (f *tomatofmt) Node(n interface{}) {
	switch t := n.(type) {
	case *gherkin.TableRow:
		f.owner = t
	case *gherkin.Scenario:
		f.owner = t
	}
}

func (f *tomatofmt) Feature(ft *gherkin.Feature, path string, c []byte) {
	f.features = append(f.features, &feature{ft, path})
}
func (f *tomatofmt) Defined(step *gherkin.Step, def *godog.StepDef) {}
func (f *tomatofmt) Failed(step *gherkin.Step, match *godog.StepDef, err error) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		err:     err,
		typ:     failed,
	}
	f.failed = append(f.failed, s)
}
func (f *tomatofmt) Passed(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     passed,
	}
	f.passed = append(f.passed, s)
}
func (f *tomatofmt) Skipped(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     skipped,
	}
	f.skipped = append(f.skipped, s)
}
func (f *tomatofmt) Undefined(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     undefined,
	}
	f.undefined = append(f.undefined, s)
}
func (f *tomatofmt) Pending(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     pending,
	}
	f.pending = append(f.pending, s)
}

func examples(ex interface{}) (*gherkin.Examples, bool) {
	t, ok := ex.(*gherkin.Examples)
	return t, ok
}

func (f *tomatofmt) Summary() {
	if len(f.undefined) > 0 {
		fmt.Fprintf(f.out, "\n")
		fmt.Fprintf(f.out, "\t%s\n", redb("Step is not defined on any of tomato resources step"))
		fmt.Fprintf(f.out, "\t%s\n", "")
		for idx, undef := range f.undefined {
			fmt.Fprintf(f.out, "\t\t%d) %s %s\n", (idx + 1), undef.scenarioDesc(), black("# "+undef.scenarioLine()))
			fmt.Fprintf(f.out, "\t\t   Step:\t%s\n\n", red(undef.step.Text))
		}
		fmt.Fprintf(f.out, "\t%s\n", ("please refer to " + whiteb("https://github.com/tomatool/tomato#resources") + " for list of resource steps"))
		fmt.Fprintf(f.out, "\n")
		return
	}
	if len(f.failed) > 0 {
		fmt.Fprintf(f.out, "\n")
		fmt.Fprintf(f.out, "\t%s\n", redb("Tomato tests failed"))
		fmt.Fprintf(f.out, "\t%s\n", "")
		for idx, failed := range f.failed {
			fmt.Fprintf(f.out, "\t\t%d) %s %s\n", (idx + 1), whiteb(failed.scenarioDesc()), black("# "+failed.scenarioLine()))
			fmt.Fprintf(f.out, "\t\t   Step:\t%s\n", white(failed.step.Text))
			if err, ok := failed.err.(*errors.Step); ok {
				fmt.Fprintf(f.out, "\t\t   Reason:\t%s\n", redb(failed.err))
				fmt.Fprintf(f.out, "\t\t   Detail:\n\n")
				for field, value := range err.Details {
					if len(err.Details) == 1 {
						fmt.Fprintf(f.out, "%s\n", indent((value), "\t\t\t"))
						break
					}
					fmt.Fprintf(f.out, "\t\t\t   %s:\n%s\n", (field), indent(redb(value), "\t\t\t\t"))
				}
				fmt.Fprintf(f.out, "\t\t\n")
			} else {
				fmt.Fprintf(f.out, "\t\t   Reason:\t%v\n", redb(failed.err))
			}
			fmt.Fprintf(f.out, "\n")
		}
		fmt.Fprintf(f.out, "\n")
		return
	}

	var total, passed, undefined int
	for _, ft := range f.features {
		for _, def := range ft.feature.ScenarioDefinitions {
			switch t := def.(type) {
			case *gherkin.Scenario:
				total++
			case *gherkin.ScenarioOutline:
				for _, ex := range t.Examples {
					if examples, hasExamples := examples(ex); hasExamples {
						total += len(examples.TableBody)
					}
				}
			}
		}
	}
	passed = total
	var owner interface{}
	for _, undef := range f.undefined {
		if owner != undef.owner {
			undefined++
			owner = undef.owner
		}
	}

	var steps, parts, scenarios []string
	nsteps := len(f.passed) + len(f.failed) + len(f.skipped) + len(f.undefined) + len(f.pending)
	if len(f.passed) > 0 {
		steps = append(steps, green(fmt.Sprintf("%d passed", len(f.passed))))
	}
	if len(f.failed) > 0 {
		passed -= len(f.failed)
		parts = append(parts, red(fmt.Sprintf("%d failed", len(f.failed))))
		steps = append(steps, parts[len(parts)-1])
	}
	if len(f.pending) > 0 {
		passed -= len(f.pending)
		parts = append(parts, yellow(fmt.Sprintf("%d pending", len(f.pending))))
		steps = append(steps, yellow(fmt.Sprintf("%d pending", len(f.pending))))
	}
	if len(f.undefined) > 0 {
		passed -= undefined
		parts = append(parts, yellow(fmt.Sprintf("%d undefined", undefined)))
		steps = append(steps, yellow(fmt.Sprintf("%d undefined", len(f.undefined))))
	}
	if len(f.skipped) > 0 {
		steps = append(steps, cyan(fmt.Sprintf("%d skipped", len(f.skipped))))
	}
	if passed > 0 {
		scenarios = append(scenarios, green(fmt.Sprintf("%d passed", passed)))
	}
	scenarios = append(scenarios, parts...)
	elapsed := time.Since(f.started)

	fmt.Fprintln(f.out, "")
	if total == 0 {
		fmt.Fprintln(f.out, "No scenarios")
	} else {
		fmt.Fprintln(f.out, fmt.Sprintf("%d scenarios (%s)", total, strings.Join(scenarios, ", ")))
	}

	if nsteps == 0 {
		fmt.Fprintln(f.out, "No steps")
	} else {
		fmt.Fprintln(f.out, fmt.Sprintf("%d steps (%s)", nsteps, strings.Join(steps, ", ")))
	}
	fmt.Fprintln(f.out, elapsed)

	// prints used randomization seed
	seed, err := strconv.ParseInt(os.Getenv("GODOG_SEED"), 10, 64)
	if err == nil && seed != 0 {
		fmt.Fprintln(f.out, "")
		fmt.Fprintln(f.out, "Randomized with seed:", colors.Yellow(seed))
	}
}

// indents a block of text with an indent string
func indent(text, indent string) string {
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}
