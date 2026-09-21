package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// jsonPathValue walks a dotted path into a JSON document and returns the
// value at it.
//
// Paths are dotted, with bracketed indexes for arrays: "data.items[0].id".
// A leading index ("[0].id") indexes the document itself.
//
// This lives at package level rather than on a handler because every
// resource that gets a JSON-shaped response back wants the same traversal,
// and a second copy is a second place for the array-index handling to drift.
func jsonPathValue(body []byte, path string) (any, error) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	current := data
	for _, part := range strings.Split(path, ".") {
		if idx := strings.Index(part, "["); idx != -1 {
			key := part[:idx]
			indexStr := part[idx+1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s", indexStr)
			}

			if key != "" {
				obj, ok := current.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("expected object at %s", key)
				}
				current = obj[key]
			}

			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("expected array at %s", part)
			}
			if index >= len(arr) {
				return nil, fmt.Errorf("array index out of bounds: %d", index)
			}
			current = arr[index]
			continue
		}

		obj, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object at %s", part)
		}
		var exists bool
		current, exists = obj[part]
		if !exists {
			return nil, fmt.Errorf("key not found: %s", part)
		}
	}

	return current, nil
}
