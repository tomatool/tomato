# Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.
{{range $group, $resource := .Resources}}
## {{t .Name}}

{{.Description}}

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: {{.Name}}
  ready_check: true {{if .Options}}
  params:
    {{range .Options}}# {{.Description}}
    {{.Name}}: # {{.Type}}
    {{end}}{{end}}
```

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
