package handler

import (
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestFormatDBValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "<nil>",
		},
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "int value",
			input:    42,
			expected: "42",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "byte slice (UUID)",
			input:    []byte("11111111-1111-4111-a111-111111111111"),
			expected: "11111111-1111-4111-a111-111111111111",
		},
		{
			name:     "byte slice (simple string)",
			input:    []byte("hello world"),
			expected: "hello world",
		},
		{
			name:     "empty byte slice",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "float value",
			input:    3.14,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDBValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatDBValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
		table   string
		want    bool
	}{
		{name: "golang-migrate", table: "schema_migrations", want: true},
		{name: "goose", table: "goose_db_version", want: true},
		{name: "flyway", table: "flyway_schema_history", want: true},
		{name: "liquibase changelog", table: "databasechangelog", want: true},
		{name: "liquibase lock", table: "databasechangeloglock", want: true},
		{name: "application table", table: "users", want: false},
		{
			name:    "user exclusion",
			options: map[string]interface{}{"exclude": []interface{}{"countries"}},
			table:   "countries",
			want:    true,
		},
		{
			name:    "user exclusion keeps defaults",
			options: map[string]interface{}{"exclude": []interface{}{"countries"}},
			table:   "flyway_schema_history",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Postgres{config: config.Resource{Options: tt.options}}
			if got := r.isExcluded(tt.table); got != tt.want {
				t.Errorf("isExcluded(%q) = %v, want %v", tt.table, got, tt.want)
			}
		})
	}
}
