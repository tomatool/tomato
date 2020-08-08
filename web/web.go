package web

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"syscall"

	"github.com/DATA-DOG/godog/colors"
	"github.com/ghodss/yaml"
	"github.com/markbates/pkger"
	"github.com/oklog/run"
	"github.com/urfave/cli"
	stdyaml "gopkg.in/yaml.v2"
)

type Web struct {
}

func New() *Web {
	return &Web{}
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

func jsonToYaml(y []byte) (map[string]interface{}, error) {
	by, err := yaml.JSONToYAML(y)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	if err := yaml.Unmarshal(by, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func (w *Web) Handler(ctx *cli.Context) error {
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

		return http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleGet(configPath, l)(w, r)
			case http.MethodPost:
				handlePost(configPath)(w, r)
			default:
				http.Error(w, "", http.StatusMethodNotAllowed)
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

func handleGet(configPath string, l net.Listener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("client") != "" {
			setupResponse(&w, r)
			switch r.Method {
			case http.MethodGet:
				f, err := pkger.Open("/dictionary.yml")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				dict, err := ioutil.ReadAll(f)
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
			if r.URL.Path == "/" {
				f, err := pkger.Open("/ui/build/index.html")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				index, err := ioutil.ReadAll(f)
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

			fs := http.FileServer(pkger.Dir("/ui/build"))
			fs.ServeHTTP(w, r)
		}
	}
}

func handlePost(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Printf("%s\n", b)
		res, err := jsonToYaml(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out, err := stdyaml.Marshal(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := ioutil.WriteFile(configPath, out, 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
