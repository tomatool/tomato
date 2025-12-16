package command

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cucumber/gherkin/go/v26"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/config"
	"github.com/urfave/cli/v2"
)

var validateCommand = &cli.Command{
	Name:  "validate",
	Usage: "Validate configuration and feature files",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "tomato.yml",
			Usage:   "config file path",
		},
		&cli.BoolFlag{
			Name:  "plain",
			Usage: "disable colors and interactive UI (for CI)",
		},
	},
	Action: runValidate,
}

// ValidationResult holds the result of a validation check
type ValidationResult struct {
	Category   string
	Item       string
	Status     string // "ok", "warning", "error"
	Message    string
	Suggestion string
}

// Validator performs all validation checks
type Validator struct {
	configPath string
	config     *config.Config
	results    []ValidationResult
	stepPatterns []*regexp.Regexp
}

func runValidate(c *cli.Context) error {
	configPath := c.String("config")
	plain := c.Bool("plain")

	v := &Validator{
		configPath: configPath,
	}

	if plain {
		return v.runPlain()
	}

	return v.runInteractive()
}

// runPlain runs validation without Bubble Tea UI
func (v *Validator) runPlain() error {
	fmt.Println("Validating tomato configuration...")
	fmt.Println()

	v.validate()

	hasErrors := false
	hasWarnings := false

	// Group results by category
	categories := make(map[string][]ValidationResult)
	for _, r := range v.results {
		categories[r.Category] = append(categories[r.Category], r)
		if r.Status == "error" {
			hasErrors = true
		}
		if r.Status == "warning" {
			hasWarnings = true
		}
	}

	// Print results
	for category, results := range categories {
		fmt.Printf("[%s]\n", category)
		for _, r := range results {
			icon := "✓"
			if r.Status == "error" {
				icon = "✗"
			} else if r.Status == "warning" {
				icon = "!"
			}

			fmt.Printf("  %s %s", icon, r.Item)
			if r.Message != "" {
				fmt.Printf(": %s", r.Message)
			}
			fmt.Println()

			if r.Suggestion != "" {
				fmt.Printf("    → %s\n", r.Suggestion)
			}
		}
		fmt.Println()
	}

	// Summary
	errorCount := 0
	warningCount := 0
	okCount := 0
	for _, r := range v.results {
		switch r.Status {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "ok":
			okCount++
		}
	}

	fmt.Printf("Summary: %d passed, %d warnings, %d errors\n", okCount, warningCount, errorCount)

	if hasErrors {
		return fmt.Errorf("validation failed with %d error(s)", errorCount)
	}
	if hasWarnings {
		fmt.Println("Validation passed with warnings")
	} else {
		fmt.Println("Validation passed!")
	}

	return nil
}

// runInteractive runs validation with Bubble Tea UI
func (v *Validator) runInteractive() error {
	p := tea.NewProgram(newValidateModel(v))
	m, err := p.Run()
	if err != nil {
		return err
	}

	model := m.(validateModel)
	if model.hasErrors {
		return fmt.Errorf("validation failed")
	}

	return nil
}

// validate performs all validation checks
func (v *Validator) validate() {
	// Load step patterns for matching
	v.loadStepPatterns()

	// 1. Config file exists
	v.validateConfigExists()
	if v.config == nil {
		return
	}

	// 2. Config structure
	v.validateConfigStructure()

	// 3. Resources
	v.validateResources()

	// 4. Containers
	v.validateContainers()

	// 5. Feature files
	v.validateFeatureFiles()
}

func (v *Validator) loadStepPatterns() {
	categories := collectStepCategories()
	for _, cat := range categories {
		for _, step := range cat.Steps {
			// Convert pattern to regex - replace {resource} with a capture group
			pattern := step.Pattern
			pattern = strings.ReplaceAll(pattern, "{resource}", `"([^"]+)"`)
			// Escape special regex chars that aren't already part of the pattern
			if re, err := regexp.Compile("(?i)^" + pattern + "$"); err == nil {
				v.stepPatterns = append(v.stepPatterns, re)
			}
		}
	}
}

func (v *Validator) validateConfigExists() {
	if _, err := os.Stat(v.configPath); os.IsNotExist(err) {
		v.results = append(v.results, ValidationResult{
			Category:   "Config",
			Item:       v.configPath,
			Status:     "error",
			Message:    "config file not found",
			Suggestion: fmt.Sprintf("Create a %s file or specify path with --config", v.configPath),
		})
		return
	}

	cfg, err := config.Load(v.configPath)
	if err != nil {
		v.results = append(v.results, ValidationResult{
			Category:   "Config",
			Item:       v.configPath,
			Status:     "error",
			Message:    err.Error(),
			Suggestion: "Check the config file syntax and structure",
		})
		return
	}

	v.config = cfg
	v.results = append(v.results, ValidationResult{
		Category: "Config",
		Item:     v.configPath,
		Status:   "ok",
		Message:  "valid configuration",
	})
}

func (v *Validator) validateConfigStructure() {
	// Check version
	if v.config.Version != 2 {
		v.results = append(v.results, ValidationResult{
			Category:   "Config",
			Item:       "version",
			Status:     "error",
			Message:    fmt.Sprintf("unsupported version %d", v.config.Version),
			Suggestion: "Set version: 2 at the top of your config",
		})
	}

	// Check feature paths exist
	for _, path := range v.config.Features.Paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			v.results = append(v.results, ValidationResult{
				Category:   "Config",
				Item:       fmt.Sprintf("features.paths: %s", path),
				Status:     "warning",
				Message:    "directory does not exist",
				Suggestion: fmt.Sprintf("Create the directory: mkdir -p %s", path),
			})
		}
	}
}

func (v *Validator) validateResources() {
	if len(v.config.Resources) == 0 {
		v.results = append(v.results, ValidationResult{
			Category:   "Resources",
			Item:       "(none)",
			Status:     "warning",
			Message:    "no resources defined",
			Suggestion: "Add resources to interact with services in your tests",
		})
		return
	}

	validTypes := map[string]bool{
		"http": true, "http-server": true,
		"postgres": true, "redis": true, "kafka": true,
		"shell": true, "websocket": true, "websocket-server": true,
	}

	for name, res := range v.config.Resources {
		// Check resource type
		if !validTypes[res.Type] {
			v.results = append(v.results, ValidationResult{
				Category:   "Resources",
				Item:       name,
				Status:     "error",
				Message:    fmt.Sprintf("unknown type %q", res.Type),
				Suggestion: "Valid types: http, http-server, postgres, redis, kafka, shell, websocket, websocket-server",
			})
			continue
		}

		// Check container reference for container-based resources
		needsContainer := map[string]bool{
			"postgres": true, "redis": true, "kafka": true,
		}
		if needsContainer[res.Type] && res.Container == "" {
			v.results = append(v.results, ValidationResult{
				Category:   "Resources",
				Item:       name,
				Status:     "warning",
				Message:    fmt.Sprintf("%s resource without container reference", res.Type),
				Suggestion: fmt.Sprintf("Add 'container: <name>' to connect to a container, or provide connection details"),
			})
		}

		v.results = append(v.results, ValidationResult{
			Category: "Resources",
			Item:     name,
			Status:   "ok",
			Message:  fmt.Sprintf("type: %s", res.Type),
		})
	}
}

func (v *Validator) validateContainers() {
	if len(v.config.Containers) == 0 {
		return // Containers are optional
	}

	for name, cont := range v.config.Containers {
		// Check image or build
		if cont.Image == "" && cont.Build == nil {
			v.results = append(v.results, ValidationResult{
				Category:   "Containers",
				Item:       name,
				Status:     "error",
				Message:    "missing image or build configuration",
				Suggestion: "Add 'image: <image:tag>' or 'build: {context: ., dockerfile: Dockerfile}'",
			})
			continue
		}

		// Check wait strategy
		if cont.WaitFor.Type == "" {
			v.results = append(v.results, ValidationResult{
				Category:   "Containers",
				Item:       name,
				Status:     "warning",
				Message:    "no wait_for strategy defined",
				Suggestion: "Add wait_for to ensure container is ready: wait_for: {type: port, target: \"5432\"}",
			})
		}

		v.results = append(v.results, ValidationResult{
			Category: "Containers",
			Item:     name,
			Status:   "ok",
			Message:  fmt.Sprintf("image: %s", cont.Image),
		})
	}
}

func (v *Validator) validateFeatureFiles() {
	var featureFiles []string

	for _, path := range v.config.Features.Paths {
		files, _ := filepath.Glob(filepath.Join(path, "*.feature"))
		featureFiles = append(featureFiles, files...)
		// Also check subdirectories
		subFiles, _ := filepath.Glob(filepath.Join(path, "**", "*.feature"))
		featureFiles = append(featureFiles, subFiles...)
	}

	if len(featureFiles) == 0 {
		v.results = append(v.results, ValidationResult{
			Category:   "Features",
			Item:       "(none)",
			Status:     "warning",
			Message:    "no feature files found",
			Suggestion: "Create .feature files in your features directory",
		})
		return
	}

	for _, file := range featureFiles {
		v.validateFeatureFile(file)
	}
}

func (v *Validator) validateFeatureFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		v.results = append(v.results, ValidationResult{
			Category: "Features",
			Item:     filepath.Base(path),
			Status:   "error",
			Message:  fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	// Parse with Gherkin
	reader := strings.NewReader(string(content))
	doc, err := gherkin.ParseGherkinDocument(reader, (&messages.Incrementing{}).NewId)
	if err != nil {
		v.results = append(v.results, ValidationResult{
			Category:   "Features",
			Item:       filepath.Base(path),
			Status:     "error",
			Message:    fmt.Sprintf("parse error: %v", err),
			Suggestion: "Check Gherkin syntax: https://cucumber.io/docs/gherkin/reference/",
		})
		return
	}

	if doc.Feature == nil {
		v.results = append(v.results, ValidationResult{
			Category:   "Features",
			Item:       filepath.Base(path),
			Status:     "error",
			Message:    "no Feature found in file",
			Suggestion: "Add 'Feature: <name>' at the top of the file",
		})
		return
	}

	// Check scenarios and steps
	scenarioCount := 0
	undefinedSteps := []string{}

	for _, child := range doc.Feature.Children {
		if child.Scenario != nil {
			scenarioCount++
			for _, step := range child.Scenario.Steps {
				if !v.isStepDefined(step.Text) {
					undefinedSteps = append(undefinedSteps, step.Text)
				}
			}
		}
		if child.Background != nil {
			for _, step := range child.Background.Steps {
				if !v.isStepDefined(step.Text) {
					undefinedSteps = append(undefinedSteps, step.Text)
				}
			}
		}
	}

	if len(undefinedSteps) > 0 {
		// Show first few undefined steps
		shown := undefinedSteps
		if len(shown) > 3 {
			shown = shown[:3]
		}
		v.results = append(v.results, ValidationResult{
			Category:   "Features",
			Item:       filepath.Base(path),
			Status:     "warning",
			Message:    fmt.Sprintf("%d undefined step(s): %s", len(undefinedSteps), strings.Join(shown, ", ")),
			Suggestion: "Run 'tomato steps' to see available steps",
		})
	} else {
		v.results = append(v.results, ValidationResult{
			Category: "Features",
			Item:     filepath.Base(path),
			Status:   "ok",
			Message:  fmt.Sprintf("%d scenario(s)", scenarioCount),
		})
	}
}

func (v *Validator) isStepDefined(stepText string) bool {
	for _, pattern := range v.stepPatterns {
		if pattern.MatchString(stepText) {
			return true
		}
	}
	return false
}

// Bubble Tea Model
type validateModel struct {
	validator   *Validator
	spinner     spinner.Model
	done        bool
	hasErrors   bool
	hasWarnings bool
}

func newValidateModel(v *Validator) validateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return validateModel{
		validator: v,
		spinner:   s,
	}
}

type validationDoneMsg struct{}

func (m validateModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			m.validator.validate()
			return validationDoneMsg{}
		},
	)
}

func (m validateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case validationDoneMsg:
		m.done = true
		for _, r := range m.validator.results {
			if r.Status == "error" {
				m.hasErrors = true
			}
			if r.Status == "warning" {
				m.hasWarnings = true
			}
		}
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m validateModel) View() string {
	var s strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	categoryStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	suggestionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("🍅 Tomato Validator"))
	s.WriteString("\n\n")

	if !m.done {
		s.WriteString(m.spinner.View())
		s.WriteString(" Validating configuration...")
		return s.String()
	}

	// Group results by category
	categories := make(map[string][]ValidationResult)
	order := []string{}
	for _, r := range m.validator.results {
		if _, exists := categories[r.Category]; !exists {
			order = append(order, r.Category)
		}
		categories[r.Category] = append(categories[r.Category], r)
	}

	for _, category := range order {
		results := categories[category]
		s.WriteString(categoryStyle.Render(category))
		s.WriteString("\n")

		for _, r := range results {
			var icon, style string
			switch r.Status {
			case "ok":
				icon = "✓"
				style = okStyle.Render(icon)
			case "warning":
				icon = "!"
				style = warnStyle.Render(icon)
			case "error":
				icon = "✗"
				style = errStyle.Render(icon)
			}

			s.WriteString(fmt.Sprintf("  %s %s", style, r.Item))
			if r.Message != "" {
				s.WriteString(fmt.Sprintf(": %s", r.Message))
			}
			s.WriteString("\n")

			if r.Suggestion != "" {
				s.WriteString(fmt.Sprintf("    %s\n", suggestionStyle.Render("→ "+r.Suggestion)))
			}
		}
		s.WriteString("\n")
	}

	// Summary
	errorCount := 0
	warningCount := 0
	okCount := 0
	for _, r := range m.validator.results {
		switch r.Status {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "ok":
			okCount++
		}
	}

	summaryParts := []string{
		okStyle.Render(fmt.Sprintf("%d passed", okCount)),
	}
	if warningCount > 0 {
		summaryParts = append(summaryParts, warnStyle.Render(fmt.Sprintf("%d warnings", warningCount)))
	}
	if errorCount > 0 {
		summaryParts = append(summaryParts, errStyle.Render(fmt.Sprintf("%d errors", errorCount)))
	}

	s.WriteString(fmt.Sprintf("Summary: %s\n", strings.Join(summaryParts, ", ")))

	if m.hasErrors {
		s.WriteString(errStyle.Render("\n✗ Validation failed\n"))
	} else if m.hasWarnings {
		s.WriteString(warnStyle.Render("\n! Validation passed with warnings\n"))
	} else {
		s.WriteString(okStyle.Render("\n✓ Validation passed!\n"))
	}

	return s.String()
}
