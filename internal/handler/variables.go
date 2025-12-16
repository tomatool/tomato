package handler

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Variables is a thread-safe store for captured values that can be used across steps
type Variables struct {
	mu        sync.RWMutex
	values    map[string]string
	sequences map[string]int
}

// Global variable store - reset between scenarios
var globalVariables = NewVariables()

// NewVariables creates a new variable store
func NewVariables() *Variables {
	return &Variables{
		values:    make(map[string]string),
		sequences: make(map[string]int),
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

// Reset clears all stored variables and sequences
func (v *Variables) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values = make(map[string]string)
	v.sequences = make(map[string]int)
}

// Replace substitutes all {{variable}} placeholders in the input string
// with their stored values or generated dynamic values
//
// Supported dynamic values:
//   - {{uuid}} - Random UUID v4
//   - {{timestamp}} - Current ISO 8601 timestamp
//   - {{timestamp:unix}} - Unix timestamp in seconds
//   - {{random:N}} - Random alphanumeric string of length N
//   - {{random:N:numeric}} - Random numeric string of length N
//   - {{sequence:name}} - Auto-incrementing sequence by name
func (v *Variables) Replace(input string) string {
	// Match {{variable_name}} pattern
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from {{name}}
		varName := strings.TrimPrefix(strings.TrimSuffix(match, "}}"), "{{")

		// Check for built-in dynamic values first
		if generated, ok := v.generateDynamicValue(varName); ok {
			return generated
		}

		// Look up stored variable
		v.mu.RLock()
		defer v.mu.RUnlock()
		if val, ok := v.values[varName]; ok {
			return val
		}

		// Return original if not found
		return match
	})
}

// generateDynamicValue handles built-in dynamic value generation
func (v *Variables) generateDynamicValue(name string) (string, bool) {
	switch {
	case name == "uuid":
		return uuid.New().String(), true

	case name == "timestamp":
		return time.Now().UTC().Format(time.RFC3339), true

	case name == "timestamp:unix":
		return strconv.FormatInt(time.Now().Unix(), 10), true

	case strings.HasPrefix(name, "random:"):
		return v.generateRandom(name)

	case strings.HasPrefix(name, "sequence:"):
		return v.generateSequence(name)

	default:
		return "", false
	}
}

// generateRandom generates random strings
// Formats: random:N or random:N:numeric
func (v *Variables) generateRandom(name string) (string, bool) {
	parts := strings.Split(name, ":")
	if len(parts) < 2 {
		return "", false
	}

	length, err := strconv.Atoi(parts[1])
	if err != nil || length <= 0 {
		return "", false
	}

	var charset string
	if len(parts) >= 3 && parts[2] == "numeric" {
		charset = "0123456789"
	} else {
		charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	}

	result := make([]byte, length)
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", false
	}

	for i := 0; i < length; i++ {
		result[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return string(result), true
}

// generateSequence generates auto-incrementing sequences by name
func (v *Variables) generateSequence(name string) (string, bool) {
	parts := strings.Split(name, ":")
	if len(parts) < 2 {
		return "", false
	}

	seqName := parts[1]

	v.mu.Lock()
	defer v.mu.Unlock()

	v.sequences[seqName]++
	return fmt.Sprintf("%d", v.sequences[seqName]), true
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
