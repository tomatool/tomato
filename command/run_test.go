package command

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tomato exits right after the run returns. The tee used to hand back before
// its pipe was drained, and a CI run lost the summary of a nested tomato run.
func TestTeeStdout_RestoreDeliversEverythingWritten(t *testing.T) {
	console, err := os.Create(filepath.Join(t.TempDir(), "console"))
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close()
	origStdout := os.Stdout
	os.Stdout = console
	defer func() { os.Stdout = origStdout }()

	var log bytes.Buffer
	restore := teeStdout(&log)
	line := strings.Repeat("x", 99) + "\n"
	for i := 0; i < 20000; i++ { // 2 MB, far more than a pipe holds
		fmt.Fprint(os.Stdout, line)
	}
	fmt.Fprintln(os.Stdout, "2 scenarios (2 passed)")
	restore()

	want := 20000*len(line) + len("2 scenarios (2 passed)\n")
	if log.Len() != want || !strings.HasSuffix(log.String(), "2 scenarios (2 passed)\n") {
		t.Errorf("the log got %d of %d bytes", log.Len(), want)
	}
	data, err := os.ReadFile(console.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != want {
		t.Errorf("the console got %d of %d bytes", len(data), want)
	}
	if os.Stdout != console {
		t.Error("stdout was not restored")
	}
}
