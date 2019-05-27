package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/formatter"
	"github.com/tomatool/tomato/handler"
)

type RunCommand struct {
	config config.Config
}

func (r *RunCommand) Usage() string {
	return "Run runs tomato test suite"
}

func (r *RunCommand) Desc() string {
	return `Run runs tomato test suite. Reads all configuration from config path, 
	and override configuration from flags (if flag being passed).
	
	Test result will be reported on stdout`
}

func (r *RunCommand) Flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "features",
			Usage: "All feature file paths",
			Value: "features/",
		},
		cli.BoolFlag{
			Name:  "randomize",
			Usage: "Randomize will be used to run scenarios in a random order",
		},
		cli.BoolFlag{
			Name:  "stop-on-failure",
			Usage: "Stops on the first failure",
		},
	}
}

func (r *RunCommand) preRun(ctx *cli.Context) error {
	if len(ctx.Args()) != 1 {
		return fmt.Errorf("This command takes one argument: <config path>\nFor additional help try 'tomato run -help'")
	}
	configPath := ctx.Args()[0]

	if err := config.Unmarshal(configPath, &r.config); err != nil {
		return errors.Wrapf(err, "Failed to load config file of %s", configPath)
	}

	if ctx.IsSet("features") {
		r.config.Features = strings.Split(ctx.String("features"), ",")
	}
	if ctx.IsSet("randomize") {
		r.config.Randomize = ctx.Bool("randomize")
	}
	if ctx.IsSet("stop-on-failure") {
		r.config.StopOnFailure = ctx.Bool("stop-on-failure")
	}

	return r.config.IsValid()
}

func (r *RunCommand) Run(ctx *cli.Context) error {
	if err := r.preRun(ctx); err != nil {
		return err
	}

	godog.Format("tomato", "tomato custom godog formatter", formatter.New)

	opts := godog.Options{
		Output:        colors.Colored(os.Stdout),
		Paths:         r.config.Features,
		Format:        "tomato",
		Strict:        true,
		StopOnFailure: r.config.StopOnFailure,
	}

	if r.config.Randomize {
		opts.Randomize = time.Now().UTC().UnixNano()
	}

	h := handler.New()

	for _, r := range r.config.Resources {
		if err := h.Register(r.Name, r); err != nil {
			return errors.Wrapf(err, "%s: %s\n", colors.Red("Error"), err.Error())
		}
		for i := 0; i < 20; i++ {
			// Try to open, then check if ready
			if err := h.Open(r.Name); err == nil {
				if err := h.Ready(r.Name); err == nil {
					break
				}
			}

			time.Sleep(time.Second)
		}
		if err := h.Open(r.Name); err != nil {
			return errors.Wrapf(err, "%s: %s\n", colors.Red("Error"), err.Error())
		}
	}

	if result := godog.RunWithOptions("godogs", h.Handler(), opts); result != 0 {
		return errors.New("Test failed")
	}
	return nil
}
