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
		if r.cache == nil {
			continue
		}
		r.cache.Reset()
	}
}
