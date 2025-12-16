package command

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/tomatool/tomato/internal/apprunner"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/runlog"
	"github.com/tomatool/tomato/internal/runner"
	"github.com/urfave/cli/v2"
)

var runCommand = &cli.Command{
	Name:      "run",
	Usage:     "Run behavioral tests",
	ArgsUsage: "[feature files...]",
	Description: `Execute behavioral tests defined in Gherkin feature files.

Containers are started automatically, tests are executed, and resources
are reset between each scenario to ensure clean state.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "tomato.yml",
			Usage:   "config file path",
		},
		&cli.StringFlag{
			Name:    "tags",
			Aliases: []string{"t"},
			Usage:   "filter scenarios by tags (e.g. \"@smoke and not @slow\")",
		},
		&cli.StringFlag{
			Name:    "scenario",
			Aliases: []string{"s"},
			Usage:   "filter scenarios by name (regex pattern)",
		},
		&cli.BoolFlag{
			Name:  "no-reset",
			Usage: "skip state reset between scenarios (for debugging)",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "verbose output (show debug logs)",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "hide application logs during startup",
		},
		&cli.StringFlag{
			Name:   "format",
			Usage:  "output format (pretty, progress, tomato)",
			Hidden: true, // Internal use for UI
		},
	},
	Action: runTests,
}

func runTests(c *cli.Context) error {
	// Check for updates (non-blocking, skipped for RC versions)
	checkForUpdate()

	// Enable debug logging if verbose
	if c.Bool("verbose") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if c.NArg() > 0 {
		cfg.Features.Paths = c.Args().Slice()
	}

	if c.String("tags") != "" {
		cfg.Features.Tags = c.String("tags")
	}

	if c.String("scenario") != "" {
		cfg.Features.Scenario = c.String("scenario")
	}

	// Create run context for logging
	runCtx, err := runlog.New()
	if err != nil {
		return fmt.Errorf("failed to create run context: %w", err)
	}

	// Set up tomato output logging
	tomatoLog, err := runCtx.CreateLogFile("tomato")
	if err != nil {
		return fmt.Errorf("failed to create tomato log: %w", err)
	}
	defer tomatoLog.Close()

	// Tee stdout to both console and log file
	origStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	go func() {
		multiWriter := io.MultiWriter(origStdout, tomatoLog)
		io.Copy(multiWriter, stdoutR)
	}()

	// Restore stdout on function exit
	defer func() {
		stdoutW.Close()
		os.Stdout = origStdout
	}()

	fmt.Println()
	fmt.Println(titleStyle.Render("🍅 Tomato"))
	fmt.Printf("  %s run: %s\n", helpStyle.Render("📋"), runCtx.ID)
	if cfg.Features.Scenario != "" {
		fmt.Printf("  %s filtering scenarios matching: %s\n", helpStyle.Render("⚡"), cfg.Features.Scenario)
	}
	fmt.Println()

	// Step 1: Start dependency containers
	if len(cfg.Containers) > 0 {
		fmt.Println(subtitleStyle.Render("Starting dependencies..."))
	}

	cm, err := container.NewManager(cfg.Containers)
	if err != nil {
		return fmt.Errorf("failed to initialize container manager: %w", err)
	}
	cm.SetRunContext(runCtx)
	defer cm.Cleanup()

	if err := cm.StartAll(c.Context); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	// Show container status
	for name := range cfg.Containers {
		fmt.Printf("  %s %s\n", checkStyle.Render("✓"), name)
	}

	// Step 2: Start the application under test
	var appRunner *apprunner.Runner
	if cfg.App.IsConfigured() {
		fmt.Println()
		fmt.Println(subtitleStyle.Render("Starting application..."))
		if !c.Bool("quiet") {
			fmt.Println(helpStyle.Render("    (use -q to hide app logs)"))
			fmt.Println()
		}

		appRunner = apprunner.NewRunner(cfg.App, cm)
		appRunner.SetRunContext(runCtx)
		appRunner.SetShowLogs(!c.Bool("quiet"))

		if err := appRunner.Start(c.Context); err != nil {
			// Show recent logs on failure (even in quiet mode)
			fmt.Println()
			fmt.Println(errorStyle.Render("Application failed to start!"))
			fmt.Println()

			recentLogs := appRunner.GetRecentLogs(20)
			if len(recentLogs) > 0 {
				fmt.Println(subtitleStyle.Render("Recent application logs:"))
				fmt.Println(helpStyle.Render("─────────────────────────────────────────"))
				for _, line := range recentLogs {
					fmt.Printf("    │ %s\n", line)
				}
				fmt.Println(helpStyle.Render("─────────────────────────────────────────"))
				fmt.Println()
			}

			fmt.Printf("  %s Full logs available at: %s\n", helpStyle.Render("📄"), runCtx.LogPath("app"))
			fmt.Println()

			return fmt.Errorf("failed to start application: %w", err)
		}
		defer appRunner.Stop()

		if !c.Bool("quiet") {
			fmt.Println()
		}
		fmt.Printf("  %s app ready at %s\n", checkStyle.Render("✓"), appRunner.GetBaseURL())

		// Give app time to fully stabilize
		waitTime := cfg.App.Wait
		if waitTime == 0 {
			waitTime = 5 * time.Second // default
		}
		if waitTime > 0 {
			fmt.Printf("  waiting %s for app to stabilize...", waitTime)
			time.Sleep(waitTime)
			fmt.Println(" ready")
		}

		// Final health check before running tests
		if err := appRunner.VerifyHealthy(c.Context); err != nil {
			return fmt.Errorf("app health check failed: %w", err)
		}
		fmt.Printf("  %s health check passed\n", checkStyle.Render("✓"))
	}

	// Step 3: Initialize test resources
	fmt.Println()
	fmt.Println(subtitleStyle.Render("Initializing resources..."))

	r, err := runner.New(cfg, cm, runner.Options{
		NoReset: c.Bool("no-reset"),
		Format:  c.String("format"),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize runner: %w", err)
	}

	// Show resource status
	for name := range cfg.Resources {
		fmt.Printf("  %s %s\n", checkStyle.Render("✓"), name)
	}

	// Step 4: Run tests
	fmt.Println()
	fmt.Println(subtitleStyle.Render("Running tests..."))
	fmt.Println()

	return r.Run(c.Context)
}
