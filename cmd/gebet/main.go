package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/colors"
	"github.com/alileza/gebet/config"
	"github.com/alileza/gebet/handler"
	"github.com/alileza/gebet/resource"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile   string
	featuresPath string
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "gebet bdd tools")
	app.Version(printVersion())
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

var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)

var versionInfoTmpl = `
{{.program}}, version {{.version}} (branch: {{.branch}}, revision: {{.revision}})
  build user:       {{.buildUser}}
  build date:       {{.buildDate}}
  go version:       {{.goVersion}}
`

func printVersion() string {
	m := map[string]string{
		"program":   "gebet",
		"version":   Version,
		"revision":  Revision,
		"branch":    Branch,
		"buildUser": BuildUser,
		"buildDate": BuildDate,
		"goVersion": GoVersion,
	}
	t := template.Must(template.New("version").Parse(versionInfoTmpl))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}
