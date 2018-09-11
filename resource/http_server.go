package resource

import (
	httpserver "github.com/alileza/tomato/resource/http/server"
)

type HTTPServer interface {
	Resource

	SetResponse(requestPath string, responseCode int, responseBody []byte) error
}

func (m *Manager) GetHTTPServer(resourceName string) (HTTPServer, error) {
	r, ok := m.resources[resourceName]
	if !ok {
		return nil, ErrNotFound
	}

	if r.cache != nil {
		return r.cache.(HTTPServer), nil
	}

	var err error
	r.cache, err = httpserver.New(r.config)
	if err != nil {
		return nil, err
	}

	m.resources[resourceName] = r

	return r.cache.(HTTPServer), nil
}
