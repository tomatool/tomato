package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/alileza/tomato/config"
	"github.com/alileza/tomato/formatter"
	"github.com/alileza/tomato/handler"
	"github.com/alileza/tomato/resource"
	"github.com/alileza/tomato/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile       string
	featuresPath     string
	resourcesTimeout time.Duration
	resourcesCheck   bool
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato - behavioral testing tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("config.file", "tomato configuration file path.").Short('c').Default("tomato.yml").StringVar(&configFile)
	app.Flag("features.path", "tomato features folder path.").Short('f').Default("features").StringVar(&featuresPath)
	app.Flag("resources.timeout", "tomato will automatically wait for resource to be ready, and at some out it giving up.").Short('t').Default("30s").DurationVar(&resourcesTimeout)
	app.Flag("resources.check", "tomato only check if the resources is all ready, and exit without executing the tests.").Short('e').Default("false").BoolVar(&resourcesCheck)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing flag"))
		os.Exit(1)
	}

	cfg, err := config.Retrieve(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	resourceManager := resource.NewManager(cfg.Resources)

	godog.Format("tomato", "tomato custom godog formatter", formatter.New)

	opts := godog.Options{
		Output: colors.Colored(os.Stdout),
		Paths:  strings.Split(featuresPath, ","),
		Format: "tomato",
		Strict: true,
	}

	if cfg.Randomize {
		opts.Randomize = time.Now().UTC().UnixNano()
	}

	if cfg.StopOnFailure {
		opts.StopOnFailure = cfg.StopOnFailure
	}

	if err := resourceManager.Ready(); err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Resources is not ready"))
		os.Exit(1)
	}

	if result := godog.RunWithOptions("godogs", handler.New(resourceManager), opts); result != 0 {
		os.Exit(result)
	}
}
