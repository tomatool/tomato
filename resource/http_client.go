package resource

import (
	"errors"

	httpclient "github.com/alileza/tomato/resource/http/client"
)

type HTTPClient interface {
	Resource

	Request(method, path string, body []byte) error
	Response() (int, []byte, error)
}

func (m *Manager) GetHTTPClient(resourceName string) (HTTPClient, error) {
	r, ok := m.resources[resourceName]
	if !ok {
		return nil, ErrNotFound
	}

	if r.cache != nil {
		return r.cache.(HTTPClient), nil
	}

	if r.config.Type != "http/client" {
		return nil, errors.New("invalid resource type " + r.config.Type)
	}

	var err error
	r.cache, err = httpclient.New(r.config)
	if err != nil {
		return nil, err
	}

	m.resources[resourceName] = r

	return r.cache.(HTTPClient), nil
}
