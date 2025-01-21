package compare

import (
	"testing"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name         string
		a            []byte
		b            []byte
		expectError  bool   // do we expect any mismatches / errors in this comparison?
		wantInOutput string // a substring we expect to appear in the output if there's a mismatch
	}{
		{
			name: "Exact Match - simple object",
			a:    []byte(`{"key":"value"}`),
			b:    []byte(`{"key":"value"}`),
			// No mismatch => expectError=false, no particular substring to check
			expectError: false,
		},
		{
			name:        "Wildcard Match - single field",
			a:           []byte(`{"key":"someValue"}`),
			b:           []byte(`{"key":"*"}`), // wildcard for `key`
			expectError: false,
		},
		{
			name: "Wildcard Nested Match",
			a: []byte(`{
                "name": "Alice",
                "nested": {"foo": "bar"}
            }`),
			b: []byte(`{
                "name": "*",
                "nested": {"foo":"bar"}
            }`),
			expectError: false,
		},
		{
			name:         "Mismatch - different string values",
			a:            []byte(`{"key":"value1"}`),
			b:            []byte(`{"key":"value2"}`),
			expectError:  true,
			wantInOutput: "value mismatch",
		},
		{
			name:         "Mismatch - key missing in a",
			a:            []byte(`{"onlyInA":"valA"}`),
			b:            []byte(`{"onlyInB":"valB"}`),
			expectError:  true,
			wantInOutput: "key missing in JSON a",
		},
		{
			name:         "Type mismatch (string vs. int)",
			a:            []byte(`{"count": 123}`),
			b:            []byte(`{"count": "123"}`),
			expectError:  true,
			wantInOutput: "type mismatch",
		},
		{
			name:        "Slice exact match",
			a:           []byte(`{"items":[1,2,3]}`),
			b:           []byte(`{"items":[1,2,3]}`),
			expectError: false,
		},
		{
			name:         "Slice mismatch length",
			a:            []byte(`{"items":[1,2,3]}`),
			b:            []byte(`{"items":[1,2]}`),
			expectError:  true,
			wantInOutput: "slice length mismatch",
		},
		{
			name:        "Slice wildcard element",
			a:           []byte(`{"items":["chocolate","music"]}`),
			b:           []byte(`{"items":["chocolate","*"]}`),
			expectError: false,
		},
		{
			name: "Nested mismatch at deeper key",
			a: []byte(`{
                "level1": {
                    "level2": {
                        "level3": "abc"
                    }
                }
            }`),
			b: []byte(`{
                "level1": {
                    "level2": {
                        "level3": "xyz"
                    }
                }
            }`),
			expectError:  true,
			wantInOutput: "value mismatch",
		},
		{
			name: "Extra key in A (optional mismatch if you enable extra-key detection)",
			a: []byte(`{
                "keyA": "valA",
                "extra": "valExtra"
            }`),
			b: []byte(`{
                "keyA": "valA"
            }`),
			// If you enable the “extra key in JSON a” check, this might cause a mismatch.
			// By default, the sample doesn't treat extra keys as mismatches, so set expectError accordingly.
			expectError: false, // or true, if you've uncommented the extra keys check
			// wantInOutput: "extra key" // Only if that check is enabled
		},
		{
			name: "Non-JSON input in a",
			a:    []byte(`{invalid...}`),
			b:    []byte(`{"key":"value"}`),
			// This should produce an unmarshal error
			expectError:  true,
			wantInOutput: "failed to unmarshal JSON a",
		},
		{
			name:         "Non-JSON input in b",
			a:            []byte(`{"key":"value"}`),
			b:            []byte(`{invalid...}`),
			expectError:  true,
			wantInOutput: "failed to unmarshal JSON b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comp, err := JSON(tc.a, tc.b, false) // using colorize=false for tests
			gotError := (err != nil || comp.errorPrefix != "")

			if tc.expectError && !gotError {
				t.Fatalf("expected mismatch/error, but got none. Output:\n%s", comp.output)
			}
			if !tc.expectError && gotError {
				t.Fatalf("did not expect mismatch/error, but got one. errorPrefix: %q, output:\n%s",
					comp.errorPrefix, comp.output)
			}

			if tc.wantInOutput != "" && !contains(comp.output, tc.wantInOutput) && !contains(comp.errorPrefix, tc.wantInOutput) {
				t.Errorf("expected output to contain %q, got:\nerrorPrefix: %q\noutput:\n%s",
					tc.wantInOutput, comp.errorPrefix, comp.output)
			}
		})
	}
}

// contains is a helper function for substring checks
func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(needle) > 0 && len(haystack) >= len(needle) &&
		// simple substring match
		func() bool {
			return (len(needle) > 0 &&
				(string([]rune(haystack)[0:len(needle)]) == needle ||
					(len(haystack) > len(needle) && contains(haystack[1:], needle))))
		}())
}
