package pdffixture

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the committed fixtures under tests/testdata/pdf")

// committed maps each generator to the file the integration suite reads from
// disk and uploads to MinIO.
var committed = map[string]func() []byte{
	"contract.pdf":   Contract,
	"accessible.pdf": Accessible,
}

const testdata = "../../tests/testdata/pdf"

// TestFixturesAreDeterministic: a test compares bytes against these, and the
// committed copies are checked against them, so two runs must agree.
func TestFixturesAreDeterministic(t *testing.T) {
	for name, gen := range committed {
		if !bytes.Equal(gen(), gen()) {
			t.Errorf("%s differs between two generations", name)
		}
	}
}

// TestCommittedFixturesMatchGenerator fails when the generator changed and
// the files were not regenerated: go test ./internal/pdffixture -update.
func TestCommittedFixturesMatchGenerator(t *testing.T) {
	for name, gen := range committed {
		path := filepath.Join(testdata, name)
		want := gen()
		if *update {
			if err := os.MkdirAll(testdata, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v; run: go test ./internal/pdffixture -update", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run: go test ./internal/pdffixture -update", path)
		}
	}
}

func TestFixturesLookLikePDFs(t *testing.T) {
	for name, gen := range committed {
		b := gen()
		if !bytes.HasPrefix(b, []byte("%PDF-1.")) || !bytes.HasSuffix(bytes.TrimSpace(b), []byte("%%EOF")) {
			t.Errorf("%s does not start with a PDF header and end with %%%%EOF", name)
		}
	}
}
