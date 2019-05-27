package main

import (
	"fmt"
	"os"

	"github.com/DATA-DOG/godog/colors"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"

	"github.com/tomatool/tomato/command"
)

type Command interface {
	Usage() string
	Desc() string
	Flags() []cli.Flag
	Run(*cli.Context) error
}

func main() {
	app := cli.NewApp()

	app.Name = "tomato"
	app.Usage = "Behavioral testing tools"
	app.Version = PrintVersion()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "load-env, e",
			Usage: "Load environment variables from a envFile",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if envFile := ctx.String("load-env"); envFile != "" {
			return godotenv.Load(envFile)
		}
		return nil
	}

	for cmdName, cmd := range map[string]Command{
		"run": &command.RunCommand{},
	} {
		app.Commands = append(app.Commands, cli.Command{
			Name:        cmdName,
			Description: cmd.Desc(),
			Usage:       cmd.Usage(),
			Flags:       cmd.Flags(),
			Action:      cmd.Run,
		})
	}

	if err := app.Run(os.Args); err != nil {
		redb := colors.Bold(colors.Red)
		fmt.Fprintf(os.Stderr, "%+v\n", redb(err))
		os.Exit(1)
	}
	os.Exit(0)
}
