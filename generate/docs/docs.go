package docs

import (
	"bytes"
	"strings"

	"github.com/alecthomas/template"
	"github.com/tomatool/tomato/dictionary"
)

type Options struct {
	Output string
}

const (
	OutputMarkdown = "markdown"
	OutputHTML     = "html"
)

var DefaultOptions = &Options{
	Output: OutputMarkdown,
}

func Generate(dict *dictionary.Dictionary, opts *Options) (*bytes.Buffer, error) {
	var tmplGlob string
	if opts == nil {
		opts = DefaultOptions
	}
	switch opts.Output {
	case OutputMarkdown:
		tmplGlob = markdownTmpl
	case OutputHTML:
		tmplGlob = htmlTmpl
	default:
		tmplGlob = markdownTmpl
	}

	tmpl, err := template.New("docs").Funcs(template.FuncMap{
		"replace": func(str, a, b string) string {
			return strings.Replace(str, a, b, -1)
		},
		"t": func(str string) string {
			s := strings.Title(strings.Replace(str, "_", " ", -1))
			s = strings.Replace(s, "Http", "HTTP", -1)
			s = strings.Replace(s, "Sql", "SQL", -1)
			s = strings.Replace(s, "/", " ", -1)
			return s
		},
	}).Parse(tmplGlob)
	if err != nil {
		return nil, err
	}

	type vals struct {
		Handlers []dictionary.Handler
		Groups   map[string][]dictionary.Handler
	}
	v := vals{
		Handlers: dict.Handlers,
	}

	out := &bytes.Buffer{}

	if err := tmpl.Execute(out, v); err != nil {
		return nil, err
	}

	return out, nil
}
