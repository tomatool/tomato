package shell

import (
	"errors"
	"testing"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/jsonmatch"
)

// `stdout is:` fails with the text it always printed, now carried by a
// Mismatch the UI can draw as a diff.
func TestStdoutShouldBeFailsWithAMismatch(t *testing.T) {
	s := newTestShell(t, nil)
	if err := s.executeCommand(`echo '{"status":"pending"}'`, s.timeout); err != nil {
		t.Fatalf("executeCommand: %v", err)
	}
	err := s.stdoutShouldBe(&godog.DocString{Content: "\n{\"status\":\"created\"}\n"})
	want := "stdout mismatch\nexpected: {\"status\":\"created\"}\nactual: {\"status\":\"pending\"}"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	var m *jsonmatch.Mismatch
	if !errors.As(err, &m) || m.Mode != jsonmatch.ModeText || m.What != "stdout" ||
		m.Expected != `{"status":"created"}` || m.Actual != `{"status":"pending"}` {
		t.Errorf("mismatch = %+v", m)
	}
}
