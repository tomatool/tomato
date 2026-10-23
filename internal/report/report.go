// Package report turns a run's events into reports in the formats test
// tooling reads. Every run records its events in its run directory
// (events.log, or tomato.log for a run started with --format tomato), so a
// report can be written while the run ends or for any past run afterwards,
// from the same stream and with the same result.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/formatter"
	"github.com/tomatool/tomato/internal/profile"
	"github.com/tomatool/tomato/internal/resource"
)

const eventPrefix = "TOMATO_EVENT:"

// Run is a run as its events tell it.
type Run struct {
	ID    string
	Start time.Time
	Stop  time.Time
	// Status is passed, failed, or interrupted when the events stop before
	// the run said how it ended.
	Status     string
	Error      *RunError
	Containers []Container
	App        *App
	Features   []*Feature
}

// RunError is a run that failed before or outside its scenarios.
type RunError struct {
	Phase   string
	Message string
	Logs    []string
}

// Container is a dependency the run started, and how long it took.
type Container struct {
	Name       string
	Status     string
	DurationMs int64
}

// App is the app under test, and how long it took to be ready.
type App struct {
	Name       string
	Status     string
	URL        string
	DurationMs int64
}

// Feature is a feature file and the scenarios of it that ran.
type Feature struct {
	Name      string
	File      string
	Scenarios []*Scenario
}

// Scenario is one scenario as it ran.
type Scenario struct {
	Name    string
	Feature string
	File    string
	// Status is passed or failed as the runner reported it, or other when
	// the run stopped while it ran.
	Status     string
	DurationMs int64
	Start      time.Time
	Stop       time.Time
	Steps      []*Step
	Profile    *profile.Summary
}

// Step is one step of a scenario, Background steps first.
type Step struct {
	Index       int
	Text        string
	Status      string
	Error       string
	DurationMs  int64
	Screenshots []resource.Screenshot
}

// FailedStep is the step the scenario failed on, or nil.
func (s *Scenario) FailedStep() *Step {
	for _, st := range s.Steps {
		if st.Status == "failed" || st.Status == "undefined" || st.Status == "ambiguous" {
			return st
		}
	}
	return nil
}

// Scenarios is every scenario of the run, in the order they ran.
func (r *Run) Scenarios() []*Scenario {
	var all []*Scenario
	for _, f := range r.Features {
		all = append(all, f.Scenarios...)
	}
	return all
}

// Parse reads a run's events: TOMATO_EVENT lines, with anything else in
// between ignored, as a run log has. start is when the run began, used for
// events from tomato versions that did not stamp them.
func Parse(r io.Reader, id string, start time.Time) (*Run, error) {
	run := &Run{ID: id, Start: start, Status: "interrupted"}
	var feature *Feature
	var scenario *Scenario
	clock := start
	seen := false
	at := func(ev formatter.Event) time.Time {
		if t, err := time.Parse(time.RFC3339Nano, ev.At); err == nil {
			return t
		}
		return clock
	}
	ensureFeature := func(name, file string) *Feature {
		if feature != nil && feature.Name == name && (file == "" || feature.File == file) {
			return feature
		}
		feature = &Feature{Name: name, File: file}
		run.Features = append(run.Features, feature)
		return feature
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		i := strings.Index(line, eventPrefix)
		if i < 0 {
			continue
		}
		var ev formatter.Event
		if err := json.Unmarshal([]byte(line[i+len(eventPrefix):]), &ev); err != nil {
			continue
		}
		t := at(ev)
		if !seen && ev.At != "" {
			run.Start, seen = t, true
		}
		switch ev.Type {
		case formatter.EventContainer:
			if ev.Status != "starting" {
				run.Containers = append(run.Containers, Container{Name: ev.Name, Status: ev.Status, DurationMs: ms(ev.DurationMs)})
				clock = clock.Add(time.Duration(ms(ev.DurationMs)) * time.Millisecond)
			}
		case formatter.EventApp:
			if ev.Status == "starting" {
				break
			}
			if run.App == nil {
				run.App = &App{}
			}
			run.App.Name, run.App.Status, run.App.DurationMs = ev.Name, ev.Status, ms(ev.DurationMs)
			if ev.URL != "" {
				run.App.URL = ev.URL
			}
		case formatter.EventFeatureStart:
			feature = nil
			ensureFeature(ev.Feature, ev.File)
		case formatter.EventScenarioStart:
			f := ensureFeature(ev.Feature, ev.File)
			scenario = &Scenario{Name: ev.Scenario, Feature: f.Name, File: f.File, Status: "other", Start: t}
			f.Scenarios = append(f.Scenarios, scenario)
		case formatter.EventStepEnd:
			if scenario == nil || ev.StepIndex == nil {
				break
			}
			scenario.Steps = append(scenario.Steps, &Step{
				Index: *ev.StepIndex, Text: ev.Step, Status: ev.Status, Error: ev.Error,
				DurationMs: ms(ev.DurationMs), Screenshots: ev.Screenshots,
			})
			clock = clock.Add(time.Duration(ms(ev.DurationMs)) * time.Millisecond)
		case formatter.EventScenarioEnd:
			if scenario == nil || scenario.Name != ev.Scenario {
				break
			}
			scenario.Status, scenario.DurationMs, scenario.Profile = ev.Status, ms(ev.DurationMs), ev.Profile
			scenario.Stop = scenario.Start.Add(time.Duration(scenario.DurationMs) * time.Millisecond)
			if ev.At != "" {
				scenario.Stop = t
			}
			scenario = nil
		case formatter.EventRunError:
			run.Error = &RunError{Phase: ev.Phase, Message: ev.Error, Logs: ev.Logs}
			run.Status = "failed"
		case formatter.EventSummary:
			run.Status = "passed"
			if ev.Failed > 0 {
				run.Status = "failed"
			}
		}
		if t.After(run.Stop) {
			run.Stop = t
		}
		if clock.Before(t) {
			clock = t
		}
	}
	if run.Stop.Before(run.Start) {
		run.Stop = run.Start
	}
	return run, sc.Err()
}

func ms(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// firstLine is an error's first line, for a one-line message.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// Load reads a past run's events from its directory: events.log, which every
// run writes, or tomato.log for a run from before that, when it was started
// with --format tomato.
func Load(dir, id string, start time.Time) (*Run, error) {
	var lastErr error
	for _, name := range []string{"events.log", "tomato.log"} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			lastErr = err
			continue
		}
		run, err := Parse(f, id, start)
		f.Close()
		if err != nil {
			return nil, err
		}
		if len(run.Features) > 0 || run.Error != nil || name == "tomato.log" {
			return run, nil
		}
	}
	return nil, fmt.Errorf("no events recorded for this run: %w", lastErr)
}

// Write writes the run in one of Formats.
func Write(w io.Writer, format string, run *Run, version string) error {
	switch format {
	case "ctrf":
		return WriteCTRF(w, run, version)
	case "junit":
		return WriteJUnit(w, run)
	}
	return fmt.Errorf("unknown report format %q (want one of %s)", format, strings.Join(Formats, ", "))
}

// Formats are the report formats Write knows.
var Formats = []string{"ctrf", "junit"}

// Extension is the usual file extension of a format.
func Extension(format string) string {
	if format == "junit" {
		return ".xml"
	}
	return ".json"
}
