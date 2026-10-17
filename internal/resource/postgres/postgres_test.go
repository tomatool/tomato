package postgres

import (
	"slices"
	"strings"
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
		schema  string
		table   string
		want    bool
	}{
		{name: "golang-migrate", table: "schema_migrations", want: true},
		{name: "goose", table: "goose_db_version", want: true},
		{name: "flyway", table: "flyway_schema_history", want: true},
		{name: "liquibase changelog", table: "databasechangelog", want: true},
		{name: "liquibase lock", table: "databasechangeloglock", want: true},
		{name: "migration table in another schema", schema: "billing", table: "flyway_schema_history", want: true},
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
		{
			name:    "bare exclusion matches in every schema",
			options: map[string]interface{}{"exclude": []interface{}{"countries"}},
			schema:  "billing",
			table:   "countries",
			want:    true,
		},
		{
			name:    "qualified exclusion matches its schema",
			options: map[string]interface{}{"exclude": []interface{}{"billing.fx_rates"}},
			schema:  "billing",
			table:   "fx_rates",
			want:    true,
		},
		{
			name:    "qualified exclusion leaves other schemas alone",
			options: map[string]interface{}{"exclude": []interface{}{"billing.fx_rates"}},
			schema:  "public",
			table:   "fx_rates",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.schema
			if schema == "" {
				schema = "public"
			}
			r := &Postgres{config: config.Resource{Options: tt.options}}
			if got := r.isExcluded(schema, tt.table); got != tt.want {
				t.Errorf("isExcluded(%q, %q) = %v, want %v", schema, tt.table, got, tt.want)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{name: "no options"},
		{name: "no schemas", options: map[string]any{"tables": []any{"users"}}},
		{name: "names", options: map[string]any{"schemas": []any{"plato", "plato_plugins"}}},
		{name: "pattern", options: map[string]any{"schemas": []any{"plugin_*"}}},
		{name: "a string, not a list", options: map[string]any{"schemas": "plato"}, wantErr: "must be a list"},
		{name: "empty value", options: map[string]any{"schemas": nil}, wantErr: "must be a list"},
		{name: "empty list", options: map[string]any{"schemas": []any{}}, wantErr: "is empty"},
		{name: "blank name", options: map[string]any{"schemas": []any{"plato", " "}}, wantErr: "schemas[1]"},
		{name: "not a string", options: map[string]any{"schemas": []any{2024}}, wantErr: "quote it"},
		{name: "bad pattern", options: map[string]any{"schemas": []any{"plugin_["}}, wantErr: "not a valid pattern"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.options)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOptions() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateOptions() = %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewRefusesInvalidSchemas(t *testing.T) {
	_, err := New("db", config.Resource{Type: "postgres", Options: map[string]any{"schemas": "plato"}}, nil)
	if err == nil {
		t.Fatal("New accepted schemas given as a string")
	}
}

func TestMatchSchemas(t *testing.T) {
	existing := []string{"public", "plato", "plato_plugins", "plugin_1", "plugin_42"}
	tests := []struct {
		name     string
		patterns []string
		want     []string
		wantErr  string
	}{
		{name: "names", patterns: []string{"plato", "plato_plugins"}, want: []string{"plato", "plato_plugins"}},
		{name: "pattern", patterns: []string{"plugin_*"}, want: []string{"plugin_1", "plugin_42"}},
		{name: "pattern overlapping a name", patterns: []string{"plato*", "plato"}, want: []string{"plato", "plato_plugins"}},
		{name: "pattern matching nothing", patterns: []string{"plato", "tenant_*"}, want: []string{"plato"}},
		{name: "missing name", patterns: []string{"plato", "platoo", "billing"}, wantErr: "the database has no schema platoo, billing,"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchSchemas(tt.patterns, existing)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("matchSchemas() error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("matchSchemas() = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("matchSchemas() = %v, want %v", got, tt.want)
			}
		})
	}
}
