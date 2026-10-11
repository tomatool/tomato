// Package registry builds the resources a tomato.yml asks for, and is the
// only package that knows the full set of them.
//
// Every resource type appears here exactly once, in handlerFactories, and
// once in canonicalTypes for the order docs and `tomato steps` present them
// in. Commands ask this package instead of naming resources themselves, so
// adding a resource means adding a package under internal/resource and two
// lines here.
package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
	"github.com/tomatool/tomato/internal/resource/aws"
	"github.com/tomatool/tomato/internal/resource/cassandra"
	"github.com/tomatool/tomato/internal/resource/grpcclient"
	"github.com/tomatool/tomato/internal/resource/httpclient"
	"github.com/tomatool/tomato/internal/resource/httpserver"
	"github.com/tomatool/tomato/internal/resource/kafka"
	"github.com/tomatool/tomato/internal/resource/postgres"
	"github.com/tomatool/tomato/internal/resource/rabbitmq"
	"github.com/tomatool/tomato/internal/resource/redis"
	"github.com/tomatool/tomato/internal/resource/s3"
	"github.com/tomatool/tomato/internal/resource/shell"
	"github.com/tomatool/tomato/internal/resource/websocket"
)

// Registry manages all configured handlers
type Registry struct {
	handlers  map[string]resource.Handler
	container *container.Manager
	mu        sync.RWMutex
	cleanedUp bool
}

// NewRegistry creates a new handler registry
func New(configs map[string]config.Resource, cm *container.Manager) (*Registry, error) {
	r := &Registry{
		handlers:  make(map[string]resource.Handler),
		container: cm,
	}

	for name, cfg := range configs {
		h, err := r.createHandler(name, cfg)
		if err != nil {
			return nil, fmt.Errorf("creating handler %s: %w", name, err)
		}
		r.handlers[name] = h
	}

	return r, nil
}

// handlerFactory builds one resource handler.
type handlerFactory func(name string, cfg config.Resource, cm *container.Manager) (resource.Handler, error)

// factory adapts a concrete constructor — they return *Postgres, *GRPC and so
// on — to the common signature, so every resource can live in one table.
func factory[T resource.Handler](f func(string, config.Resource, *container.Manager) (T, error)) handlerFactory {
	return func(name string, cfg config.Resource, cm *container.Manager) (resource.Handler, error) {
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
	"postgres":         factory(postgres.New),
	"postgresql":       factory(postgres.New),
	"scylladb":         factory(cassandra.New),
	"cassandra":        factory(cassandra.New),
	"redis":            factory(redis.New),
	"rabbitmq":         factory(rabbitmq.New),
	"kafka":            factory(kafka.New),
	"http":             factory(httpclient.New),
	"http-client":      factory(httpclient.New),
	"http-server":      factory(httpserver.New),
	"grpc":             factory(grpcclient.New),
	"grpc-client":      factory(grpcclient.New),
	"websocket":        factory(websocket.NewClient),
	"websocket-client": factory(websocket.NewClient),
	"websocket-server": factory(websocket.NewServer),
	"s3":               factory(s3.New),
	"minio":            factory(s3.New),
	"shell":            factory(shell.New),
	"aws":              factory(aws.New),
}

// unimplementedTypes are resource types people reach for that tomato does not
// support yet, each with what to use instead. They used to be registered as
// no-op stubs, so a config naming them validated, ran, and silently tested
// nothing; now they fail up front with a pointer.
var unimplementedTypes = map[string]string{
	"mysql":    "MySQL is not supported yet; use postgres, or drive MySQL through a shell resource",
	"wiremock": "use type: http-server to mock HTTP dependencies",
}

// UnimplementedTypeHint reports whether typ is a known but unsupported resource
// type, and what to use instead.
func UnimplementedTypeHint(typ string) (string, bool) {
	hint, ok := unimplementedTypes[typ]
	return hint, ok
}

// createHandler instantiates a handler based on its type
func (r *Registry) createHandler(name string, cfg config.Resource) (resource.Handler, error) {
	build, ok := handlerFactories[cfg.Type]
	if !ok {
		if hint, planned := unimplementedTypes[cfg.Type]; planned {
			return nil, fmt.Errorf("resource type %q is not implemented: %s", cfg.Type, hint)
		}
		return nil, fmt.Errorf("unknown handler type: %s", cfg.Type)
	}
	return build(name, cfg, r.container)
}

// Get returns a handler by name
func (r *Registry) Get(name string) (resource.Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("handler not found: %s", name)
	}
	return h, nil
}

// InitAppEnvProviders initializes the resources the application depends on
// while it starts (see resource.AppEnvProvider) and returns the environment they
// provide, merged in resource-name order.
func (r *Registry) InitAppEnvProviders(ctx context.Context) (resource.AppEnv, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)

	env := resource.AppEnv{Set: make(map[string]string)}
	for _, name := range names {
		h := r.handlers[name]
		provider, ok := h.(resource.AppEnvProvider)
		if !ok {
			continue
		}
		log.Debug().Str("handler", name).Msg("initializing handler before the app")
		if err := h.Init(ctx); err != nil {
			return resource.AppEnv{}, fmt.Errorf("initializing %s: %w", name, err)
		}
		provided := provider.AppEnv()
		for k, v := range provided.Set {
			env.Set[k] = v
		}
		env.Unset = append(env.Unset, provided.Unset...)
	}
	return env, nil
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

	// Every resource is reset before every scenario. A suite that wants a
	// starting state builds it in a Background, which runs after this.
	for name, h := range r.handlers {
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

// Cleanup releases all handlers: it closes their connections and servers and
// removes their files. It runs once; later calls do nothing, since some
// clients (sarama's producer) panic when they are closed twice.
func (r *Registry) Cleanup(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cleanedUp {
		return nil
	}
	r.cleanedUp = true

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

// canonicalTypes names each resource exactly once, in the order docs and
// `tomato steps` present them. handlerFactories also holds aliases
// ("postgresql" for postgres, "minio" for s3), so iterating that would list
// some resources twice; this is the deduplicated view of it.
//
// A resource missing from here has no documentation page and is invisible to
// `tomato steps`, so TestCanonicalTypesCoverEveryResource fails if the two
// lists drift apart.
var canonicalTypes = []string{
	"http-client",
	"http-server",
	"postgres",
	"cassandra",
	"redis",
	"kafka",
	"rabbitmq",
	"shell",
	"websocket-client",
	"websocket-server",
	"s3",
	"grpc",
	"aws",
}

// CanonicalTypes returns one type name per resource, in presentation order.
func CanonicalTypes() []string {
	return append([]string(nil), canonicalTypes...)
}

// AllStepCategories returns every resource's step definitions, in a stable
// order. It is what `tomato steps`, `tomato docs` and `tomato validate` read;
// no connection is made and no container is started.
//
// Commands call this rather than building each handler themselves: that way
// adding a resource does not mean editing three commands that each knew the
// full list.
func AllStepCategories() []resource.StepCategory {
	categories := make([]resource.StepCategory, 0, len(canonicalTypes))
	for _, typ := range canonicalTypes {
		if cat, ok := StepCategoryForType(typ); ok {
			categories = append(categories, cat)
		}
	}
	return categories
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
		"postgres", "postgresql",
		"scylladb", "cassandra",
		"redis", "rabbitmq", "kafka",
		"s3", "minio",
	}
}

// StepCategoryForType returns the step definitions of a resource type, built
// from a throwaway handler. It is what `tomato steps`, `tomato docs` and
// `tomato coverage` read; no connection is made.
func StepCategoryForType(typ string) (resource.StepCategory, bool) {
	build, ok := handlerFactories[typ]
	if !ok {
		return resource.StepCategory{}, false
	}
	h, err := build("resource", resource.DummyConfig(), nil)
	if err != nil {
		return resource.StepCategory{}, false
	}
	provider, ok := h.(resource.StepProvider)
	if !ok {
		return resource.StepCategory{}, false
	}
	return provider.Steps(), true
}
