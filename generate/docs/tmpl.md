# Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.
{{range $group, $handler := .Handlers}}
## {{t .Name}}

{{.Description}}

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | {{range $resource := .Resources}}{{ $resource }} | {{end}}
  {{if .Options}}
  options:
    {{range .Options}}# {{.Description}}
    {{.Name}}: # {{.Type}}
    {{end}}
  {{end}}
```

### Resources

{{range $resource := .Resources}}* {{ $resource }}
{{end}}

### Actions
{{range .Actions}}
#### **{{t .Name}}**
{{.Description}}
```gherkin
{{range $expr := .Expressions}}Given {{ $expr }}
{{end}}
```
{{end}}

{{end}}
