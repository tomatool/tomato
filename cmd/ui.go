package cmd

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/dictionary"
	"github.com/urfave/cli/v2"
)

func uiFileServerMux(staticFilePath string) http.Handler {
	return http.FileServer(http.Dir(staticFilePath))
}

func uiProxyMux(uiProxyURL string) http.Handler {
	origin, err := url.Parse(uiProxyURL)
	if err != nil {
		panic("Failed to parse UIProxyURL: " + err.Error())
	}

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", origin.Host)
			req.URL.Scheme = origin.Scheme
			req.URL.Host = origin.Host
		},
	}
}

func getDirPath(str string) string {
	l := strings.Split(str, "/")
	l = l[:len(l)-1]
	return strings.Join(l, "/")
}

func visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".feature" {
			return nil
		}
		*files = append(*files, path)
		return nil
	}
}

func unique(s []string) []string {
	unique := make(map[string]struct{}, len(s))
	us := make([]string, len(unique))
	for _, elem := range s {
		if len(elem) != 0 {
			if _, ok := unique[elem]; !ok {
				us = append(us, elem)
				unique[elem] = struct{}{}
			}
		}
	}
	return us
}

func getFiles(basePath string, dirs []string) ([]string, error) {
	var result []string
	for _, dir := range dirs {
		var files []string
		err := filepath.Walk(basePath+"/"+dir, visit(&files))
		if err != nil {
			return nil, err
		}
		result = append(result, files...)
	}

	re, err := regexp.Compile("/+")
	if err != nil {
		return nil, err
	}
	for i := range result {
		result[i] = re.ReplaceAllLiteralString(result[i], "/")
	}
	return unique(result), nil
}

func getConfigMux(configPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configDirPath := getDirPath(configPath)

		config, err := config.Retrieve(configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		files, err := getFiles(configDirPath, config.FeaturesPaths)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		config.FeaturesPaths = files
		if err := json.NewEncoder(w).Encode(config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func getDictionaryMux(dictionaryPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dict, err := dictionary.Retrieve(dictionaryPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(dict); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

var UIInputs struct {
	Addr           string
	ProxyEnabled   bool
	ProxyURL       string
	StaticFilePath string
	DictionaryPath string
}

var UICmd *cli.Command = &cli.Command{
	Name:  "ui",
	Usage: "Open tomato UI",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "addr",
			Destination: &UIInputs.Addr,
			Usage:       "Specifies the HTTP address for the server to listen on",
			Value:       "0.0.0.0:9000",
		},
		&cli.StringFlag{
			Name:        "static-file-path",
			Destination: &UIInputs.StaticFilePath,
		},
		&cli.BoolFlag{
			Name:        "proxy-enabled",
			Destination: &UIInputs.ProxyEnabled,
			Hidden:      true,
		},
		&cli.StringFlag{
			Name:        "proxy-url",
			Destination: &UIInputs.ProxyURL,
			Usage:       "Specifies the HTTP address server to proxy to, this feature is intentionally exist for development purpose.",
			Value:       "http://localhost:3000",
			Hidden:      true,
		},
		&cli.StringFlag{
			Name:        "dictionary-path",
			Destination: &UIInputs.DictionaryPath,
			Usage:       "Tomato dictionary path.",
			Value:       "dictionary.yml",
		},
	},
	Action: func(ctx *cli.Context) error {
		var configPath string
		if ctx.Args().Len() == 1 {
			configPath = ctx.Args().First()
		}

		if configPath == "" {
			return errors.New("This command takes one argument: <config path>\nFor additional help try 'tomato ui -help'")
		}

		mux := http.NewServeMux()
		if UIInputs.ProxyEnabled {
			mux.Handle("/", uiProxyMux(UIInputs.ProxyURL))
		} else {
			mux.Handle("/", uiFileServerMux(UIInputs.StaticFilePath))
		}
		mux.Handle("/api/config", getConfigMux(configPath))
		mux.Handle("/api/dictionary", getDictionaryMux(UIInputs.DictionaryPath))
		srv := &http.Server{
			Addr:    UIInputs.Addr,
			Handler: mux,
		}
		log.Printf("Server run on: %s", srv.Addr)
		return srv.ListenAndServe()
	},
}
