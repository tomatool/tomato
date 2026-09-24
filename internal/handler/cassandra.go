package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/gocql/gocql"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// Cassandra drives ScyllaDB and Apache Cassandra over CQL. Both speak the same
// protocol, so one handler serves the `scylladb` and `cassandra` types.
type Cassandra struct {
	name      string
	config    config.Resource
	container *container.Manager
	cluster   *gocql.ClusterConfig
	session   *gocql.Session
	keyspace  string
	skipReset bool // remote target without `reset: true`
}

func NewCassandra(name string, cfg config.Resource, cm *container.Manager) (*Cassandra, error) {
	return &Cassandra{name: name, config: cfg, container: cm}, nil
}

func (r *Cassandra) Name() string { return r.name }

func (r *Cassandra) Init(ctx context.Context) error {
	host, port, err := r.address(ctx)
	if err != nil {
		return err
	}
	r.skipReset = remoteResetGuard(r.name, r.config, host)

	cluster := gocql.NewCluster(net.JoinHostPort(host, strconv.Itoa(port)))
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.ProtoVersion = 4
	// A container advertises its Docker-internal address, which the host
	// cannot reach. Skip peer discovery and send every connection to the
	// address tomato knows about.
	cluster.DisableInitialHostLookup = true
	cluster.AddressTranslator = gocql.AddressTranslatorFunc(func(net.IP, int) (net.IP, int) {
		ip := net.ParseIP(host)
		if ip == nil {
			ip = net.ParseIP("127.0.0.1")
		}
		return ip, port
	})

	tlsCfg, err := tlsFromOptions(r.config.Options)
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		cluster.SslOpts = &gocql.SslOptions{Config: tlsCfg, EnableHostVerification: !tlsCfg.InsecureSkipVerify}
	}

	if user, ok := r.config.Options["user"].(string); ok && user != "" {
		password, _ := r.config.Options["password"].(string)
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: user, Password: password}
	}

	consistency := gocql.One
	if c, ok := r.config.Options["consistency"].(string); ok && c != "" {
		parsed, err := gocql.ParseConsistencyWrapper(strings.ToUpper(c))
		if err != nil {
			return fmt.Errorf("invalid consistency %q: %w", c, err)
		}
		consistency = parsed
	}
	cluster.Consistency = consistency

	r.cluster = cluster
	r.keyspace = r.config.Database
	if ks, ok := r.config.Options["keyspace"].(string); ok && ks != "" {
		r.keyspace = ks
	}
	return nil
}

// address resolves where to reach the database: an explicit `options.hosts`
// entry, or the mapped CQL port of the configured container.
func (r *Cassandra) address(ctx context.Context) (string, int, error) {
	if hosts, ok := r.config.Options["hosts"].([]any); ok && len(hosts) > 0 {
		h, _ := hosts[0].(string)
		hostPart, portPart, err := net.SplitHostPort(h)
		if err != nil {
			return h, 9042, nil
		}
		p, err := strconv.Atoi(portPart)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port in host %q", h)
		}
		return hostPart, p, nil
	}

	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return "", 0, fmt.Errorf("getting container host: %w", err)
	}
	portStr, err := r.container.GetPort(ctx, r.config.Container, "9042/tcp")
	if err != nil {
		return "", 0, fmt.Errorf("getting container port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid container port %q", portStr)
	}
	return host, port, nil
}

// Ready waits for CQL to accept queries. Scylla opens its port well before it
// serves CQL, so a single ping is not enough; poll until `ready_timeout`.
func (r *Cassandra) Ready(ctx context.Context) error {
	timeout := 90 * time.Second
	if s, ok := r.config.Options["ready_timeout"].(string); ok && s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid ready_timeout %q: %w", s, err)
		}
		timeout = d
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		session, err := r.cluster.CreateSession()
		if err == nil {
			err = session.Query("SELECT now() FROM system.local").WithContext(ctx).Exec()
			if err == nil {
				r.session = session
				break
			}
			session.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if r.session == nil {
		return fmt.Errorf("CQL not ready after %s: %w", timeout, lastErr)
	}

	if err := r.bootstrap(ctx); err != nil {
		return err
	}
	return nil
}

// bootstrap creates the keyspace (unless `create_keyspace: false`), runs the
// `schema` files, then rebinds the session to the keyspace so steps and raw
// CQL can use unqualified table names.
func (r *Cassandra) bootstrap(ctx context.Context) error {
	if r.keyspace != "" && optionBool(r.config.Options, "create_keyspace", true) {
		rf := 1
		if v, ok := r.config.Options["replication_factor"].(int); ok && v > 0 {
			rf = v
		}
		stmt := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}", r.keyspace, rf)
		if err := r.session.Query(stmt).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("creating keyspace %s: %w", r.keyspace, err)
		}
	}

	for _, path := range optionStrings(r.config.Options, "schema") {
		if err := r.ExecSQLFile(ctx, path); err != nil {
			return fmt.Errorf("running schema file %s: %w", path, err)
		}
	}

	if r.keyspace != "" {
		r.session.Close()
		r.cluster.Keyspace = r.keyspace
		session, err := r.cluster.CreateSession()
		if err != nil {
			return fmt.Errorf("connecting to keyspace %s: %w", r.keyspace, err)
		}
		r.session = session
	}
	return nil
}

// Reset truncates every table in the reset keyspaces (`options.keyspaces`,
// defaulting to the resource's keyspace), except those in `exclude`.
func (r *Cassandra) Reset(ctx context.Context) error {
	if r.skipReset {
		return nil
	}
	tables, err := r.tablesToReset(ctx)
	if err != nil {
		return err
	}
	for _, table := range tables {
		if err := r.session.Query("TRUNCATE " + table).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("truncating %s: %w", table, err)
		}
	}
	return nil
}

func (r *Cassandra) resetKeyspaces() []string {
	if ks := optionStrings(r.config.Options, "keyspaces"); len(ks) > 0 {
		return ks
	}
	if r.keyspace != "" {
		return []string{r.keyspace}
	}
	return nil
}

func (r *Cassandra) tablesToReset(ctx context.Context) ([]string, error) {
	if configured := optionStrings(r.config.Options, "tables"); len(configured) > 0 {
		return configured, nil
	}
	var tables []string
	for _, ks := range r.resetKeyspaces() {
		iter := r.session.Query("SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?", ks).WithContext(ctx).Iter()
		var table string
		for iter.Scan(&table) {
			if !r.isExcluded(ks, table) {
				tables = append(tables, ks+"."+table)
			}
		}
		if err := iter.Close(); err != nil {
			return nil, fmt.Errorf("listing tables in %s: %w", ks, err)
		}
	}
	return tables, nil
}

// isExcluded matches an `exclude` entry against the bare table name or the
// keyspace-qualified one.
func (r *Cassandra) isExcluded(keyspace, table string) bool {
	for _, e := range optionStrings(r.config.Options, "exclude") {
		if e == table || e == keyspace+"."+table {
			return true
		}
	}
	return false
}

func (r *Cassandra) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the ScyllaDB/Cassandra handler
func (r *Cassandra) Steps() StepCategory {
	return StepCategory{
		Name:        "ScyllaDB",
		Description: "Steps for interacting with ScyllaDB and Apache Cassandra over CQL",
		Steps: []StepDef{
			// Data Setup
			{
				Group:       "Data Setup",
				Pattern:     `^"{resource}" table "([^"]*)" has values:$`,
				Description: "Insert rows from table (values are converted to the column types)",
				Example:     `"scylla" table "users" has values:`,
				Handler:     r.setTableValues,
			},
			{
				Group:       "Data Setup",
				Pattern:     `^"{resource}" clears table "([^"]*)"$`,
				Description: "Truncate a table (removes all rows)",
				Example:     `"scylla" clears table "users"`,
				Handler:     r.clearTable,
			},
			{
				Group:       "Data Setup",
				Pattern:     `^"{resource}" executes:$`,
				Description: "Execute CQL (several statements separated by `;`)",
				Example:     `"scylla" executes:`,
				Handler:     r.executeCQL,
			},
			{
				Group:       "Data Setup",
				Pattern:     `^"{resource}" executes file "([^"]*)"$`,
				Description: "Execute CQL from file",
				Example:     `"scylla" executes file "fixtures/schema.cql"`,
				Handler:     r.executeCQLFile,
			},

			// Assertions
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" table "([^"]*)" contains:$`,
				Description: "Assert table contains rows (in any order)",
				Example:     `"scylla" table "users" contains:`,
				Handler:     r.tableShouldContain,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" table "([^"]*)" is empty$`,
				Description: "Assert table is empty",
				Example:     `"scylla" table "users" is empty`,
				Handler:     r.tableShouldBeEmpty,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" table "([^"]*)" has "(\d+)" rows$`,
				Description: "Assert row count",
				Example:     `"scylla" table "users" has "5" rows`,
				Handler:     r.tableShouldHaveRows,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" query "([^"]*)" returns:$`,
				Description: "Assert exact match of query result rows",
				Example:     `"scylla" query "SELECT id, name FROM users WHERE id = 1" returns:`,
				Handler:     r.queryReturns,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" query result of "([^"]*)" contains:$`,
				Description: "Assert query result contains expected rows (superset)",
				Example:     `"scylla" query result of "SELECT id, name FROM users" contains:`,
				Handler:     r.queryResultContains,
			},
		},
	}
}

func (r *Cassandra) clearTable(table string) error {
	return r.session.Query("TRUNCATE " + table).Exec()
}

// setTableValues inserts each row with `INSERT ... JSON`. CQL's JSON input
// accepts strings for scalar types (int, uuid, timestamp, boolean, ...), so
// Gherkin cells need no knowledge of the column types.
func (r *Cassandra) setTableValues(table string, data *godog.Table) error {
	if len(data.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}
	columns := cellValues(data.Rows[0])
	for i, row := range data.Rows[1:] {
		doc, err := rowJSON(columns, cellValues(row))
		if err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
		if err := r.session.Query(fmt.Sprintf("INSERT INTO %s JSON ?", table), doc).Exec(); err != nil {
			return fmt.Errorf("inserting row %d: %w", i+1, err)
		}
	}
	return nil
}

// rowJSON builds the document for `INSERT ... JSON`. Cells are strings, with
// two exceptions: `null` becomes a JSON null, and a cell that is itself a JSON
// array or object (a list, set, map or UDT) is embedded as-is.
func rowJSON(columns, values []string) (string, error) {
	if len(values) != len(columns) {
		return "", fmt.Errorf("expected %d cells, got %d", len(columns), len(values))
	}
	doc := make(map[string]any, len(columns))
	for i, col := range columns {
		v := ReplaceVariables(values[i])
		trimmed := strings.TrimSpace(v)
		switch {
		case trimmed == "null":
			doc[col] = nil
		case (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")) && json.Valid([]byte(trimmed)):
			doc[col] = json.RawMessage(trimmed)
		default:
			doc[col] = v
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (r *Cassandra) tableShouldContain(table string, expected *godog.Table) error {
	if len(expected.Rows) < 2 {
		return fmt.Errorf("expected table must have headers and at least one data row")
	}
	columns := cellValues(expected.Rows[0])
	actual, err := r.selectRows(fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), table), columns)
	if err != nil {
		return err
	}
	// CQL has no global row order, so match rows as a set.
	return containsRows(actual, expected.Rows[1:], columns)
}

func (r *Cassandra) countRows(table string) (int64, error) {
	var count int64
	err := r.session.Query(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	return count, err
}

func (r *Cassandra) tableShouldBeEmpty(table string) error {
	count, err := r.countRows(table)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("table %s has %d rows, expected 0", table, count)
	}
	return nil
}

func (r *Cassandra) tableShouldHaveRows(table string, expected int) error {
	count, err := r.countRows(table)
	if err != nil {
		return err
	}
	if count != int64(expected) {
		return fmt.Errorf("table %s has %d rows, expected %d", table, count, expected)
	}
	return nil
}

func (r *Cassandra) executeCQL(query *godog.DocString) error {
	return r.execStatements(context.Background(), ReplaceVariables(query.Content))
}

func (r *Cassandra) executeCQLFile(path string) error {
	return r.ExecSQLFile(context.Background(), path)
}

// ExecSQL runs one CQL statement. It implements SQLExecutor so `sql` hooks
// work against CQL resources too; CQL reports no affected-row count.
func (r *Cassandra) ExecSQL(ctx context.Context, query string) (int64, error) {
	return 0, r.execStatements(ctx, query)
}

// ExecSQLFile runs every statement in a CQL file.
func (r *Cassandra) ExecSQLFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading CQL file: %w", err)
	}
	return r.execStatements(ctx, string(content))
}

// execStatements runs a script one statement at a time; the CQL protocol
// accepts a single statement per query.
func (r *Cassandra) execStatements(ctx context.Context, script string) error {
	for _, stmt := range splitCQL(script) {
		if err := r.session.Query(stmt).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("executing %q: %w", abbreviate(stmt, 80), err)
		}
	}
	return nil
}

func (r *Cassandra) queryReturns(query string, expected *godog.Table) error {
	if len(expected.Rows) < 2 {
		return fmt.Errorf("expected table must have headers and at least one data row")
	}
	columns := cellValues(expected.Rows[0])
	actual, err := r.selectRows(ReplaceVariables(query), columns)
	if err != nil {
		return err
	}
	expectedRows := expected.Rows[1:]
	if len(actual) != len(expectedRows) {
		return fmt.Errorf("expected %d rows, got %d", len(expectedRows), len(actual))
	}
	for i, expectedRow := range expectedRows {
		for j, cell := range expectedRow.Cells {
			want := ReplaceVariables(cell.Value)
			if actual[i][j] != want {
				return fmt.Errorf("row %d, column %s: expected %q, got %q", i+1, columns[j], want, actual[i][j])
			}
		}
	}
	return nil
}

func (r *Cassandra) queryResultContains(query string, expected *godog.Table) error {
	if len(expected.Rows) < 2 {
		return fmt.Errorf("expected table must have headers and at least one data row")
	}
	columns := cellValues(expected.Rows[0])
	actual, err := r.selectRows(ReplaceVariables(query), columns)
	if err != nil {
		return err
	}
	return containsRows(actual, expected.Rows[1:], columns)
}

// selectRows runs a query and returns the named columns of every row as
// strings. Columns are looked up by name, so the header row may list them in
// any order and may use aliases (`count(*) AS cnt`).
func (r *Cassandra) selectRows(query string, columns []string) ([][]string, error) {
	iter := r.session.Query(query).Iter()
	var rows [][]string
	for {
		row := make(map[string]any)
		if !iter.MapScan(row) {
			break
		}
		values := make([]string, len(columns))
		for i, col := range columns {
			v, ok := row[col]
			if !ok {
				v, ok = row[strings.ToLower(col)]
			}
			if !ok {
				iter.Close()
				return nil, fmt.Errorf("column %q not in query result", col)
			}
			values[i] = formatCQLValue(v)
		}
		rows = append(rows, values)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	return rows, nil
}

func containsRows(actual [][]string, expected []*messages.PickleTableRow, columns []string) error {
	for i, expectedRow := range expected {
		want := make([]string, len(expectedRow.Cells))
		for j, cell := range expectedRow.Cells {
			want[j] = ReplaceVariables(cell.Value)
		}
		if !slices.ContainsFunc(actual, func(row []string) bool { return slices.Equal(row, want) }) {
			vals := make([]string, len(want))
			for j, v := range want {
				vals[j] = fmt.Sprintf("%s=%q", columns[j], v)
			}
			return fmt.Errorf("expected row %d not found in results: %s", i+1, strings.Join(vals, ", "))
		}
	}
	return nil
}

// formatCQLValue renders a value decoded by gocql the way it is written in a
// feature file. Unset columns come back as nil or as a zero pointer.
func formatCQLValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "<nil>"
	case []byte:
		return string(val)
	case time.Time:
		if val.IsZero() {
			return "<nil>"
		}
		return val.UTC().Format(time.RFC3339Nano)
	case gocql.UUID:
		return val.String()
	case fmt.Stringer:
		return val.String()
	case map[string]any, []any:
		out, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(out)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// splitCQL splits a CQL script into statements on `;`, ignoring semicolons
// inside 'strings', "quoted identifiers", $$dollar strings$$ and comments
// (`--`, `//`, `/* */`). Comments are dropped and blank statements skipped.
func splitCQL(script string) []string {
	var stmts []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}

	for i := 0; i < len(script); i++ {
		c := script[i]
		switch {
		case c == '\'' || c == '"':
			// Quoted: copy through the closing quote; a doubled quote escapes.
			cur.WriteByte(c)
			for i++; i < len(script); i++ {
				cur.WriteByte(script[i])
				if script[i] == c {
					if i+1 < len(script) && script[i+1] == c {
						i++
						cur.WriteByte(c)
						continue
					}
					break
				}
			}
		case c == '$' && strings.HasPrefix(script[i:], "$$"):
			end := strings.Index(script[i+2:], "$$")
			if end < 0 {
				cur.WriteString(script[i:])
				i = len(script)
				break
			}
			cur.WriteString(script[i : i+2+end+2])
			i += 2 + end + 1
		case (c == '-' || c == '/') && i+1 < len(script) && script[i+1] == c:
			for i < len(script) && script[i] != '\n' {
				i++
			}
			cur.WriteByte('\n')
		case c == '/' && i+1 < len(script) && script[i+1] == '*':
			end := strings.Index(script[i+2:], "*/")
			if end < 0 {
				i = len(script)
				break
			}
			i += 2 + end + 1
			cur.WriteByte(' ')
		case c == ';':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return stmts
}

func cellValues(row *messages.PickleTableRow) []string {
	values := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		values[i] = cell.Value
	}
	return values
}

func optionStrings(options map[string]any, key string) []string {
	switch v := options[key].(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func optionBool(options map[string]any, key string, def bool) bool {
	if v, ok := options[key].(bool); ok {
		return v
	}
	return def
}

func abbreviate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (r *Cassandra) Cleanup(ctx context.Context) error {
	if r.session != nil {
		r.session.Close()
	}
	return nil
}

var _ Handler = (*Cassandra)(nil)
var _ SQLExecutor = (*Cassandra)(nil)
