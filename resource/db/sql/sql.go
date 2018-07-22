package sql

import (
	"errors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	Name = "db/sql"

	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
)

var (
	ErrInvalidDriver = errors.New("invalid driver")
)

type SQL interface {
	Clear(tableName string) error
	Set(tableName string, rows []map[string]string) error
	Compare(tableName string, rows []map[string]string) error
}

func Cast(r interface{}) SQL {
	return r.(SQL)
}

func New(params map[string]string) SQL {
	driver, ok := params["driver"]
	if !ok {
		panic("db/sql: driver is required")
	}
	datasource, ok := params["datasource"]
	if !ok {
		panic("db/sql: datasource is required")
	}

	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		panic("db/sql: " + driver + ":" + datasource + " > " + err.Error())
	}

	switch driver {
	case DriverMySQL:
		return &mysql{db}
	case DriverPostgres:
		return &postgres{db}
	}
	panic("db/sql: invalid driver > " + driver)
}
