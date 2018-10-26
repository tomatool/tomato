package resource

import (
	"time"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
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

func (m *Manager) Ready() error {
	for _, r := range m.resources {
		if !r.config.ReadyCheck {
			continue
		}

		var (
			rr  Resource
			err error
		)

		for i := 0; i < 30; i++ {
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
				// fmt.Printf("%d [%s] not yet ready : %v \n%+v\n", (i + 1), r.config.Name, err, r.config.Params)
				time.Sleep(time.Second * 2)
				continue
			}

			if err = rr.Ready(); err != nil {
				// fmt.Printf("%d [%s] not yet ready : %v \n%+v\n", (i + 1), r.config.Name, err, r.config.Params)
				time.Sleep(time.Second * 2)
				continue
			}
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
