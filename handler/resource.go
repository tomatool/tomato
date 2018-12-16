package handler

import (
	"fmt"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/handler/database/sql"
	"github.com/tomatool/tomato/handler/queue"
	"github.com/tomatool/tomato/resource/database/sql/mysql"
	"github.com/tomatool/tomato/resource/database/sql/postgres"
	"github.com/tomatool/tomato/resource/http/client"
	"github.com/tomatool/tomato/resource/http/server"
	"github.com/tomatool/tomato/resource/queue/nsq"
	"github.com/tomatool/tomato/resource/queue/rabbitmq"
	"github.com/tomatool/tomato/resource/shell"
)

func (h *Handler) Register(name string, cfg *config.Resource) error {
	var (
		err error
	)

	resources := map[string]string{
		"httpclient": "http/client",
		"httpserver": "http/server",
		"postgres":   "database/sql",
		"mysql":      "database/sql",
		"rabbitmq":   "queue",
		"nsq":        "queue",
	}

	switch resources[cfg.Type] {
	case "database/sql":
		h.sqlDatabases[cfg.Name], err = createSQL(cfg)
		h.resources[cfg.Name] = h.sqlDatabases[cfg.Name]
	case "shell":
		h.shells[cfg.Name], err = shell.New(cfg)
		h.resources[cfg.Name] = h.shells[cfg.Name]
	case "http/client":
		h.httpClients[cfg.Name], err = client.New(cfg)
		h.resources[cfg.Name] = h.httpClients[cfg.Name]
	case "http/server":
		h.httpServers[cfg.Name], err = server.New(cfg)
		h.resources[cfg.Name] = h.httpServers[cfg.Name]
	case "queue":
		h.queues[cfg.Name], err = createQueue(cfg)
		h.resources[cfg.Name] = h.queues[cfg.Name]
	}
	if err != nil {
		return err
	}

	return nil
}

func createSQL(cfg *config.Resource) (sql.Resource, error) {
	driver, ok := cfg.Params["driver"]
	if !ok {
		return nil, fmt.Errorf("params driver is required for database/sql resource")
	}

	switch driver {
	case "mysql":
		return mysql.New(cfg)
	case "postgres":
		return postgres.New(cfg)
	}
	return nil, fmt.Errorf("%s is not registered driver for database/sql", driver)
}

func createQueue(cfg *config.Resource) (queue.Resource, error) {
	driver, ok := cfg.Params["driver"]
	if !ok {
		return nil, fmt.Errorf("params driver is required for queue resource")
	}
	switch driver {
	case "rabbitmq":
		return rabbitmq.New(cfg)
	case "nsq":
		return nsq.New(cfg)
	}
	return nil, fmt.Errorf("%s is not registered driver for queue", driver)
}

func (h *Handler) reset() {
	for _, r := range h.resources {
		r.Reset()
	}
}

func (h *Handler) Ready(name string) error {
	return h.resources[name].Ready()
}
