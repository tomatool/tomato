package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tomatool/tomato/internal/handler"
	"github.com/urfave/cli/v2"
)

var docsCommand = &cli.Command{
	Name:   "docs",
	Usage:  "Generate documentation for available steps",
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output directory for mkdocs format, or file for other formats",
		},
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "mkdocs",
			Usage:   "Output format: mkdocs (generates docs/resources/), markdown (single file), html",
		},
	},
	Action: runDocs,
}

// resourceFileMapping maps handler names to their output filenames
var resourceFileMapping = map[string]string{
	"HTTP Client":      "http-client.md",
	"HTTP Server":      "http-server.md",
	"PostgreSQL":       "postgres.md",
	"Redis":            "redis.md",
	"Kafka":            "kafka.md",
	"Shell":            "shell.md",
	"WebSocket Client": "websocket-client.md",
	"WebSocket Server": "websocket-server.md",
}

func runDocs(ctx *cli.Context) error {
	format := ctx.String("format")
	output := ctx.String("output")

	// Collect all step categories from handlers
	categories := collectStepCategories()

	switch format {
	case "mkdocs":
		outputDir := output
		if outputDir == "" {
			outputDir = "docs/resources"
		}
		return generateMkDocs(outputDir, categories)
	case "markdown":
		var w io.Writer = os.Stdout
		if output != "" {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			w = f
		}
		return generateMarkdown(w, categories)
	case "html":
		var w io.Writer = os.Stdout
		if output != "" {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			w = f
		}
		return generateHTML(w, categories)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// collectStepCategories returns all step categories from all handler types
func collectStepCategories() []handler.StepCategory {
	categories := []handler.StepCategory{}

	// HTTP Client
	httpHandler, _ := handler.NewHTTPClient("api", handler.DummyConfig(), nil)
	categories = append(categories, httpHandler.Steps())

	// HTTP Server
	httpServerHandler, _ := handler.NewHTTPServer("mock", handler.DummyConfig(), nil)
	categories = append(categories, httpServerHandler.Steps())

	// PostgreSQL
	postgresHandler, _ := handler.NewPostgres("db", handler.DummyConfig(), nil)
	categories = append(categories, postgresHandler.Steps())

	// Redis
	redisHandler, _ := handler.NewRedis("cache", handler.DummyConfig(), nil)
	categories = append(categories, redisHandler.Steps())

	// Kafka
	kafkaHandler, _ := handler.NewKafka("queue", handler.DummyConfig(), nil)
	categories = append(categories, kafkaHandler.Steps())

	// Shell
	shellHandler, _ := handler.NewShell("shell", handler.DummyConfig(), nil)
	categories = append(categories, shellHandler.Steps())

	// WebSocket Client
	wsClientHandler, _ := handler.NewWebSocketClient("ws", handler.DummyConfig(), nil)
	categories = append(categories, wsClientHandler.Steps())

	// WebSocket Server
	wsServerHandler, _ := handler.NewWebSocketServer("wsmock", handler.DummyConfig(), nil)
	categories = append(categories, wsServerHandler.Steps())

	return categories
}

// GroupedStep is a step with processed fields for docs
type GroupedStep struct {
	Example     string
	Description string
}

// StepGroup groups steps by their group name
type StepGroup struct {
	Name  string
	Steps []GroupedStep
}

// CategoryWithGroups is a category with steps grouped
type CategoryWithGroups struct {
	Name        string
	Description string
	Groups      []StepGroup
}

// DocsData is the data structure for the docs template
type DocsData struct {
	Categories []CategoryWithGroups
}

func buildCategoryWithGroups(cat handler.StepCategory) CategoryWithGroups {
	catWithGroups := CategoryWithGroups{
		Name:        cat.Name,
		Description: cat.Description,
		Groups:      make([]StepGroup, 0),
	}

	groupMap := make(map[string][]GroupedStep)
	groupOrder := make([]string, 0)

	for _, step := range cat.Steps {
		groupName := step.Group
		if groupName == "" {
			groupName = "General"
		}

		if _, exists := groupMap[groupName]; !exists {
			groupOrder = append(groupOrder, groupName)
		}

		groupMap[groupName] = append(groupMap[groupName], GroupedStep{
			Example:     step.Example,
			Description: step.Description,
		})
	}

	for _, groupName := range groupOrder {
		catWithGroups.Groups = append(catWithGroups.Groups, StepGroup{
			Name:  groupName,
			Steps: groupMap[groupName],
		})
	}

	return catWithGroups
}

const mkdocsResourceTemplate = `# {{.Name}}

{{.Description}}

{{range .Groups}}
## {{.Name}}

| Step | Description |
|------|-------------|
{{range .Steps}}| ` + "`" + `{{.Example}}` + "`" + ` | {{.Description}} |
{{end}}
{{end}}`

const mkdocsIndexTemplate = `# Available Resources

Tomato supports the following resource types for behavioral testing.

| Resource | Description |
|----------|-------------|
{{range .}}| [{{.Name}}]({{.File}}) | {{.Description}} |
{{end}}

## Variables and Dynamic Values

Tomato supports variables that can be used in URLs, headers, and request bodies. Variables use the ` + "`" + `{{"{{name}}"}}` + "`" + ` syntax.

### Dynamic Value Generation

Built-in functions generate unique values on each use:

| Function | Example Output | Description |
|----------|----------------|-------------|
| ` + "`" + `{{"{{uuid}}"}}` + "`" + ` | ` + "`f47ac10b-58cc-4372-a567-0e02b2c3d479`" + ` | Random UUID v4 |
| ` + "`" + `{{"{{timestamp}}"}}` + "`" + ` | ` + "`2024-01-15T10:30:00Z`" + ` | Current ISO 8601 timestamp |
| ` + "`" + `{{"{{timestamp:unix}}"}}` + "`" + ` | ` + "`1705315800`" + ` | Unix timestamp in seconds |
| ` + "`" + `{{"{{random:N}}"}}` + "`" + ` | ` + "`A8kL2mN9pQ`" + ` | Random alphanumeric string of length N |
| ` + "`" + `{{"{{random:N:numeric}}"}}` + "`" + ` | ` + "`8472910384`" + ` | Random numeric string of length N |
| ` + "`" + `{{"{{sequence:name}}"}}` + "`" + ` | ` + "`1`" + `, ` + "`2`" + `, ` + "`3`" + `... | Auto-incrementing sequence by name |

**Example:**
` + "```gherkin" + `
When "api" sends "POST" to "/api/users" with json:
  """
  {
    "id": "{{"{{uuid}}"}}",
    "username": "user_{{"{{random:8}}"}}",
    "created_at": "{{"{{timestamp}}"}}"
  }
  """
` + "```" + `

### Capturing Response Values

Save values from responses to use in subsequent requests:

| Step | Description |
|------|-------------|
| ` + "`" + `"api" response json "path" saved as "{{"{{var}}"}}"` + "`" + ` | Save JSON path value to variable |
| ` + "`" + `"api" response header "Name" saved as "{{"{{var}}"}}"` + "`" + ` | Save response header to variable |

**Example - CRUD workflow:**
` + "```gherkin" + `
Scenario: Create and retrieve a user
  # Create user and capture the ID
  When "api" sends "POST" to "/api/users" with json:
    """
    { "name": "Alice" }
    """
  Then "api" response status is "201"
  And "api" response json "id" saved as "{{"{{user_id}}"}}"

  # Use captured ID in subsequent request
  When "api" sends "GET" to "/api/users/{{"{{user_id}}"}}"
  Then "api" response status is "200"
  And "api" response json "name" is "Alice"

  # Delete the user
  When "api" sends "DELETE" to "/api/users/{{"{{user_id}}"}}"
  Then "api" response status is "204"
` + "```" + `

Variables and sequences are automatically reset between scenarios.

## JSON Matchers

When using ` + "`" + `response json matches:` + "`" + ` or ` + "`" + `response json contains:` + "`" + `, you can use these matchers:

### Type Matchers

| Matcher | Description |
|---------|-------------|
| ` + "`@string`" + ` | Matches any string value |
| ` + "`@number`" + ` | Matches any numeric value |
| ` + "`@boolean`" + ` | Matches true or false |
| ` + "`@array`" + ` | Matches any array |
| ` + "`@object`" + ` | Matches any object |
| ` + "`@null`" + ` | Matches null |
| ` + "`@notnull`" + ` | Matches any non-null value |
| ` + "`@any`" + ` | Matches any value |
| ` + "`@empty`" + ` | Matches empty string, array, or object |
| ` + "`@notempty`" + ` | Matches non-empty string, array, or object |

### String Matchers

| Matcher | Description |
|---------|-------------|
| ` + "`@regex:pattern`" + ` | Matches string against regex pattern |
| ` + "`@contains:text`" + ` | Matches if string contains text |
| ` + "`@startswith:text`" + ` | Matches if string starts with text |
| ` + "`@endswith:text`" + ` | Matches if string ends with text |

### Numeric Matchers

| Matcher | Description |
|---------|-------------|
| ` + "`@gt:n`" + ` | Matches if value > n |
| ` + "`@gte:n`" + ` | Matches if value >= n |
| ` + "`@lt:n`" + ` | Matches if value < n |
| ` + "`@lte:n`" + ` | Matches if value <= n |

### Length Matcher

| Matcher | Description |
|---------|-------------|
| ` + "`@len:n`" + ` | Matches if length equals n (for strings, arrays, objects) |

### Example

` + "```gherkin" + `
Then "api" response json contains:
  """
  {
    "id": "@regex:^[0-9a-f-]{36}$",
    "name": "@notempty",
    "email": "@contains:@",
    "age": "@gt:0",
    "tags": "@array"
  }
  """
` + "```" + `
`

type ResourceInfo struct {
	Name        string
	Description string
	File        string
}

func generateMkDocs(outputDir string, categories []handler.StepCategory) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	tmpl, err := template.New("resource").Parse(mkdocsResourceTemplate)
	if err != nil {
		return fmt.Errorf("parsing resource template: %w", err)
	}

	var resources []ResourceInfo

	// Generate individual resource files
	for _, cat := range categories {
		filename, ok := resourceFileMapping[cat.Name]
		if !ok {
			filename = strings.ToLower(strings.ReplaceAll(cat.Name, " ", "-")) + ".md"
		}

		resources = append(resources, ResourceInfo{
			Name:        cat.Name,
			Description: cat.Description,
			File:        filename,
		})

		filePath := filepath.Join(outputDir, filename)
		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", filePath, err)
		}

		catWithGroups := buildCategoryWithGroups(cat)
		if err := tmpl.Execute(f, catWithGroups); err != nil {
			f.Close()
			return fmt.Errorf("executing template for %s: %w", cat.Name, err)
		}
		f.Close()

		fmt.Printf("Generated %s\n", filePath)
	}

	// Generate index file
	indexTmpl, err := template.New("index").Parse(mkdocsIndexTemplate)
	if err != nil {
		return fmt.Errorf("parsing index template: %w", err)
	}

	indexPath := filepath.Join(outputDir, "index.md")
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("creating index file: %w", err)
	}
	defer indexFile.Close()

	if err := indexTmpl.Execute(indexFile, resources); err != nil {
		return fmt.Errorf("executing index template: %w", err)
	}

	fmt.Printf("Generated %s\n", indexPath)
	return nil
}

const markdownTemplate = `# Step Reference

This document lists all available Gherkin steps organized by resource type.

> **Note:** This documentation is auto-generated from the source code.

{{range .Categories}}
---

## {{.Name}}

{{.Description}}

{{range .Groups}}
### {{.Name}}

| Step | Description |
|------|-------------|
{{range .Steps}}| ` + "`" + `{{.Example}}` + "`" + ` | {{.Description}} |
{{end}}
{{end}}
{{end}}`

func generateMarkdown(w io.Writer, categories []handler.StepCategory) error {
	tmpl, err := template.New("docs").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	data := DocsData{Categories: make([]CategoryWithGroups, 0)}
	for _, cat := range categories {
		data.Categories = append(data.Categories, buildCategoryWithGroups(cat))
	}

	return tmpl.Execute(w, data)
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Tomato Step Reference</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; }
        h1 { color: #e74c3c; }
        h2 { color: #2c3e50; border-bottom: 2px solid #e74c3c; padding-bottom: 10px; }
        h3 { color: #34495e; }
        table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f8f9fa; }
        code { background: #f8f9fa; padding: 2px 6px; border-radius: 4px; font-family: monospace; }
    </style>
</head>
<body>
    <h1>Tomato Step Reference</h1>
    {{range .Categories}}
    <div class="category">
        <h2>{{.Name}}</h2>
        <p>{{.Description}}</p>
        {{range .Groups}}
        <h3>{{.Name}}</h3>
        <table>
            <tr><th>Step</th><th>Description</th></tr>
            {{range .Steps}}
            <tr><td><code>{{.Example}}</code></td><td>{{.Description}}</td></tr>
            {{end}}
        </table>
        {{end}}
    </div>
    {{end}}
</body>
</html>`

func generateHTML(w io.Writer, categories []handler.StepCategory) error {
	tmpl, err := template.New("docs").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	data := DocsData{Categories: make([]CategoryWithGroups, 0)}
	for _, cat := range categories {
		data.Categories = append(data.Categories, buildCategoryWithGroups(cat))
	}

	return tmpl.Execute(w, data)
}
