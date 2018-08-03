package resource

import (
	"log"
	"sync"

	"github.com/alileza/tomato/config"
	"github.com/alileza/tomato/resource/db/sql"
	"github.com/alileza/tomato/resource/filestore"
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

type Resource interface {
	// Ready returns nil, if resource is ready to be use.
	Ready() error

	// Close returns nil, if resource successfully terminated.
	Close() error
}

type Manager interface {
	Get(name string) (Resource, error)
	Close()

	// Ready returns nil, if all resources is ready to be use.
	Ready() error
}

type manager struct {
	resources []*config.Resource
	cache     sync.Map
	log       log.Logger
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

			var (
				r   Resource
				err error
			)
			switch resourceCfg.Type {
			case client.Name:
				r, err = client.Open(resourceCfg.Params)
			case sql.Name:
				r, err = sql.Open(resourceCfg.Params)
			case server.Name:
				r, err = server.Open(resourceCfg.Params)
			case queue.Name:
				r, err = queue.Open(resourceCfg.Params)
			case filestore.Name:
				r, err = filestore.Open(resourceCfg.Params)
			default:
				return nil, ErrInvalidType
			}
			if err != nil {
				return nil, err
			}
			mgr.cache.Store(resourceCfg, r)

			return r, nil
		}
	}
	return nil, errors.WithMessage(ErrNotFound, "resource:"+name)
}

func (mgr *manager) Close() {
	mgr.cache.Range(func(resourceCfg interface{}, resource interface{}) bool {
		cfg := resource.(*config.Resource)
		r := resource.(Resource)
		if err := r.Close(); err != nil {
			mgr.log.Printf("[ERR] %s: failed to terminate resource : %v\n", cfg.Key(), err)
			return false
		}
		return true
	})
}

func (mgr *manager) Ready() error {
	for _, cfg := range mgr.resources {
		r, err := mgr.Get(cfg.Name)
		if err != nil {
			return err
		}
		if err := r.Ready(); err != nil {
			return err
		}
	}
	return nil
}
