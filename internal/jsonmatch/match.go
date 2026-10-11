// Package jsonmatch compares JSON values for the step assertions that need it.
//
// Several resources assert on JSON-shaped payloads — an HTTP response body, a
// gRPC reply, a Kafka message, an S3 object — and they all want the same two
// things: walk a dotted path into a document, and compare an expected value
// against an actual one with tomato's "@" matchers. Keeping that here means
// there is one definition of what "@regex:" or a partial match means, rather
// than one per resource that can drift.
package jsonmatch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Compare compares expected and actual JSON values.
// If partial is true, extra keys in actual objects are ignored (contains mode).
// If partial is false, objects must match exactly (matches mode).
// Exported for testing.
func Compare(expected, actual interface{}, path string, partial bool) error {
	switch e := expected.(type) {
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("at %s: expected object, got %T", path, actual)
		}
		// In non-partial mode, check for extra keys
		if !partial {
			for key := range a {
				if _, exists := e[key]; !exists {
					keyPath := key
					if path != "" {
						keyPath = path + "." + key
					}
					return fmt.Errorf("at %s: unexpected key %q", keyPath, key)
				}
			}
		}
		for key, val := range e {
			newPath := path + "." + key
			if path == "" {
				newPath = key
			}
			actualVal, exists := a[key]
			if !exists {
				return fmt.Errorf("at %s: key %q not found", path, key)
			}
			if err := Compare(val, actualVal, newPath, partial); err != nil {
				return err
			}
		}
	case []interface{}:
		a, ok := actual.([]interface{})
		if !ok {
			return fmt.Errorf("at %s: expected array, got %T", path, actual)
		}
		if len(e) != len(a) {
			return fmt.Errorf("at %s: expected array length %d, got %d", path, len(e), len(a))
		}
		for i, val := range e {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := Compare(val, a[i], newPath, partial); err != nil {
				return err
			}
		}
	default:
		if str, ok := expected.(string); ok {
			if strings.HasPrefix(str, "@") {
				return Special(str, actual, path)
			}
		}
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			return fmt.Errorf("at %s: expected %v, got %v", path, expected, actual)
		}
	}
	return nil
}

// Special handles special matchers like @string, @regex:pattern, etc.
// Exported for testing.
func Special(matcher string, actual interface{}, path string) error {
	// Handle parameterized matchers first
	if strings.HasPrefix(matcher, "@regex:") {
		pattern := strings.TrimPrefix(matcher, "@regex:")
		return matchRegex(pattern, actual, path)
	}
	if strings.HasPrefix(matcher, "@contains:") {
		substr := strings.TrimPrefix(matcher, "@contains:")
		return matchContains(substr, actual, path)
	}
	if strings.HasPrefix(matcher, "@startswith:") {
		prefix := strings.TrimPrefix(matcher, "@startswith:")
		return matchStartsWith(prefix, actual, path)
	}
	if strings.HasPrefix(matcher, "@endswith:") {
		suffix := strings.TrimPrefix(matcher, "@endswith:")
		return matchEndsWith(suffix, actual, path)
	}
	if strings.HasPrefix(matcher, "@gt:") {
		value := strings.TrimPrefix(matcher, "@gt:")
		return matchGreaterThan(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@gte:") {
		value := strings.TrimPrefix(matcher, "@gte:")
		return matchGreaterThanOrEqual(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@lt:") {
		value := strings.TrimPrefix(matcher, "@lt:")
		return matchLessThan(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@lte:") {
		value := strings.TrimPrefix(matcher, "@lte:")
		return matchLessThanOrEqual(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@len:") {
		value := strings.TrimPrefix(matcher, "@len:")
		return matchLength(value, actual, path)
	}

	// Handle simple type matchers
	switch matcher {
	case "@string":
		if _, ok := actual.(string); !ok {
			return fmt.Errorf("at %s: expected string, got %T", path, actual)
		}
	case "@number":
		if _, ok := actual.(float64); !ok {
			return fmt.Errorf("at %s: expected number, got %T", path, actual)
		}
	case "@boolean":
		if _, ok := actual.(bool); !ok {
			return fmt.Errorf("at %s: expected boolean, got %T", path, actual)
		}
	case "@array":
		if _, ok := actual.([]interface{}); !ok {
			return fmt.Errorf("at %s: expected array, got %T", path, actual)
		}
	case "@object":
		if _, ok := actual.(map[string]interface{}); !ok {
			return fmt.Errorf("at %s: expected object, got %T", path, actual)
		}
	case "@any":
		// Always matches
	case "@null":
		if actual != nil {
			return fmt.Errorf("at %s: expected null, got %v", path, actual)
		}
	case "@notnull":
		if actual == nil {
			return fmt.Errorf("at %s: expected non-null value", path)
		}
	case "@empty":
		return matchEmpty(actual, path)
	case "@notempty":
		return matchNotEmpty(actual, path)
	default:
		return fmt.Errorf("unknown matcher: %s", matcher)
	}
	return nil
}

func matchRegex(pattern string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @regex requires string value, got %T", path, actual)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("at %s: invalid regex pattern %q: %w", path, pattern, err)
	}
	if !re.MatchString(str) {
		return fmt.Errorf("at %s: value %q does not match pattern %q", path, str, pattern)
	}
	return nil
}

func matchContains(substr string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @contains requires string value, got %T", path, actual)
	}
	if !strings.Contains(str, substr) {
		return fmt.Errorf("at %s: value %q does not contain %q", path, str, substr)
	}
	return nil
}

func matchStartsWith(prefix string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @startswith requires string value, got %T", path, actual)
	}
	if !strings.HasPrefix(str, prefix) {
		return fmt.Errorf("at %s: value %q does not start with %q", path, str, prefix)
	}
	return nil
}

func matchEndsWith(suffix string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @endswith requires string value, got %T", path, actual)
	}
	if !strings.HasSuffix(str, suffix) {
		return fmt.Errorf("at %s: value %q does not end with %q", path, str, suffix)
	}
	return nil
}

func matchGreaterThan(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @gt requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @gt requires numeric actual value, got %T", path, actual)
	}
	if num <= expected {
		return fmt.Errorf("at %s: expected value > %v, got %v", path, expected, num)
	}
	return nil
}

func matchGreaterThanOrEqual(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @gte requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @gte requires numeric actual value, got %T", path, actual)
	}
	if num < expected {
		return fmt.Errorf("at %s: expected value >= %v, got %v", path, expected, num)
	}
	return nil
}

func matchLessThan(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @lt requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @lt requires numeric actual value, got %T", path, actual)
	}
	if num >= expected {
		return fmt.Errorf("at %s: expected value < %v, got %v", path, expected, num)
	}
	return nil
}

func matchLessThanOrEqual(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @lte requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @lte requires numeric actual value, got %T", path, actual)
	}
	if num > expected {
		return fmt.Errorf("at %s: expected value <= %v, got %v", path, expected, num)
	}
	return nil
}

func matchLength(value string, actual interface{}, path string) error {
	expected, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("at %s: @len requires integer value: %w", path, err)
	}

	var actualLen int
	switch v := actual.(type) {
	case string:
		actualLen = len(v)
	case []interface{}:
		actualLen = len(v)
	case map[string]interface{}:
		actualLen = len(v)
	default:
		return fmt.Errorf("at %s: @len requires string, array, or object, got %T", path, actual)
	}

	if actualLen != expected {
		return fmt.Errorf("at %s: expected length %d, got %d", path, expected, actualLen)
	}
	return nil
}

func matchEmpty(actual interface{}, path string) error {
	switch v := actual.(type) {
	case string:
		if v != "" {
			return fmt.Errorf("at %s: expected empty string, got %q", path, v)
		}
	case []interface{}:
		if len(v) != 0 {
			return fmt.Errorf("at %s: expected empty array, got %d elements", path, len(v))
		}
	case map[string]interface{}:
		if len(v) != 0 {
			return fmt.Errorf("at %s: expected empty object, got %d keys", path, len(v))
		}
	case nil:
		// nil is considered empty
	default:
		return fmt.Errorf("at %s: @empty requires string, array, object, or null, got %T", path, actual)
	}
	return nil
}

func matchNotEmpty(actual interface{}, path string) error {
	switch v := actual.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("at %s: expected non-empty string", path)
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("at %s: expected non-empty array", path)
		}
	case map[string]interface{}:
		if len(v) == 0 {
			return fmt.Errorf("at %s: expected non-empty object", path)
		}
	case nil:
		return fmt.Errorf("at %s: expected non-empty value, got null", path)
	default:
		return fmt.Errorf("at %s: @notempty requires string, array, object, or null, got %T", path, actual)
	}
	return nil
}
