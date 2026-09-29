package apprunner

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
)

// Stop must reach the processes the app command starts, not just the command:
// `go run ./app` does not forward SIGTERM, and its compiled app used to outlive
// the run and hold the app port.
func TestStopCommand_StopsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "app.sh")
	// A parent that ignores SIGTERM, like `go run`, with a child that does not.
	body := "#!/bin/sh\ntrap '' TERM\nsleep 300 &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(config.AppConfig{Command: script}, nil)
	// Start the process the way startCommand does, without its ready check.
	if err := r.startProcess(context.Background()); err != nil {
		t.Fatal(err)
	}

	var childPid int
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if data, err := os.ReadFile(pidFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			childPid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			break
		}
	}
	if childPid == 0 {
		t.Fatal("the app never started its child")
	}

	if err := r.stopCommand(); err != nil {
		t.Fatalf("stopCommand: %v", err)
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if syscall.Kill(childPid, 0) != nil {
			return
		}
	}
	_ = syscall.Kill(childPid, syscall.SIGKILL)
	t.Fatalf("the app's child %d outlived Stop", childPid)
}
