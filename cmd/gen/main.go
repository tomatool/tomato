package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alileza/tomato/dictionary"
	"github.com/alileza/tomato/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	dictionaryPath string
	handlerPath    string
)

const (
	fileTmpl = `/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the cmd/gen/main.go tool */
package handler

import (
	"github.com/DATA-DOG/godog"
	"github.com/alileza/tomato/resource"
)

type Handler struct {
	resource resource.Manager
}

func New(r resource.Manager) func(s *godog.Suite) {
	h := &Handler{r}
	return func(s *godog.Suite) {
%s
    }
}`
)

func step(expr, handle string) string {
	return fmt.Sprintf("\t\ts.Step(`^%s`, h.%s)", expr, handle)
}

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato - handler generator")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("dictionary", "tomato dictionary file path.").Short('d').Default("dictionary.yml").StringVar(&dictionaryPath)
	app.Flag("output", "output of handler.go.").Short('o').Default("handler/handler.go").StringVar(&handlerPath)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing flag"))
		os.Exit(1)
	}

	dict, err := dictionary.Retrieve(dictionaryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	os.Remove(handlerPath)

	w, err := os.OpenFile(handlerPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error opening file"))
		os.Exit(1)
	}
	defer w.Close()

	steps := bytes.NewBuffer(nil)

	for _, resource := range dict.Resources.List {
		for _, action := range resource.Actions {
			for _, expr := range action.Expr() {
				fmt.Fprintf(steps, step(expr, action.Handle)+"\n")
			}
		}
	}

	fmt.Fprintf(w, fileTmpl, steps.String())
}
