//go:build integration

package main

import (
	"os"
	"testing"
)

// TestMainWithCoverage runs the main function for integration test coverage.
// Build with: go test -coverpkg=./... -c -tags integration -o tomato.test
// Run with: ./tomato.test -test.run "^TestMainWithCoverage$" -test.coverprofile=coverage.out run tests/tomato.yml
func TestMainWithCoverage(t *testing.T) {
	// Skip if no args provided (running as unit test)
	if len(os.Args) < 2 {
		t.Skip("No arguments provided, skipping integration test")
	}

	// Run main - coverage will be collected
	main()
}
