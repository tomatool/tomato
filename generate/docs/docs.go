package docs

import (
	"bytes"
	"strings"

	"github.com/alecthomas/template"
	"github.com/alileza/tomato/dictionary"
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
	}).Parse(tmplGlob)
	if err != nil {
		return nil, err
	}

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

	out := &bytes.Buffer{}

	if err := tmpl.Execute(out, v); err != nil {
		return nil, err
	}

	return out, nil
}
