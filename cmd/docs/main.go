package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alecthomas/template"
	"github.com/alileza/tomato/dictionary"
	"github.com/alileza/tomato/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	dictionaryPath string
	docsPath       string
)

func step(expr, handle string) string {
	return fmt.Sprintf("\t\ts.Step(`^%s`, h.%s)", expr, handle)
}

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato - docs generator")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("dictionary", "tomato dictionary file path.").Short('d').Default("dictionary.yml").StringVar(&dictionaryPath)
	app.Flag("output", "output of docs markdown.").Short('o').Default("docs/resources.md").StringVar(&docsPath)

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

	os.Remove(docsPath)

	w, err := os.OpenFile(docsPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error opening file"))
		os.Exit(1)
	}
	defer w.Close()

	t, _ := template.ParseFiles("cmd/docs/resources.tpl")

	type vals struct {
		Resources []dictionary.Resource
		Groups    map[string][]dictionary.Resource
	}
	v := vals{
		Groups:    make(map[string][]dictionary.Resource),
		Resources: dict.Resources.List,
	}
	for _, r := range dict.Resources.List {
		tmp := v.Groups[r.Group]
		v.Groups[r.Group] = append(tmp, r)
	}

	t.Execute(w, v)

	/*
		for _, resource := range dict.Resources.List {
			for _, action := range resource.Actions {
				for _, expr := range action.Expr() {
					fmt.Fprintf(steps, step(expr, action.Handle)+"\n")
				}
			}
		}

		fmt.Fprintf(w, fileTmpl, steps.String())
	*/
}
