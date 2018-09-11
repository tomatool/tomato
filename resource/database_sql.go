package resource

import (
	"errors"

	"github.com/alileza/tomato/resource/database/sql/mysql"
	"github.com/alileza/tomato/resource/database/sql/postgres"
)

type DatabaseSQL interface {
	Resource

	Select(tableName string, condition map[string]string) ([]map[string]string, error)
	Insert(tableName string, rows []map[string]string) error
	Delete(tableName string, condition map[string]string) (int, error)
}

func (m *Manager) GetDatabaseSQL(resourceName string) (DatabaseSQL, error) {
	r, ok := m.resources[resourceName]
	if !ok {
		return nil, ErrNotFound
	}

	if r.cache != nil {
		return r.cache.(DatabaseSQL), nil
	}

	if r.config.Type != "database/sql" {
		return nil, errors.New("invalid resource type " + r.config.Type)
	}

	var (
		conn Resource
		err  error
	)
	switch r.config.Params["driver"] {
	case "postgres":
		conn, err = postgres.New(r.config)
	case "mysql":
		conn, err = mysql.New(r.config)
	default:
		err = errors.New("driver not found")
	}
	if err != nil {
		return nil, err
	}

	r.cache = conn
	m.resources[resourceName] = r

	return r.cache.(DatabaseSQL), nil
}
