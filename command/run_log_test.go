package command

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// A run that fails has to say why in its run log. main prints the error to
// stderr after the action returns, and the tee only copies stdout and is torn
// down by then — so without a deliberate write, a run that died while
// starting up left a log containing the banner and "Starting dependencies..."
// and nothing else, which is exactly when you need it most.
func TestRunLog_RecordsWhyTheRunFailed(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "tomato.yml"), `version: 2
resources:
  db:
    type: mysql
features:
  paths: ["./features"]
`)
	if err := os.MkdirAll(filepath.Join(dir, "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "features", "f.feature"),
		"Feature: f\n\n  Scenario: s\n    Given something\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	set := flag.NewFlagSet("run", flag.ContinueOnError)
	set.String("config", "tomato.yml", "")
	set.String("format", "", "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)

	runErr := runTests(ctx)
	if runErr == nil {
		t.Fatal("expected the run to fail on an unsupported resource type")
	}

	runs, err := os.ReadDir(filepath.Join(dir, ".tomato", "runs"))
	if err != nil || len(runs) == 0 {
		t.Fatalf("no run directory was created: %v", err)
	}
	logPath := filepath.Join(dir, ".tomato", "runs", runs[0].Name(), "tomato.log")
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading run log: %v", err)
	}

	if !strings.Contains(string(body), "Error:") {
		t.Errorf("the run log does not record the failure.\nlog was:\n%s", body)
	}
	if !strings.Contains(string(body), "mysql") {
		t.Errorf("the run log does not name the cause.\nlog was:\n%s", body)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
