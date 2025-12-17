package handler

import (
	"context"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// MySQL provides MySQL database testing capabilities (stub)
type MySQL struct {
	name      string
	config    config.Resource
	container *container.Manager
}

func NewMySQL(name string, cfg config.Resource, cm *container.Manager) (*MySQL, error) {
	return &MySQL{name: name, config: cfg, container: cm}, nil
}

func (r *MySQL) Name() string                                         { return r.name }
func (r *MySQL) Init(ctx context.Context) error                       { return nil }
func (r *MySQL) Ready(ctx context.Context) error                      { return nil }
func (r *MySQL) Reset(ctx context.Context) error                      { return nil }
func (r *MySQL) RegisterSteps(ctx *godog.ScenarioContext)             {}
func (r *MySQL) Cleanup(ctx context.Context) error                    { return nil }
func (r *MySQL) ExecSQL(ctx context.Context, q string) (int64, error) { return 0, nil }
func (r *MySQL) ExecSQLFile(ctx context.Context, p string) error      { return nil }

// Wiremock provides HTTP mocking capabilities (stub)
type Wiremock struct {
	name      string
	config    config.Resource
	container *container.Manager
}

func NewWiremock(name string, cfg config.Resource, cm *container.Manager) (*Wiremock, error) {
	return &Wiremock{name: name, config: cfg, container: cm}, nil
}

func (r *Wiremock) Name() string                             { return r.name }
func (r *Wiremock) Init(ctx context.Context) error           { return nil }
func (r *Wiremock) Ready(ctx context.Context) error          { return nil }
func (r *Wiremock) Reset(ctx context.Context) error          { return nil }
func (r *Wiremock) RegisterSteps(ctx *godog.ScenarioContext) {}
func (r *Wiremock) Cleanup(ctx context.Context) error        { return nil }

// Interface implementations
var (
	_ Handler     = (*MySQL)(nil)
	_ Handler     = (*Wiremock)(nil)
	_ SQLExecutor = (*MySQL)(nil)
)
