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
	"github.com/alileza/tomato/handler"
	"github.com/alileza/tomato/resource"
	"github.com/alileza/tomato/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile   string
	featuresPath string
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato bdd tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("config.file", "tomato configuration file path.").Short('c').Default("tomato.yml").StringVar(&configFile)
	app.Flag("features.path", "tomato features folder path.").Short('f').Default("features").StringVar(&featuresPath)

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

	defer resourceManager.Close()

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
