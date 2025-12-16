package command

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/tomatool/tomato/internal/handler"
	"github.com/urfave/cli/v2"
)

var docsCommand = &cli.Command{
	Name:  "docs",
	Usage: "Generate documentation for available steps",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output file (default: stdout)",
		},
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "markdown",
			Usage:   "Output format: markdown, html",
		},
	},
	Action: runDocs,
}

func runDocs(ctx *cli.Context) error {
	format := ctx.String("format")
	output := ctx.String("output")

	var w io.Writer = os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	// Collect all step categories from handlers
	categories := collectStepCategories()

	switch format {
	case "markdown":
		return generateMarkdown(w, categories)
	case "html":
		return generateHTML(w, categories)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// collectStepCategories returns all step categories from all handler types
func collectStepCategories() []handler.StepCategory {
	// Create dummy handlers to extract step definitions
	// We use nil for dependencies since we only need the step metadata
	categories := []handler.StepCategory{}

	// HTTP steps
	httpHandler, _ := handler.NewHTTPClient("api", handler.DummyConfig(), nil)
	categories = append(categories, httpHandler.Steps())

	// Redis steps
	redisHandler, _ := handler.NewRedis("cache", handler.DummyConfig(), nil)
	categories = append(categories, redisHandler.Steps())

	// Postgres steps
	postgresHandler, _ := handler.NewPostgres("db", handler.DummyConfig(), nil)
	categories = append(categories, postgresHandler.Steps())

	// Kafka steps
	kafkaHandler, _ := handler.NewKafka("kafka", handler.DummyConfig(), nil)
	categories = append(categories, kafkaHandler.Steps())

	// WebSocket steps
	wsHandler, _ := handler.NewWebSocketClient("ws", handler.DummyConfig(), nil)
	categories = append(categories, wsHandler.Steps())

	// Shell steps
	shellHandler, _ := handler.NewShell("shell", handler.DummyConfig(), nil)
	categories = append(categories, shellHandler.Steps())

	return categories
}

const markdownTemplate = `---
layout: default
title: Step Reference
nav_order: 4
---

# Tomato Step Reference

This document lists all available Gherkin steps in Tomato.

{: .note }
This documentation is auto-generated from the source code. Run ` + "`tomato docs`" + ` to regenerate.

{{range .}}
## {{.Name}}

{{.Description}}

{{range .Steps}}
### {{.Description}}

**Pattern:** ` + "`" + `{{.Pattern}}` + "`" + `

**Example:**
` + "```gherkin" + `
{{.Example}}
` + "```" + `

{{end}}
{{end}}
`

func generateMarkdown(w io.Writer, categories []handler.StepCategory) error {
	tmpl, err := template.New("docs").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	// Replace {resource} with example resource name for docs
	for i := range categories {
		for j := range categories[i].Steps {
			categories[i].Steps[j].Pattern = strings.ReplaceAll(categories[i].Steps[j].Pattern, "{resource}", "<resource>")
			categories[i].Steps[j].Example = strings.ReplaceAll(categories[i].Steps[j].Example, "{resource}", "api")
		}
	}

	return tmpl.Execute(w, categories)
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
        .pattern { background: #f8f9fa; padding: 10px; border-radius: 4px; font-family: monospace; overflow-x: auto; }
        .example { background: #2c3e50; color: #ecf0f1; padding: 15px; border-radius: 4px; overflow-x: auto; }
        .example pre { margin: 0; white-space: pre-wrap; }
        .step { margin-bottom: 30px; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px; }
        .category { margin-bottom: 40px; }
    </style>
</head>
<body>
    <h1>Tomato Step Reference</h1>
    {{range .}}
    <div class="category">
        <h2>{{.Name}}</h2>
        <p>{{.Description}}</p>
        {{range .Steps}}
        <div class="step">
            <h3>{{.Description}}</h3>
            <p><strong>Pattern:</strong></p>
            <div class="pattern">{{.Pattern}}</div>
            <p><strong>Example:</strong></p>
            <div class="example"><pre>{{.Example}}</pre></div>
        </div>
        {{end}}
    </div>
    {{end}}
</body>
</html>
`

func generateHTML(w io.Writer, categories []handler.StepCategory) error {
	tmpl, err := template.New("docs").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	// Replace {resource} with example resource name for docs
	for i := range categories {
		for j := range categories[i].Steps {
			categories[i].Steps[j].Pattern = strings.ReplaceAll(categories[i].Steps[j].Pattern, "{resource}", "&lt;resource&gt;")
			categories[i].Steps[j].Example = strings.ReplaceAll(categories[i].Steps[j].Example, "{resource}", "api")
		}
	}

	return tmpl.Execute(w, categories)
}
