package cmd

import (
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/tomato"
)

var RunCmd *cli.Command = &cli.Command{
	Name:  "run",
	Usage: "Run tomato testing suite",
	Action: func(ctx *cli.Context) error {
		log := log.New(os.Stdout, "", 0)

		var configPath string
		if ctx.Args().Len() == 1 {
			configPath = ctx.Args().First()
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
	},
}
