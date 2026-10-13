// Package resource is the contract every tomato resource implements.
//
// A resource is one kind of thing a test can talk to: a database, a queue, an
// HTTP mock. Each lives in its own package under this one and depends on
// nothing but this contract, so adding or changing one resource does not
// reach into any other.
package resource

import (
	"context"

	"github.com/cucumber/godog"
)

// Handler defines the interface that all resource handlers must implement
type Handler interface {
	// Name returns the handler identifier
	Name() string

	// Init initializes the handler connection
	Init(ctx context.Context) error

	// Ready checks if the handler is ready to use
	Ready(ctx context.Context) error

	// Reset clears the handler state
	Reset(ctx context.Context) error

	// RegisterSteps registers Gherkin step definitions
	RegisterSteps(ctx *godog.ScenarioContext)

	// Cleanup releases resources
	Cleanup(ctx context.Context) error
}

// AppEnv is environment a resource provides to the application under test.
type AppEnv struct {
	// Set is added to the app's environment; the app's own env settings win.
	Set map[string]string
	// Unset is removed from the environment the app inherits from tomato.
	Unset []string
}

// AppEnvProvider is implemented by resources the application depends on while
// it starts, like the aws resource it gets credentials from. They are
// initialized before the app starts, and their AppEnv is added to its
// environment. Their Init must be safe to call twice: every resource is
// initialized again after the app is up.
type AppEnvProvider interface {
	AppEnv() AppEnv
}

// ConditionalAppEnvProvider is an AppEnvProvider that provides environment
// only for some configurations, like a kafka resource with a Glue registry.
// When ProvidesAppEnv is false the resource is not initialized before the
// app, and starts with every other resource as usual.
type ConditionalAppEnvProvider interface {
	AppEnvProvider
	ProvidesAppEnv() bool
}

// AppControl is the application under test, for resources that stop and
// start it mid-scenario.
type AppControl interface {
	// StopApp stops the app, as when it crashes.
	StopApp() error
	// StartApp starts it again and waits until it is ready, with overrides
	// on top of app.env (nil for the configured environment).
	StartApp(ctx context.Context, envOverrides map[string]string) error
	// Running reports whether the app is up.
	Running() bool
}

// AppControlled is implemented by resources that act on the app. The runner
// hands them the app once it has started.
type AppControlled interface {
	SetAppControl(AppControl)
}

// ResetsFirst is implemented by resources whose Reset brings back what other
// resources reset: the lifecycle resource restarts the containers a scenario
// stopped, which the postgres resource then truncates. They reset first.
type ResetsFirst interface {
	ResetsFirst() bool
}

// SQLExecutor is implemented by handlers that can execute SQL
type SQLExecutor interface {
	ExecSQL(ctx context.Context, query string) (int64, error)
	ExecSQLFile(ctx context.Context, path string) error
}

// MessagePublisher is implemented by handlers that can publish messages
type MessagePublisher interface {
	Publish(ctx context.Context, target string, payload []byte, headers map[string]string) error
}

// CacheStore is implemented by handlers that provide key-value storage
type CacheStore interface {
	Set(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// WebSocketClientInterface is implemented by handlers that can handle WebSocket connections
type WebSocketClientInterface interface {
	Connect(ctx context.Context, headers map[string]string) error
	Send(ctx context.Context, message []byte) error
	Receive(ctx context.Context, timeout int) ([]byte, error)
	Disconnect(ctx context.Context) error
}
