package mysql

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/util/sqlutil"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type MySQL struct {
	db     *sqlx.DB
	dbname string
}

func New(cfg *config.Resource) (*MySQL, error) {
	datasource, ok := cfg.Params["datasource"]
	if !ok {
		return nil, errors.New("datasource is required")
	}

	u, err := url.Parse("mysql://" + datasource + "?uyeah")
	if err != nil {
		return nil, err
	}

	db, err := sqlx.Open("mysql", datasource)
	if err != nil {
		return nil, err
	}

	return &MySQL{db: db, dbname: strings.Replace(u.Path, "/", "", -1)}, nil
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

func (d *MySQL) Select(tableName string, condition map[string]string) ([]map[string]string, error) {
	result := make([]map[string]string, 0)
	rows, err := d.db.Queryx("SELECT * FROM " + tableName)
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
		query := sqlutil.NewQueryBuilder("mysql", "INSERT INTO "+tableName)
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
