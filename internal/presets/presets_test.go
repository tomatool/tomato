package presets

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The embedded jar holds a class for every source in kafka/src, compiled for
// Java 17 (class file version 61), the oldest Java Kafka brokers run on. A
// source added or renamed without `make kafka-plugin` fails here; CI's rebuild
// check catches a changed source.
func TestMskIamJar_HoldsEverySourceForJava17(t *testing.T) {
	jar, err := zip.NewReader(bytes.NewReader(MskIamJar), int64(len(MskIamJar)))
	if err != nil {
		t.Fatalf("the embedded jar is not a jar: %v", err)
	}
	classes := map[string]*zip.File{}
	for _, f := range jar.File {
		if strings.HasSuffix(f.Name, ".class") {
			classes[f.Name] = f
		}
	}

	const pkg = "io/github/tomatool/mskiam"
	sources, err := filepath.Glob(filepath.Join("kafka", "src", filepath.FromSlash(pkg), "*.java"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources found: %v", err)
	}
	for _, src := range sources {
		name := pkg + "/" + strings.TrimSuffix(filepath.Base(src), ".java") + ".class"
		f, ok := classes[name]
		if !ok {
			t.Errorf("%s has no class in the jar; run make kafka-plugin", src)
			continue
		}
		if major := classMajorVersion(t, f); major != 61 {
			t.Errorf("%s is class file version %d, want 61 (Java 17)", name, major)
		}
	}
	if len(classes) < len(sources) {
		t.Errorf("%d classes for %d sources", len(classes), len(sources))
	}

	if _, err := os.Stat(filepath.Join("kafka", "Dockerfile")); err != nil {
		t.Errorf("the jar's build is gone: %v", err)
	}
}

func classMajorVersion(t *testing.T, f *zip.File) int {
	t.Helper()
	r, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	header := make([]byte, 8) // magic, minor, major
	if _, err := io.ReadFull(r, header); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(header) != 0xCAFEBABE {
		t.Fatalf("%s is not a class file", f.Name)
	}
	return int(binary.BigEndian.Uint16(header[6:]))
}
