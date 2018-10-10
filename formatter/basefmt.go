//
// this file is copied from https://github.com/DATA-DOG/godog/blob/master/fmt_progress.go
// and modified for tomato needs.
//

package formatter

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/DATA-DOG/godog/gherkin"
)

var timeNowFunc = func() time.Time {
	return time.Now()
}

// some snippet formatting regexps
var snippetExprCleanup = regexp.MustCompile("([\\/\\[\\]\\(\\)\\\\^\\$\\.\\|\\?\\*\\+\\'])")
var snippetExprQuoted = regexp.MustCompile("(\\W|^)\"(?:[^\"]*)\"(\\W|$)")
var snippetMethodName = regexp.MustCompile("[^a-zA-Z\\_\\ ]")
var snippetNumbers = regexp.MustCompile("(\\d+)")

var snippetHelperFuncs = template.FuncMap{
	"backticked": func(s string) string {
		return "`" + s + "`"
	},
}

var undefinedSnippetsTpl = template.Must(template.New("snippets").Funcs(snippetHelperFuncs).Parse(`
{{ range . }}func {{ .Method }}({{ .Args }}) error {
	return godog.ErrPending
}
{{end}}func FeatureContext(s *godog.Suite) { {{ range . }}
	s.Step({{ backticked .Expr }}, {{ .Method }}){{end}}
}
`))

var (
	red    = colors.Red
	redb   = colors.Bold(colors.Red)
	green  = colors.Green
	black  = colors.Black
	yellow = colors.Yellow
	cyan   = colors.Cyan
	cyanb  = colors.Bold(colors.Cyan)
	whiteb = colors.Bold(colors.White)
)

type feature struct {
	*gherkin.Feature
	Content   []byte `json:"-"`
	Path      string `json:"path"`
	scenarios map[int]bool
	order     int
}

type undefinedSnippet struct {
	Method   string
	Expr     string
	argument interface{} // gherkin step argument
}

type stepType int

const (
	passed stepType = iota
	failed
	skipped
	undefined
	pending
)

type stepResult struct {
	typ     stepType
	feature *feature
	owner   interface{}
	step    *gherkin.Step
	def     *godog.StepDef
	err     error
}

func (f stepResult) line() string {
	return fmt.Sprintf("%s:%d", f.feature.Path, f.step.Location.Line)
}

func (f stepResult) scenarioDesc() string {
	if sc, ok := f.owner.(*gherkin.Scenario); ok {
		return fmt.Sprintf("%s: %s", sc.Keyword, sc.Name)
	}

	if row, ok := f.owner.(*gherkin.TableRow); ok {
		for _, def := range f.feature.Feature.ScenarioDefinitions {
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
		return fmt.Sprintf("%s:%d", f.feature.Path, sc.Location.Line)
	}

	if row, ok := f.owner.(*gherkin.TableRow); ok {
		for _, def := range f.feature.Feature.ScenarioDefinitions {
			out, ok := def.(*gherkin.ScenarioOutline)
			if !ok {
				continue
			}

			for _, ex := range out.Examples {
				for _, rw := range ex.TableBody {
					if rw.Location.Line == row.Location.Line {
						return fmt.Sprintf("%s:%d", f.feature.Path, out.Location.Line)
					}
				}
			}
		}
	}
	return f.line() // was not expecting different owner
}

type basefmt struct {
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

func (f *basefmt) Node(n interface{}) {
	switch t := n.(type) {
	case *gherkin.TableRow:
		f.owner = t
	case *gherkin.Scenario:
		f.owner = t
	}
}

func (f *basefmt) Defined(*gherkin.Step, *godog.StepDef) {

}

func (f *basefmt) Feature(ft *gherkin.Feature, p string, c []byte) {
	f.features = append(f.features, &feature{Path: p, Feature: ft})
}

func (f *basefmt) Passed(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     passed,
	}
	f.passed = append(f.passed, s)
}

func (f *basefmt) Skipped(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     skipped,
	}
	f.skipped = append(f.skipped, s)
}

func (f *basefmt) Undefined(step *gherkin.Step, match *godog.StepDef) {
	s := &stepResult{
		owner:   f.owner,
		feature: f.features[len(f.features)-1],
		step:    step,
		def:     match,
		typ:     undefined,
	}
	f.undefined = append(f.undefined, s)
}

func (f *basefmt) Failed(step *gherkin.Step, match *godog.StepDef, err error) {
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

func (f *basefmt) Pending(step *gherkin.Step, match *godog.StepDef) {
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

func (f *basefmt) Summary() {
	var total, passed, undefined int
	for _, ft := range f.features {
		for _, def := range ft.ScenarioDefinitions {
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
	elapsed := timeNowFunc().Sub(f.started)

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

	if text := f.snippets(); text != "" {
		fmt.Fprintln(f.out, yellow("\nYou can implement step definitions for undefined steps with these snippets:"))
		fmt.Fprintln(f.out, yellow(text))
	}
}

func (s *undefinedSnippet) Args() (ret string) {
	var (
		args      []string
		pos       int
		breakLoop bool
	)
	for !breakLoop {
		part := s.Expr[pos:]
		ipos := strings.Index(part, "(\\d+)")
		spos := strings.Index(part, "\"([^\"]*)\"")
		switch {
		case spos == -1 && ipos == -1:
			breakLoop = true
		case spos == -1:
			pos += ipos + len("(\\d+)")
			args = append(args, reflect.Int.String())
		case ipos == -1:
			pos += spos + len("\"([^\"]*)\"")
			args = append(args, reflect.String.String())
		case ipos < spos:
			pos += ipos + len("(\\d+)")
			args = append(args, reflect.Int.String())
		case spos < ipos:
			pos += spos + len("\"([^\"]*)\"")
			args = append(args, reflect.String.String())
		}
	}
	if s.argument != nil {
		switch s.argument.(type) {
		case *gherkin.DocString:
			args = append(args, "*gherkin.DocString")
		case *gherkin.DataTable:
			args = append(args, "*gherkin.DataTable")
		}
	}

	var last string
	for i, arg := range args {
		if last == "" || last == arg {
			ret += fmt.Sprintf("arg%d, ", i+1)
		} else {
			ret = strings.TrimRight(ret, ", ") + fmt.Sprintf(" %s, arg%d, ", last, i+1)
		}
		last = arg
	}
	return strings.TrimSpace(strings.TrimRight(ret, ", ") + " " + last)
}

func (f *basefmt) snippets() string {
	if len(f.undefined) == 0 {
		return ""
	}

	var index int
	var snips []*undefinedSnippet
	// build snippets
	for _, u := range f.undefined {
		steps := []string{u.step.Text}
		arg := u.step.Argument
		if u.def != nil {
			steps = nil
			arg = nil
		}
		for _, step := range steps {
			expr := snippetExprCleanup.ReplaceAllString(step, "\\$1")
			expr = snippetNumbers.ReplaceAllString(expr, "(\\d+)")
			expr = snippetExprQuoted.ReplaceAllString(expr, "$1\"([^\"]*)\"$2")
			expr = "^" + strings.TrimSpace(expr) + "$"

			name := snippetNumbers.ReplaceAllString(step, " ")
			name = snippetExprQuoted.ReplaceAllString(name, " ")
			name = strings.TrimSpace(snippetMethodName.ReplaceAllString(name, ""))
			var words []string
			for i, w := range strings.Split(name, " ") {
				switch {
				case i != 0:
					w = strings.Title(w)
				case len(w) > 0:
					w = string(unicode.ToLower(rune(w[0]))) + w[1:]
				}
				words = append(words, w)
			}
			name = strings.Join(words, "")
			if len(name) == 0 {
				index++
				name = fmt.Sprintf("stepDefinition%d", index)
			}

			var found bool
			for _, snip := range snips {
				if snip.Expr == expr {
					found = true
					break
				}
			}
			if !found {
				snips = append(snips, &undefinedSnippet{Method: name, Expr: expr, argument: arg})
			}
		}
	}

	var buf bytes.Buffer
	if err := undefinedSnippetsTpl.Execute(&buf, snips); err != nil {
		panic(err)
	}
	// there may be trailing spaces
	return strings.Replace(buf.String(), " \n", "\n", -1)
}

func (f *basefmt) isLastStep(s *gherkin.Step) bool {
	ft := f.features[len(f.features)-1]

	for _, def := range ft.ScenarioDefinitions {
		if outline, ok := def.(*gherkin.ScenarioOutline); ok {
			for n, step := range outline.Steps {
				if step.Location.Line == s.Location.Line {
					return n == len(outline.Steps)-1
				}
			}
		}

		if scenario, ok := def.(*gherkin.Scenario); ok {
			for n, step := range scenario.Steps {
				if step.Location.Line == s.Location.Line {
					return n == len(scenario.Steps)-1
				}
			}
		}
	}
	return false
}
