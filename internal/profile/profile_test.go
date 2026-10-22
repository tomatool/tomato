package profile

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/resource"
)

func TestLogClassifier(t *testing.T) {
	c := defaultLogClassifier()
	for _, tc := range []struct {
		line         string
		err, warning bool
	}{
		{"2026/10/22 07:01:02 ERROR could not reach payments", true, false},
		{`{"time":"…","level":"error","msg":"timeout"}`, true, false},
		{`{"severity":"FATAL","message":"boom"}`, true, false},
		{"time=… level=warn msg=retrying", false, true},
		{"[WARN] pool almost full", false, true},
		{"E| connection refused", false, false}, // a bare letter is not a level
		{"error: config not found", true, false},
		{"INFO user errors=0 saved", false, false},
		{"processed 3 orders without warnings", false, false},
		{"GET /errors 200", false, false},
	} {
		gotErr, gotWarn := c.classify(tc.line)
		if gotErr != tc.err || gotWarn != tc.warning {
			t.Errorf("%q: error %v warning %v, want %v %v", tc.line, gotErr, gotWarn, tc.err, tc.warning)
		}
	}
}

// fakeProcess is an app whose CPU and memory a test sets.
type fakeProcess struct {
	mu sync.Mutex
	r  processReading
}

func (f *fakeProcess) set(cpu time.Duration, rss uint64, fds int32) {
	f.mu.Lock()
	f.r = processReading{cpu: cpu, rss: rss, threads: 8, fds: fds}
	f.mu.Unlock()
}

func (f *fakeProcess) read(int) (processReading, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.r, true
}

type fakeMeter struct{ t resource.DependencyTraffic }

func (m fakeMeter) MeterDependency() resource.DependencyTraffic { return m.t }

func TestScenarioSummary(t *testing.T) {
	proc := &fakeProcess{}
	proc.set(2*time.Second, 100<<20, 10)
	p := New(Sources{
		AppProcessGroup: func() int { return 42 },
		Meters: map[string]resource.DependencyMeter{
			"postgres-network": fakeMeter{resource.DependencyTraffic{Container: "postgres", Connections: 2, RoundTrips: 412, WaitMs: 1400}},
		},
	})
	p.proc = proc

	p.ScenarioStarted("refund")
	proc.set(2*time.Second+450*time.Millisecond, 180<<20, 12)
	p.sampleOnce() // the peak
	proc.set(2*time.Second+500*time.Millisecond, 140<<20, 13)
	p.containerReading("postgres", StatsReading{At: time.Now(), CPU: 5e9, Memory: 64 << 20})
	p.ObserveLog("ERROR payment declined")
	p.ObserveLog("level=warn msg=retry")
	p.ObserveLog("INFO ok")
	s := p.ScenarioEnded("refund")

	if s == nil || s.App == nil {
		t.Fatalf("no summary: %+v", s)
	}
	if s.App.CPUMs != 500 {
		t.Errorf("CPU %dms, want 500", s.App.CPUMs)
	}
	if s.App.PeakRSS != 180<<20 || s.App.RSSStart != 100<<20 || s.App.RSSEnd != 140<<20 {
		t.Errorf("memory %+v", s.App)
	}
	if s.App.FDsStart != 10 || s.App.FDsEnd != 13 {
		t.Errorf("fds %d → %d", s.App.FDsStart, s.App.FDsEnd)
	}
	if s.Logs == nil || s.Logs.Errors != 1 || s.Logs.Warnings != 1 || len(s.Logs.Examples) != 2 {
		t.Errorf("logs %+v", s.Logs)
	}
	if got := s.Network["postgres-network"]; got.RoundTrips != 412 || got.WaitMs != 1400 {
		t.Errorf("network %+v", s.Network)
	}
	// postgres appeared during the scenario: its CPU counts from then.
	if c := s.Containers["postgres"]; c.CPUMs != 0 || c.PeakMemory != 64<<20 {
		t.Errorf("container %+v", c)
	}
	if again := p.TakeSummary("refund"); again != s {
		t.Error("TakeSummary did not return the scenario's summary")
	}
	if p.TakeSummary("refund") != nil {
		t.Error("TakeSummary returned it twice")
	}
	if len(p.TakeSamples()) != 1 || len(p.TakeSamples()) != 0 {
		t.Error("samples are not handed over once")
	}
}

func TestContainerCPUIsTheScenariosShare(t *testing.T) {
	p := New(Sources{})
	t0 := time.Now()
	p.containerReading("postgres", StatsReading{At: t0, CPU: 10e9, Memory: 50 << 20})
	p.ScenarioStarted("s")
	p.containerReading("postgres", StatsReading{At: t0.Add(time.Second), CPU: 10.8e9, Memory: 70 << 20})
	s := p.ScenarioEnded("s")
	c := s.Containers["postgres"]
	if c.CPUMs != 800 || c.PeakMemory != 70<<20 {
		t.Errorf("%+v, want 800ms and 70MB", c)
	}
	if got := p.TakeSamples(); len(got) != 0 {
		t.Errorf("samples without an app: %d", len(got))
	}
}

func TestLinesOutsideAScenarioCountTowardNone(t *testing.T) {
	p := New(Sources{})
	p.ObserveLog("ERROR during startup")
	p.ScenarioStarted("s")
	if s := p.ScenarioEnded("s"); s.Logs != nil {
		t.Errorf("startup's error counted: %+v", s.Logs)
	}
}

// The group reader sees real CPU: a busy shell in a process group of its own.
func TestGroupReaderMeasuresAProcessGroup(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "while :; do :; done")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() }()

	time.Sleep(50 * time.Millisecond)
	start, ok := groupReader{}.read(cmd.Process.Pid)
	if !ok {
		t.Fatal("found no process in the group")
	}
	time.Sleep(400 * time.Millisecond)
	end, _ := groupReader{}.read(cmd.Process.Pid)
	used := end.cpu - start.cpu
	if used < 250*time.Millisecond || used > 500*time.Millisecond {
		t.Errorf("a busy loop used %s of CPU in 400ms", used)
	}
	if end.rss == 0 || end.threads == 0 {
		t.Errorf("reading %+v", end)
	}
	if _, ok := (groupReader{}).read(999999); ok {
		t.Error("found a group that does not exist")
	}
}

func TestStartAndStop(t *testing.T) {
	proc := &fakeProcess{}
	proc.set(time.Second, 1<<20, 3)
	p := New(Sources{AppProcessGroup: func() int { return 1 }, Containers: []string{"db"}, Stats: streamer{}})
	p.proc = proc
	p.interval = 10 * time.Millisecond
	p.Start()
	if Active() != p {
		t.Error("not the active profiler")
	}
	time.Sleep(60 * time.Millisecond)
	p.Stop()
	if Active() != nil {
		t.Error("still active after Stop")
	}
	samples := p.Result().Samples
	if len(samples) < 3 {
		t.Fatalf("%d samples in 60ms at 10ms", len(samples))
	}
	if _, ok := samples[len(samples)-1].Containers["db"]; !ok {
		t.Error("the container's reading is not in the samples")
	}
}

type streamer struct{}

func (streamer) StreamStats(ctx context.Context, name string, fn func(StatsReading)) error {
	fn(StatsReading{At: time.Now(), CPU: 1e9, Memory: 1 << 20})
	<-ctx.Done()
	return nil
}
