package main

import (
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
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile       string
	featuresPath     string
	resourcesTimeout time.Duration
	resourcesCheck   bool
	debug            bool
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomato - behavioral testing tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	app.Flag("config.file", "tomato configuration file path.").Short('c').Default("tomato.yml").StringVar(&configFile)
	app.Flag("features.path", "tomato features folder path.").Short('f').Default("features").StringVar(&featuresPath)
	app.Flag("resources.timeout", "tomato will automatically wait for resource to be ready, and at some out it giving up.").Short('t').Default("10s").DurationVar(&resourcesTimeout)
	app.Flag("resources.check", "tomato only check if the resources is all ready, and exit without executing the tests.").Short('e').Default("false").BoolVar(&resourcesCheck)
	app.Flag("debug", "run in debug mode").Short('d').BoolVar(&debug)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		logrus.WithField("Error", err).Fatalf("Error parsing flag")
		os.Exit(1)
	}

	cfg, err := config.Retrieve(configFile)
	if err != nil {
		logrus.WithField("Error", err).Fatalf("Error retrieving config")
		os.Exit(1)
	}

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	resourceManager := resource.NewManager(cfg.Resources)

	defer resourceManager.Close()

	readyChan := make(chan struct{})
	go func(ch chan struct{}) {
		for {
			err := resourceManager.Ready()
			if err != nil {
				logrus.WithField("Error", err).Warn("Waiting for resource...")
				time.Sleep(time.Second)
				continue
			}
			if err == nil {
				readyChan <- struct{}{}
				break
			}
		}
	}(readyChan)

	select {
	case <-readyChan:
		logrus.Info("All resources are ready!")
	case <-time.After(resourcesTimeout):
		logrus.Info("Resources not ready, timeout exceeded")
		os.Exit(1)
	}
	if resourcesCheck {
		os.Exit(0)
	}

	opts := godog.Options{
		Output: colors.Colored(os.Stdout),
		Paths:  strings.Split(featuresPath, ","),
		Format: "progress",
		Strict: true,
	}

	if cfg.Randomize {
		opts.Randomize = time.Now().UTC().UnixNano()
	}

	if cfg.StopOnFailure {
		opts.StopOnFailure = cfg.StopOnFailure
	}

	os.Exit(
		godog.RunWithOptions("godogs", handler.New(resourceManager), opts),
	)
}
