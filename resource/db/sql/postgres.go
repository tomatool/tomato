package sql

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type postgres struct {
	db *sqlx.DB
}

func (c *postgres) Clear(tableName string) error {
	_, err := c.db.Exec(`TRUNCATE TABLE "` + tableName + `" RESTART IDENTITY CASCADE`)
	return err
}

func (c *postgres) Set(tableName string, rows []map[string]string) error {
	if err := c.Clear(tableName); err != nil {
		return err
	}

	tx, err := c.db.Begin()
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

		query := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, tableName, strings.Join(keys, ","), strings.Join(valctr, ","))
		if _, err := tx.Exec(query, vals...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (c *postgres) Compare(tableName string, rows []map[string]string) error {
	var rowCount int
	if err := c.db.Get(&rowCount, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName)); err != nil {
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

		query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE %s`, tableName, strings.Join(keys, " AND "))
		if err := c.db.Get(&rowCount, query, vals...); err != nil {
			return err
		}

		if rowCount != 1 {
			return fmt.Errorf("row [%+v] not found", row)
		}
	}
	return nil
}
