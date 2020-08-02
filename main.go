package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/DATA-DOG/godog/colors"
	"github.com/ghodss/yaml"
	"github.com/gobuffalo/packr"
	"github.com/joho/godotenv"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/tomato"
)

// AppHelpTemplate is the text template for the Default help topic.
// cli.go uses text/template to render templates. You can
// render custom help text by setting this variable.
const AppHelpTemplate = `Usage: {{if .UsageText}}{{.UsageText}}{{else}}tomato {{if .VisibleFlags}}[options]{{end}}{{if .ArgsUsage}}{{.ArgsUsage}}{{else}} <config path>{{end}}{{end}}

Options:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}
`

func main() {
	cli.AppHelpTemplate = AppHelpTemplate

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
		cli.StringFlag{
			Name:   "config.file, c",
			Usage:  "[DEPRECATED PLEASE USE ARGUMENT] configuration file path",
			Hidden: true,
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:        "edit",
			Description: "edit tomato related file",
			Action:      editServer,
		},
		cli.Command{
			Name:        "run",
			Description: "Run tomato testing suite",
			Flags:       app.Flags,
			Before: func(ctx *cli.Context) error {
				if envFile := ctx.String("env.file"); envFile != "" {
					return godotenv.Load(envFile)
				}

				return nil
			},
			Action: func(ctx *cli.Context) error {
				// Initialize astilectron
				var configPath string

				// backward compability
				if c := ctx.String("config.file"); c != "" {
					log.Printf(colors.Bold(colors.Yellow)("Flag --config.file, -c is deprecated, please use args instead. For additional help try 'tomato -help'"))
					configPath = c
				}

				if len(ctx.Args()) == 1 {
					configPath = ctx.Args()[0]
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
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Printf("%v", colors.Bold(colors.Red)(err))
		os.Exit(1)
	}
}

func openbrowser(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return
}

func yamlToJSON(y []byte) (map[string]interface{}, error) {
	by, err := yaml.YAMLToJSON(y)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(by, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func editServer(ctx *cli.Context) error {
	var g run.Group
	var configPath string
	if len(ctx.Args()) == 1 {
		configPath = ctx.Args()[0]
	}
	if configPath == "" {
		return errors.New("This command takes one argument: <config path>\nFor additional help try 'tomato edit -help'")
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	printErr := func(err error) {
		l.Close()
		if err != nil {
			log.Printf(colors.Bold(colors.Red)("ERR: %v"), err)
		}
	}
	g.Add(func() error {

		if err := openbrowser("http://" + l.Addr().String() + "/index.html"); err != nil {
			return err
		}

		box := packr.NewBox(".")

		return http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("client") != "" {
				setupResponse(&w, r)
				switch r.Method {
				case http.MethodGet:
					dict, err := box.Find("./dictionary.yml")
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					dictMap, err := yamlToJSON(dict)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					cfg, err := ioutil.ReadFile(configPath)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					config, err := yamlToJSON(cfg)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					b, err := json.Marshal(map[string]interface{}{
						"config":     config,
						"dictionary": dictMap,
					})
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.Write(b)
					return
				}
			} else {
				if r.URL.Path == "" {
					index, err := box.Find("./ui/build/index.html")
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					var buf bytes.Buffer

					t := template.Must(template.New("config").Parse(string(index)))
					if err := t.ExecuteTemplate(&buf, "config", map[string]string{"ServerURL": "http://" + l.Addr().String()}); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					w.Write(index)
					return
				}

				fs := http.FileServer(http.Dir("./ui/build"))
				fs.ServeHTTP(w, r)
			}
		}))
	}, printErr)

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	g.Add(func() error {
		select {
		case <-term:
			log.Printf(colors.Bold(colors.Yellow)("Received SIGTERM, exiting gracefully..."))
		}
		return nil
	}, printErr)

	return g.Run()
}
