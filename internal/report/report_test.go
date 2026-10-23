package report

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

var start = time.Date(2026, 10, 23, 5, 4, 17, 0, time.UTC)

// A run of two features as `tomato run` records it: the start-up, one
// scenario passing, one failing on its second step with a screenshot, a
// third that is skipped, and the summary. Output in between is ignored.
const events = `
🍅 Tomato
  📋 run: 216b61f6
TOMATO_EVENT:{"type":"phase","phase":"containers","at":"2026-10-23T05:04:17Z"}
TOMATO_EVENT:{"type":"container","name":"postgres","status":"starting","at":"2026-10-23T05:04:17Z"}
TOMATO_EVENT:{"type":"container","name":"postgres","status":"ready","durationMs":2300,"at":"2026-10-23T05:04:19.3Z"}
  ✓ postgres 2.3s
TOMATO_EVENT:{"type":"app","name":"orderly","status":"starting","at":"2026-10-23T05:04:19.4Z"}
TOMATO_EVENT:{"type":"app","name":"orderly","status":"ready","url":"http://localhost:8099","durationMs":6100,"at":"2026-10-23T05:04:25.5Z"}
TOMATO_EVENT:{"type":"feature_start","feature":"Orders","file":"features/orders.feature","at":"2026-10-23T05:04:26Z"}
TOMATO_EVENT:{"type":"scenario_start","feature":"Orders","scenario":"An order is placed","file":"features/orders.feature","at":"2026-10-23T05:04:26Z"}
TOMATO_EVENT:{"type":"step_end","scenario":"An order is placed","step":"\"api\" sends \"POST\" to \"/orders\"","status":"passed","stepIndex":0,"durationMs":12,"at":"2026-10-23T05:04:26.012Z"}
TOMATO_EVENT:{"type":"scenario_end","feature":"Orders","scenario":"An order is placed","status":"passed","durationMs":15,"profile":{"start":"2026-10-23T05:04:26Z","end":"2026-10-23T05:04:26.015Z","durationMs":15,"app":{"cpuMs":40,"peakRss":167837696,"rssStart":167837696,"rssEnd":167841792,"threadsStart":31,"threadsEnd":31,"fdsStart":25,"fdsEnd":25}},"at":"2026-10-23T05:04:26.015Z"}
TOMATO_EVENT:{"type":"scenario_start","feature":"Orders","scenario":"The shop shows it","file":"features/orders.feature","at":"2026-10-23T05:04:27Z"}
TOMATO_EVENT:{"type":"step_end","scenario":"The shop shows it","step":"\"shopper\" opens \"/\"","status":"passed","stepIndex":0,"durationMs":500,"at":"2026-10-23T05:04:27.5Z"}
TOMATO_EVENT:{"type":"step_end","scenario":"The shop shows it","step":"\"shopper\" sees \"TMT-01\"","status":"failed","error":"did not see \"TMT-01\" within 5s\nthe page shows: Shop","stepIndex":1,"durationMs":5000,"screenshots":[{"resource":"shopper","path":".tomato/screenshots/failed-shop.png"}],"at":"2026-10-23T05:04:32.5Z"}
TOMATO_EVENT:{"type":"step_end","scenario":"The shop shows it","step":"\"shopper\" clicks \"Track\"","status":"skipped","stepIndex":2,"durationMs":0,"at":"2026-10-23T05:04:32.5Z"}
TOMATO_EVENT:{"type":"scenario_end","feature":"Orders","scenario":"The shop shows it","status":"failed","error":"did not see","durationMs":5500,"at":"2026-10-23T05:04:32.5Z"}
TOMATO_EVENT:{"type":"feature_end","feature":"Orders","at":"2026-10-23T05:04:32.5Z"}
TOMATO_EVENT:{"type":"feature_start","feature":"Health","file":"features/health.feature","at":"2026-10-23T05:04:33Z"}
TOMATO_EVENT:{"type":"scenario_start","feature":"Health","scenario":"Health answers","file":"features/health.feature","at":"2026-10-23T05:04:33Z"}
TOMATO_EVENT:{"type":"step_end","scenario":"Health answers","step":"\"api\" sends \"GET\" to \"/health\"","status":"passed","stepIndex":0,"durationMs":3,"at":"2026-10-23T05:04:33.003Z"}
TOMATO_EVENT:{"type":"scenario_end","feature":"Health","scenario":"Health answers","status":"passed","durationMs":4,"at":"2026-10-23T05:04:33.004Z"}
TOMATO_EVENT:{"type":"summary","total":3,"passed":2,"failed":1,"at":"2026-10-23T05:04:33.1Z"}
3 scenarios (2 passed, 1 failed)
`

func parse(t *testing.T, s string) *Run {
	t.Helper()
	run, err := Parse(strings.NewReader(s), "216b61f6", start)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestParse(t *testing.T) {
	run := parse(t, events)
	if run.Status != "failed" || run.Error != nil {
		t.Errorf("status %q error %v, want failed and no run error", run.Status, run.Error)
	}
	if !run.Start.Equal(start) || !run.Stop.Equal(time.Date(2026, 10, 23, 5, 4, 33, 100e6, time.UTC)) {
		t.Errorf("run from %v to %v", run.Start, run.Stop)
	}
	if len(run.Containers) != 1 || run.Containers[0] != (Container{"postgres", "ready", 2300}) {
		t.Errorf("containers %+v", run.Containers)
	}
	if run.App == nil || *run.App != (App{"orderly", "ready", "http://localhost:8099", 6100}) {
		t.Errorf("app %+v", run.App)
	}
	if len(run.Features) != 2 || len(run.Features[0].Scenarios) != 2 || run.Features[1].File != "features/health.feature" {
		t.Fatalf("features %+v", run.Features)
	}
	shop := run.Features[0].Scenarios[1]
	if shop.Status != "failed" || shop.DurationMs != 5500 || len(shop.Steps) != 3 {
		t.Errorf("scenario %+v", shop)
	}
	if f := shop.FailedStep(); f == nil || f.Index != 1 || len(f.Screenshots) != 1 {
		t.Errorf("failed step %+v", f)
	}
	if p := run.Features[0].Scenarios[0].Profile; p == nil || p.App.CPUMs != 40 {
		t.Errorf("profile %+v", p)
	}
}

func TestParse_RunThatStoppedShort(t *testing.T) {
	cut := events[:strings.Index(events, `TOMATO_EVENT:{"type":"step_end","scenario":"The shop shows it","step":"\"shopper\" sees`)]
	run := parse(t, cut)
	if run.Status != "interrupted" {
		t.Errorf("status %q, want interrupted", run.Status)
	}
	if s := run.Features[0].Scenarios[1]; s.Status != "other" {
		t.Errorf("the scenario the run stopped in is %q, want other", s.Status)
	}
}

func TestParse_RunThatFailedToStart(t *testing.T) {
	run := parse(t, `TOMATO_EVENT:{"type":"run_error","phase":"app","status":"failed","error":"app port 8099 is already in use","logs":["last line"]}`)
	if run.Status != "failed" || run.Error == nil || run.Error.Phase != "app" || run.Error.Logs[0] != "last line" {
		t.Errorf("run %+v error %+v", run, run.Error)
	}
	// Events without times, from an older tomato, keep the given start.
	if !run.Start.Equal(start) {
		t.Errorf("start %v", run.Start)
	}
}

func TestWriteCTRF(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCTRF(&buf, parse(t, events), "v2.3.0"); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		ReportFormat string `json:"reportFormat"`
		Results      struct {
			Tool    struct{ Name, Version string }
			Summary map[string]int64
			Tests   []struct {
				Name, Status, Suite, Message, Trace, FilePath string
				Duration                                      int64
				Start, Stop                                   int64
				Steps                                         []struct{ Name, Status string }
				Attachments                                   []struct{ Name, ContentType, Path string }
				Extra                                         map[string]any
			}
			Extra map[string]any
		}
	}
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	r := rep.Results
	if rep.ReportFormat != "CTRF" || r.Tool.Name != "tomato" || r.Tool.Version != "v2.3.0" {
		t.Errorf("header %+v %+v", rep.ReportFormat, r.Tool)
	}
	want := map[string]int64{"tests": 3, "passed": 2, "failed": 1, "pending": 0, "skipped": 0, "other": 0,
		"start": start.UnixMilli(), "stop": time.Date(2026, 10, 23, 5, 4, 33, 100e6, time.UTC).UnixMilli()}
	for k, v := range want {
		if r.Summary[k] != v {
			t.Errorf("summary.%s = %d, want %d", k, r.Summary[k], v)
		}
	}
	shop := r.Tests[1]
	if shop.Name != "The shop shows it" || shop.Status != "failed" || shop.Suite != "Orders" || shop.FilePath != "features/orders.feature" || shop.Duration != 5500 {
		t.Errorf("test %+v", shop)
	}
	if shop.Message != `did not see "TMT-01" within 5s` || !strings.Contains(shop.Trace, `step 2: "shopper" sees "TMT-01"`) {
		t.Errorf("message %q trace %q", shop.Message, shop.Trace)
	}
	if len(shop.Steps) != 3 || shop.Steps[2].Status != "skipped" || len(shop.Attachments) != 1 || shop.Attachments[0].Path != ".tomato/screenshots/failed-shop.png" {
		t.Errorf("steps %+v attachments %+v", shop.Steps, shop.Attachments)
	}
	if r.Tests[0].Extra["profile"] == nil || r.Extra["runId"] != "216b61f6" || r.Extra["app"] == nil {
		t.Errorf("extra %+v / %+v", r.Tests[0].Extra, r.Extra)
	}
}

func TestWriteCTRF_RunThatFailedToStartIsAFailedTest(t *testing.T) {
	var buf bytes.Buffer
	run := parse(t, `TOMATO_EVENT:{"type":"run_error","phase":"app","status":"failed","error":"app port 8099 is already in use"}`)
	if err := WriteCTRF(&buf, run, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "tomato run: starting the app"`) || !strings.Contains(buf.String(), `"failed": 1`) {
		t.Errorf("a run that failed to start should be one failed test:\n%s", buf.String())
	}
}

func TestWriteJUnit(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJUnit(&buf, parse(t, events)); err != nil {
		t.Fatal(err)
	}
	var doc junitSuites
	if err := xml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("not XML: %v\n%s", err, buf.String())
	}
	if doc.Tests != 3 || doc.Failures != 1 || len(doc.Suites) != 2 {
		t.Fatalf("suites %+v", doc)
	}
	shop := doc.Suites[0].Cases[1]
	if shop.Failure == nil || shop.Failure.Message != `did not see "TMT-01" within 5s` || shop.Time != "5.500" {
		t.Errorf("case %+v", shop)
	}
	if !strings.Contains(shop.SystemOut, "[[ATTACHMENT|.tomato/screenshots/failed-shop.png]]") || !strings.Contains(shop.SystemOut, `skipped   "shopper" clicks "Track"`) {
		t.Errorf("system-out %q", shop.SystemOut)
	}
	if p := doc.Suites[0].Cases[0].Props; p == nil || p.Props[0] != (junitProp{"app.cpuMs", "40"}) {
		t.Errorf("properties %+v", p)
	}
}

func TestWrite_UnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, "tap", parse(t, events), ""); err == nil || !strings.Contains(err.Error(), "ctrf, junit") {
		t.Errorf("err %v", err)
	}
}
