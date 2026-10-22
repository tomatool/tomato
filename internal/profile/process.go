package profile

import (
	"context"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// processReading is the app's process group at one moment.
type processReading struct {
	cpu     time.Duration // CPU time used so far, user and system, summed over the group
	rss     uint64
	threads int32
	fds     int32
}

type processReader interface {
	read(pgid int) (processReading, bool)
}

// groupReader reads every process in a process group. The app is started in
// a group of its own, so the group is the app and whatever it started: the
// compiled server under `go run`, the workers of a process manager. A child
// that exits takes its CPU time with it, so the group's total can drop; a
// window never counts below zero.
type groupReader struct{}

func (groupReader) read(pgid int) (processReading, bool) {
	pids, err := process.Pids()
	if err != nil {
		return processReading{}, false
	}
	var r processReading
	found := false
	for _, pid := range pids {
		if g, err := syscall.Getpgid(int(pid)); err != nil || g != pgid {
			continue
		}
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		found = true
		if t, err := p.Times(); err == nil {
			r.cpu += time.Duration((t.User + t.System) * float64(time.Second))
		}
		if m, err := p.MemoryInfo(); err == nil {
			r.rss += m.RSS
		}
		if n, err := p.NumThreads(); err == nil {
			r.threads += n
		}
		if n, err := p.NumFDs(); err == nil {
			r.fds += n
		}
	}
	return r, found
}

// StatsReading is one Docker stats reading of a container.
type StatsReading struct {
	At     time.Time
	CPU    uint64 // nanoseconds since the container started
	Memory uint64
}

// StatsStreamer streams a container's stats until ctx is done.
type StatsStreamer interface {
	StreamStats(ctx context.Context, name string, fn func(StatsReading)) error
}

// contextUntil is a context cancelled when stop closes.
func contextUntil(stop <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
