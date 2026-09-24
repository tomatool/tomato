package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tomatool/tomato/internal/apprunner"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/handler"
	"github.com/tomatool/tomato/internal/runlog"
	"github.com/urfave/cli/v2"
)

// EXPERIMENTAL. `tomato serve` starts everything `tomato run` starts
// (containers, the app, resources) and then, instead of running feature
// files, serves tomato's steps over a local HTTP API. Tests written in
// another language (see sdk/kotlin) drive tomato through it.
var serveCommand = &cli.Command{
	Name:  "serve",
	Usage: "[experimental] start containers, app and resources, and serve steps over HTTP",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "tomato.yml", Usage: "path to config file"},
		&cli.IntFlag{Name: "port", Value: 0, Usage: "port to listen on (0 picks a free one)"},
		&cli.StringFlag{Name: "ready-file", Usage: "write the API base URL to this file once everything is ready"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "hide application logs"},
	},
	Action: runServe,
}

func runServe(c *cli.Context) error {
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return err
	}
	runCtx, err := runlog.New()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(c.Context, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(cfg.Containers) > 0 || cfg.App.UseContainer() {
		if err := container.CheckDockerAvailable(); err != nil {
			return err
		}
	}
	cm, err := container.NewManager(cfg.Containers)
	if err != nil {
		return err
	}
	cm.SetRunContext(runCtx)
	defer cm.Cleanup()
	fmt.Fprintln(os.Stderr, "tomato serve: starting dependencies...")
	if err := cm.StartAll(ctx); err != nil {
		return fmt.Errorf("failed to start containers: %w", err)
	}

	if cfg.App.IsConfigured() {
		app := apprunner.NewRunner(cfg.App, cm)
		app.SetRunContext(runCtx)
		app.SetShowLogs(!c.Bool("quiet"))
		app.SetResources(cfg.Resources)
		fmt.Fprintln(os.Stderr, "tomato serve: starting application...")
		if err := app.Start(ctx); err != nil {
			return fmt.Errorf("failed to start application: %w (logs: %s)", err, runCtx.LogPath("app"))
		}
		defer app.Stop()
		if cfg.App.Wait > 0 {
			time.Sleep(cfg.App.Wait)
		}
		if appContainer := app.GetContainer(); appContainer != nil {
			cm.RegisterContainer(cfg.App.GetName(), appContainer)
		}
	}

	registry, err := handler.NewRegistry(cfg.Resources, cm)
	if err != nil {
		return err
	}
	defer registry.Cleanup(context.Background())
	if err := registry.WaitReady(ctx); err != nil {
		return fmt.Errorf("resources not ready: %w", err)
	}
	executor, err := handler.NewExecutor(registry)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.Int("port")))
	if err != nil {
		return err
	}
	baseURL := "http://" + listener.Addr().String()
	api := &serveAPI{registry: registry, executor: executor, shutdown: stop}
	server := &http.Server{Handler: api.routes()}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "tomato serve:", err)
			stop()
		}
	}()

	if path := c.String("ready-file"); path != "" {
		if err := os.WriteFile(path, []byte(baseURL), 0o644); err != nil {
			return err
		}
		defer os.Remove(path)
	}
	fmt.Fprintf(os.Stderr, "tomato serve: ready at %s\n", baseURL)

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "tomato serve: shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

// serveAPI is the HTTP surface of `tomato serve`:
//
//	GET  /v1/health           200 once everything is up
//	GET  /v1/steps            every step, bound to its resource name
//	POST /v1/reset            what runs before every scenario: reset all
//	                          resources and clear variables
//	POST /v1/steps/run        {"step": "...", "docString": "...", "table": [[...]]}
//	                          200 {"ok": true} or 422 {"ok": false, "error": "..."}
//	GET  /v1/variables/{name} a variable a step stored ("... saved as {{x}}")
//	POST /v1/shutdown         stop containers and the app, and exit
type serveAPI struct {
	registry *handler.Registry
	executor *handler.Executor
	shutdown func()
	mu       sync.Mutex // one step at a time, like a scenario
}

func (a *serveAPI) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /v1/steps", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, a.executor.Steps())
	})
	mux.HandleFunc("POST /v1/reset", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if err := a.registry.ResetAll(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		handler.ResetGlobalVariables()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /v1/steps/run", func(w http.ResponseWriter, r *http.Request) {
		var call handler.StepCall
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request: " + err.Error()})
			return
		}
		a.mu.Lock()
		err := a.executor.Run(call)
		a.mu.Unlock()
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /v1/variables/{name}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := handler.GetVariable(r.PathValue("name"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no variable " + r.PathValue("name")})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "value": v})
	})
	mux.HandleFunc("POST /v1/shutdown", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		go a.shutdown()
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
