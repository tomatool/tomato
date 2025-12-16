package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

// Styles for interactive CLI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	checkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Bold(true)

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)
)

var initCommand = &cli.Command{
	Name:  "init",
	Usage: "Initialize a new tomato project",
	Description: `Create tomato.yml configuration interactively.

Guides you through selecting your application dependencies and
configuring how to run your application for testing.`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "overwrite existing files",
		},
	},
	Action: runInit,
}

// Resource represents a selectable dependency
type resource struct {
	name        string
	description string
	key         string
}

var availableResources = []resource{
	{"PostgreSQL", "SQL database for relational data", "postgresql"},
	{"MySQL", "SQL database for relational data", "mysql"},
	{"Redis", "In-memory cache and key-value store", "redis"},
	{"Kafka", "Distributed event streaming platform", "kafka"},
	{"RabbitMQ", "Message broker for async messaging", "rabbitmq"},
	{"External HTTP", "Mock external HTTP API dependencies", "http"},
}

type initStep int

const (
	stepDependencies initStep = iota
	stepAppRunner
	stepDockerfile
	stepCommand
	stepConfirm
)

type initModel struct {
	step     initStep
	cursor   int
	selected map[string]bool

	// App runner config
	runnerType     string // "docker" or "command"
	dockerfile     string
	customCommand  string
	dockerfiles    []string // found dockerfiles

	// Text input state
	textInput    string
	textCursor   int

	done      bool
	cancelled bool
}

func initialInitModel() initModel {
	// Find dockerfiles in current directory
	dockerfiles := findDockerfiles()

	return initModel{
		step:        stepDependencies,
		selected:    make(map[string]bool),
		dockerfiles: dockerfiles,
	}
}

func findDockerfiles() []string {
	var files []string

	// Check common dockerfile names
	candidates := []string{
		"Dockerfile",
		"dockerfile",
		"Dockerfile.dev",
		"Dockerfile.test",
		"docker/Dockerfile",
		"build/Dockerfile",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			files = append(files, c)
		}
	}

	// Also search for any Dockerfile* in current dir
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "Dockerfile") {
			// Avoid duplicates
			found := false
			for _, f := range files {
				if f == e.Name() {
					found = true
					break
				}
			}
			if !found {
				files = append(files, e.Name())
			}
		}
	}

	return files
}

func (m initModel) Init() tea.Cmd {
	return nil
}

func (m initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle text input mode
		if m.step == stepCommand {
			return m.handleTextInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.step != stepCommand {
				m.cancelled = true
				return m, tea.Quit
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			max := m.getMaxCursor()
			if m.cursor < max {
				m.cursor++
			}

		case " ", "x":
			if m.step == stepDependencies {
				key := availableResources[m.cursor].key
				m.selected[key] = !m.selected[key]
			}

		case "enter":
			return m.handleEnter()

		case "a":
			if m.step == stepDependencies {
				for _, r := range availableResources {
					m.selected[r.key] = true
				}
			}

		case "n":
			if m.step == stepDependencies {
				m.selected = make(map[string]bool)
			}
		}
	}

	return m, nil
}

func (m initModel) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		m.customCommand = m.textInput
		m.step = stepConfirm
		m.cursor = 0
		return m, nil
	case "backspace":
		if len(m.textInput) > 0 {
			m.textInput = m.textInput[:len(m.textInput)-1]
		}
	case "esc":
		m.step = stepAppRunner
		m.cursor = 0
		m.textInput = ""
	default:
		if len(msg.String()) == 1 {
			m.textInput += msg.String()
		} else if msg.String() == "space" {
			m.textInput += " "
		}
	}
	return m, nil
}

func (m initModel) getMaxCursor() int {
	switch m.step {
	case stepDependencies:
		return len(availableResources) - 1
	case stepAppRunner:
		return 2 // Docker, Command, Skip
	case stepDockerfile:
		return len(m.dockerfiles) // dockerfiles + "Other path"
	case stepConfirm:
		return 1 // Create, Cancel
	default:
		return 0
	}
}

func (m initModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepDependencies:
		m.step = stepAppRunner
		m.cursor = 0

	case stepAppRunner:
		switch m.cursor {
		case 0: // Docker
			m.runnerType = "docker"
			if len(m.dockerfiles) > 0 {
				m.step = stepDockerfile
			} else {
				m.step = stepCommand
				m.textInput = "./Dockerfile"
			}
			m.cursor = 0
		case 1: // Custom command
			m.runnerType = "command"
			m.step = stepCommand
			m.textInput = ""
			m.cursor = 0
		case 2: // Skip
			m.runnerType = ""
			m.step = stepConfirm
			m.cursor = 0
		}

	case stepDockerfile:
		if m.cursor < len(m.dockerfiles) {
			m.dockerfile = m.dockerfiles[m.cursor]
			m.step = stepConfirm
			m.cursor = 0
		} else {
			// Other path - go to text input
			m.step = stepCommand
			m.textInput = "./Dockerfile"
		}

	case stepConfirm:
		if m.cursor == 0 {
			m.done = true
		} else {
			m.cancelled = true
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m initModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🍅 Tomato Init"))
	s.WriteString("\n")

	switch m.step {
	case stepDependencies:
		s.WriteString(subtitleStyle.Render("What are your application's dependencies?"))
		s.WriteString("\n\n")

		for i, r := range availableResources {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			checked := "[ ]"
			if m.selected[r.key] {
				checked = checkStyle.Render("[✓]")
			}

			nameStyle := unselectedStyle
			if i == m.cursor {
				nameStyle = selectedStyle
			}

			s.WriteString(fmt.Sprintf("%s%s %s", cursor, checked, nameStyle.Render(r.name)))
			if i == m.cursor {
				s.WriteString(helpStyle.Render(fmt.Sprintf("  %s", r.description)))
			}
			s.WriteString("\n")
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("SPACE select • a all • n none • ENTER continue"))

	case stepAppRunner:
		s.WriteString(subtitleStyle.Render("How do you run your application?"))
		s.WriteString("\n\n")

		options := []struct {
			name string
			desc string
		}{
			{"Docker", "Build and run from Dockerfile"},
			{"Custom command", "Run with shell command (e.g., go run, npm start)"},
			{"Skip", "Configure manually later"},
		}

		for i, opt := range options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}

			s.WriteString(fmt.Sprintf("%s%s", cursor, style.Render(opt.name)))
			if i == m.cursor {
				s.WriteString(helpStyle.Render(fmt.Sprintf("  %s", opt.desc)))
			}
			s.WriteString("\n")
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("ENTER select"))

	case stepDockerfile:
		s.WriteString(subtitleStyle.Render("Select Dockerfile"))
		s.WriteString("\n\n")

		for i, df := range m.dockerfiles {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}

			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(df)))
		}

		// Other option
		cursor := "  "
		if m.cursor == len(m.dockerfiles) {
			cursor = "> "
		}
		style := unselectedStyle
		if m.cursor == len(m.dockerfiles) {
			style = selectedStyle
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render("Other path...")))

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("ENTER select"))

	case stepCommand:
		if m.runnerType == "docker" {
			s.WriteString(subtitleStyle.Render("Enter Dockerfile path:"))
		} else {
			s.WriteString(subtitleStyle.Render("Enter command to run your application:"))
		}
		s.WriteString("\n\n")

		// Show text input
		s.WriteString("> ")
		s.WriteString(m.textInput)
		s.WriteString("█")
		s.WriteString("\n\n")

		if m.runnerType == "command" {
			s.WriteString(helpStyle.Render("Examples: go run ./cmd/server, npm start, python app.py"))
			s.WriteString("\n")
		}
		s.WriteString(helpStyle.Render("ENTER confirm • ESC back"))

	case stepConfirm:
		s.WriteString(subtitleStyle.Render("Ready to create configuration"))
		s.WriteString("\n\n")

		// Summary
		s.WriteString("Dependencies:\n")
		hasAny := false
		for _, r := range availableResources {
			if m.selected[r.key] {
				s.WriteString(fmt.Sprintf("  • %s\n", r.name))
				hasAny = true
			}
		}
		if !hasAny {
			s.WriteString("  (none)\n")
		}
		s.WriteString("\n")

		s.WriteString("Application:\n")
		switch m.runnerType {
		case "docker":
			s.WriteString(fmt.Sprintf("  Docker: %s\n", m.dockerfile))
		case "command":
			s.WriteString(fmt.Sprintf("  Command: %s\n", m.customCommand))
		default:
			s.WriteString("  (not configured)\n")
		}
		s.WriteString("\n")

		options := []string{"Create tomato.yml", "Cancel"}
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
	}

	return s.String()
}

func runInit(c *cli.Context) error {
	// Check if tomato.yml already exists
	if _, err := os.Stat("tomato.yml"); err == nil && !c.Bool("force") {
		return fmt.Errorf("tomato.yml already exists (use --force to overwrite)")
	}

	// Run interactive wizard
	m := initialInitModel()
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running init: %w", err)
	}

	finalModel := result.(initModel)

	if finalModel.cancelled || !finalModel.done {
		fmt.Println("\nCancelled.")
		return nil
	}

	// Generate config
	config := generateTomatoConfig(finalModel)

	// Write tomato.yml
	if err := os.WriteFile("tomato.yml", []byte(config), 0644); err != nil {
		return fmt.Errorf("creating tomato.yml: %w", err)
	}

	// Create features directory
	if err := os.MkdirAll("./features", 0755); err != nil {
		return fmt.Errorf("creating features directory: %w", err)
	}

	// Create example feature file
	examplePath := filepath.Join("./features", "example.feature")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) || c.Bool("force") {
		feature := generateExampleFeatureFile(finalModel)
		if err := os.WriteFile(examplePath, []byte(feature), 0644); err != nil {
			return fmt.Errorf("creating example feature: %w", err)
		}
	}

	fmt.Println("\n" + successStyle.Render("✓ Created tomato.yml"))
	fmt.Println(successStyle.Render("✓ Created ./features/example.feature"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review tomato.yml and adjust as needed")
	fmt.Println("  2. Write your feature files")
	fmt.Println("  3. Run " + selectedStyle.Render("tomato run"))
	fmt.Println()

	return nil
}

func generateTomatoConfig(m initModel) string {
	var s strings.Builder

	s.WriteString("version: 2\n\n")

	// Settings
	s.WriteString("settings:\n")
	s.WriteString("  timeout: 5m\n")
	s.WriteString("  parallel: 1\n")
	s.WriteString("  fail_fast: false\n")
	s.WriteString("  output: pretty\n")
	s.WriteString("  reset:\n")
	s.WriteString("    level: scenario\n")
	s.WriteString("\n")

	// App section - how to run the application under test
	if m.runnerType != "" {
		s.WriteString("# Application under test\n")
		s.WriteString("app:\n")

		if m.runnerType == "docker" && m.dockerfile != "" {
			s.WriteString("  build:\n")
			s.WriteString(fmt.Sprintf("    dockerfile: %s\n", m.dockerfile))
		} else if m.runnerType == "command" && m.customCommand != "" {
			s.WriteString(fmt.Sprintf("  command: %s\n", m.customCommand))
		}

		s.WriteString("  port: 8080\n")
		s.WriteString("  ready:\n")
		s.WriteString("    type: http\n")
		s.WriteString("    path: /health\n")
		s.WriteString("    timeout: 30s\n")
		s.WriteString("  wait: 5s  # time to wait after ready check before running tests\n")
		s.WriteString("  # Environment variables for your app (uses container connection info)\n")
		s.WriteString("  env:\n")

		// Add database connection
		if m.selected["postgresql"] {
			s.WriteString("    DATABASE_URL: postgres://postgres:postgres@{{.postgres.host}}:{{.postgres.port.5432}}/testdb\n")
		}
		if m.selected["mysql"] {
			s.WriteString("    DATABASE_URL: mysql://root:root@{{.mysql.host}}:{{.mysql.port.3306}}/testdb\n")
		}
		if m.selected["redis"] {
			s.WriteString("    REDIS_URL: redis://{{.redis.host}}:{{.redis.port.6379}}\n")
		}
		if m.selected["kafka"] {
			s.WriteString("    KAFKA_BROKERS: {{.kafka.host}}:{{.kafka.port.9092}}\n")
		}
		if m.selected["rabbitmq"] {
			s.WriteString("    RABBITMQ_URL: amqp://guest:guest@{{.rabbitmq.host}}:{{.rabbitmq.port.5672}}\n")
		}

		s.WriteString("\n")
	}

	// Containers (dependencies)
	s.WriteString("# Dependency containers\n")
	s.WriteString("containers:\n")

	if m.selected["postgresql"] {
		s.WriteString("  postgres:\n")
		s.WriteString("    image: postgres:15-alpine\n")
		s.WriteString("    env:\n")
		s.WriteString("      POSTGRES_USER: postgres\n")
		s.WriteString("      POSTGRES_PASSWORD: postgres\n")
		s.WriteString("      POSTGRES_DB: testdb\n")
		s.WriteString("    ports:\n")
		s.WriteString("      - \"5432/tcp\"\n")
		s.WriteString("    wait_for:\n")
		s.WriteString("      type: port\n")
		s.WriteString("      target: \"5432/tcp\"\n")
		s.WriteString("      timeout: 30s\n")
		s.WriteString("\n")
	}

	if m.selected["mysql"] {
		s.WriteString("  mysql:\n")
		s.WriteString("    image: mysql:8\n")
		s.WriteString("    env:\n")
		s.WriteString("      MYSQL_ROOT_PASSWORD: root\n")
		s.WriteString("      MYSQL_DATABASE: testdb\n")
		s.WriteString("    ports:\n")
		s.WriteString("      - \"3306/tcp\"\n")
		s.WriteString("    wait_for:\n")
		s.WriteString("      type: log\n")
		s.WriteString("      target: \"ready for connections\"\n")
		s.WriteString("      timeout: 60s\n")
		s.WriteString("\n")
	}

	if m.selected["redis"] {
		s.WriteString("  redis:\n")
		s.WriteString("    image: redis:7-alpine\n")
		s.WriteString("    ports:\n")
		s.WriteString("      - \"6379/tcp\"\n")
		s.WriteString("    wait_for:\n")
		s.WriteString("      type: port\n")
		s.WriteString("      target: \"6379/tcp\"\n")
		s.WriteString("\n")
	}

	if m.selected["kafka"] {
		s.WriteString("  kafka:\n")
		s.WriteString("    image: confluentinc/cp-kafka:7.5.0\n")
		s.WriteString("    env:\n")
		s.WriteString("      KAFKA_NODE_ID: \"1\"\n")
		s.WriteString("      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT\n")
		s.WriteString("      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093\n")
		s.WriteString("      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092\n")
		s.WriteString("      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@localhost:9093\n")
		s.WriteString("      KAFKA_PROCESS_ROLES: broker,controller\n")
		s.WriteString("      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER\n")
		s.WriteString("      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk\n")
		s.WriteString("    ports:\n")
		s.WriteString("      - \"9092/tcp\"\n")
		s.WriteString("    wait_for:\n")
		s.WriteString("      type: log\n")
		s.WriteString("      target: \"Kafka Server started\"\n")
		s.WriteString("      timeout: 60s\n")
		s.WriteString("\n")
	}

	if m.selected["rabbitmq"] {
		s.WriteString("  rabbitmq:\n")
		s.WriteString("    image: rabbitmq:3-management-alpine\n")
		s.WriteString("    env:\n")
		s.WriteString("      RABBITMQ_DEFAULT_USER: guest\n")
		s.WriteString("      RABBITMQ_DEFAULT_PASS: guest\n")
		s.WriteString("    ports:\n")
		s.WriteString("      - \"5672/tcp\"\n")
		s.WriteString("      - \"15672/tcp\"\n")
		s.WriteString("    wait_for:\n")
		s.WriteString("      type: port\n")
		s.WriteString("      target: \"5672/tcp\"\n")
		s.WriteString("      timeout: 30s\n")
		s.WriteString("\n")
	}

	// Resources
	s.WriteString("resources:\n")

	if m.selected["postgresql"] {
		s.WriteString("  db:\n")
		s.WriteString("    type: postgres\n")
		s.WriteString("    container: postgres\n")
		s.WriteString("    database: testdb\n")
		s.WriteString("    options:\n")
		s.WriteString("      user: postgres\n")
		s.WriteString("      password: postgres\n")
		s.WriteString("\n")
	}

	if m.selected["mysql"] {
		s.WriteString("  db:\n")
		s.WriteString("    type: mysql\n")
		s.WriteString("    container: mysql\n")
		s.WriteString("    database: testdb\n")
		s.WriteString("    options:\n")
		s.WriteString("      user: root\n")
		s.WriteString("      password: root\n")
		s.WriteString("\n")
	}

	if m.selected["redis"] {
		s.WriteString("  cache:\n")
		s.WriteString("    type: redis\n")
		s.WriteString("    container: redis\n")
		s.WriteString("    options:\n")
		s.WriteString("      db: 0\n")
		s.WriteString("\n")
	}

	if m.selected["kafka"] {
		s.WriteString("  queue:\n")
		s.WriteString("    type: kafka\n")
		s.WriteString("    container: kafka\n")
		s.WriteString("    options:\n")
		s.WriteString("      topics:\n")
		s.WriteString("        - events\n")
		s.WriteString("\n")
	}

	if m.selected["rabbitmq"] {
		s.WriteString("  queue:\n")
		s.WriteString("    type: rabbitmq\n")
		s.WriteString("    container: rabbitmq\n")
		s.WriteString("\n")
	}

	if m.selected["http"] {
		s.WriteString("  http:\n")
		s.WriteString("    type: wiremock\n")
		s.WriteString("    options:\n")
		s.WriteString("      # Configure external API mocks here\n")
		s.WriteString("\n")
	}

	// Shell is always useful
	s.WriteString("  shell:\n")
	s.WriteString("    type: shell\n")
	s.WriteString("    options:\n")
	s.WriteString("      timeout: 30s\n")
	if m.runnerType == "command" && m.customCommand != "" {
		s.WriteString(fmt.Sprintf("      # app_command: %s\n", m.customCommand))
	}
	s.WriteString("\n")

	// Hooks
	s.WriteString("hooks:\n")
	s.WriteString("  before_all: []\n")
	s.WriteString("  after_all: []\n")
	s.WriteString("  before_scenario: []\n")
	s.WriteString("  after_scenario: []\n")
	s.WriteString("\n")

	// Features
	s.WriteString("features:\n")
	s.WriteString("  paths:\n")
	s.WriteString("    - ./features\n")

	return s.String()
}

func generateExampleFeatureFile(m initModel) string {
	var s strings.Builder

	s.WriteString("Feature: Example feature\n")
	s.WriteString("  As a developer\n")
	s.WriteString("  I want to test my application\n")
	s.WriteString("  So that I can ensure it works correctly\n\n")

	if m.selected["postgresql"] || m.selected["mysql"] {
		s.WriteString("  Scenario: Database operations\n")
		s.WriteString("    Given I set \"db\" table \"users\" with values:\n")
		s.WriteString("      | id | name  | email           |\n")
		s.WriteString("      | 1  | Alice | alice@test.com  |\n")
		s.WriteString("    Then \"db\" table \"users\" should have \"1\" rows\n\n")
	}

	if m.selected["redis"] {
		s.WriteString("  Scenario: Cache operations\n")
		s.WriteString("    Given I set \"cache\" key \"session:123\" with value \"user_data\"\n")
		s.WriteString("    Then \"cache\" key \"session:123\" should exist\n\n")
	}

	if m.selected["kafka"] {
		s.WriteString("  Scenario: Kafka messaging\n")
		s.WriteString("    Given I start consuming from \"queue\" topic \"events\"\n")
		s.WriteString("    When I publish JSON to \"queue\" topic \"events\":\n")
		s.WriteString("      \"\"\"\n")
		s.WriteString("      {\"type\": \"test_event\"}\n")
		s.WriteString("      \"\"\"\n")
		s.WriteString("    Then I should receive message from \"queue\" topic \"events\" within \"5s\"\n\n")
	}

	// Always include shell example
	s.WriteString("  Scenario: Shell commands\n")
	s.WriteString("    When I run \"echo hello\" on \"shell\"\n")
	s.WriteString("    Then \"shell\" should succeed\n")
	s.WriteString("    And \"shell\" stdout should contain \"hello\"\n")

	return s.String()
}
