# Resources

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
