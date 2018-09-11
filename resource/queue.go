package resource

import (
	"errors"

	"github.com/alileza/tomato/resource/queue/rabbitmq"
)

type Queue interface {
	Resource

	Listen(target string) error
	Fetch(target string) ([][]byte, error)
	Publish(target string, payload []byte) error
}

func (m *Manager) GetQueue(resourceName string) (Queue, error) {
	r, ok := m.resources[resourceName]
	if !ok {
		return nil, ErrNotFound
	}

	if r.cache != nil {
		return r.cache.(Queue), nil
	}

	if r.config.Type != "queue" {
		return nil, errors.New("invalid resource type " + r.config.Type)
	}

	var (
		conn Resource
		err  error
	)
	switch r.config.Params["driver"] {
	case "rabbitmq":
		conn, err = rabbitmq.New(r.config)
	default:
		err = errors.New("driver not found")
	}
	if err != nil {
		return nil, err
	}

	r.cache = conn
	m.resources[resourceName] = r

	return r.cache.(Queue), nil
}
