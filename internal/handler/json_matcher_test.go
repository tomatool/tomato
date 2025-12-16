package handler

import (
	"testing"
)

func TestMatchSpecial_TypeMatchers(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		// @string
		{"string matches string", "@string", "hello", false},
		{"string fails on number", "@string", 123.0, true},
		{"string fails on bool", "@string", true, true},
		{"string fails on nil", "@string", nil, true},

		// @number
		{"number matches float", "@number", 123.456, false},
		{"number matches int as float", "@number", 42.0, false},
		{"number fails on string", "@number", "123", true},
		{"number fails on bool", "@number", false, true},

		// @boolean
		{"boolean matches true", "@boolean", true, false},
		{"boolean matches false", "@boolean", false, false},
		{"boolean fails on string", "@boolean", "true", true},
		{"boolean fails on number", "@boolean", 1.0, true},

		// @array
		{"array matches array", "@array", []interface{}{"a", "b"}, false},
		{"array matches empty array", "@array", []interface{}{}, false},
		{"array fails on string", "@array", "[]", true},
		{"array fails on object", "@array", map[string]interface{}{}, true},

		// @object
		{"object matches object", "@object", map[string]interface{}{"key": "value"}, false},
		{"object matches empty object", "@object", map[string]interface{}{}, false},
		{"object fails on array", "@object", []interface{}{}, true},
		{"object fails on string", "@object", "{}", true},

		// @any
		{"any matches string", "@any", "anything", false},
		{"any matches number", "@any", 42.0, false},
		{"any matches bool", "@any", true, false},
		{"any matches array", "@any", []interface{}{}, false},
		{"any matches object", "@any", map[string]interface{}{}, false},
		{"any matches nil", "@any", nil, false},

		// @null
		{"null matches nil", "@null", nil, false},
		{"null fails on string", "@null", "null", true},
		{"null fails on empty string", "@null", "", true},
		{"null fails on zero", "@null", 0.0, true},

		// @notnull
		{"notnull matches string", "@notnull", "hello", false},
		{"notnull matches number", "@notnull", 0.0, false},
		{"notnull matches empty string", "@notnull", "", false},
		{"notnull fails on nil", "@notnull", nil, true},

		// @empty
		{"empty matches empty string", "@empty", "", false},
		{"empty matches empty array", "@empty", []interface{}{}, false},
		{"empty matches empty object", "@empty", map[string]interface{}{}, false},
		{"empty matches nil", "@empty", nil, false},
		{"empty fails on non-empty string", "@empty", "hello", true},
		{"empty fails on non-empty array", "@empty", []interface{}{"a"}, true},
		{"empty fails on non-empty object", "@empty", map[string]interface{}{"k": "v"}, true},

		// @notempty
		{"notempty matches non-empty string", "@notempty", "hello", false},
		{"notempty matches non-empty array", "@notempty", []interface{}{"a"}, false},
		{"notempty matches non-empty object", "@notempty", map[string]interface{}{"k": "v"}, false},
		{"notempty fails on empty string", "@notempty", "", true},
		{"notempty fails on empty array", "@notempty", []interface{}{}, true},
		{"notempty fails on empty object", "@notempty", map[string]interface{}{}, true},
		{"notempty fails on nil", "@notempty", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_RegexMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		{"regex matches simple pattern", "@regex:^hello$", "hello", false},
		{"regex matches partial pattern", "@regex:ell", "hello", false},
		{"regex matches email pattern", "@regex:^[a-z]+@[a-z]+\\.[a-z]+$", "user@example.com", false},
		{"regex matches UUID pattern", "@regex:^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", "550e8400-e29b-41d4-a716-446655440000", false},
		{"regex fails on no match", "@regex:^hello$", "world", true},
		{"regex fails on non-string", "@regex:.*", 123.0, true},
		{"regex fails on invalid pattern", "@regex:[invalid", "test", true},
		{"regex with special chars", "@regex:hello\\s+world", "hello   world", false},
		{"regex anchored start", "@regex:^user-", "user-123", false},
		{"regex anchored end", "@regex:-admin$", "super-admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_ContainsMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		{"contains finds substring", "@contains:world", "hello world", false},
		{"contains finds at start", "@contains:hello", "hello world", false},
		{"contains finds at end", "@contains:world", "hello world", false},
		{"contains finds exact", "@contains:hello", "hello", false},
		{"contains fails on no match", "@contains:foo", "hello world", true},
		{"contains fails on non-string", "@contains:test", 123.0, true},
		{"contains is case sensitive", "@contains:Hello", "hello", true},
		{"contains with special chars", "@contains:@example", "user@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_StartsWithMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		{"startswith matches prefix", "@startswith:hello", "hello world", false},
		{"startswith matches exact", "@startswith:hello", "hello", false},
		{"startswith fails on middle", "@startswith:world", "hello world", true},
		{"startswith fails on end", "@startswith:world", "hello world", true},
		{"startswith fails on non-string", "@startswith:test", 123.0, true},
		{"startswith is case sensitive", "@startswith:Hello", "hello", true},
		{"startswith with url prefix", "@startswith:https://", "https://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_EndsWithMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		{"endswith matches suffix", "@endswith:world", "hello world", false},
		{"endswith matches exact", "@endswith:hello", "hello", false},
		{"endswith fails on start", "@endswith:hello", "hello world", true},
		{"endswith fails on middle", "@endswith:lo wo", "hello world", true},
		{"endswith fails on non-string", "@endswith:test", 123.0, true},
		{"endswith is case sensitive", "@endswith:World", "hello world", true},
		{"endswith with file extension", "@endswith:.json", "config.json", false},
		{"endswith with domain", "@endswith:.com", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_NumericComparisons(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		// @gt (greater than)
		{"gt passes when greater", "@gt:10", 15.0, false},
		{"gt fails when equal", "@gt:10", 10.0, true},
		{"gt fails when less", "@gt:10", 5.0, true},
		{"gt works with floats", "@gt:3.14", 3.15, false},
		{"gt fails on string", "@gt:10", "15", true},
		{"gt fails on invalid value", "@gt:abc", 10.0, true},

		// @gte (greater than or equal)
		{"gte passes when greater", "@gte:10", 15.0, false},
		{"gte passes when equal", "@gte:10", 10.0, false},
		{"gte fails when less", "@gte:10", 5.0, true},

		// @lt (less than)
		{"lt passes when less", "@lt:10", 5.0, false},
		{"lt fails when equal", "@lt:10", 10.0, true},
		{"lt fails when greater", "@lt:10", 15.0, true},

		// @lte (less than or equal)
		{"lte passes when less", "@lte:10", 5.0, false},
		{"lte passes when equal", "@lte:10", 10.0, false},
		{"lte fails when greater", "@lte:10", 15.0, true},

		// Negative numbers
		{"gt with negative", "@gt:-5", -3.0, false},
		{"lt with negative", "@lt:-5", -10.0, false},

		// Zero
		{"gt zero", "@gt:0", 1.0, false},
		{"lt zero", "@lt:0", -1.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_LengthMatcher(t *testing.T) {
	tests := []struct {
		name    string
		matcher string
		actual  interface{}
		wantErr bool
	}{
		// String length
		{"len matches string length", "@len:5", "hello", false},
		{"len fails on wrong string length", "@len:5", "hi", true},
		{"len matches empty string", "@len:0", "", false},

		// Array length
		{"len matches array length", "@len:3", []interface{}{"a", "b", "c"}, false},
		{"len fails on wrong array length", "@len:3", []interface{}{"a", "b"}, true},
		{"len matches empty array", "@len:0", []interface{}{}, false},

		// Object length (number of keys)
		{"len matches object keys", "@len:2", map[string]interface{}{"a": 1, "b": 2}, false},
		{"len fails on wrong object keys", "@len:2", map[string]interface{}{"a": 1}, true},
		{"len matches empty object", "@len:0", map[string]interface{}{}, false},

		// Error cases
		{"len fails on number", "@len:5", 12345.0, true},
		{"len fails on invalid length", "@len:abc", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MatchSpecial(tt.matcher, tt.actual, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchSpecial(%q, %v) error = %v, wantErr %v", tt.matcher, tt.actual, err, tt.wantErr)
			}
		})
	}
}

func TestMatchSpecial_UnknownMatcher(t *testing.T) {
	err := MatchSpecial("@unknown", "value", "test")
	if err == nil {
		t.Error("expected error for unknown matcher")
	}
}

func TestCompareJSON_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantErr  bool
	}{
		// Simple values
		{"string exact match", "hello", "hello", false},
		{"string mismatch", "hello", "world", true},
		{"number exact match", 42.0, 42.0, false},
		{"number mismatch", 42.0, 43.0, true},
		{"boolean match", true, true, false},
		{"boolean mismatch", true, false, true},
		{"null match", nil, nil, false},

		// Objects
		{
			"object exact match",
			map[string]interface{}{"name": "John", "age": 30.0},
			map[string]interface{}{"name": "John", "age": 30.0},
			false,
		},
		{
			"object missing key",
			map[string]interface{}{"name": "John", "age": 30.0},
			map[string]interface{}{"name": "John"},
			true,
		},
		{
			"object extra key fails in exact mode",
			map[string]interface{}{"name": "John"},
			map[string]interface{}{"name": "John", "age": 30.0},
			true,
		},
		{
			"object value mismatch",
			map[string]interface{}{"name": "John"},
			map[string]interface{}{"name": "Jane"},
			true,
		},

		// Arrays
		{
			"array exact match",
			[]interface{}{"a", "b", "c"},
			[]interface{}{"a", "b", "c"},
			false,
		},
		{
			"array length mismatch",
			[]interface{}{"a", "b"},
			[]interface{}{"a", "b", "c"},
			true,
		},
		{
			"array element mismatch",
			[]interface{}{"a", "b", "c"},
			[]interface{}{"a", "x", "c"},
			true,
		},

		// Nested structures
		{
			"nested object match",
			map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"address": map[string]interface{}{
						"city": "NYC",
					},
				},
			},
			map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"address": map[string]interface{}{
						"city": "NYC",
					},
				},
			},
			false,
		},
		{
			"nested object mismatch",
			map[string]interface{}{
				"user": map[string]interface{}{
					"address": map[string]interface{}{
						"city": "NYC",
					},
				},
			},
			map[string]interface{}{
				"user": map[string]interface{}{
					"address": map[string]interface{}{
						"city": "LA",
					},
				},
			},
			true,
		},

		// Type mismatches
		{"type mismatch object vs array", map[string]interface{}{}, []interface{}{}, true},
		{"type mismatch array vs object", []interface{}{}, map[string]interface{}{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareJSON(tt.expected, tt.actual, "", false)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompareJSON_PartialMatch(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantErr  bool
	}{
		// Partial object matching (extra keys ignored)
		{
			"partial allows extra keys",
			map[string]interface{}{"name": "John"},
			map[string]interface{}{"name": "John", "age": 30.0, "city": "NYC"},
			false,
		},
		{
			"partial still requires specified keys",
			map[string]interface{}{"name": "John", "email": "john@example.com"},
			map[string]interface{}{"name": "John", "age": 30.0},
			true,
		},
		{
			"partial matches nested objects",
			map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
				},
			},
			map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"age":  30.0,
				},
				"metadata": map[string]interface{}{
					"created": "2024-01-01",
				},
			},
			false,
		},

		// Arrays still require exact length match in partial mode
		{
			"partial arrays still require same length",
			[]interface{}{"a", "b"},
			[]interface{}{"a", "b", "c"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareJSON(tt.expected, tt.actual, "", true)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareJSON() partial error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompareJSON_WithMatchers(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		partial  bool
		wantErr  bool
	}{
		// Type matchers in objects
		{
			"object with type matchers",
			map[string]interface{}{
				"id":        "@string",
				"count":     "@number",
				"active":    "@boolean",
				"tags":      "@array",
				"metadata":  "@object",
				"anything":  "@any",
				"deleted":   "@null",
				"timestamp": "@notnull",
			},
			map[string]interface{}{
				"id":        "abc-123",
				"count":     42.0,
				"active":    true,
				"tags":      []interface{}{"a", "b"},
				"metadata":  map[string]interface{}{"key": "value"},
				"anything":  "whatever",
				"deleted":   nil,
				"timestamp": "2024-01-01",
			},
			false,
			false,
		},

		// Regex matchers
		{
			"object with regex matcher",
			map[string]interface{}{
				"email": "@regex:^[a-z]+@[a-z]+\\.[a-z]+$",
				"id":    "@regex:^[0-9a-f-]{36}$",
			},
			map[string]interface{}{
				"email": "user@example.com",
				"id":    "550e8400-e29b-41d4-a716-446655440000",
			},
			false,
			false,
		},

		// String matchers
		{
			"object with string matchers",
			map[string]interface{}{
				"url":      "@startswith:https://",
				"filename": "@endswith:.json",
				"message":  "@contains:success",
			},
			map[string]interface{}{
				"url":      "https://api.example.com",
				"filename": "config.json",
				"message":  "Operation completed with success",
			},
			false,
			false,
		},

		// Numeric matchers
		{
			"object with numeric matchers",
			map[string]interface{}{
				"score":   "@gt:50",
				"percent": "@lte:100",
				"count":   "@gte:0",
			},
			map[string]interface{}{
				"score":   75.0,
				"percent": 100.0,
				"count":   0.0,
			},
			false,
			false,
		},

		// Length matchers
		{
			"object with length matchers",
			map[string]interface{}{
				"name":  "@len:4",
				"items": "@len:3",
			},
			map[string]interface{}{
				"name":  "John",
				"items": []interface{}{"a", "b", "c"},
			},
			false,
			false,
		},

		// Mixed matchers with partial mode
		{
			"partial with matchers",
			map[string]interface{}{
				"id":   "@regex:^user-[0-9]+$",
				"name": "@notempty",
			},
			map[string]interface{}{
				"id":        "user-12345",
				"name":      "John",
				"extraKey1": "ignored",
				"extraKey2": 123.0,
			},
			true,
			false,
		},

		// Matchers in nested objects
		{
			"nested matchers",
			map[string]interface{}{
				"data": map[string]interface{}{
					"user": map[string]interface{}{
						"id":    "@regex:^[0-9]+$",
						"email": "@contains:@",
					},
				},
			},
			map[string]interface{}{
				"data": map[string]interface{}{
					"user": map[string]interface{}{
						"id":    "12345",
						"email": "user@example.com",
					},
				},
			},
			false,
			false,
		},

		// Matchers in arrays
		{
			"matchers in arrays",
			[]interface{}{
				map[string]interface{}{"id": "@number"},
				map[string]interface{}{"id": "@number"},
			},
			[]interface{}{
				map[string]interface{}{"id": 1.0},
				map[string]interface{}{"id": 2.0},
			},
			false,
			false,
		},

		// Failing matchers
		{
			"regex matcher fails",
			map[string]interface{}{"email": "@regex:^[a-z]+@[a-z]+\\.[a-z]+$"},
			map[string]interface{}{"email": "invalid-email"},
			false,
			true,
		},
		{
			"contains matcher fails",
			map[string]interface{}{"message": "@contains:success"},
			map[string]interface{}{"message": "error occurred"},
			false,
			true,
		},
		{
			"numeric matcher fails",
			map[string]interface{}{"score": "@gt:100"},
			map[string]interface{}{"score": 50.0},
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareJSON(tt.expected, tt.actual, "", tt.partial)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompareJSON_ErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		expected     interface{}
		actual       interface{}
		partial      bool
		errContains  string
	}{
		{
			"missing key error includes path",
			map[string]interface{}{"user": map[string]interface{}{"name": "John"}},
			map[string]interface{}{"user": map[string]interface{}{}},
			false,
			"user",
		},
		{
			"value mismatch error includes path",
			map[string]interface{}{"data": map[string]interface{}{"count": 10.0}},
			map[string]interface{}{"data": map[string]interface{}{"count": 20.0}},
			false,
			"data.count",
		},
		{
			"array error includes index",
			[]interface{}{1.0, 2.0, 3.0},
			[]interface{}{1.0, 2.0, 999.0},
			false,
			"[2]",
		},
		{
			"unexpected key error in exact mode",
			map[string]interface{}{"a": 1.0},
			map[string]interface{}{"a": 1.0, "b": 2.0},
			false,
			"unexpected key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareJSON(tt.expected, tt.actual, "", tt.partial)
			if err == nil {
				t.Error("expected error but got nil")
				return
			}
			if !contains(err.Error(), tt.errContains) {
				t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
