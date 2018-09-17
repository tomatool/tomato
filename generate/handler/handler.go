package handler

import (
	"bytes"
	"fmt"

	"github.com/alileza/tomato/dictionary"
)

const (
	handlerTmpl = `/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the cmd/gen/main.go tool */
package handler

import (
	"github.com/DATA-DOG/godog"
	"github.com/alileza/tomato/resource"
)

type Handler struct {
	resource *resource.Manager
}

func New(r *resource.Manager) func(s *godog.Suite) {
	h := &Handler{r}
	return func(s *godog.Suite) {
		s.BeforeScenario(func(_ interface{}) {
			h.resource.Reset()
		})
%s
    }
}`
)

func step(expr, handle string) string {
	return fmt.Sprintf("\t\ts.Step(`^%s`, h.%s)", expr, handle)
}

func Generate(dict *dictionary.Dictionary) (*bytes.Buffer, error) {
	steps := bytes.NewBuffer(nil)
	for _, resource := range dict.Resources.List {
		for _, action := range resource.Actions {
			for _, expr := range action.Expr() {
				fmt.Fprintf(steps, step(expr, action.Handle)+"\n")
			}
		}
	}
	return bytes.NewBufferString(fmt.Sprintf(handlerTmpl, steps.String())), nil
}
