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

// SQLExecutor is implemented by handlers that can execute SQL
type SQLExecutor interface {
	ExecSQL(ctx context.Context, query string) (int64, error)
	ExecSQLFile(ctx context.Context, path string) error
}

// MessagePublisher is implemented by handlers that can publish messages
type MessagePublisher interface {
	Publish(ctx context.Context, target string, payload []byte, headers map[string]string) error
}

// MessageConsumer is implemented by handlers that can consume messages
type MessageConsumer interface {
	Consume(ctx context.Context, target string, timeout int) ([]byte, error)
	Count(ctx context.Context, target string) (int, error)
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
