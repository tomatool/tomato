package handler

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// Registry manages all configured handlers
type Registry struct {
	handlers    map[string]Handler
	resetConfig map[string]*bool // per-handler reset configuration
	container   *container.Manager
	mu          sync.RWMutex
}

// NewRegistry creates a new handler registry
func NewRegistry(configs map[string]config.Resource, cm *container.Manager) (*Registry, error) {
	r := &Registry{
		handlers:    make(map[string]Handler),
		resetConfig: make(map[string]*bool),
		container:   cm,
	}

	for name, cfg := range configs {
		h, err := r.createHandler(name, cfg)
		if err != nil {
			return nil, fmt.Errorf("creating handler %s: %w", name, err)
		}
		r.handlers[name] = h
		r.resetConfig[name] = cfg.Reset
	}

	return r, nil
}

// handlerFactory builds one resource handler.
type handlerFactory func(name string, cfg config.Resource, cm *container.Manager) (Handler, error)

// factory adapts a concrete constructor — they return *Postgres, *GRPC and so
// on — to the common signature, so every resource can live in one table.
func factory[T Handler](f func(string, config.Resource, *container.Manager) (T, error)) handlerFactory {
	return func(name string, cfg config.Resource, cm *container.Manager) (Handler, error) {
		h, err := f(name, cfg, cm)
		if err != nil {
			return nil, err
		}
		return h, nil
	}
}

// handlerFactories is the ONE place a resource type is registered.
//
// It is a map rather than a switch because ValidResourceTypes is derived from
// it. While those were two hand-maintained lists, a type could be added to
// one and not the other — which is exactly what happened when the grpc
// resource landed: `tomato run` constructed it happily while `tomato validate`
// rejected it as unknown, so the failure only appeared in CI, where the
// GitHub Action validates before running.
var handlerFactories = map[string]handlerFactory{
	"postgres":         factory(NewPostgres),
	"postgresql":       factory(NewPostgres),
	"mysql":            factory(NewMySQL),
	"redis":            factory(NewRedis),
	"rabbitmq":         factory(NewRabbitMQ),
	"kafka":            factory(NewKafka),
	"http":             factory(NewHTTPClient),
	"http-client":      factory(NewHTTPClient),
	"http-server":      factory(NewHTTPServer),
	"grpc":             factory(NewGRPC),
	"grpc-client":      factory(NewGRPC),
	"websocket":        factory(NewWebSocketClient),
	"websocket-client": factory(NewWebSocketClient),
	"websocket-server": factory(NewWebSocketServer),
	"wiremock":         factory(NewWiremock),
	"s3":               factory(NewS3),
	"minio":            factory(NewS3),
	"shell":            factory(NewShell),
}

// createHandler instantiates a handler based on its type
func (r *Registry) createHandler(name string, cfg config.Resource) (Handler, error) {
	build, ok := handlerFactories[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unknown handler type: %s", cfg.Type)
	}
	return build(name, cfg, r.container)
}

// Get returns a handler by name
func (r *Registry) Get(name string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("handler not found: %s", name)
	}
	return h, nil
}

// WaitReady waits for all handlers to be ready
func (r *Registry) WaitReady(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, h := range r.handlers {
		log.Debug().Str("handler", name).Msg("initializing handler")
		if err := h.Init(ctx); err != nil {
			return fmt.Errorf("initializing %s: %w", name, err)
		}

		log.Debug().Str("handler", name).Msg("checking handler readiness")
		if err := h.Ready(ctx); err != nil {
			return fmt.Errorf("handler %s not ready: %w", name, err)
		}
	}

	return nil
}

// ResetAll resets all handlers to clean state
func (r *Registry) ResetAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, h := range r.handlers {
		// Check per-handler reset configuration
		if resetCfg := r.resetConfig[name]; resetCfg != nil && !*resetCfg {
			log.Debug().Str("handler", name).Msg("skipping reset (disabled)")
			continue
		}

		log.Debug().Str("handler", name).Msg("resetting handler")
		if err := h.Reset(ctx); err != nil {
			return fmt.Errorf("resetting %s: %w", name, err)
		}
	}

	return nil
}

// RegisterSteps registers step definitions from all handlers
func (r *Registry) RegisterSteps(ctx *godog.ScenarioContext) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, h := range r.handlers {
		h.RegisterSteps(ctx)
	}
}

// Cleanup releases all handlers
func (r *Registry) Cleanup(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, h := range r.handlers {
		if err := h.Cleanup(ctx); err != nil {
			errs = append(errs, fmt.Errorf("cleaning up %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// ValidResourceTypes returns all valid resource type names
func ValidResourceTypes() []string {
	types := make([]string, 0, len(handlerFactories))
	for t := range handlerFactories {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// ContainerBasedTypes returns resource types that typically need a container reference
func ContainerBasedTypes() []string {
	return []string{
		"postgres", "postgresql", "mysql",
		"redis", "rabbitmq", "kafka",
		"wiremock",
		"s3", "minio",
	}
}
