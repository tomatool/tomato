package handler

import (
	"regexp"
	"strings"
	"sync"
)

// Variables is a thread-safe store for captured values that can be used across steps
type Variables struct {
	mu     sync.RWMutex
	values map[string]string
}

// Global variable store - reset between scenarios
var globalVariables = NewVariables()

// NewVariables creates a new variable store
func NewVariables() *Variables {
	return &Variables{
		values: make(map[string]string),
	}
}

// Set stores a value with the given key
func (v *Variables) Set(key, value string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[key] = value
}

// Get retrieves a value by key
func (v *Variables) Get(key string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	val, ok := v.values[key]
	return val, ok
}

// Reset clears all stored variables
func (v *Variables) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values = make(map[string]string)
}

// Replace substitutes all {{variable}} placeholders in the input string
// with their stored values
func (v *Variables) Replace(input string) string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Match {{variable_name}} pattern
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from {{name}}
		varName := strings.TrimPrefix(strings.TrimSuffix(match, "}}"), "{{")
		if val, ok := v.values[varName]; ok {
			return val
		}
		// Return original if not found
		return match
	})
}

// GetGlobalVariables returns the global variable store
func GetGlobalVariables() *Variables {
	return globalVariables
}

// ResetGlobalVariables clears all global variables
func ResetGlobalVariables() {
	globalVariables.Reset()
}

// SetVariable sets a variable in the global store
func SetVariable(key, value string) {
	globalVariables.Set(key, value)
}

// GetVariable gets a variable from the global store
func GetVariable(key string) (string, bool) {
	return globalVariables.Get(key)
}

// ReplaceVariables replaces {{var}} placeholders in a string
func ReplaceVariables(input string) string {
	return globalVariables.Replace(input)
}
