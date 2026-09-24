package handler

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestStepHandler_NotDeprecatedIsUnchanged(t *testing.T) {
	called := false
	step := StepDef{Handler: func(name string) error { called = true; return nil }}

	h, ok := step.stepHandler(`^"api" sends$`).(func(string) error)
	if !ok {
		t.Fatalf("handler type changed: %T", step.stepHandler(`^"api" sends$`))
	}
	if err := h("x"); err != nil || !called {
		t.Fatalf("handler not called correctly: err=%v called=%v", err, called)
	}
}

func TestStepHandler_DeprecatedWarnsOnceAndStillRuns(t *testing.T) {
	// "Once per run" is process-wide state; start clean so the test also
	// passes when repeated (-count, -cpu).
	warnedDeprecated = sync.Map{}
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	pattern := `^"api" old step "([^"]*)"$`
	var got []string
	step := StepDef{
		Deprecated: `use "api" new step instead`,
		Handler: func(name string) error {
			got = append(got, name)
			return nil
		},
	}

	h, ok := step.stepHandler(pattern).(func(string) error)
	if !ok {
		t.Fatalf("wrapper must keep the handler signature for godog, got %T", step.stepHandler(pattern))
	}
	for _, arg := range []string{"a", "b"} {
		if err := h(arg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if strings.Join(got, ",") != "a,b" {
		t.Errorf("deprecated step must still run, got calls %v", got)
	}
	if n := strings.Count(buf.String(), "deprecated step"); n != 1 {
		t.Errorf("expected exactly one warning, got %d: %s", n, buf.String())
	}
	if !strings.Contains(buf.String(), `use \"api\" new step instead`) {
		t.Errorf("warning should carry the replacement hint, got %s", buf.String())
	}
}
