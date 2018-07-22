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

type client struct {
	options map[string]string
	conn    SQL
}

func T(i interface{}) *client {
	return i.(*client)
}

func New(cfg map[string]string) *client {
	return &client{cfg, nil}
}

func (c *client) db() (SQL, error) {
	if c.conn != nil {
		return c.conn, nil
	}

	driver, ok := c.options["driver"]
	if !ok {
		return nil, errors.New(driver + ": invalid driver")
	}
	datasource, ok := c.options["datasource"]
	if !ok {
		return nil, errors.New("datasource is required")
	}

	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, err
	}

	switch driver {
	case DriverMySQL:
		c.conn = &mysql{db}
	}

	return c.conn, err
}

func (c *client) Close() {}

func (c *client) Clear(tableName string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	return conn.Clear(tableName)
}

func (c *client) Set(tableName string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	if err := conn.Clear(tableName); err != nil {
		return err
	}

	return conn.Set(tableName, rows)
}

func (c *client) Compare(tableName string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	return conn.Compare(tableName, rows)
}
