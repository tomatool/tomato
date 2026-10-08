package command

import (
	"regexp"
	"sort"
	"strings"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/handler"
)

// stepRegexp compiles a step pattern for one resource name.
func stepRegexp(def handler.StepDef, resource string) (*regexp.Regexp, error) {
	return regexp.Compile(strings.ReplaceAll(def.Pattern, "{resource}", regexp.QuoteMeta(resource)))
}

// stepMatch is the step definition a line of a feature file runs, and the
// resource it runs against.
type stepMatch struct {
	Resource string
	Type     string
	Def      handler.StepDef
}

type stepMatcherEntry struct {
	re    *regexp.Regexp
	match stepMatch
}

// stepMatcher finds the step definition behind a step's text, the way godog
// will at run time, without starting anything.
type stepMatcher struct {
	entries []stepMatcherEntry
}

func newStepMatcher(resources map[string]config.Resource) *stepMatcher {
	names := make([]string, 0, len(resources))
	for name := range resources {
		names = append(names, name)
	}
	sort.Strings(names)

	m := &stepMatcher{}
	for _, name := range names {
		typ := resources[name].Type
		cat, ok := handler.StepCategoryForType(typ)
		if !ok {
			continue
		}
		for _, def := range cat.Steps {
			re, err := stepRegexp(def, name)
			if err != nil {
				continue
			}
			m.entries = append(m.entries, stepMatcherEntry{
				re:    re,
				match: stepMatch{Resource: name, Type: typ, Def: def},
			})
		}
	}
	return m
}

func (m *stepMatcher) match(text string) (stepMatch, bool) {
	for _, e := range m.entries {
		if e.re.MatchString(text) {
			return e.match, true
		}
	}
	return stepMatch{}, false
}
