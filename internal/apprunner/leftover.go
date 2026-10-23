package apprunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// appPidFile is the file in a run's directory that records the app process
// group while the app runs. Stop removes it, so one found later belongs to a
// run that ended without stopping its app: killed, or crashed.
const appPidFile = "app.pid"

// leftoverApp is an app a previous run started and did not stop.
type leftoverApp struct {
	runID   string
	pgid    int
	tomato  int // pid of the tomato that started it
	command string
}

// writePidFile records the running app's process group, the tomato that
// started it and its command in the run directory.
func (r *Runner) writePidFile() {
	if r.runCtx == nil || r.cmd == nil || r.cmd.Process == nil {
		return
	}
	body := fmt.Sprintf("%d\n%d\n%s\n", r.cmd.Process.Pid, os.Getpid(), r.config.Command)
	if err := os.WriteFile(filepath.Join(r.runCtx.Dir, appPidFile), []byte(body), 0o644); err != nil {
		log.Debug().Err(err).Msg("failed to record the app process group")
	}
}

func (r *Runner) removePidFile() {
	if r.runCtx == nil {
		return
	}
	_ = os.Remove(filepath.Join(r.runCtx.Dir, appPidFile))
}

// stopLeftoverApps stops the apps that earlier runs of this suite started and
// left running, and reports which ones it stopped. A run stopped with SIGKILL
// (or one that crashed) cannot stop its app, and the app kept the app port,
// failing every run after it until someone found and killed it by hand.
//
// Only a process group a run recorded is stopped, and only while its leader
// still runs the app command, so a pid reused by something else is left alone.
// An app whose tomato is still running belongs to a run in progress, not to a
// dead one, and is left alone too.
func (r *Runner) stopLeftoverApps() []string {
	if r.runCtx == nil {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(r.runCtx.Dir), "*", appPidFile))
	var stopped []string
	for _, path := range files {
		if filepath.Dir(path) == r.runCtx.Dir {
			continue
		}
		app, ok := readPidFile(path)
		if !ok || !app.isLeftover() {
			continue
		}
		log.Debug().Int("pgid", app.pgid).Str("run", app.runID).Msg("stopping app left running by an earlier run")
		stopGroup(app.pgid, 5*time.Second)
		_ = os.Remove(path)
		stopped = append(stopped, app.runID)
	}
	return stopped
}

func readPidFile(path string) (leftoverApp, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return leftoverApp{}, false
	}
	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(lines) != 3 {
		return leftoverApp{}, false
	}
	pgid, err1 := strconv.Atoi(lines[0])
	tomato, err2 := strconv.Atoi(lines[1])
	if err1 != nil || err2 != nil || pgid <= 1 {
		return leftoverApp{}, false
	}
	// Run directories are named <date>_<time>_<run id>.
	dir := filepath.Base(filepath.Dir(path))
	runID := dir[strings.LastIndex(dir, "_")+1:]
	return leftoverApp{runID: runID, pgid: pgid, tomato: tomato, command: lines[2]}, true
}

// isLeftover reports whether the app is still running with no tomato to stop it.
func (a leftoverApp) isLeftover() bool {
	if a.tomato > 0 && syscall.Kill(a.tomato, 0) == nil {
		return false
	}
	if pgid, err := syscall.Getpgid(a.pgid); err != nil || pgid != a.pgid {
		return false
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(a.pgid)).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == strings.Join(strings.Fields(a.command), " ")
}

// stopGroup sends SIGTERM to a process group, and SIGKILL if it is still
// there after grace.
func stopGroup(pgid int, grace time.Duration) {
	if syscall.Kill(-pgid, syscall.SIGTERM) != nil {
		return
	}
	for deadline := time.Now().Add(grace); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if syscall.Kill(-pgid, 0) != nil {
			return
		}
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
