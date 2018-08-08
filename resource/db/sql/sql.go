package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alileza/tomato/util/sqlutil"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	TruncateAll() error
	Truncate(tableName string) error
	Set(tableName string, rows []map[string]string) error
	Count(tableName string, condition map[string]string) (int, error)
}

func Cast(r interface{}) SQL {
	return r.(SQL)
}

func Open(params map[string]string) (*client, error) {
	driver, ok := params["driver"]
	if !ok {
		return nil, errors.New("db/sql: parameter driver is required")
	}
	datasource, ok := params["datasource"]
	if !ok {
		return nil, errors.New("db/sql: parameter datasource is required")
	}

	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, errors.New("db/sql: " + driver + ":" + datasource + " > " + err.Error())
	}

	return &client{db}, nil
}

type client struct {
	db *sqlx.DB
}

func (c *client) Ready() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		return errors.Wrapf(err, "db/sql: driver %s is not ready", c.db.DriverName())
	}
	return nil
}

func (c *client) Close() error {
	return c.db.Close()
}

func (c *client) TruncateAll() error {
	var (
		tables []string
		query  string
	)

	switch c.db.DriverName() {
	case DriverMySQL:
		query = `SELECT table_name FROM information_schema.tables WHERE table_type = 'base table'`
	case DriverPostgres:
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'`
	}
	logrus.WithFields(logrus.Fields{"query": query}).Debug("Getting list of tables to truncate from information schema.")
	if err := c.db.Select(&tables, query); err != nil {
		return err
	}

	for _, table := range tables {
		if err := c.Truncate(table); err != nil {
			return err
		}
	}

	return nil
}

func (c *client) Truncate(tableName string) error {
	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tableName = c.t(tableName)

	switch c.db.DriverName() {
	case DriverMySQL:

		logrus.WithFields(logrus.Fields{"query": "SET FOREIGN_KEY_CHECKS=0"}).Debug("Ignoring foreign key checks.")
		if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
			return err
		}

		logrus.WithFields(logrus.Fields{"query": "TRUNCATE TABLE " + tableName}).Debug("Truncating table.")
		if _, err := tx.Exec("TRUNCATE TABLE " + tableName); err != nil {
			e, ok := err.(*mysql.MySQLError)
			if ok && e.Number == 1146 {
				return nil
			}
			return err
		}

		logrus.WithFields(logrus.Fields{"query": "SET FOREIGN_KEY_CHECKS=1"}).Debug("Enabling foreign key checks.")
		if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
			return err
		}
	case DriverPostgres:
		logrus.WithFields(logrus.Fields{"query": `TRUNCATE TABLE ` + tableName + ` RESTART IDENTITY CASCADE`}).Debug("Truncating table.")
		if _, err := tx.Exec(`TRUNCATE TABLE ` + tableName + ` RESTART IDENTITY CASCADE`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (c *client) Set(tableName string, rows []map[string]string) error {
	tableName = c.t(tableName)

	if err := c.Truncate(tableName); err != nil {
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
		logrus.WithFields(logrus.Fields{"query": query.Query(), "arguments": query.Arguments()}).Debug("Inserting row.")
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
		if val == "*" {
			continue
		}
		if strings.ToLower(val) == "null" {
			query.Where(key, "IS", nil)
			continue
		}
		query.Where(key, "=", val)
	}
	logrus.WithFields(logrus.Fields{"query": query.Query(), "arguments": query.Arguments()}).Debug("Retrieving row count.")
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
