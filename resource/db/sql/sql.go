package sql

import (
	"fmt"
	"strings"

	"github.com/alileza/tomato/util/sqlutil"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
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
	Count(tableName string, condition map[string]string) (int, error)
}

func Cast(r interface{}) SQL {
	return r.(SQL)
}

func New(params map[string]string) *client {
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

	return &client{db}
}

type client struct {
	db *sqlx.DB
}

func (c *client) Ready() error {
	if _, err := c.db.Exec("SELECT 1"); err != nil {
		return errors.Wrapf(err, "db/sql: driver %s is not ready", c.db.DriverName())
	}
	return nil
}

func (c *client) Close() error {
	return c.db.Close()
}

func (c *client) Clear(tableName string) error {
	tableName = c.t(tableName)
	query := `TRUNCATE TABLE ` + tableName
	switch c.db.DriverName() {
	case DriverMySQL:
		query = `TRUNCATE TABLE ` + tableName
	case DriverPostgres:
		query = `TRUNCATE TABLE ` + tableName + ` RESTART IDENTITY CASCADE`
	}
	_, err := c.db.Exec(query)
	return err
}

func (c *client) Set(tableName string, rows []map[string]string) error {
	tableName = c.t(tableName)

	if err := c.Clear(tableName); err != nil {
		return err
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, row := range rows {
		query := sqlutil.NewQueryBuilder(c.db.DriverName(), "INSERT INTO "+tableName)
		for key, val := range row {
			if val == "" || strings.ToLower(val) == "null" {
				continue
			}
			query.Value(key, val)
		}
		if _, err := tx.Exec(query.Query(), query.Arguments()...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *client) Count(tableName string, conditions map[string]string) (int, error) {
	tableName = c.t(tableName)

	var count int
	query := sqlutil.NewQueryBuilder(c.db.DriverName(), "SELECT COUNT(*) FROM "+tableName)
	for key, val := range conditions {
		query.Where(key, "=", val)
	}
	if err := c.db.Get(
		&count,
		query.Query(),
		query.Arguments()...,
	); err != nil {
		return 0, err
	}
	return count, nil
}

func (c *client) count(tableName string, conditions map[string]string) (int, error) {
	tableName = c.t(tableName)

	var count int
	query := sqlutil.NewQueryBuilder(c.db.DriverName(), "SELECT COUNT(*) FROM "+tableName)
	for key, val := range conditions {
		query.Where(key, "=", val)
	}
	if err := c.db.Get(
		&count,
		query.Query(),
		query.Arguments()...,
	); err != nil {
		return 0, err
	}
	return count, nil
}

func (c *client) t(tableName string) string {
	switch c.db.DriverName() {
	case DriverMySQL:
		if tableName[0] == '`' {
			return tableName
		}
		return fmt.Sprintf("`%s`", tableName)
	case DriverPostgres:
		if tableName[0] == '"' {
			return tableName
		}
		return fmt.Sprintf(`"%s"`, tableName)
	}
	return tableName
}
