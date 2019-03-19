package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/formatter"
	"github.com/tomatool/tomato/handler"
	"github.com/tomatool/tomato/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile       string
	featuresPath     string
	envFile          string
	resourcesTimeout time.Duration
	resourcesCheck   bool
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato - behavioral testing tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("config.file", "tomato configuration file path.").Short('c').Default("tomato.yml").StringVar(&configFile)
	app.Flag("features.path", "tomato features folder path.").Short('f').Default("features").StringVar(&featuresPath)
	app.Flag("env.file", "environment file path.").Short('e').StringVar(&envFile)
	app.Flag("resources.timeout", "tomato will automatically wait for resource to be ready, and at some out it giving up.").Short('t').Default("30s").DurationVar(&resourcesTimeout)
	app.Flag("resources.check", "tomato only check if the resources is all ready, and exit without executing the tests.").Short('r').Default("false").BoolVar(&resourcesCheck)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing flag"))
		os.Exit(1)
	}

	err = godotenv.Load(envFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error loading env file"))
		os.Exit(1)
	}

	cfg, err := config.Retrieve(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	featuresPaths := strings.Split(featuresPath, ",")
	if len(cfg.FeaturesPaths) > 0 {
		featuresPaths = cfg.FeaturesPaths
	}

	godog.Format("tomato", "tomato custom godog formatter", formatter.New)

	opts := godog.Options{
		Output: colors.Colored(os.Stdout),
		Paths:  featuresPaths,
		Format: "tomato",
		Strict: true,
	}

	if cfg.Randomize {
		opts.Randomize = time.Now().UTC().UnixNano()
	}

	if cfg.StopOnFailure {
		opts.StopOnFailure = cfg.StopOnFailure
	}

	h := handler.New()

	for _, r := range cfg.Resources {
		if err := h.Register(r.Name, r); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", colors.Red("Error"), err.Error())
			os.Exit(1)
		}
		if r.ReadyCheck {
			for i := 0; i < 20; i++ {
				if err := h.Ready(r.Name); err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			if err := h.Ready(r.Name); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", colors.Red("Error"), err.Error())
				os.Exit(1)
			}
		}
	}

	if result := godog.RunWithOptions("godogs", h.Handler(), opts); result != 0 {
		os.Exit(result)
	}
}
