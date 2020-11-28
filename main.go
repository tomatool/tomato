package main

import (
	"fmt"
	"os"

	"github.com/DATA-DOG/godog/colors"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/tomatool/tomato/cmd"
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
	// cli.AppHelpTemplate = AppHelpTemplate

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "env.file",
			Aliases: []string{"e"},
			Usage:   "environment variable file path",
		},
	}
	app.Commands = []*cli.Command{
		cmd.InitCmd,
		cmd.RunCmd,
	}

	app.Before = func(ctx *cli.Context) error {
		if envFile := ctx.String("env.file"); envFile != "" {
			return godotenv.Load(envFile)
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", colors.Bold(colors.Red)(err))
		os.Exit(1)
	}
}
