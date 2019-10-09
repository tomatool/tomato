package handler

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tomatool/tomato/dictionary"
)

const (
	handlerTmpl = `/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package %s

import "github.com/DATA-DOG/godog"

func (h *Handler) Register(s *godog.Suite) {%s
}
`
)

func step(expr, handle string) string {
	return fmt.Sprintf("\n\ts.Step(`^%s`, h.%s)", expr, handle)
}

func Generate(dict *dictionary.Dictionary) (map[string]*bytes.Buffer, error) {
	m := make(map[string]*bytes.Buffer)
	for _, resource := range dict.Handlers {
		steps := bytes.NewBuffer(nil)
		for _, action := range resource.Actions {
			for _, expr := range action.Expr() {
				fmt.Fprintf(steps, step(expr, action.Handle))
			}
		}
		s := strings.Split(resource.Name, "/")
		m[resource.Name] = bytes.NewBufferString(fmt.Sprintf(handlerTmpl, s[len(s)-1], steps.String()))
	}
	return m, nil
}
