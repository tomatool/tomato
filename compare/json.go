package compare

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// JSON compares JSON in `a` against `b`.
// If a key in `b` has value "*", that key automatically "matches" any value in `a`.
func JSON(a, b []byte, colorize bool) (Comparison, error) {
	var objA, objB interface{}

	// Unmarshal JSON a -> objA
	if err := json.Unmarshal(a, &objA); err != nil {
		return Comparison{}, fmt.Errorf("failed to unmarshal JSON a: %w", err)
	}
	// Unmarshal JSON b -> objB
	if err := json.Unmarshal(b, &objB); err != nil {
		return Comparison{}, fmt.Errorf("failed to unmarshal JSON b: %w", err)
	}

	// Normalize the JSON structures (e.g. decode numbers into float64 consistently, etc.)
	normalizedA := normalize(objA)
	normalizedB := normalize(objB)

	// Compare the two structures and track mismatches.
	mismatches := compareValues("", normalizedA, normalizedB, colorize)

	var comp Comparison
	if len(mismatches) == 0 {
		// Everything matched

		// comp.output = color(fmt.Sprintf("✓ JSON matches\n"), colorize, "green")
	} else {
		// There were mismatches
		comp.errorPrefix = color("MISMATCHES FOUND:", colorize, "red")
		comp.output = joinLines(mismatches)
	}

	return comp, nil
}

// normalize helps convert all JSON-decoded structures into map[string]interface{},
// []interface{}, float64, string, bool, or nil for consistency.
func normalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Normalize each entry in the map
		m := make(map[string]interface{})
		for k, subVal := range val {
			m[k] = normalize(subVal)
		}
		return m
	case []interface{}:
		// Normalize each element in the slice
		arr := make([]interface{}, len(val))
		for i, subVal := range val {
			arr[i] = normalize(subVal)
		}
		return arr
	default:
		// string, float64, bool, nil remain as is
		return val
	}
}

// compareValues recursively compares a vs b.
// If b is the string "*", it's considered a wildcard => always matches.
// Returns a list of mismatch descriptions (empty if all is good).
func compareValues(path string, a, b interface{}, colorize bool) []string {
	// If b is literally "*", we ignore a's value and pass.
	if str, ok := b.(string); ok && str == "*" {
		return nil
	}

	// Type-check
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)

	if typeA != typeB {
		return []string{
			color(fmt.Sprintf("%s: type mismatch (got %T vs %T)", path, a, b),
				colorize, "red"),
		}
	}

	switch valB := b.(type) {
	// Compare map => map
	case map[string]interface{}:
		valA := a.(map[string]interface{})
		return compareMaps(path, valA, valB, colorize)
	// Compare slice => slice
	case []interface{}:
		valA := a.([]interface{})
		return compareSlices(path, valA, valB, colorize)
	// Compare primitive values (string, float64, bool, nil)
	default:
		if !reflect.DeepEqual(a, b) {
			return []string{
				color(fmt.Sprintf("%s: value mismatch (got %v vs %v)", path, a, b),
					colorize, "red"),
			}
		}
	}

	return nil
}

func compareMaps(path string, a, b map[string]interface{}, colorize bool) []string {
	mismatches := make([]string, 0)

	// Check all keys in b
	for k, valB := range b {
		newPath := makePath(path, k)
		valA, ok := a[k]
		if !ok {
			// Key is missing in a
			mismatches = append(mismatches,
				color(fmt.Sprintf("%s: key missing in JSON a", newPath), colorize, "red"))
			continue
		}
		// Compare recursively
		subMismatches := compareValues(newPath, valA, valB, colorize)
		mismatches = append(mismatches, subMismatches...)
	}

	// If you also want to detect extra keys in `a` that are not in `b`,
	// you can loop over `a`'s keys here:
	/*
	   for k := range a {
	       if _, ok := b[k]; !ok {
	           newPath := makePath(path, k)
	           mismatches = append(mismatches,
	               color(fmt.Sprintf("%s: extra key in JSON a not in b", newPath), colorize, "yellow"))
	       }
	   }
	*/

	return mismatches
}

func compareSlices(path string, a, b []interface{}, colorize bool) []string {
	mismatches := make([]string, 0)

	// Compare length first (if you want an exact match in slice length)
	if len(a) != len(b) {
		mismatches = append(mismatches,
			color(fmt.Sprintf("%s: slice length mismatch (got %d vs %d)",
				path, len(a), len(b)), colorize, "red"))
		// Optionally return here or keep comparing smaller range
		// return mismatches
	}

	// Compare element by element (up to the min length)
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		newPath := fmt.Sprintf("%s[%d]", path, i)
		subMismatches := compareValues(newPath, a[i], b[i], colorize)
		mismatches = append(mismatches, subMismatches...)
	}

	return mismatches
}

// makePath helper to build a dotted path notation
func makePath(base, next string) string {
	if base == "" {
		return next
	}
	return base + "." + next
}

// color applies ANSI color codes if colorize is true.
func color(text string, colorize bool, colorName string) string {
	if !colorize {
		return text
	}

	var code string
	switch colorName {
	case "red":
		code = "\033[31m"
	case "green":
		code = "\033[32m"
	case "yellow":
		code = "\033[33m"
	default:
		code = "\033[0m"
	}
	reset := "\033[0m"
	return code + text + reset
}

// joinLines helper to turn a slice of strings into a single output
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
