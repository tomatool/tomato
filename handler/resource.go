package handler

import (
	"fmt"

	"github.com/DATA-DOG/godog/colors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/handler/database/sql"
	"github.com/tomatool/tomato/handler/http/client"
	"github.com/tomatool/tomato/handler/http/server"
	"github.com/tomatool/tomato/handler/queue"
	"github.com/tomatool/tomato/handler/shell"
	"github.com/tomatool/tomato/resource"
	httpclient_r "github.com/tomatool/tomato/resource/httpclient"
	mysql_r "github.com/tomatool/tomato/resource/mysql"
	nsq_r "github.com/tomatool/tomato/resource/nsq"
	postgres_r "github.com/tomatool/tomato/resource/postgres"
	rabbitmq_r "github.com/tomatool/tomato/resource/rabbitmq"
	shell_r "github.com/tomatool/tomato/resource/shell"
	wiremock_r "github.com/tomatool/tomato/resource/wiremock"
)

var resources = map[string]string{
	"httpclient": "http/client",
	"wiremock":   "http/server",
	"postgres":   "database/sql",
	"mysql":      "database/sql",
	"rabbitmq":   "queue",
	"nsq":        "queue",
	"shell":      "shell",
}

func createResource(cfg *config.Resource) (resource.Resource, error) {
	switch cfg.Type {
	case "mysql":
		return mysql_r.New(cfg)
	case "postgres":
		return postgres_r.New(cfg)
	case "rabbitmq":
		return rabbitmq_r.New(cfg)
	case "nsq":
		return nsq_r.New(cfg)
	case "httpclient":
		return httpclient_r.New(cfg)
	case "wiremock":
		return wiremock_r.New(cfg)
	case "shell":
		return shell_r.New(cfg)
	}
	return nil, fmt.Errorf("resource type `%s` is not defined\nplease refer to %s for list of available resources",
		cfg.Type,
		colors.Bold(colors.White)("https://github.com/tomatool/tomato#resources"))
}

func (h *Handler) Register(name string, cfg *config.Resource) error {
	r, err := createResource(cfg)
	if err != nil {
		return err
	}
	h.resources[cfg.Name] = r

	switch resources[cfg.Type] {
	case "database/sql":
		h.sqlDatabases[cfg.Name] = r.(sql.Resource)
	case "shell":
		h.shells[cfg.Name] = r.(shell.Resource)
	case "http/client":
		h.httpClients[cfg.Name] = r.(client.Resource)
	case "http/server":
		h.httpServers[cfg.Name] = r.(server.Resource)
	case "queue":
		h.queues[cfg.Name] = r.(queue.Resource)
	}
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) reset() {
	for _, r := range h.resources {
		r.Reset()
	}
}

func (h *Handler) Ready(name string) error {
	r, ok := h.resources[name]
	if !ok {
		return fmt.Errorf("resource %s not found", name)
	}
	return r.Ready()
}
