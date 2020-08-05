package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/sql"
)

type PostgreSQL struct {
	db         *sqlx.DB
	datasource string
}

func New(cfg *config.Resource) (*PostgreSQL, error) {
	datasource, ok := cfg.Options["datasource"]
	if !ok || datasource == "" || datasource == "<no value>" {
		return nil, errors.New("datasource is required")
	}

	return &PostgreSQL{datasource: datasource}, nil
}

func (d *PostgreSQL) Open() error {
	var err error
	d.db, err = sqlx.Open("postgres", d.datasource)
	if err != nil {
		return err
	}
	return nil
}

func (d *PostgreSQL) Ready() error {
	return d.db.Ping()
}

func (d *PostgreSQL) Reset() error {
	var (
		tables []string
		query  string
	)

	query = `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'`
	if err := d.db.Select(&tables, query); err != nil {
		return err
	}

	tx, err := d.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range tables {
		if _, err := tx.Exec(`TRUNCATE TABLE ` + table + ` RESTART IDENTITY CASCADE`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *PostgreSQL) Close() error {
	return d.db.Close()
}

func addDoubleQuotes(s string) string {
	if s[0] == '"' {
		return s
	}
	return fmt.Sprintf(`"%s"`, s)
}

func (d *PostgreSQL) Select(tableName string, condition map[string]string) ([]map[string]string, error) {
	result := make([]map[string]string, 0)
	q := sql.NewQueryBuilder("postgres", "SELECT * FROM "+addDoubleQuotes(tableName))
	for key, val := range condition {
		q.Where(key, "=", val)
	}
	rows, err := d.db.Queryx(q.Query(), q.Arguments()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		r := make(map[string]interface{})
		if err := rows.MapScan(r); err != nil {
			return nil, err
		}
		z := make(map[string]string)
		for key, v := range r {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			z[key] = fmt.Sprintf("%v", v)
		}
		result = append(result, z)
	}

	return result, nil
}
func (d *PostgreSQL) Insert(tableName string, rows []map[string]string) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, row := range rows {
		query := sql.NewQueryBuilder("postgres", "INSERT INTO "+addDoubleQuotes(tableName))
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
func (d *PostgreSQL) Delete(tableName string, condition map[string]string) (int, error) {
	return 0, errors.New("not implemented")
}
