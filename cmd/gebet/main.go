package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/alileza/gebet/config"
	"github.com/alileza/gebet/handler"
	"github.com/alileza/gebet/resource"
	"github.com/alileza/gebet/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile   string
	featuresPath string
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "gebet bdd tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("config.file", "gebet configuration file path.").Short('c').Default("gebet.yml").StringVar(&configFile)
	app.Flag("features.path", "gebet features folder path.").Short('f').Default("features").StringVar(&featuresPath)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	cfg, err := config.Retrieve(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	resourceManager := resource.NewManager(cfg.Resources)

	os.Exit(
		godog.RunWithOptions("godogs", handler.New(resourceManager), godog.Options{
			StopOnFailure: true,
			Output:        colors.Colored(os.Stdout),
			Paths:         strings.Split(featuresPath, ","),
			Format:        "progress",
			Randomize:     time.Now().UTC().UnixNano(), // randomize scenario execution order
		}),
	)
}
