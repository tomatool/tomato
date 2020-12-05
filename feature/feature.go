package feature

import (
	"fmt"
	"strings"

	"github.com/cucumber/gherkin-go"
	"github.com/schollz/closestmatch"
	"github.com/tomatool/tomato/dictionary"
)

type Feature struct {
	Name      string      `json:"name"`
	Scenarios []*Scenario `json:"scenarios"`
}

type Scenario struct {
	Name  string  `json:"name"`
	Steps []*Step `json:"steps"`
}

type Step struct {
	Action     dictionary.Action `json:"action"`
	Expr       string            `json:"expr"`
	Resource   string            `json:"resource"`
	Text       string            `json:"text"`
	Parameters []string          `json:"parameters"`
}

type Parser struct {
	Dictionary *dictionary.Dictionary

	actionMap   map[string]action
	expressions []string
}

func (p *Parser) Parse(doc *gherkin.GherkinDocument) (*Feature, error) {
	scenarios, err := p.parseScenarios(doc.Feature.Children)
	if err != nil {
		return nil, err
	}
	f := &Feature{
		Name:      doc.Feature.Name,
		Scenarios: scenarios,
	}
	return f, nil
}

func (p *Parser) parseScenarios(scenarios []interface{}) (resp []*Scenario, err error) {
	for _, s := range scenarios {
		scenario, ok := s.(*gherkin.Scenario)
		if !ok {
			return nil, fmt.Errorf("Type is not scenario")
		}
		steps, err := p.parseSteps(scenario.Steps)
		if err != nil {
			return nil, err
		}
		resp = append(resp, &Scenario{
			Name:  scenario.Name,
			Steps: steps,
		})
	}
	return resp, nil
}

type action struct {
	Handler dictionary.Handler
	Action  dictionary.Action
}

func (p *Parser) getActionMap() (map[string]action, []string) {
	if p.actionMap != nil {
		return p.actionMap, p.expressions
	}

	p.actionMap = make(map[string]action)
	for _, h := range p.Dictionary.Handlers {
		for _, a := range h.Actions {
			for _, expr := range a.Expressions {
				p.actionMap[expr] = action{h, a}
				p.expressions = append(p.expressions, expr)
			}
		}
	}

	return p.actionMap, p.expressions
}

func (p *Parser) parseStep(str string) (*Step, error) {
	actionMap, expressions := p.getActionMap()

	cm := closestmatch.New(expressions, []int{6})
	key := cm.Closest(str)

	action := actionMap[key]

	var tokens []string
	for _, s := range strings.Split(str, " ") {
		if s[0] == '"' && s[len(s)-1] == '"' {
			tokens = append(tokens, strings.TrimPrefix(strings.TrimSuffix(s, "\""), "\""))
		}
	}

	return &Step{
		Action:     action.Action,
		Text:       str,
		Resource:   tokens[0],
		Parameters: tokens[1:],
	}, nil
}

func (p *Parser) parseSteps(steps []*gherkin.Step) (resp []*Step, err error) {
	for _, s := range steps {
		step, err := p.parseStep(s.Text)
		if err != nil {
			return nil, err
		}

		arg, ok := s.Argument.(*gherkin.DocString)
		if ok {
			step.Parameters = append(step.Parameters, arg.Content)
		}

		resp = append(resp, step)
	}
	return resp, nil
}
