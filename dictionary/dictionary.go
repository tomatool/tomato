package dictionary

import (
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

var (
	Base Dictionary
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
				e := Base.ExpressionMap[param.Type]
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
	for _, param := range append(a.Parameters, Base.Resources.Parameters...) {
		if param.Name == name {
			return &param
		}
	}
	return nil
}

type Resource struct {
	Name        string   `yaml:"name"`
	Group       string   `yaml:"group"`
	Description string   `yaml:"description"`
	Options     []Option `yaml:"options"`
	Actions     []Action `yaml:"actions"`
}

func (r *Resource) Action(name string) *Action {
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

type Resources struct {
	Parameters []Parameter `yaml:"parameters"`
	List       []Resource  `yaml:"list"`
}

func (r *Resources) Find(name string) *Resource {
	for _, v := range r.List {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

type Dictionary struct {
	ExpressionMap map[string]string `yaml:"expression_map"`
	Resources     Resources         `yaml:"resources"`
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
