package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/alecthomas/template"
	"github.com/alileza/tomato/config"
	"github.com/alileza/tomato/handler"
	"github.com/alileza/tomato/resource"
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
	app.Version(printVersion())
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

	defer resourceManager.Close()

	readyChan := make(chan struct{})
	go func(ch chan struct{}) {
		for {
			err := resourceManager.Ready()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stdout, "all resources ready!")
	case <-time.After(resourcesTimeout):
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "resource is not ready, giving up"))
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

// App version information
var Version, Revision, Branch, BuildUser, BuildDate string

func printVersion() string {
	var versionInfoTmpl = `
  {{.program}}, version {{.version}} (branch: {{.branch}}, revision: {{.revision}})
    build user:       {{.buildUser}}
    build date:       {{.buildDate}}
    go version:       {{.goVersion}}
  `

	m := map[string]string{
		"program":   "tomato",
		"version":   Version,
		"revision":  Revision,
		"branch":    Branch,
		"buildUser": BuildUser,
		"buildDate": BuildDate,
		"goVersion": runtime.Version(),
	}

	t := template.Must(template.New("version").Parse(versionInfoTmpl))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}
