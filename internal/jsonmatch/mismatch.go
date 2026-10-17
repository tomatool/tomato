package jsonmatch

import "fmt"

// Mode is how a step compared what it expected with what it got. It decides
// which differences are the mistake: in a subset match, keys present only in
// the actual value are allowed, so they are not.
type Mode string

const (
	// ModeText compares the two values as text, byte for byte after trimming.
	ModeText Mode = "text"
	// ModeJSON compares two JSON documents exactly: every key on both sides,
	// with Compare's "@" matchers where the step supports them.
	ModeJSON Mode = "json"
	// ModeSubset compares two JSON documents with keys in the actual value
	// that the expected one does not name ignored (Compare with partial).
	ModeSubset Mode = "subset"
)

// Mismatch is a failed comparison that keeps both sides, so a reader that
// wants more than the message — the web UI's diff — does not have to parse it
// back out of the text. Error returns exactly the text the step reported
// before Mismatch existed, so console output and reports do not change.
type Mismatch struct {
	// What was compared: "message", "body", "stdout", "JSON".
	What     string
	Expected string
	Actual   string
	Mode     Mode
	// NoColon spells the heading "<What> mismatch" without the trailing
	// colon, as the shell resource always has.
	NoColon bool
	// Cause, when set, is the comparison error the step reports — the
	// "at status: expected created, got pending" of Compare — and Error
	// returns its text instead of the "expected:/actual:" form.
	Cause error
}

// Error returns "<What> mismatch:\nexpected: X\nactual: Y", or Cause's text
// when there is one.
func (m *Mismatch) Error() string {
	if m.Cause != nil {
		return m.Cause.Error()
	}
	colon := ":"
	if m.NoColon {
		colon = ""
	}
	return fmt.Sprintf("%s mismatch%s\nexpected: %s\nactual: %s", m.What, colon, m.Expected, m.Actual)
}

// Unwrap returns Cause.
func (m *Mismatch) Unwrap() error { return m.Cause }

// CompareDocs runs Compare and, when it fails, returns its error as a
// Mismatch carrying both documents as text, so the failure can be shown as a
// diff. The error text is Compare's, unchanged.
func CompareDocs(what string, expected, actual any, expectedText, actualText string, partial bool) error {
	err := Compare(expected, actual, "", partial)
	if err == nil {
		return nil
	}
	mode := ModeJSON
	if partial {
		mode = ModeSubset
	}
	return &Mismatch{What: what, Expected: expectedText, Actual: actualText, Mode: mode, Cause: err}
}
