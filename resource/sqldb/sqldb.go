package sqldb

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Client struct {
	options *Options
	conn    *sqlx.DB
}

type Options struct {
	Driver     string
	Datasource string
}

func New(o *Options) *Client {
	return &Client{o, nil}
}

func (c *Client) db() (*sqlx.DB, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	var err error
	c.conn, err = sqlx.Open(c.options.Driver, c.options.Datasource)
	return c.conn, err
}

func (c *Client) Clear(table string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	_, err = conn.DB.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE")
	return err
}

func (c *Client) Set(table string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

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

		query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(keys, ","), strings.Join(valctr, ","))
		if _, err := tx.Exec(query, vals...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (c *Client) Cmp(table string, rows []map[string]string) error {
	conn, err := c.db()
	if err != nil {
		return err
	}

	var rowCount int
	if err := conn.Get(&rowCount, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)); err != nil {
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

		query := fmt.Sprintf("\n\n\nSELECT COUNT(*) FROM %s WHERE %s", table, strings.Join(keys, " AND "))
		if err := conn.Get(&rowCount, query, vals...); err != nil {
			return err
		}

		if rowCount != 1 {
			return fmt.Errorf("row [%+v] not found", row)
		}
	}

	return nil
}
