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
	Expression string `json:"expression" yaml:"expression"`
}

type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
}

type Option struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
}

type Action struct {
	Name        string      `json:"name" yaml:"name"`
	Handle      string      `json:"handle" yaml:"handle"`
	Description string      `json:"description"yaml:"description"`
	Expressions []string    `json:"expressions" yaml:"expressions"`
	Parameters  []Parameter `json:"parameters" yaml:"parameters"`
	Examples    []string    `json:"examples" yaml:"examples"`
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
	Name        string   `json:"name" yaml:"name"`
	Resources   []string `json:"resources" yaml:"resources"`
	Description string   `json:"description" yaml:"description"`
	Options     []Option `json:"options" yaml:"options"`
	Actions     []Action `json:"actions" yaml:"actions"`
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
	Handlers []Handler `json:"handlers" yaml:"handlers"`
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
