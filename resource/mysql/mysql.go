package mysql

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/sql"
)

type MySQL struct {
	db         *sqlx.DB
	dbname     string
	datasource string
}

func New(cfg *config.Resource) (*MySQL, error) {
	datasource, ok := cfg.Options["datasource"]
	if !ok || datasource == "" || datasource == "<no value>" {
		return nil, errors.New("datasource is required")
	}

	return &MySQL{datasource: datasource, dbname: getDatabaseName(datasource)}, nil
}

func getDatabaseName(datasource string) string {
	if strings.HasPrefix(datasource, "mysql://") {
		datasource = datasource[8:]
	}
	if s := strings.Split(datasource, "/"); len(s) == 2 {
		return strings.Split(s[len(s)-1], "?")[0]
	}

	return datasource
}

func (d *MySQL) Open() error {
	var err error
	d.db, err = sqlx.Open("mysql", d.datasource)
	if err != nil {
		return err
	}
	return nil
}

func (d *MySQL) Ready() error {
	return d.db.Ping()
}

func (d *MySQL) Reset() error {
	var (
		tables []string
	)

	query := `SELECT table_name FROM information_schema.tables WHERE table_type = 'base table' AND table_schema='` + d.dbname + `'`
	if err := d.db.Select(&tables, query); err != nil {
		return err
	}

	tx, err := d.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}

	for _, table := range tables {
		if _, err := tx.Exec("TRUNCATE TABLE " + table); err != nil {
			e, ok := err.(*mysql.MySQLError)
			if ok && e.Number == 1146 {
				return nil
			}
			return err
		}
	}

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *MySQL) Close() error {
	return d.db.Close()
}

func (d *MySQL) Select(tableName string, condition map[string]string) ([]map[string]string, error) {
	result := make([]map[string]string, 0)
	q := sql.NewQueryBuilder("mysql", "SELECT * FROM "+tableName)
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
func (d *MySQL) Insert(tableName string, rows []map[string]string) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, row := range rows {
		query := sql.NewQueryBuilder("mysql", "INSERT INTO "+tableName)
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
func (d *MySQL) Delete(tableName string, condition map[string]string) (int, error) {
	return 0, errors.New("not implemented")
}
