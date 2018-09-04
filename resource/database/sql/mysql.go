package database

import (
	"strings"

	"github.com/alileza/tomato/util/sqlutil"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type mysql struct {
	db *sqlx.DB
}

func newMySQL(driver, datasource string) (*mysql, error) {
	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, errors.New("db/sql: " + driver + ":" + datasource + " > " + err.Error())
	}

	return &mysql{db}, nil
}

func (c *mysql) Insert(tableName string, rows Rows) error {
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

func (c *mysql) Delete(string, Rows) (int, error) {
	return 0, errors.New("not implemented")
}

func (c *mysql) Count(tableName string, conditions Row) (int, error) {
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

func (c *mysql) Reset() error {
	var tables []string

	if err := c.db.Select(&tables, `SELECT table_name FROM information_schema.tables WHERE table_type = 'base table'`); err != nil {
		return err
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if err := c.truncate(tx, table); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (c *mysql) truncate(tx *sqlx.Tx, tableName string) error {
	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}

	if _, err := tx.Exec("TRUNCATE TABLE " + tableName); err != nil {
		e, ok := err.(*mysqldriver.MySQLError)
		if ok && e.Number == 1146 {
			return nil
		}
		return err
	}

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
		return err
	}

	return nil
}

func (c *mysql) Close() error {
	return c.db.Close()
}
