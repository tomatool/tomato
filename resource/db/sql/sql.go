package sql

import (
	"errors"
	"fmt"
	"strings"

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

type Client struct {
	options map[string]string
	conn    *sqlx.DB
}

func T(i interface{}) *Client {
	return i.(*Client)
}

func New(cfg map[string]string) *Client {
	return &Client{cfg, nil}
}

func (c *Client) db() (*sqlx.DB, error) {
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

	var err error
	c.conn, err = sqlx.Open(driver, datasource)
	return c.conn, err
}

func (c *Client) Close() {
	if c.conn == nil {
		return
	}
	c.conn.Close()
}

func (c *Client) Clear(table string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	switch conn.DriverName() {
	case DriverMySQL:
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		_, err = tx.Exec(`TRUNCATE TABLE ` + table)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`ALTER TABLE ` + table + ` AUTO_INCREMENT = 1`)
		if err != nil {
			return err
		}

		return tx.Commit()
	case DriverPostgres:
		_, err := conn.Exec(`TRUNCATE TABLE "` + table + `" RESTART IDENTITY CASCADE`)
		return err
	}

	return ErrInvalidDriver
}

func (c *Client) Set(table string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}
	driverName := conn.DriverName()

	if err := c.Clear(table); err != nil {
		return err
	}

	tx, err := conn.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, row := range rows {
		var (
			keys   []string
			valctr []string
			vals   []interface{}
		)
		counter := 1
		for key, val := range row {
			if val == "" || strings.ToLower(val) == "null" {
				continue
			}
			keys = append(keys, key)

			valctr = append(valctr, fmt.Sprintf("$%d", counter))
			vals = append(vals, val)
			counter++
		}

		var query string
		switch driverName {
		case DriverPostgres:
			query = fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, table, strings.Join(keys, ","), strings.Join(valctr, ","))
		case DriverMySQL:
			for i := range valctr {
				valctr[i] = "?"
			}
			query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(keys, ","), strings.Join(valctr, ","))
		default:
			return ErrInvalidDriver
		}

		if _, err := tx.Exec(query, vals...); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (c *Client) Cmp(table string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	var rowCount int
	if err := conn.Get(&rowCount, countQuery(conn.DriverName(), table, nil)); err != nil {
		return err
	}

	if rowCount != len(rows) {
		return fmt.Errorf("expecting row count to be %d, got %d", len(rows), rowCount)
	}

	for _, row := range rows {
		var (
			keys []string
			vals []interface{}
		)
		counter := 1
		for key, val := range row {
			if val == "NULL" {
				keys = append(keys, fmt.Sprintf("%s is NULL", key))
				continue
			}

			vals = append(vals, val)
			if key == "metadata" {
				keys = append(keys, fmt.Sprintf("%s :: text = $%d", key, counter))
				counter++
				continue
			}
			if key == "message" {
				keys = append(keys, fmt.Sprintf("%s :: text = $%d", key, counter))
				counter++
				continue
			}

			keys = append(keys, fmt.Sprintf("%s=$%d", key, counter))
			counter++
		}

		query := countQuery(conn.DriverName(), table, keys)
		if err := conn.Get(&rowCount, query, vals...); err != nil {
			return err
		}

		if rowCount != 1 {
			return fmt.Errorf("row [%+v] not found", row)
		}
	}

	return nil
}

func countQuery(driver string, table string, keys []string) string {
	switch driver {
	case DriverPostgres:
		q := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)
		if len(keys) > 0 {
			q += fmt.Sprintf("WHERE %s", strings.Join(keys, " AND "))
		}
		return q
	case DriverMySQL:
		for i, val := range keys {
			keys[i] = strings.Replace(val, fmt.Sprintf("=$%d", i+1), "=?", -1)
		}
		q := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)
		if len(keys) > 0 {
			q += fmt.Sprintf("WHERE %s", strings.Join(keys, " AND "))
		}
		return q
	}
	return ErrInvalidDriver.Error()
}
