/* GENERATED FILE - DO NOT EDIT */
/* Rebuild from the tomatool generate handler tool */
package handler

import (
	"github.com/DATA-DOG/godog"
	"github.com/DATA-DOG/godog/gherkin"
	"github.com/tomatool/tomato/handler/cache"
	"github.com/tomatool/tomato/handler/database/sql"
	"github.com/tomatool/tomato/handler/http/client"
	"github.com/tomatool/tomato/handler/http/server"
	"github.com/tomatool/tomato/handler/queue"
	"github.com/tomatool/tomato/handler/shell"
	"github.com/tomatool/tomato/resource"
)

type Handler struct {
	resources    map[string]resource.Resource
	httpClients  map[string]client.Resource
	sqlDatabases map[string]sql.Resource
	queues       map[string]queue.Resource
	shells       map[string]shell.Resource
	httpServers  map[string]server.Resource
	caches       map[string]cache.Resource
}

func New() *Handler {
	h := &Handler{
		resources:    make(map[string]resource.Resource),
		httpClients:  make(map[string]client.Resource),
		sqlDatabases: make(map[string]sql.Resource),
		queues:       make(map[string]queue.Resource),
		shells:       make(map[string]shell.Resource),
		httpServers:  make(map[string]server.Resource),
		caches:       make(map[string]cache.Resource),
	}
	return h
}

func (h *Handler) Handler() func(s *godog.Suite) {
	return func(s *godog.Suite) {
		s.BeforeFeature(func(_ *gherkin.Feature) {
			h.reset()
		})
		s.AfterScenario(func(_ interface{}, _ error) {
			h.reset()
		})
		server.New(h.httpServers).Register(s)
		client.New(h.httpClients).Register(s)
		sql.New(h.sqlDatabases).Register(s)
		queue.New(h.queues).Register(s)
		shell.New(h.shells).Register(s)
		cache.New(h.caches).Register(s)
	}
}
