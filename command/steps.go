package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tomatool/tomato/internal/handler"
	"github.com/urfave/cli/v2"
)

var stepsCommand = &cli.Command{
	Name:  "steps",
	Usage: "List available Gherkin steps",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "filter",
			Aliases: []string{"f"},
			Usage:   "Filter steps by keyword",
		},
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			Usage:   "Filter by handler type (http, redis, postgres, kafka, websocket, shell)",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "Output in JSON format",
		},
	},
	Action: runSteps,
}

func runSteps(ctx *cli.Context) error {
	filter := strings.ToLower(ctx.String("filter"))
	typeFilter := strings.ToLower(ctx.String("type"))
	jsonOutput := ctx.Bool("json")

	categories := collectStepCategories()

	var filteredCategories []handler.StepCategory

	for _, cat := range categories {
		// Filter by type (match against name or type prefix)
		if typeFilter != "" {
			nameLower := strings.ToLower(cat.Name)
			if nameLower != typeFilter && !strings.HasPrefix(nameLower, typeFilter) {
				continue
			}
		}

		var matchingSteps []handler.StepDef
		for _, step := range cat.Steps {
			// Filter by keyword
			if filter != "" {
				if !strings.Contains(strings.ToLower(step.Description), filter) &&
					!strings.Contains(strings.ToLower(step.Pattern), filter) {
					continue
				}
			}
			matchingSteps = append(matchingSteps, step)
		}

		if len(matchingSteps) == 0 {
			continue
		}

		filteredCategories = append(filteredCategories, handler.StepCategory{
			Name:        cat.Name,
			Description: cat.Description,
			Steps:       matchingSteps,
		})
	}

	if jsonOutput {
		output, err := json.MarshalIndent(filteredCategories, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	for _, cat := range filteredCategories {
		fmt.Printf("\n\033[1;36m%s\033[0m\n", cat.Name)
		fmt.Printf("\033[90m%s\033[0m\n\n", cat.Description)

		for _, step := range cat.Steps {
			// Replace {resource} with placeholder
			pattern := strings.ReplaceAll(step.Pattern, "{resource}", "<resource>")
			example := strings.ReplaceAll(step.Example, "{resource}", "api")

			fmt.Printf("  \033[1m%s\033[0m\n", step.Description)
			fmt.Printf("  \033[33m%s\033[0m\n", pattern)

			// Show first line of example
			exampleLines := strings.Split(example, "\n")
			fmt.Printf("  \033[90mExample: %s\033[0m\n\n", exampleLines[0])
		}
	}

	return nil
}
