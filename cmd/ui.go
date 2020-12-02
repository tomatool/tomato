package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
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

func getConfigMux(config *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

var UIInputs struct {
	Addr           string
	ProxyEnabled   bool
	ProxyURL       string
	StaticFilePath string
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
	},
	Action: func(ctx *cli.Context) error {
		var configPath string
		if ctx.Args().Len() == 1 {
			configPath = ctx.Args().First()
		}

		if configPath == "" {
			return errors.New("This command takes one argument: <config path>\nFor additional help try 'tomato ui -help'")
		}

		conf, err := config.Retrieve(configPath)
		if err != nil {
			return errors.Wrap(err, "Failed to retrieve config")
		}

		mux := http.NewServeMux()
		if UIInputs.ProxyEnabled {
			mux.Handle("/", uiProxyMux(UIInputs.ProxyURL))
		} else {
			mux.Handle("/", uiFileServerMux(UIInputs.StaticFilePath))
		}
		mux.Handle("/api/config", getConfigMux(conf))
		srv := &http.Server{
			Addr:    UIInputs.Addr,
			Handler: mux,
		}
		return srv.ListenAndServe()
	},
}
