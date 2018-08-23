package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/alileza/tomato/resource/db/sql/sqlutil"
	"github.com/go-sql-driver/mysql"
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
	TruncateAll() error
	Truncate(tableName string) error
	Set(tableName string, tabl *gherkin.DataTable) error
	Count(tableName string, condition map[string]string) (int, error)
	Contains(tableName string, row *gherkin.TableRow)
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
		if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
			return err
		}
		if _, err := tx.Exec("TRUNCATE TABLE " + tableName); err != nil {
			e, ok := err.(*mysql.MySQLError)
			if ok && e.Number == 1146 {
				return nil
			}
			return err
		}
		if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
			return err
		}
	case DriverPostgres:
		if _, err := tx.Exec(`TRUNCATE TABLE ` + tableName + ` RESTART IDENTITY CASCADE`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (c *client) Set(tableName string, tbl *gherkin.DataTable) error {
	tableName = c.t(tableName)

	if err := c.Truncate(tableName); err != nil {
		return err
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := gherkinTableToRows(tbl)
	if err != nil {
		return err
	}

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
		if val == "*" {
			continue
		}
		if strings.ToLower(val) == "null" {
			query.Where(key, "IS", nil)
			continue
		}
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

// TODO: Add Contains(tbl *gherkin.DataTable, *gherkin.TableRow )

func gherkinTableToRows(tbl *gherkin.DataTable) ([]map[string]string, error) {
	hash := make([]map[string]string, len(tbl.Rows)-1)

	columns := tbl.Rows[0].Cells
	columnCount := len(columns)
	for i := 1; i < len(tbl.Rows); i++ {
		row := tbl.Rows[i]
		if len(row.Cells) != columnCount {
			return nil, fmt.Errorf("Invalid cells in row %v", i)
		}

		rowHash := make(map[string]string, columnCount)
		for i, cell := range row.Cells {
			rowHash[columns[i].Value] = cell.Value
		}

		hash[i-1] = rowHash
	}

	return hash, nil
}
