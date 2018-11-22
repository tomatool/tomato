package dictionary

import (
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

var (
	Base        Dictionary
	expressions = map[string]string{
		"string":   `"([^"]*)"`,
		"number":   `(\d+)`,
		"duration": `(["]*)`,
		"json":     "$",
		"table":    "$",
	}
	parameters = []Parameter{
		{
			Name:        "resource",
			Description: "selected resource that is going to be used",
			Type:        "string",
		},
	}
)

type Type struct {
	Expression string `yaml:"expression"`
}

type Parameter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
}

type Option struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
}

type Action struct {
	Name        string      `yaml:"name"`
	Handle      string      `yaml:"handle"`
	Description string      `yaml:"description"`
	Expressions []string    `yaml:"expressions"`
	Parameters  []Parameter `yaml:"parameters"`
	Examples    []string    `yaml:"examples"`
}

func (a *Action) Expr() []string {
	if a == nil {
		return []string{}
	}

	var result []string
	for _, expr := range a.Expressions {
		var rendered []string
		for _, word := range strings.Split(expr, " ") {
			if word[0] == '$' {
				param := a.Param(word[1:])
				e := expressions[param.Type]
				if e == "$" {
					continue
				}
				rendered = append(rendered, e)
				continue
			}
			rendered = append(rendered, word)
		}
		renderedExpr := strings.Join(rendered, " ")
		if renderedExpr[len(renderedExpr)-1] != '$' {
			renderedExpr += "$"
		}

		result = append(result, renderedExpr)
	}
	return result
}

func (a *Action) Param(name string) *Parameter {
	for _, param := range append(a.Parameters, parameters...) {
		if param.Name == name {
			return &param
		}
	}
	return nil
}

type Handler struct {
	Name        string   `yaml:"name"`
	Resources   []string `yaml:"resources"`
	Description string   `yaml:"description"`
	Options     []Option `yaml:"options"`
	Actions     []Action `yaml:"actions"`
}

func (r *Handler) Action(name string) *Action {
	if r == nil {
		return nil
	}
	for _, a := range r.Actions {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

type Dictionary struct {
	Handlers []Handler `yaml:"handlers"`
}

func Retrieve(filepath string) (*Dictionary, error) {
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(b, &Base); err != nil {
		return nil, errors.Wrapf(err, "unmarshal yaml : %s", string(b))
	}

	return &Base, nil
}
