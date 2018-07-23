package resource

import (
	"sync"

	"github.com/alileza/tomato/config"
	"github.com/alileza/tomato/resource/db/sql"
	"github.com/alileza/tomato/resource/http/client"
	"github.com/alileza/tomato/resource/http/server"
	"github.com/alileza/tomato/resource/queue"

	"github.com/pkg/errors"
)

var (
	ErrInvalidType   = errors.New("invalid resource type")
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidParams = errors.New("invalid resource params")
)

type Resource interface{}

type Manager interface {
	Get(name string) (Resource, error)
}

type manager struct {
	resources []*config.Resource
	cache     sync.Map
}

func NewManager(cfgs []*config.Resource) *manager {
	return &manager{resources: cfgs}
}

func (mgr *manager) Get(name string) (Resource, error) {
	for _, resourceCfg := range mgr.resources {
		if resourceCfg.Name == name {
			cache, ok := mgr.cache.Load(resourceCfg)
			if ok {
				return cache.(Resource), nil
			}

			var r Resource
			switch resourceCfg.Type {
			case client.Name:
				r = client.New(resourceCfg.Params)
			case sql.Name:
				r = sql.New(resourceCfg.Params)
			case server.Name:
				r = server.New(resourceCfg.Params)
			case queue.Name:
				r = queue.New(resourceCfg.Params)
			default:
				return nil, ErrInvalidType
			}
			mgr.cache.Store(resourceCfg, r)

			return r, nil
		}
	}
	return nil, errors.WithMessage(ErrNotFound, "resource:"+name)
}
