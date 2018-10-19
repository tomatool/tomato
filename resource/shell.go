package resource

import (
	"errors"

	"github.com/tomatool/tomato/resource/shell"
)

type Shell interface {
	Resource

	Exec(command string, arguments ...string) error
	Stdout() (string, error)
	Stderr() (string, error)
}

func (m *Manager) GetShell(resourceName string) (Shell, error) {
	r, ok := m.resources[resourceName]
	if !ok {
		return nil, ErrNotFound
	}

	if r.cache != nil {
		return r.cache.(Shell), nil
	}

	if r.config.Type != "shell" {
		return nil, errors.New("invalid resource type " + r.config.Type)
	}

	sh, err := shell.New(r.config)
	if err != nil {
		return nil, err
	}

	r.cache = sh
	m.resources[resourceName] = r

	return r.cache.(Shell), nil
}
