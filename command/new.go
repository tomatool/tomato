package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tomatool/tomato/internal/config"
	"github.com/urfave/cli/v2"
)

var newCommand = &cli.Command{
	Name:  "new",
	Usage: "Create a new feature file interactively",
	Description: `Guided wizard to create feature files and scenarios based on
the resources defined in your tomato.yml configuration.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "tomato.yml",
			Usage:   "config file path",
		},
	},
	Action: newFeature,
}

type newStep int

const (
	newStepFeatureName newStep = iota
	newStepFeatureDesc
	newStepScenarioName
	newStepSelectResource
	newStepSelectAction
	newStepConfigureAction
	newStepAddMore
	newStepAddScenario
	newStepConfirm
	newStepDone
)

type stepAction struct {
	resource   string
	actionType string
	action     string
	params     map[string]string
}

type scenario struct {
	name  string
	steps []stepAction
}

type newModel struct {
	step       newStep
	cursor     int
	textInputs []textinput.Model
	activeInput int

	// Config
	cfg       *config.Config
	resources []string

	// Feature data
	featureName string
	featureDesc string
	scenarios   []scenario

	// Current scenario being built
	currentScenario    scenario
	currentResource    string
	currentActionType  string // given, when, then
	availableActions   []string
	selectedAction     string
	actionParams       map[string]string

	err error
}

func newInitialModel(cfg *config.Config) newModel {
	// Extract resource names
	var resources []string
	for name := range cfg.Resources {
		resources = append(resources, name)
	}

	// Create text inputs
	inputs := make([]textinput.Model, 4)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "User authentication"
	inputs[0].Focus()
	inputs[0].Width = 50

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "As a user, I want to authenticate so that I can access the system"
	inputs[1].Width = 70

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Successful login with valid credentials"
	inputs[2].Width = 50

	inputs[3] = textinput.New()
	inputs[3].Placeholder = ""
	inputs[3].Width = 50

	return newModel{
		step:            newStepFeatureName,
		textInputs:      inputs,
		activeInput:     0,
		cfg:             cfg,
		resources:       resources,
		scenarios:       []scenario{},
		currentScenario: scenario{steps: []stepAction{}},
		actionParams:    make(map[string]string),
	}
}

func (m newModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m newModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.isSelectStep() && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.isSelectStep() {
				m.cursor = m.incrementCursor()
			}

		case "enter":
			return m.handleEnter()

		case "esc":
			if m.step == newStepConfigureAction {
				m.step = newStepSelectAction
				m.cursor = 0
			}
		}
	}

	// Update text input if on text input step
	if m.isTextInputStep() {
		var cmd tea.Cmd
		m.textInputs[m.activeInput], cmd = m.textInputs[m.activeInput].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m newModel) isSelectStep() bool {
	return m.step == newStepSelectResource ||
		m.step == newStepSelectAction ||
		m.step == newStepAddMore ||
		m.step == newStepAddScenario ||
		m.step == newStepConfirm
}

func (m newModel) isTextInputStep() bool {
	return m.step == newStepFeatureName ||
		m.step == newStepFeatureDesc ||
		m.step == newStepScenarioName ||
		m.step == newStepConfigureAction
}

func (m newModel) incrementCursor() int {
	max := m.getMaxCursor()
	if m.cursor < max {
		return m.cursor + 1
	}
	return m.cursor
}

func (m newModel) getMaxCursor() int {
	switch m.step {
	case newStepSelectResource:
		return len(m.resources) - 1
	case newStepSelectAction:
		return len(m.availableActions) - 1
	case newStepAddMore:
		return 2 // Given/When/Then, Done with steps
	case newStepAddScenario:
		return 1 // Add another, Done
	case newStepConfirm:
		return 1 // Create, Cancel
	default:
		return 0
	}
}

func (m newModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case newStepFeatureName:
		name := m.textInputs[0].Value()
		if name == "" {
			name = "New feature"
		}
		m.featureName = name
		m.step = newStepFeatureDesc
		m.activeInput = 1
		m.textInputs[1].Focus()

	case newStepFeatureDesc:
		m.featureDesc = m.textInputs[1].Value()
		m.step = newStepScenarioName
		m.activeInput = 2
		m.textInputs[2].Focus()

	case newStepScenarioName:
		name := m.textInputs[2].Value()
		if name == "" {
			name = "New scenario"
		}
		m.currentScenario.name = name
		m.step = newStepAddMore
		m.cursor = 0

	case newStepAddMore:
		switch m.cursor {
		case 0: // Add Given
			m.currentActionType = "Given"
			m.step = newStepSelectResource
			m.cursor = 0
		case 1: // Add When
			m.currentActionType = "When"
			m.step = newStepSelectResource
			m.cursor = 0
		case 2: // Add Then
			m.currentActionType = "Then"
			m.step = newStepSelectResource
			m.cursor = 0
		case 3: // Done with steps
			m.scenarios = append(m.scenarios, m.currentScenario)
			m.step = newStepAddScenario
			m.cursor = 0
		}

	case newStepSelectResource:
		if m.cursor < len(m.resources) {
			m.currentResource = m.resources[m.cursor]
			m.availableActions = m.getActionsForResource(m.currentResource)
			m.step = newStepSelectAction
			m.cursor = 0
		}

	case newStepSelectAction:
		if m.cursor < len(m.availableActions) {
			m.selectedAction = m.availableActions[m.cursor]
			// Add the step
			step := stepAction{
				resource:   m.currentResource,
				actionType: m.currentActionType,
				action:     m.selectedAction,
			}
			m.currentScenario.steps = append(m.currentScenario.steps, step)
			m.step = newStepAddMore
			m.cursor = 0
		}

	case newStepAddScenario:
		if m.cursor == 0 { // Add another scenario
			m.currentScenario = scenario{steps: []stepAction{}}
			m.step = newStepScenarioName
			m.activeInput = 2
			m.textInputs[2].SetValue("")
			m.textInputs[2].Focus()
		} else { // Done
			m.step = newStepConfirm
			m.cursor = 0
		}

	case newStepConfirm:
		if m.cursor == 0 { // Create
			m.step = newStepDone
			return m, tea.Quit
		}
		// Cancel
		return m, tea.Quit
	}

	return m, nil
}

func (m newModel) getActionsForResource(resource string) []string {
	res, ok := m.cfg.Resources[resource]
	if !ok {
		return []string{}
	}

	switch res.Type {
	case "postgres", "postgresql", "mysql":
		return []string{
			fmt.Sprintf(`I set "%s" table "TABLE" with values`, resource),
			fmt.Sprintf(`I execute SQL on "%s"`, resource),
			fmt.Sprintf(`I execute SQL file "PATH" on "%s"`, resource),
			fmt.Sprintf(`"%s" table "TABLE" should contain`, resource),
			fmt.Sprintf(`"%s" table "TABLE" should be empty`, resource),
			fmt.Sprintf(`"%s" table "TABLE" should have "N" rows`, resource),
		}
	case "redis":
		return []string{
			fmt.Sprintf(`I set "%s" key "KEY" with value "VALUE"`, resource),
			fmt.Sprintf(`I set "%s" key "KEY" with JSON`, resource),
			fmt.Sprintf(`I delete "%s" key "KEY"`, resource),
			fmt.Sprintf(`"%s" key "KEY" should exist`, resource),
			fmt.Sprintf(`"%s" key "KEY" should not exist`, resource),
			fmt.Sprintf(`"%s" key "KEY" should have value "VALUE"`, resource),
			fmt.Sprintf(`"%s" should be empty`, resource),
		}
	case "kafka":
		return []string{
			fmt.Sprintf(`I create kafka topic "TOPIC" on "%s"`, resource),
			fmt.Sprintf(`I publish message to "%s" topic "TOPIC"`, resource),
			fmt.Sprintf(`I publish JSON to "%s" topic "TOPIC"`, resource),
			fmt.Sprintf(`I start consuming from "%s" topic "TOPIC"`, resource),
			fmt.Sprintf(`I should receive message from "%s" topic "TOPIC" within "5s"`, resource),
			fmt.Sprintf(`"%s" topic "TOPIC" should be empty`, resource),
		}
	case "rabbitmq":
		return []string{
			fmt.Sprintf(`I publish message to "%s" queue "QUEUE"`, resource),
			fmt.Sprintf(`I consume message from "%s" queue "QUEUE"`, resource),
		}
	case "http":
		return []string{
			fmt.Sprintf(`I set "%s" header "KEY" to "VALUE"`, resource),
			fmt.Sprintf(`I send "GET" request to "%s" "PATH"`, resource),
			fmt.Sprintf(`I send "POST" request to "%s" "PATH" with JSON`, resource),
			fmt.Sprintf(`I send "PUT" request to "%s" "PATH" with JSON`, resource),
			fmt.Sprintf(`I send "DELETE" request to "%s" "PATH"`, resource),
			fmt.Sprintf(`"%s" response status should be "200"`, resource),
			fmt.Sprintf(`"%s" response body should contain "TEXT"`, resource),
			fmt.Sprintf(`"%s" response JSON "PATH" should be "VALUE"`, resource),
		}
	case "websocket":
		return []string{
			fmt.Sprintf(`I connect to websocket "%s"`, resource),
			fmt.Sprintf(`I send message to websocket "%s"`, resource),
			fmt.Sprintf(`I send JSON to websocket "%s"`, resource),
			fmt.Sprintf(`I should receive message from websocket "%s" within "5s"`, resource),
			fmt.Sprintf(`I disconnect from websocket "%s"`, resource),
		}
	case "shell":
		return []string{
			fmt.Sprintf(`I run "COMMAND" on "%s"`, resource),
			fmt.Sprintf(`I run command on "%s"`, resource),
			fmt.Sprintf(`I run script "PATH" on "%s"`, resource),
			fmt.Sprintf(`I set "%s" environment variable "KEY" to "VALUE"`, resource),
			fmt.Sprintf(`I set "%s" working directory to "PATH"`, resource),
			fmt.Sprintf(`"%s" should succeed`, resource),
			fmt.Sprintf(`"%s" should fail`, resource),
			fmt.Sprintf(`"%s" exit code should be "0"`, resource),
			fmt.Sprintf(`"%s" stdout should contain "TEXT"`, resource),
			fmt.Sprintf(`"%s" stderr should be empty`, resource),
			fmt.Sprintf(`"%s" file "PATH" should exist`, resource),
		}
	default:
		return []string{}
	}
}

func (m newModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🍅 Tomato New Feature"))
	s.WriteString("\n")

	switch m.step {
	case newStepFeatureName:
		s.WriteString(subtitleStyle.Render("What's the name of your feature?"))
		s.WriteString("\n\n")
		s.WriteString(m.textInputs[0].View())
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Enter to continue"))

	case newStepFeatureDesc:
		s.WriteString(subtitleStyle.Render("Describe the feature (optional)"))
		s.WriteString("\n\n")
		s.WriteString(m.textInputs[1].View())
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Enter to continue"))

	case newStepScenarioName:
		s.WriteString(subtitleStyle.Render("What's the name of your scenario?"))
		s.WriteString("\n\n")
		s.WriteString(m.textInputs[2].View())
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Enter to continue"))

	case newStepAddMore:
		s.WriteString(subtitleStyle.Render(fmt.Sprintf("Scenario: %s", m.currentScenario.name)))
		s.WriteString("\n\n")

		// Show current steps
		if len(m.currentScenario.steps) > 0 {
			s.WriteString("Current steps:\n")
			for _, step := range m.currentScenario.steps {
				s.WriteString(fmt.Sprintf("  %s %s\n", step.actionType, step.action))
			}
			s.WriteString("\n")
		}

		s.WriteString("Add a step:\n")
		options := []string{"Given (setup)", "When (action)", "Then (assertion)", "Done with this scenario"}
		for i, opt := range options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt)))
		}

	case newStepSelectResource:
		s.WriteString(subtitleStyle.Render(fmt.Sprintf("Select resource for %s step", m.currentActionType)))
		s.WriteString("\n\n")
		for i, res := range m.resources {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			resType := m.cfg.Resources[res].Type
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s (%s)\n", cursor, style.Render(res), resType))
		}

	case newStepSelectAction:
		s.WriteString(subtitleStyle.Render(fmt.Sprintf("Select action for %s", m.currentResource)))
		s.WriteString("\n\n")
		for i, action := range m.availableActions {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(action)))
		}
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Replace TABLE, KEY, VALUE, etc. with actual values after generating"))

	case newStepAddScenario:
		s.WriteString(subtitleStyle.Render("Scenario added!"))
		s.WriteString("\n\n")
		options := []string{"Add another scenario", "Done - generate feature file"}
		for i, opt := range options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt)))
		}

	case newStepConfirm:
		s.WriteString(subtitleStyle.Render("Ready to create feature file!"))
		s.WriteString("\n\n")
		s.WriteString(m.renderPreview())
		s.WriteString("\n")
		options := []string{"Create feature file", "Cancel"}
		for i, opt := range options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt)))
		}

	case newStepDone:
		s.WriteString(successStyle.Render("✓ Feature file created!"))
	}

	return s.String()
}

func (m newModel) renderPreview() string {
	var s strings.Builder
	s.WriteString("Preview:\n")
	s.WriteString("─────────────────────────────────────────\n")
	s.WriteString(fmt.Sprintf("Feature: %s\n", m.featureName))
	if m.featureDesc != "" {
		s.WriteString(fmt.Sprintf("  %s\n", m.featureDesc))
	}
	s.WriteString("\n")
	for _, sc := range m.scenarios {
		s.WriteString(fmt.Sprintf("  Scenario: %s\n", sc.name))
		for _, step := range sc.steps {
			s.WriteString(fmt.Sprintf("    %s %s\n", step.actionType, step.action))
		}
		s.WriteString("\n")
	}
	s.WriteString("─────────────────────────────────────────\n")
	return s.String()
}

func (m newModel) generateFeatureFile() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("Feature: %s\n", m.featureName))
	if m.featureDesc != "" {
		s.WriteString(fmt.Sprintf("  %s\n", m.featureDesc))
	}
	s.WriteString("\n")

	for _, sc := range m.scenarios {
		s.WriteString(fmt.Sprintf("  Scenario: %s\n", sc.name))
		for _, step := range sc.steps {
			s.WriteString(fmt.Sprintf("    %s %s\n", step.actionType, step.action))
		}
		s.WriteString("\n")
	}

	return s.String()
}

func newFeature(c *cli.Context) error {
	// Load config
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w\nRun 'tomato init' first to create a config file", err)
	}

	if len(cfg.Resources) == 0 {
		return fmt.Errorf("no resources defined in config\nAdd resources to tomato.yml first")
	}

	// Run interactive wizard
	m := newInitialModel(cfg)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running wizard: %w", err)
	}

	finalModel := result.(newModel)

	if finalModel.step != newStepDone {
		fmt.Println("\nCancelled.")
		return nil
	}

	// Generate feature file
	content := finalModel.generateFeatureFile()

	// Determine filename
	filename := strings.ToLower(strings.ReplaceAll(finalModel.featureName, " ", "_")) + ".feature"

	// Use features path from config
	featuresPath := "./features"
	if len(cfg.Features.Paths) > 0 {
		featuresPath = cfg.Features.Paths[0]
	}

	// Ensure directory exists
	if err := os.MkdirAll(featuresPath, 0755); err != nil {
		return fmt.Errorf("creating features directory: %w", err)
	}

	filepath := filepath.Join(featuresPath, filename)

	// Check if file exists
	if _, err := os.Stat(filepath); err == nil {
		fmt.Printf("File %s already exists. Overwrite? [y/N] ", filepath)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing feature file: %w", err)
	}

	fmt.Println("\n" + successStyle.Render(fmt.Sprintf("✓ Created %s", filepath)))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the file and replace placeholder values (TABLE, KEY, VALUE, etc.)")
	fmt.Println("  2. Run " + selectedStyle.Render("tomato run"))
	fmt.Println()

	return nil
}
