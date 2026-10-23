package report

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// CTRF is the Common Test Report Format (https://ctrf.io): one JSON document
// with a summary and a test per scenario, which the ctrf-io GitHub Actions
// and reporters turn into PR comments, job summaries and flaky-test
// history. What tomato knows beyond pass and fail — the failed step, the
// screenshots, what the scenario cost the app — goes into each test's
// `extra`, `steps` and `attachments`, and the run's start-up into the
// results' `extra`.
type ctrfReport struct {
	ReportFormat string      `json:"reportFormat"`
	SpecVersion  string      `json:"specVersion"`
	Timestamp    string      `json:"timestamp,omitempty"`
	GeneratedBy  string      `json:"generatedBy"`
	Results      ctrfResults `json:"results"`
}

type ctrfResults struct {
	Tool        ctrfTool         `json:"tool"`
	Summary     ctrfSummary      `json:"summary"`
	Tests       []ctrfTest       `json:"tests"`
	Environment *ctrfEnvironment `json:"environment,omitempty"`
	Extra       map[string]any   `json:"extra,omitempty"`
}

type ctrfTool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ctrfSummary struct {
	Tests   int   `json:"tests"`
	Passed  int   `json:"passed"`
	Failed  int   `json:"failed"`
	Pending int   `json:"pending"`
	Skipped int   `json:"skipped"`
	Other   int   `json:"other"`
	Start   int64 `json:"start"`
	Stop    int64 `json:"stop"`
}

type ctrfTest struct {
	Name        string           `json:"name"`
	Status      string           `json:"status"`
	Duration    int64            `json:"duration"`
	Start       int64            `json:"start,omitempty"`
	Stop        int64            `json:"stop,omitempty"`
	Suite       string           `json:"suite,omitempty"`
	Message     string           `json:"message,omitempty"`
	Trace       string           `json:"trace,omitempty"`
	FilePath    string           `json:"filePath,omitempty"`
	Type        string           `json:"type,omitempty"`
	Steps       []ctrfStep       `json:"steps,omitempty"`
	Attachments []ctrfAttachment `json:"attachments,omitempty"`
	Extra       map[string]any   `json:"extra,omitempty"`
}

type ctrfStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ctrfAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}

type ctrfEnvironment struct {
	AppName string         `json:"appName,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// ctrfStatus maps a status tomato reports to one of CTRF's five.
func ctrfStatus(s string) string {
	switch s {
	case "passed", "failed", "skipped", "pending":
		return s
	case "undefined", "ambiguous":
		return "failed"
	default:
		return "other"
	}
}

// WriteCTRF writes the run as a CTRF JSON report. A run that failed before
// any scenario is one failed test named after the phase it failed in, so a
// reader that only counts tests still sees the run failed.
func WriteCTRF(w io.Writer, run *Run, version string) error {
	rep := ctrfReport{
		ReportFormat: "CTRF",
		SpecVersion:  "0.0.0",
		GeneratedBy:  "tomato",
		Results: ctrfResults{
			Tool:  ctrfTool{Name: "tomato", Version: version},
			Tests: []ctrfTest{},
		},
	}
	if !run.Start.IsZero() {
		rep.Timestamp = run.Start.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	sum := &rep.Results.Summary
	sum.Start, sum.Stop = run.Start.UnixMilli(), run.Stop.UnixMilli()

	for _, s := range run.Scenarios() {
		t := ctrfTest{
			Name: s.Name, Status: ctrfStatus(s.Status), Duration: s.DurationMs,
			Suite: s.Feature, FilePath: s.File, Type: "e2e",
		}
		if !s.Start.IsZero() {
			t.Start = s.Start.UnixMilli()
		}
		if !s.Stop.IsZero() {
			t.Stop = s.Stop.UnixMilli()
		}
		for _, st := range s.Steps {
			t.Steps = append(t.Steps, ctrfStep{Name: st.Text, Status: ctrfStatus(st.Status)})
			for _, shot := range st.Screenshots {
				t.Attachments = append(t.Attachments, ctrfAttachment{
					Name: fmt.Sprintf("%s · %s", shot.Resource, filepath.Base(shot.Path)), ContentType: "image/png", Path: shot.Path,
				})
			}
		}
		extra := map[string]any{"feature": s.Feature}
		if f := s.FailedStep(); f != nil {
			t.Message = firstLine(f.Error)
			t.Trace = fmt.Sprintf("step %d: %s\n%s", f.Index+1, f.Text, f.Error)
			extra["failedStep"] = f.Text
		} else if s.Status == "other" {
			t.Message = "the run stopped while this scenario was running"
		}
		if s.Profile != nil {
			extra["profile"] = s.Profile
		}
		t.Extra = extra
		rep.Results.Tests = append(rep.Results.Tests, t)
	}

	if run.Error != nil {
		rep.Results.Tests = append(rep.Results.Tests, ctrfTest{
			Name:    fmt.Sprintf("tomato run: %s", phaseName(run.Error.Phase)),
			Status:  "failed",
			Suite:   "tomato",
			Message: firstLine(run.Error.Message),
			Trace:   strings.Join(append([]string{run.Error.Message}, run.Error.Logs...), "\n"),
			Type:    "setup",
		})
	}

	for _, t := range rep.Results.Tests {
		sum.Tests++
		switch t.Status {
		case "passed":
			sum.Passed++
		case "failed":
			sum.Failed++
		case "skipped":
			sum.Skipped++
		case "pending":
			sum.Pending++
		default:
			sum.Other++
		}
	}

	extra := map[string]any{"runId": run.ID, "status": run.Status}
	if len(run.Containers) > 0 {
		cs := make([]map[string]any, 0, len(run.Containers))
		for _, c := range run.Containers {
			cs = append(cs, map[string]any{"name": c.Name, "status": c.Status, "durationMs": c.DurationMs})
		}
		extra["containers"] = cs
	}
	if run.App != nil {
		extra["app"] = map[string]any{"name": run.App.Name, "status": run.App.Status, "url": run.App.URL, "durationMs": run.App.DurationMs}
		rep.Results.Environment = &ctrfEnvironment{AppName: run.App.Name}
	}
	rep.Results.Extra = extra

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// phaseName says where a run failed, for a test name.
func phaseName(phase string) string {
	switch phase {
	case "config":
		return "reading tomato.yml"
	case "containers":
		return "starting the dependencies"
	case "resources":
		return "initialising the resources"
	case "app":
		return "starting the app"
	}
	return "starting up"
}
