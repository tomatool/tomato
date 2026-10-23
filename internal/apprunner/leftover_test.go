package apprunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/runlog"
)

// startGroup starts command in its own process group, the way startProcess
// starts the app, and kills the group when the test ends.
func startGroup(t *testing.T, name string, args ...string) int {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	pgid := cmd.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
	return pgid
}

// deadPid returns the pid of a process that has exited.
func deadPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func writeLeftover(t *testing.T, runs, dir string, pgid, tomato int, command string) {
	t.Helper()
	path := filepath.Join(runs, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("%d\n%d\n%s\n", pgid, tomato, command)
	if err := os.WriteFile(filepath.Join(path, appPidFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gone(pgid int) bool {
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if syscall.Kill(-pgid, 0) != nil {
			return true
		}
	}
	return false
}

// A run killed with SIGKILL cannot stop its app. The next run stops it rather
// than failing on the port it holds.
func TestStopLeftoverApps_StopsAnAppItsRunLeftBehind(t *testing.T) {
	runs := t.TempDir()
	pgid := startGroup(t, "sleep", "300")
	writeLeftover(t, runs, "2026-10-23_050220_4d586811", pgid, deadPid(t), "sleep 300")

	current := filepath.Join(runs, "2026-10-23_050417_216b61f6")
	r := NewRunner(config.AppConfig{Command: "sleep 300"}, nil)
	r.SetRunContext(&runlog.RunContext{Dir: current})

	stopped := r.stopLeftoverApps()
	if len(stopped) != 1 || stopped[0] != "4d586811" {
		t.Fatalf("stopped %v, want [4d586811]", stopped)
	}
	if !gone(pgid) {
		t.Fatal("the leftover app is still running")
	}
	if _, err := os.Stat(filepath.Join(runs, "2026-10-23_050220_4d586811", appPidFile)); !os.IsNotExist(err) {
		t.Error("the leftover's pid file should be removed once it is stopped")
	}
}

func TestStopLeftoverApps_LeavesOtherProcessesAlone(t *testing.T) {
	for name, tc := range map[string]struct {
		recorded string
		tomato   func(t *testing.T) int
	}{
		// The pid was reused by a process that is not the app.
		"another command": {recorded: "go run ./cmd/app", tomato: deadPid},
		// The tomato that started it is still running: a run in progress.
		"a run in progress": {recorded: "sleep 300", tomato: func(*testing.T) int { return os.Getpid() }},
	} {
		t.Run(name, func(t *testing.T) {
			runs := t.TempDir()
			pgid := startGroup(t, "sleep", "300")
			writeLeftover(t, runs, "2026-10-23_050220_4d586811", pgid, tc.tomato(t), tc.recorded)

			r := NewRunner(config.AppConfig{Command: "sleep 300"}, nil)
			r.SetRunContext(&runlog.RunContext{Dir: filepath.Join(runs, "2026-10-23_050417_216b61f6")})

			if stopped := r.stopLeftoverApps(); len(stopped) != 0 {
				t.Fatalf("stopped %v, want nothing", stopped)
			}
			if syscall.Kill(-pgid, 0) != nil {
				t.Fatal("a process that is not a leftover app was stopped")
			}
		})
	}
}

// The pid file exists while the app runs and is gone once Stop stops it, so
// only a run that did not stop its app leaves one.
func TestPidFile_RecordedWhileTheAppRuns(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner(config.AppConfig{Command: "sleep 300"}, nil)
	r.SetRunContext(&runlog.RunContext{Dir: dir})
	if err := r.startProcess(t.Context()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, appPidFile)
	app, ok := readPidFile(path)
	if !ok {
		r.Stop()
		t.Fatal("no pid file while the app runs")
	}
	if app.pgid != r.cmd.Process.Pid || app.tomato != os.Getpid() || app.command != "sleep 300" {
		t.Errorf("pid file records %+v", app)
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the pid file should be removed when the app is stopped")
	}
}
