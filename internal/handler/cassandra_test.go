package handler

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/tomatool/tomato/internal/config"
)

func TestSplitCQL(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{name: "empty", script: "  \n ", want: nil},
		{name: "single without semicolon", script: "SELECT * FROM t", want: []string{"SELECT * FROM t"}},
		{
			name:   "several statements",
			script: "CREATE TABLE a (id int PRIMARY KEY);\nCREATE TABLE b (id int PRIMARY KEY);\n",
			want:   []string{"CREATE TABLE a (id int PRIMARY KEY)", "CREATE TABLE b (id int PRIMARY KEY)"},
		},
		{
			name:   "semicolon inside string",
			script: "INSERT INTO t (id, v) VALUES (1, 'a;b'); SELECT 1",
			want:   []string{"INSERT INTO t (id, v) VALUES (1, 'a;b')", "SELECT 1"},
		},
		{
			name:   "escaped quote inside string",
			script: "INSERT INTO t (v) VALUES ('it''s; fine');",
			want:   []string{"INSERT INTO t (v) VALUES ('it''s; fine')"},
		},
		{
			name:   "quoted identifier",
			script: `SELECT "odd;name" FROM t;`,
			want:   []string{`SELECT "odd;name" FROM t`},
		},
		{
			name:   "dollar string",
			script: "INSERT INTO t (v) VALUES ($$a;b$$); SELECT 2",
			want:   []string{"INSERT INTO t (v) VALUES ($$a;b$$)", "SELECT 2"},
		},
		{
			name:   "comments are dropped",
			script: "-- setup; ignored\n// also; ignored\nSELECT 1; /* block; comment */ SELECT 2;",
			want:   []string{"SELECT 1", "SELECT 2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitCQL(tt.script); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCQL(%q) = %#v, want %#v", tt.script, got, tt.want)
			}
		})
	}
}

func TestRowJSON(t *testing.T) {
	got, err := rowJSON(
		[]string{"id", "name", "tags", "attrs", "deleted_at"},
		[]string{"42", "Alice", `["a","b"]`, `{"k":"v"}`, "null"},
	)
	if err != nil {
		t.Fatalf("rowJSON returned %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("rowJSON produced invalid JSON %q: %v", got, err)
	}
	want := map[string]any{
		"id":         "42", // scalars stay strings; CQL converts them to the column type
		"name":       "Alice",
		"tags":       []any{"a", "b"},
		"attrs":      map[string]any{"k": "v"},
		"deleted_at": nil,
	}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("rowJSON = %v, want %v", doc, want)
	}

	if _, err := rowJSON([]string{"id", "name"}, []string{"1"}); err == nil {
		t.Error("expected an error when a row has fewer cells than headers")
	}

	// Text that only looks like JSON stays a string.
	got, _ = rowJSON([]string{"v"}, []string{"[not json"})
	if got != `{"v":"[not json"}` {
		t.Errorf("rowJSON kept invalid JSON as %q", got)
	}
}

func TestFormatCQLValue(t *testing.T) {
	uuid, _ := gocql.ParseUUID("11111111-1111-4111-a111-111111111111")
	ts := time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "nil", input: nil, want: "<nil>"},
		{name: "text", input: "hello", want: "hello"},
		{name: "int", input: 42, want: "42"},
		{name: "bigint", input: int64(7), want: "7"},
		{name: "boolean", input: true, want: "true"},
		{name: "uuid", input: uuid, want: "11111111-1111-4111-a111-111111111111"},
		{name: "timestamp", input: ts, want: "2026-09-24T10:30:00Z"},
		{name: "unset timestamp", input: time.Time{}, want: "<nil>"},
		{name: "blob", input: []byte("raw"), want: "raw"},
		{name: "list", input: []any{"a", "b"}, want: `["a","b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCQLValue(tt.input); got != tt.want {
				t.Errorf("formatCQLValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCassandraResetScope(t *testing.T) {
	r := &Cassandra{
		keyspace: "app",
		config: config.Resource{Options: map[string]any{
			"exclude": []any{"countries", "audit.events"},
		}},
	}
	if got := r.resetKeyspaces(); !reflect.DeepEqual(got, []string{"app"}) {
		t.Errorf("resetKeyspaces() = %v, want [app]", got)
	}
	cases := []struct {
		keyspace, table string
		want            bool
	}{
		{"app", "countries", true},
		{"app", "users", false},
		{"audit", "events", true},
		{"app", "events", false},
	}
	for _, c := range cases {
		if got := r.isExcluded(c.keyspace, c.table); got != c.want {
			t.Errorf("isExcluded(%q, %q) = %v, want %v", c.keyspace, c.table, got, c.want)
		}
	}

	r.config.Options["keyspaces"] = []any{"app", "audit"}
	if got := r.resetKeyspaces(); !reflect.DeepEqual(got, []string{"app", "audit"}) {
		t.Errorf("resetKeyspaces() with keyspaces option = %v", got)
	}
}
