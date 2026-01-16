package handler

import "testing"

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
