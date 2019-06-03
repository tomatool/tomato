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
)

func main() {
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
	}

	app.Before = func(ctx *cli.Context) error {
		if envFile := ctx.String("env.file"); envFile != "" {
			return godotenv.Load(envFile)
		}

		return nil
	}

	app.Action = func(ctx *cli.Context) error {
		if len(ctx.Args()) != 1 {
			return errors.New("This command takes one argument: <config path>\nFor additional help try 'tomato -help'")
		}

		conf, err := config.Retrieve(ctx.Args()[0])
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

	if err := app.Run(os.Args); err != nil {
		log.Printf("%v", colors.Bold(colors.Red)(err))
		os.Exit(1)
	}
}
