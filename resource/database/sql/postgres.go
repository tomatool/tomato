package database

import (
	"strings"

	"github.com/alileza/tomato/util/sqlutil"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

type postgres struct {
	db *sqlx.DB
}

func newPostgres(driver, datasource string) (*postgres, error) {
	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, errors.New("db/sql: " + driver + ":" + datasource + " > " + err.Error())
	}

	return &postgres{db}, nil
}

func (c *postgres) Insert(tableName string, rows Rows) error {
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
func (c *postgres) Delete(string, Rows) (int, error) {
	return 0, nil
}
func (c *postgres) Count(tableName string, conditions Row) (int, error) {
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

func (c *postgres) Reset() error {
	var tables []string

	if err := c.db.Select(&tables, `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'`); err != nil {
		return err
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if _, err := tx.Exec(`TRUNCATE TABLE ` + table + ` RESTART IDENTITY CASCADE`); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (c *postgres) Close() error {
	return c.db.Close()
}
