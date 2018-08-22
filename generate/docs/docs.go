package docs

import (
	"bytes"

	"github.com/alecthomas/template"
	"github.com/alileza/tomato/dictionary"
)

const (
	markdownTmpl = `# Resources

  Resources are objects that are going to be used for step evaluations in the cucumber scenario. They are listed under the resources key in the pipeline configuration.

  Supported resources:
  {{range $group, $resource := .Groups}}
  * {{$group}}
    {{range $resource}}\
   - [{{.Name}}](#{{.Group}}/{{.Name}})
    {{end}}\
  {{end}}
  ---
  {{ range $group, $resource := .Groups}}
  # {{$group}}\
  {{range $resource}}
  ## {{.Name}}
  **Parameters**
    {{range .Options}}
      - {{.Name}}: {{.Description}}\
    {{end}}
  **Available functions**
  {{range .Actions}}
    1. **{{.Name}}**
      {{.Description}}\
      {{range .Expressions}}
        {{.}}
      {{end}}\
      {{range .Examples}}
        {{.}}\
      {{end}}\
  {{end}}\
  {{end}}\
  {{end}}\
`
)

type Options struct {
	Output string
}

const (
	OutputMarkdown = "markdown"
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
	default:
		tmplGlob = markdownTmpl
	}

	tmpl, err := template.New("docs").Parse(tmplGlob)
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
