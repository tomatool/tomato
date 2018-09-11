package resource

import (
	"github.com/alileza/tomato/config"
	"github.com/pkg/errors"
)

var (
	ErrInvalidType   = errors.New("invalid resource type")
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidParams = errors.New("invalid resource params")
)

type Resource interface {
	Ready() error
	Reset() error
}

type resource struct {
	config *config.Resource
	cache  Resource
}

type Manager struct {
	resources map[string]*resource
}

func NewManager(cfg []*config.Resource) *Manager {
	m := &Manager{
		resources: make(map[string]*resource),
	}
	for _, c := range cfg {
		m.resources[c.Name] = &resource{c, nil}
	}
	return m
}

func (m *Manager) Reset() {
	for _, r := range m.resources {
		var (
			rr  Resource
			err error
		)

		switch r.config.Type {
		case "database/sql":
			rr, err = m.GetDatabaseSQL(r.config.Name)
		case "queue":
			rr, err = m.GetQueue(r.config.Name)
		case "http/server":
			rr, err = m.GetHTTPServer(r.config.Name)
		case "http/client":
			rr, err = m.GetHTTPClient(r.config.Name)
		default:
			err = errors.New("he")
		}
		if err != nil {
			return
		}
		rr.Reset()
	}
}
