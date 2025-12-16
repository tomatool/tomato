package handler

import (
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestVariables_SetAndGet(t *testing.T) {
	v := NewVariables()

	v.Set("user_id", "123")
	v.Set("name", "test")

	val, ok := v.Get("user_id")
	if !ok || val != "123" {
		t.Errorf("expected '123', got '%s'", val)
	}

	val, ok = v.Get("name")
	if !ok || val != "test" {
		t.Errorf("expected 'test', got '%s'", val)
	}

	_, ok = v.Get("nonexistent")
	if ok {
		t.Error("expected nonexistent key to return false")
	}
}

func TestVariables_Reset(t *testing.T) {
	v := NewVariables()

	v.Set("key", "value")
	v.Reset()

	_, ok := v.Get("key")
	if ok {
		t.Error("expected key to be removed after reset")
	}
}

func TestVariables_Replace(t *testing.T) {
	v := NewVariables()

	v.Set("user_id", "123")
	v.Set("name", "alice")

	tests := []struct {
		input    string
		expected string
	}{
		{"/users/{{user_id}}", "/users/123"},
		{"/users/{{user_id}}/posts", "/users/123/posts"},
		{"Hello {{name}}!", "Hello alice!"},
		{"{{user_id}} and {{name}}", "123 and alice"},
		{"no variables here", "no variables here"},
		{"{{unknown}} stays", "{{unknown}} stays"},
		{`{"id": "{{user_id}}"}`, `{"id": "123"}`},
	}

	for _, tt := range tests {
		result := v.Replace(tt.input)
		if result != tt.expected {
			t.Errorf("Replace(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGlobalVariables(t *testing.T) {
	ResetGlobalVariables()

	SetVariable("global_key", "global_value")

	val, ok := GetVariable("global_key")
	if !ok || val != "global_value" {
		t.Errorf("expected 'global_value', got '%s'", val)
	}

	result := ReplaceVariables("/api/{{global_key}}")
	if result != "/api/global_value" {
		t.Errorf("expected '/api/global_value', got '%s'", result)
	}

	ResetGlobalVariables()

	_, ok = GetVariable("global_key")
	if ok {
		t.Error("expected global key to be removed after reset")
	}
}

func TestVariables_UUID(t *testing.T) {
	v := NewVariables()

	result := v.Replace("id: {{uuid}}")

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	uuidPattern := regexp.MustCompile(`^id: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(result) {
		t.Errorf("expected UUID format, got %q", result)
	}

	// Each call should generate a different UUID
	result2 := v.Replace("{{uuid}}")
	result3 := v.Replace("{{uuid}}")
	if result2 == result3 {
		t.Error("expected different UUIDs for each call")
	}
}

func TestVariables_Timestamp(t *testing.T) {
	v := NewVariables()

	before := time.Now().UTC()
	result := v.Replace("{{timestamp}}")
	after := time.Now().UTC()

	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("failed to parse timestamp %q: %v", result, err)
	}

	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("timestamp %v not within expected range [%v, %v]", parsed, before, after)
	}
}

func TestVariables_TimestampUnix(t *testing.T) {
	v := NewVariables()

	before := time.Now().Unix()
	result := v.Replace("{{timestamp:unix}}")
	after := time.Now().Unix()

	unix, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		t.Fatalf("failed to parse unix timestamp %q: %v", result, err)
	}

	if unix < before || unix > after {
		t.Errorf("unix timestamp %d not within expected range [%d, %d]", unix, before, after)
	}
}

func TestVariables_Random(t *testing.T) {
	v := NewVariables()

	tests := []struct {
		input   string
		length  int
		numeric bool
	}{
		{"{{random:10}}", 10, false},
		{"{{random:5}}", 5, false},
		{"{{random:20}}", 20, false},
		{"{{random:8:numeric}}", 8, true},
		{"{{random:12:numeric}}", 12, true},
	}

	for _, tt := range tests {
		result := v.Replace(tt.input)

		if len(result) != tt.length {
			t.Errorf("Replace(%q): expected length %d, got %d (%q)", tt.input, tt.length, len(result), result)
			continue
		}

		if tt.numeric {
			if _, err := strconv.ParseInt(result, 10, 64); err != nil {
				t.Errorf("Replace(%q): expected numeric string, got %q", tt.input, result)
			}
		} else {
			alphanumeric := regexp.MustCompile(`^[A-Za-z0-9]+$`)
			if !alphanumeric.MatchString(result) {
				t.Errorf("Replace(%q): expected alphanumeric string, got %q", tt.input, result)
			}
		}
	}

	// Each call should generate different values
	r1 := v.Replace("{{random:20}}")
	r2 := v.Replace("{{random:20}}")
	if r1 == r2 {
		t.Error("expected different random values for each call")
	}
}

func TestVariables_RandomInvalid(t *testing.T) {
	v := NewVariables()

	tests := []string{
		"{{random:}}",
		"{{random:abc}}",
		"{{random:0}}",
		"{{random:-5}}",
	}

	for _, input := range tests {
		result := v.Replace(input)
		if result != input {
			t.Errorf("Replace(%q): expected unchanged input, got %q", input, result)
		}
	}
}

func TestVariables_Sequence(t *testing.T) {
	v := NewVariables()

	// First sequence should start at 1
	result := v.Replace("{{sequence:order}}")
	if result != "1" {
		t.Errorf("expected '1', got %q", result)
	}

	// Second call should increment
	result = v.Replace("{{sequence:order}}")
	if result != "2" {
		t.Errorf("expected '2', got %q", result)
	}

	// Third call should increment
	result = v.Replace("{{sequence:order}}")
	if result != "3" {
		t.Errorf("expected '3', got %q", result)
	}

	// Different sequence name should start fresh
	result = v.Replace("{{sequence:user}}")
	if result != "1" {
		t.Errorf("expected '1' for new sequence, got %q", result)
	}

	// Original sequence should continue
	result = v.Replace("{{sequence:order}}")
	if result != "4" {
		t.Errorf("expected '4', got %q", result)
	}
}

func TestVariables_SequenceReset(t *testing.T) {
	v := NewVariables()

	v.Replace("{{sequence:test}}")
	v.Replace("{{sequence:test}}")

	v.Reset()

	// After reset, sequence should start from 1 again
	result := v.Replace("{{sequence:test}}")
	if result != "1" {
		t.Errorf("expected '1' after reset, got %q", result)
	}
}

func TestVariables_DynamicInTemplate(t *testing.T) {
	v := NewVariables()
	v.Set("name", "test")

	// Mix of stored variables and dynamic values
	result := v.Replace(`{"name": "{{name}}", "id": "{{uuid}}", "seq": {{sequence:item}}}`)

	// Should contain stored variable value
	if !regexp.MustCompile(`"name": "test"`).MatchString(result) {
		t.Errorf("expected name to be replaced, got %q", result)
	}

	// Should contain UUID
	if !regexp.MustCompile(`"id": "[0-9a-f-]{36}"`).MatchString(result) {
		t.Errorf("expected UUID to be replaced, got %q", result)
	}

	// Should contain sequence
	if !regexp.MustCompile(`"seq": 1`).MatchString(result) {
		t.Errorf("expected sequence to be replaced, got %q", result)
	}
}
