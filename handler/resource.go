package handler

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/DATA-DOG/godog/colors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/handler/cache"
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
	redis_r "github.com/tomatool/tomato/resource/redis"
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
	"redis":      "cache",
}

var defaultImages = map[string]string{
	"httpclient": "pstauffer/curl",
	"wiremock":   "rodolpheche/wiremock",
	"postgres":   "postgres:9.5",
	"mysql":      "mysql:5.6.34",
	"rabbitmq":   "rabbitmq:3.6.1-management",
	"nsq":        "queue",
	"shell":      "alpine",
	"redis":      "redis:5.0.6-alpine",
}

func DeleteResource(ctx context.Context, c *dockerclient.Client, cfg *config.Resource) error {
	return nil
}

func CreateResource(ctx context.Context, c *dockerclient.Client, cfg *config.Resource) (resource.Resource, error) {
	imageName := defaultImages[cfg.Type]
	reader, err := c.ImagePull(ctx, "docker.io/library/"+imageName, types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	io.Copy(os.Stdout, reader)

	resp, err := c.ContainerCreate(ctx, &container.Config{
		Image: imageName,
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := c.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

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
	case "redis":
		return redis_r.New(cfg)
	}
	return nil, fmt.Errorf("resource type `%s` is not defined\nplease refer to %s for list of available resources",
		cfg.Type,
		colors.Bold(colors.White)("https://github.com/tomatool/tomato#resources"))
}

func (h *Handler) Register(cfg *config.Resource, r resource.Resource) {
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
	case "cache":
		h.caches[cfg.Name] = r.(cache.Resource)
	}

}
func (h *Handler) Resources() map[string]resource.Resource {
	return h.resources
}

func (h *Handler) reset() {
	for _, r := range h.resources {
		r.Reset()
	}
}

func AttachNetwork(ctx context.Context, c *dockerclient.Client, containerA, containerB resource.Resource) error {
	return nil
}
