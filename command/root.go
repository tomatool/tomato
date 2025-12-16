package command

import (
	"github.com/tomatool/tomato/internal/version"
	"github.com/urfave/cli/v2"
)

func Run(args []string) error {
	app := &cli.App{
		Name:    "tomato",
		Usage:   "Behavioral testing toolkit with built-in container orchestration",
		Version: version.Version,
		Description: `Tomato is a language-agnostic behavioral testing framework that manages
your test infrastructure automatically. Define containers, resources, and tests
in a single tomato.yml file.

One config to rule them all.`,
		Commands: []*cli.Command{
			initCommand,
			runCommand,
			validateCommand,
			docsCommand,
			stepsCommand,
			uiCommand,
			versionCommand,
			updateCommand,
		},
	}

	return app.Run(args)
}
