package database

import (
	"errors"
)

type Row map[string]string

type Rows []Row

type Database interface {
	Insert(string, Rows) error
	Delete(string, Rows) (int, error)
	Count(string, Row) (int, error)

	Reset() error
	Close() error
}

func Cast(r interface{}) Database {
	return r.(Database)
}

func Open(params map[string]string) (Database, error) {
	driver, ok := params["driver"]
	if !ok {
		return nil, errors.New("database: driver is required")
	}
	datasource, ok := params["datasource"]
	if !ok {
		return nil, errors.New("database: parameter datasource is required")
	}

	switch driver {
	case "postgres":
		return newPostgres(driver, datasource)
	case "mysql":
		return newMySQL(driver, datasource)
	}

	return nil, errors.New("database: invalid driver => " + driver)
}
