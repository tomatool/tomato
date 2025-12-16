package handler

import (
	"context"
	"fmt"
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

// createHandler instantiates a handler based on its type
func (r *Registry) createHandler(name string, cfg config.Resource) (Handler, error) {
	switch cfg.Type {
	case "postgres", "postgresql":
		return NewPostgres(name, cfg, r.container)
	case "mysql":
		return NewMySQL(name, cfg, r.container)
	case "redis":
		return NewRedis(name, cfg, r.container)
	case "rabbitmq":
		return NewRabbitMQ(name, cfg, r.container)
	case "kafka":
		return NewKafka(name, cfg, r.container)
	case "http-client", "http":
		return NewHTTPClient(name, cfg, r.container)
	case "http-server":
		return NewHTTPServer(name, cfg, r.container)
	case "websocket-client", "websocket":
		return NewWebSocketClient(name, cfg, r.container)
	case "websocket-server":
		return NewWebSocketServer(name, cfg, r.container)
	case "wiremock":
		return NewWiremock(name, cfg, r.container)
	case "shell":
		return NewShell(name, cfg, r.container)
	default:
		return nil, fmt.Errorf("unknown handler type: %s", cfg.Type)
	}
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
	return []string{
		"http", "http-client", "http-server",
		"postgres", "postgresql", "mysql",
		"redis", "rabbitmq", "kafka",
		"shell",
		"websocket", "websocket-client", "websocket-server",
		"wiremock",
	}
}

// ContainerBasedTypes returns resource types that typically need a container reference
func ContainerBasedTypes() []string {
	return []string{
		"postgres", "postgresql", "mysql",
		"redis", "rabbitmq", "kafka",
		"wiremock",
	}
}
