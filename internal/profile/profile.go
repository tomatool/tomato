// Package profile measures the application under test while the scenarios
// run: its CPU and memory, the containers it depends on, the traffic to each
// dependency, and the errors it logs. Everything is read from outside the
// application, so it needs no change to its code.
//
// A Profiler samples continuously and closes a window per scenario. The
// window's CPU time is exact, read from the process's own counters at the
// scenario's start and end; memory and the containers' figures come from
// samples, about four a second for the app and one for each container.
package profile

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tomatool/tomato/internal/resource"
)

// Sample is a reading of the app and its containers at one moment.
type Sample struct {
	At  time.Time      `json:"at"`
	App *ProcessSample `json:"app,omitempty"`
	// Containers holds each dependency container's latest reading.
	Containers map[string]ContainerSample `json:"containers,omitempty"`
}

// ProcessSample is the app's processes at one moment: its process group in
// command mode, so a `go run` or `npm start` counts the server it starts.
type ProcessSample struct {
	// CPU is the share of one core used since the last sample, in percent.
	CPU float64 `json:"cpu"`
	// RSS is resident memory in bytes.
	RSS     uint64 `json:"rss"`
	Threads int32  `json:"threads"`
	FDs     int32  `json:"fds"`
}

// ContainerSample is a container at one moment.
type ContainerSample struct {
	CPU    float64 `json:"cpu"`
	Memory uint64  `json:"memory"`
}

// Summary is what one scenario cost.
type Summary struct {
	Start      time.Time                             `json:"start"`
	End        time.Time                             `json:"end"`
	DurationMs int64                                 `json:"durationMs"`
	App        *AppSummary                           `json:"app,omitempty"`
	Containers map[string]ContainerSummary           `json:"containers,omitempty"`
	Network    map[string]resource.DependencyTraffic `json:"network,omitempty"`
	Logs       *LogSummary                           `json:"logs,omitempty"`
}

// AppSummary is the app's share of a scenario.
type AppSummary struct {
	// CPUMs is the CPU time the app used during the scenario, exactly.
	CPUMs int64 `json:"cpuMs"`
	// PeakRSS is the most memory seen during the scenario, RSSStart and
	// RSSEnd the memory at its edges: an end well above the start that does
	// not come down in later scenarios is a leak.
	PeakRSS  uint64 `json:"peakRss"`
	RSSStart uint64 `json:"rssStart"`
	RSSEnd   uint64 `json:"rssEnd"`
	// Threads and FDs at the edges, for the same reason.
	ThreadsStart int32 `json:"threadsStart"`
	ThreadsEnd   int32 `json:"threadsEnd"`
	FDsStart     int32 `json:"fdsStart"`
	FDsEnd       int32 `json:"fdsEnd"`
}

// ContainerSummary is a container's share of a scenario.
type ContainerSummary struct {
	// CPUMs is the container's CPU time during the scenario, to within the
	// second Docker reports at.
	CPUMs      int64  `json:"cpuMs"`
	PeakMemory uint64 `json:"peakMemory"`
}

// LogSummary counts what the app logged as errors and warnings.
type LogSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	// Examples are the first few such lines, as the app wrote them.
	Examples []string `json:"examples,omitempty"`
}

// Sources is what a Profiler reads.
type Sources struct {
	// AppProcessGroup returns the app's process group id in command mode,
	// 0 while it is not running. nil when the app runs in a container.
	AppProcessGroup func() int
	// AppContainer is the app's container in container mode.
	AppContainer string
	// Containers are the dependency containers to stream stats for.
	Containers []string
	Stats      StatsStreamer
	// Meters are the resources that see the app's traffic to a dependency.
	Meters map[string]resource.DependencyMeter
}

// Profiler samples the app and closes a Summary per scenario.
type Profiler struct {
	src      Sources
	interval time.Duration
	proc     processReader

	mu       sync.Mutex
	pending  []Sample // not yet reported by the formatter
	all      []Sample // kept for the run's profile file
	last     *ProcessSample
	lastCPU  time.Duration
	lastAt   time.Time
	ctr      map[string]containerState
	window   *window
	done     map[string]*Summary
	history  []NamedSummary
	logLevel logClassifier
	stop     chan struct{}
	wg       sync.WaitGroup
}

type containerState struct {
	latest   ContainerSample
	cpuTotal uint64
	at       time.Time
}

// window is the scenario that is running.
type window struct {
	name     string
	start    time.Time
	cpuStart time.Duration
	app      *AppSummary
	ctrCPU   map[string]uint64
	ctrPeak  map[string]uint64
	logs     LogSummary
}

// New returns a Profiler; Start begins sampling.
func New(src Sources) *Profiler {
	return &Profiler{
		src:      src,
		interval: 250 * time.Millisecond,
		proc:     groupReader{},
		ctr:      map[string]containerState{},
		done:     map[string]*Summary{},
		logLevel: defaultLogClassifier(),
		stop:     make(chan struct{}),
	}
}

var active atomic.Pointer[Profiler]

// Active is the run's profiler, for the formatter; nil when profiling is off.
func Active() *Profiler { return active.Load() }

// Start begins sampling and makes this the run's profiler.
func (p *Profiler) Start() {
	active.Store(p)
	p.wg.Add(1)
	go p.sampleApp()
	for _, name := range p.src.Containers {
		p.wg.Add(1)
		go p.streamContainer(name)
	}
	if p.src.AppContainer != "" {
		p.wg.Add(1)
		go p.streamContainer(p.src.AppContainer)
	}
}

// Stop ends sampling.
func (p *Profiler) Stop() {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	p.wg.Wait()
	active.CompareAndSwap(p, nil)
}

func (p *Profiler) sampleApp() {
	defer p.wg.Done()
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		p.sampleOnce()
		select {
		case <-p.stop:
			return
		case <-t.C:
		}
	}
}

// sampleOnce reads the app's processes and records a sample.
func (p *Profiler) sampleOnce() {
	now := time.Now()
	var app *ProcessSample
	var cpu time.Duration
	if p.src.AppProcessGroup != nil {
		if pgid := p.src.AppProcessGroup(); pgid > 0 {
			if r, ok := p.proc.read(pgid); ok {
				app, cpu = &ProcessSample{RSS: r.rss, Threads: r.threads, FDs: r.fds}, r.cpu
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if app != nil {
		if p.last != nil && now.After(p.lastAt) && cpu >= p.lastCPU {
			app.CPU = 100 * float64(cpu-p.lastCPU) / float64(now.Sub(p.lastAt))
		}
		p.last, p.lastCPU, p.lastAt = app, cpu, now
		if w := p.window; w != nil && w.app != nil && app.RSS > w.app.PeakRSS {
			w.app.PeakRSS = app.RSS
		}
	} else if p.src.AppContainer != "" {
		if c, ok := p.ctr[p.src.AppContainer]; ok {
			app = &ProcessSample{CPU: c.latest.CPU, RSS: c.latest.Memory}
		}
	}
	s := Sample{At: now, App: app}
	if len(p.ctr) > 0 {
		s.Containers = make(map[string]ContainerSample, len(p.ctr))
		for name, c := range p.ctr {
			if name != p.src.AppContainer {
				s.Containers[name] = c.latest
			}
		}
	}
	p.pending = append(p.pending, s)
	p.all = append(p.all, s)
}

func (p *Profiler) streamContainer(name string) {
	defer p.wg.Done()
	if p.src.Stats == nil {
		return
	}
	for {
		ctx, cancel := contextUntil(p.stop)
		_ = p.src.Stats.StreamStats(ctx, name, func(s StatsReading) { p.containerReading(name, s) })
		cancel()
		select {
		case <-p.stop:
			return
		case <-time.After(time.Second):
			// The container stopped, or a scenario restarted it: stream again.
		}
	}
}

func (p *Profiler) containerReading(name string, s StatsReading) {
	p.mu.Lock()
	defer p.mu.Unlock()
	prev, ok := p.ctr[name]
	cur := containerState{cpuTotal: s.CPU, at: s.At, latest: ContainerSample{Memory: s.Memory}}
	if ok && s.At.After(prev.at) && s.CPU >= prev.cpuTotal {
		cur.latest.CPU = 100 * float64(s.CPU-prev.cpuTotal) / float64(s.At.Sub(prev.at))
	}
	p.ctr[name] = cur
	if w := p.window; w != nil {
		if s.Memory > w.ctrPeak[name] {
			w.ctrPeak[name] = s.Memory
		}
		if _, seen := w.ctrCPU[name]; !seen {
			// The container came back during the scenario: count from here.
			w.ctrCPU[name] = s.CPU
		}
	}
}

// ScenarioStarted opens the scenario's window, after tomato's resets, so
// their work is not counted to it.
func (p *Profiler) ScenarioStarted(name string) {
	now := time.Now()
	r, ok := p.readApp()
	p.mu.Lock()
	defer p.mu.Unlock()
	w := &window{name: name, start: now, ctrCPU: map[string]uint64{}, ctrPeak: map[string]uint64{}}
	if ok {
		w.cpuStart = r.cpu
		w.app = &AppSummary{PeakRSS: r.rss, RSSStart: r.rss, ThreadsStart: r.threads, FDsStart: r.fds}
	}
	for name, c := range p.ctr {
		w.ctrCPU[name] = c.cpuTotal
		w.ctrPeak[name] = c.latest.Memory
	}
	p.window = w
}

// ScenarioEnded closes the window and keeps its Summary for the formatter.
func (p *Profiler) ScenarioEnded(name string) *Summary {
	now := time.Now()
	r, ok := p.readApp()
	traffic := map[string]resource.DependencyTraffic{}
	for res, m := range p.src.Meters {
		traffic[res] = m.MeterDependency()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	w := p.window
	if w == nil || w.name != name {
		return nil
	}
	p.window = nil
	s := &Summary{Start: w.start, End: now, DurationMs: now.Sub(w.start).Milliseconds()}
	if w.app != nil && ok {
		a := *w.app
		if r.cpu >= w.cpuStart {
			a.CPUMs = (r.cpu - w.cpuStart).Milliseconds()
		}
		a.RSSEnd, a.ThreadsEnd, a.FDsEnd = r.rss, r.threads, r.fds
		if r.rss > a.PeakRSS {
			a.PeakRSS = r.rss
		}
		s.App = &a
	}
	if len(p.ctr) > 0 {
		s.Containers = map[string]ContainerSummary{}
		for name, c := range p.ctr {
			start, seen := w.ctrCPU[name]
			sum := ContainerSummary{PeakMemory: w.ctrPeak[name]}
			if seen && c.cpuTotal >= start {
				sum.CPUMs = int64(time.Duration(c.cpuTotal - start).Milliseconds())
			}
			if name == p.src.AppContainer {
				s.App = &AppSummary{CPUMs: sum.CPUMs, PeakRSS: sum.PeakMemory, RSSEnd: c.latest.Memory}
				continue
			}
			s.Containers[name] = sum
		}
	}
	if len(traffic) > 0 {
		s.Network = traffic
	}
	if w.logs.Errors > 0 || w.logs.Warnings > 0 {
		l := w.logs
		s.Logs = &l
	}
	p.done[name] = s
	p.history = append(p.history, NamedSummary{Scenario: name, Summary: *s})
	return s
}

func (p *Profiler) readApp() (processReading, bool) {
	if p.src.AppProcessGroup == nil {
		return processReading{}, false
	}
	pgid := p.src.AppProcessGroup()
	if pgid <= 0 {
		return processReading{}, false
	}
	return p.proc.read(pgid)
}

// TakeSamples returns the samples since the last call.
func (p *Profiler) TakeSamples() []Sample {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.pending
	p.pending = nil
	return out
}

// TakeSummary returns a finished scenario's Summary, once.
func (p *Profiler) TakeSummary(scenario string) *Summary {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.done[scenario]
	delete(p.done, scenario)
	return s
}

// Result is a whole run's profile: every scenario's Summary, in the order
// they ran, and every sample. It is what a run writes to profile.json.
type Result struct {
	Scenarios []NamedSummary `json:"scenarios"`
	Samples   []Sample       `json:"samples"`
}

// NamedSummary is a Summary with its scenario's name.
type NamedSummary struct {
	Scenario string `json:"scenario"`
	Summary
}

// Result returns the run's profile so far.
func (p *Profiler) Result() Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := Result{Scenarios: make([]NamedSummary, len(p.history)), Samples: make([]Sample, len(p.all))}
	copy(r.Scenarios, p.history)
	copy(r.Samples, p.all)
	sort.Slice(r.Samples, func(i, j int) bool { return r.Samples[i].At.Before(r.Samples[j].At) })
	return r
}
