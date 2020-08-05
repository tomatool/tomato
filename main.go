package main

import (
	"log"
	"os"
	"strings"

	"github.com/DATA-DOG/godog/colors"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/tomato"
	"github.com/tomatool/tomato/web"
)

// AppHelpTemplate is the text template for the Default help topic.
// cli.go uses text/template to render templates. You can
// render custom help text by setting this variable.
const AppHelpTemplate = `Usage: {{if .UsageText}}{{.UsageText}}{{else}}tomato {{if .VisibleFlags}}[options]{{end}}{{if .ArgsUsage}}{{.ArgsUsage}}{{else}} <config path>{{end}}{{end}}

Options:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}
`

func main() {
	cli.AppHelpTemplate = AppHelpTemplate

	log := log.New(os.Stdout, "", 0)

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "env.file, e",
			Usage: "environment variable file path",
		},
		cli.StringFlag{
			Name:  "features.path, f",
			Usage: "features directory/file path (comma separated for multi path)",
		},
		cli.StringFlag{
			Name:   "config.file, c",
			Usage:  "[DEPRECATED PLEASE USE ARGUMENT] configuration file path",
			Hidden: true,
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:        "edit",
			Description: "edit tomato related file",
			Action:      web.New().Handler,
		},
		cli.Command{
			Name:        "run",
			Description: "Run tomato testing suite",
			Flags:       app.Flags,
			Before: func(ctx *cli.Context) error {
				if envFile := ctx.String("env.file"); envFile != "" {
					return godotenv.Load(envFile)
				}

				return nil
			},
			Action: runHandler(log),
		},
	}
	app.Action = runHandler(log)

	if err := app.Run(os.Args); err != nil {
		log.Printf("%v", colors.Bold(colors.Red)(err))
		os.Exit(1)
	}
}

func runHandler(log *log.Logger) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		// Initialize astilectron
		var configPath string

		// backward compability
		if c := ctx.String("config.file"); c != "" {
			log.Printf(colors.Bold(colors.Yellow)("Flag --config.file, -c is deprecated, please use args instead. For additional help try 'tomato -help'"))
			configPath = c
		}

		if len(ctx.Args()) == 1 {
			configPath = ctx.Args()[0]
		}

		if configPath == "" {
			return errors.New("This command takes one argument: <config path>\nFor additional help try 'tomato -help'")
		}

		conf, err := config.Retrieve(configPath)
		if err != nil {
			return errors.Wrap(err, "Failed to retrieve config")
		}

		if featuresPath := ctx.String("features.path"); featuresPath != "" {
			conf.FeaturesPaths = strings.Split(featuresPath, ",")
		}

		t := tomato.New(conf, log)

		if err := t.Verify(); err != nil {
			return errors.Wrap(err, "Verification failed")
		}

		return t.Run()
	}
}
