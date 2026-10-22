package command

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/profile"
	"github.com/tomatool/tomato/internal/resource"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func scenario(name string, cpuMs int64, rssStart, rssEnd uint64, fds int32, open int) profile.NamedSummary {
	return profile.NamedSummary{Scenario: name, Summary: profile.Summary{
		DurationMs: cpuMs * 2,
		App:        &profile.AppSummary{CPUMs: cpuMs, PeakRSS: rssEnd, RSSStart: rssStart, RSSEnd: rssEnd, FDsStart: fds, FDsEnd: fds},
		Network: map[string]resource.DependencyTraffic{
			"postgres-network": {Container: "postgres", WaitMs: cpuMs / 2, RoundTrips: 3, Open: open},
		},
	}}
}

func TestProfileReportListsTheHeaviestScenarios(t *testing.T) {
	var all []profile.NamedSummary
	for i := 0; i < 12; i++ {
		all = append(all, scenario(fmt.Sprintf("scenario %02d", i), int64(100*i), 50<<20, 50<<20, 20, 2))
	}
	all[3].Logs = &profile.LogSummary{Errors: 2}
	var b bytes.Buffer
	printProfileReport(&b, profile.Result{Scenarios: all}, ".tomato/runs/x/profile.json")
	out := ansi.ReplaceAllString(b.String(), "")

	if !strings.Contains(out, "scenario 11") || strings.Contains(out, "scenario 01 ") {
		t.Errorf("not the ten heaviest:\n%s", out)
	}
	if strings.Index(out, "scenario 11") > strings.Index(out, "scenario 10") {
		t.Errorf("not heaviest first:\n%s", out)
	}
	for _, want := range []string{
		"postgres 550ms (3 round trips)",
		"the 10 heaviest of 12 scenarios",
		"The app logged errors in 1 scenario:",
		"scenario 03: 2 errors",
		"full profile: .tomato/runs/x/profile.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "grew") {
		t.Errorf("nothing grew, yet:\n%s", out)
	}
}

func TestProfileReportNamesWhatGrewOverTheRun(t *testing.T) {
	all := []profile.NamedSummary{
		scenario("first", 10, 50<<20, 60<<20, 20, 2),
		scenario("middle", 10, 60<<20, 150<<20, 60, 9),
		scenario("last", 10, 150<<20, 200<<20, 90, 30),
	}
	var b bytes.Buffer
	printProfileReport(&b, profile.Result{Scenarios: all}, "p.json")
	out := ansi.ReplaceAllString(b.String(), "")
	for _, want := range []string{
		"The app's memory grew by 150 MB over the run (50 MB → 200 MB).",
		"The app's open files grew by 70 over the run (20 → 90).",
		"The app's open connections to postgres grew from 2 to 30 over the run.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestProfileReportWithNoScenariosSaysNothing(t *testing.T) {
	var b bytes.Buffer
	printProfileReport(&b, profile.Result{}, "p.json")
	if b.Len() != 0 {
		t.Errorf("%q", b.String())
	}
}

func TestFormats(t *testing.T) {
	for in, want := range map[int64]string{5: "5ms", 1500: "1.5s", 125000: "2m5s"} {
		if got := fmtMs(in); got != want {
			t.Errorf("fmtMs(%d) = %q, want %q", in, got, want)
		}
	}
	if got := fmtDelta(-3 << 20); got != "-3 MB" {
		t.Errorf("fmtDelta = %q", got)
	}
	if got := truncate("a long scenario name", 8); got != "a long …" {
		t.Errorf("truncate = %q", got)
	}
}
