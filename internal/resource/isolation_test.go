package resource

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestResourcesDoNotImportEachOther is the point of the one-package-per-resource
// layout: a contributor working on Kafka should not be able to break Postgres.
//
// Before the split, every resource lived in one package, so the coupling was
// invisible — the JSON matcher sat inside the HTTP client's file and kafka,
// grpc and s3 all called into it, and a polling helper lived on a
// websocket-specific timeout that kafka and rabbitmq silently inherited.
// Shared code belongs in this package or in internal/jsonmatch, where it is
// named and tested as shared; it does not belong in a sibling resource.
func TestResourcesDoNotImportEachOther(t *testing.T) {
	const prefix = "github.com/tomatool/tomato/internal/resource/"

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		self := prefix + e.Name()
		files, err := filepath.Glob(filepath.Join(e.Name(), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			for _, imp := range f.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				if strings.HasPrefix(path, prefix) && path != self {
					t.Errorf("%s imports %s: a resource must not depend on another resource. "+
						"Move what they share into internal/resource or internal/jsonmatch.", file, path)
				}
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no resource packages found; this test is not looking where it thinks it is")
	}
}
