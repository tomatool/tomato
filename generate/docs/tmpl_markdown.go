package docs

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

{{.Description}}

### resource parameters
{{range .Options}}
1. **{{.Name}}** *({{.Type}})*

   {{.Description}}
{{end}}

## actions
{{range .Actions}}
### **{{.Name}}**

   {{.Description}}

   **expressions**
	 {{range $expr := .Expressions}}
   - {{ $expr }}
	 {{end}}

   **parameters**
	 {{range .Parameters}}
   - {{.Name}} *({{.Type}})*

     {{.Description}}
	 {{end}}

{{end}}\

---

{{end}}\
{{end}}\
`
)
