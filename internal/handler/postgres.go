package handler

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"
	_ "github.com/lib/pq"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type Postgres struct {
	name      string
	config    config.Resource
	container *container.Manager
	db        *sql.DB
}

func NewPostgres(name string, cfg config.Resource, cm *container.Manager) (*Postgres, error) {
	return &Postgres{name: name, config: cfg, container: cm}, nil
}

func (r *Postgres) Name() string { return r.name }

func (r *Postgres) Init(ctx context.Context) error {
	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return fmt.Errorf("getting container host: %w", err)
	}
	port, err := r.container.GetPort(ctx, r.config.Container, "5432/tcp")
	if err != nil {
		return fmt.Errorf("getting container port: %w", err)
	}
	dbName := r.config.Database
	if dbName == "" {
		dbName = "postgres"
	}
	user, password := "postgres", "postgres"
	if u, ok := r.config.Options["user"].(string); ok {
		user = u
	}
	if p, ok := r.config.Options["password"].(string); ok {
		password = p
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	r.db = db
	return nil
}

func (r *Postgres) Ready(ctx context.Context) error { return r.db.PingContext(ctx) }

func (r *Postgres) Reset(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		if !r.isExcluded(table) {
			tables = append(tables, table)
		}
	}
	if len(tables) == 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", strings.Join(tables, ", ")))
	return err
}

func (r *Postgres) isExcluded(table string) bool {
	excludeList := []string{"schema_migrations", "goose_db_version"}
	if exclude, ok := r.config.Options["exclude"].([]interface{}); ok {
		for _, e := range exclude {
			if s, ok := e.(string); ok {
				excludeList = append(excludeList, s)
			}
		}
	}
	for _, e := range excludeList {
		if e == table {
			return true
		}
	}
	return false
}

func (r *Postgres) RegisterSteps(ctx *godog.ScenarioContext) {
	ctx.Step(fmt.Sprintf(`^I set "%s" table "([^"]*)" with values:$`, r.name), r.setTableValues)
	ctx.Step(fmt.Sprintf(`^"%s" table "([^"]*)" should contain:$`, r.name), r.tableShouldContain)
	ctx.Step(fmt.Sprintf(`^"%s" table "([^"]*)" should be empty$`, r.name), r.tableShouldBeEmpty)
	ctx.Step(fmt.Sprintf(`^"%s" table "([^"]*)" should have "(\d+)" rows$`, r.name), r.tableShouldHaveRows)
	ctx.Step(fmt.Sprintf(`^I execute SQL on "%s":$`, r.name), r.executeSQL)
	ctx.Step(fmt.Sprintf(`^I execute SQL file "([^"]*)" on "%s"$`, r.name), r.executeSQLFile)
}

func (r *Postgres) setTableValues(table string, data *godog.Table) error {
	if len(data.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}
	headers := data.Rows[0].Cells
	columns := make([]string, len(headers))
	for i, cell := range headers {
		columns[i] = cell.Value
	}
	for _, row := range data.Rows[1:] {
		values := make([]string, len(row.Cells))
		for i, cell := range row.Cells {
			values[i] = fmt.Sprintf("'%s'", cell.Value)
		}
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(values, ", "))
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("inserting row: %w", err)
		}
	}
	return nil
}

func (r *Postgres) tableShouldContain(table string, expected *godog.Table) error {
	if len(expected.Rows) < 2 {
		return fmt.Errorf("expected table must have headers and at least one data row")
	}
	headers := expected.Rows[0].Cells
	columns := make([]string, len(headers))
	for i, cell := range headers {
		columns[i] = cell.Value
	}
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), table)
	rows, err := r.db.Query(query)
	if err != nil {
		return fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()
	var actual [][]string
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		row := make([]string, len(columns))
		for i, v := range values {
			row[i] = fmt.Sprintf("%v", v)
		}
		actual = append(actual, row)
	}
	for i, expectedRow := range expected.Rows[1:] {
		if i >= len(actual) {
			return fmt.Errorf("missing row %d", i+1)
		}
		for j, cell := range expectedRow.Cells {
			if actual[i][j] != cell.Value {
				return fmt.Errorf("row %d, column %s: expected %q, got %q", i+1, columns[j], cell.Value, actual[i][j])
			}
		}
	}
	return nil
}

func (r *Postgres) tableShouldBeEmpty(table string) error {
	var count int
	if err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("table %s has %d rows, expected 0", table, count)
	}
	return nil
}

func (r *Postgres) tableShouldHaveRows(table string, expected int) error {
	var count int
	if err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return err
	}
	if count != expected {
		return fmt.Errorf("table %s has %d rows, expected %d", table, count, expected)
	}
	return nil
}

func (r *Postgres) executeSQL(query *godog.DocString) error {
	_, err := r.db.Exec(query.Content)
	return err
}

func (r *Postgres) executeSQLFile(path string) error {
	return r.ExecSQLFile(context.Background(), path)
}

func (r *Postgres) ExecSQL(ctx context.Context, query string) (int64, error) {
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Postgres) ExecSQLFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading SQL file: %w", err)
	}
	_, err = r.db.ExecContext(ctx, string(content))
	return err
}

func (r *Postgres) Cleanup(ctx context.Context) error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

var _ Handler = (*Postgres)(nil)
var _ SQLExecutor = (*Postgres)(nil)
