package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func newShellExecutor(t *testing.T) *Executor {
	t.Helper()
	reg, err := NewRegistry(map[string]config.Resource{"sh": {Type: "shell"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.WaitReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	e, err := NewExecutor(reg)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func str(s string) *string { return &s }

func TestExecutorRunsStepsByText(t *testing.T) {
	e := newShellExecutor(t)

	steps := []StepCall{
		{Text: `"sh" env "WHO" is "tomato"`},
		{Text: `"sh" runs:`, DocString: str(`echo "hello $WHO"`)},
		{Text: `"sh" exit code is "0"`}, // int argument
		{Text: `"sh" stdout contains "hello tomato"`},
	}
	for _, s := range steps {
		if err := e.Run(s); err != nil {
			t.Fatalf("%s: %v", s.Text, err)
		}
	}
	// A failing assertion comes back as the step's own error.
	err := e.Run(StepCall{Text: `"sh" stdout contains "goodbye"`})
	if err == nil {
		t.Fatal("a failing assertion must return an error")
	}
}

func TestExecutorErrors(t *testing.T) {
	e := newShellExecutor(t)
	cases := map[string]StepCall{
		"undefined step":      {Text: `"sh" does a dance`},
		"unknown resource":    {Text: `"nope" runs "true"`},
		"missing docString":   {Text: `"sh" runs:`},
		"non-integer capture": {Text: `"sh" exit code is "x"`},
	}
	for name, call := range cases {
		if err := e.Run(call); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if err := e.Run(StepCall{Text: `"sh" does a dance`}); !strings.Contains(err.Error(), "undefined step") {
		t.Errorf("undefined steps should say so, got %v", err)
	}
}

func TestExecutorListsBoundSteps(t *testing.T) {
	e := newShellExecutor(t)
	steps := e.Steps()
	if len(steps) == 0 {
		t.Fatal("expected the shell resource's steps")
	}
	for _, s := range steps {
		if s.Resource != "sh" || strings.Contains(s.Pattern, "{resource}") {
			t.Errorf("steps should be bound to the resource name: %+v", s)
		}
	}
}

func TestToTableAndConvertArg(t *testing.T) {
	tbl := toTable([][]string{{"id", "name"}, {"1", "Ada"}})
	if len(tbl.Rows) != 2 || tbl.Rows[1].Cells[1].Value != "Ada" {
		t.Errorf("table not converted: %+v", tbl)
	}
}
