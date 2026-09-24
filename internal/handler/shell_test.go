package handler

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
)

func newTestShell(t *testing.T, opts map[string]interface{}) *Shell {
	t.Helper()
	s, err := NewShell("shell", config.Resource{Type: "shell", Options: opts}, nil)
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func TestShellResetRestoresConfiguredEnvAndWorkDir(t *testing.T) {
	s := newTestShell(t, map[string]interface{}{
		"workdir": "/tmp",
		"env":     map[string]interface{}{"CONFIGURED": "yes"},
	})

	_ = s.setEnvVar("LEAKY", "1")
	_ = s.setEnvVar("CONFIGURED", "overridden")
	_ = s.setWorkDir("/")

	if err := s.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if _, ok := s.env["LEAKY"]; ok {
		t.Errorf("env set by a step leaked past Reset: %v", s.env)
	}
	if got := s.env["CONFIGURED"]; got != "yes" {
		t.Errorf("configured env = %q, want %q", got, "yes")
	}
	if s.workDir != "/tmp" {
		t.Errorf("workDir = %q, want %q", s.workDir, "/tmp")
	}
}

func TestShellOutputAssertions(t *testing.T) {
	s := newTestShell(t, nil)
	if err := s.executeCommand(`echo '{"status": "ok"}'; echo 'warn: "x"' >&2`, s.timeout); err != nil {
		t.Fatalf("executeCommand: %v", err)
	}
	doc := func(c string) *godog.DocString { return &godog.DocString{Content: c} }

	checks := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"stdout contains doc with quotes", s.stdoutShouldContainDoc(doc(`"status": "ok"`)), false},
		{"stdout contains doc missing", s.stdoutShouldContainDoc(doc(`"status": "fail"`)), true},
		{"stdout does not contain doc", s.stdoutShouldNotContainDoc(doc(`"error"`)), false},
		{"stdout does not contain doc present", s.stdoutShouldNotContainDoc(doc(`"ok"`)), true},
		{"stderr contains doc with quotes", s.stderrShouldContainDoc(doc("warn: \"x\"\n")), false},
		{"stderr does not contain", s.stderrShouldNotContain("panic"), false},
		{"stderr does not contain present", s.stderrShouldNotContain("warn"), true},
	}
	for _, c := range checks {
		if (c.err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, c.err, c.wantErr)
		}
	}
}
