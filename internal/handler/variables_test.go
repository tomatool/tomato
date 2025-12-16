package handler

import (
	"testing"
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
