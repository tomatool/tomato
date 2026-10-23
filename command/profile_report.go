package command

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/tomatool/tomato/internal/profile"
)

// profileReportRows is how many scenarios the terminal report lists.
const profileReportRows = 10

// printProfileReport writes the run's profile to the terminal: the heaviest
// scenarios, the ones that made the app log errors, and what grew over the
// run, which is what a leak looks like. The whole profile is in path.
func printProfileReport(w io.Writer, r profile.Result, path string) {
	if len(r.Scenarios) == 0 {
		return
	}
	rows := make([]profile.NamedSummary, len(r.Scenarios))
	copy(rows, r.Scenarios)
	sort.SliceStable(rows, func(i, j int) bool { return appCPU(rows[i]) > appCPU(rows[j]) })
	if len(rows) > profileReportRows {
		rows = rows[:profileReportRows]
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, subtitleStyle.Render("Profile"))
	fmt.Fprintf(w, "  %-44s %7s %8s %9s %8s  %s\n", "scenario", "time", "app CPU", "peak mem", "Δ mem", "waiting on")
	for _, s := range rows {
		cpu, peak, delta := "-", "-", "-"
		if s.App != nil {
			cpu = fmtMs(s.App.CPUMs)
			peak = fmtBytes(s.App.PeakRSS)
			delta = fmtDelta(int64(s.App.RSSEnd) - int64(s.App.RSSStart))
		}
		fmt.Fprintf(w, "  %-44s %7s %8s %9s %8s  %s\n", truncate(s.Scenario, 44), fmtMs(s.DurationMs), cpu, peak, delta, waitingOn(s))
	}
	if len(r.Scenarios) > len(rows) {
		fmt.Fprintf(w, "  %s\n", helpStyle.Render(fmt.Sprintf("the %d heaviest of %d scenarios, by the app's CPU", len(rows), len(r.Scenarios))))
	}

	var logged []string
	for _, s := range r.Scenarios {
		if s.Logs != nil && s.Logs.Errors > 0 {
			logged = append(logged, fmt.Sprintf("%s: %s", s.Scenario, plural(s.Logs.Errors, "error")))
		}
	}
	if len(logged) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s\n", warnStyle.Render(fmt.Sprintf("The app logged errors in %s:", plural(len(logged), "scenario"))))
		for i, l := range logged {
			if i == 5 {
				fmt.Fprintf(w, "    … and %d more\n", len(logged)-5)
				break
			}
			fmt.Fprintf(w, "    %s\n", l)
		}
	}

	for _, g := range growth(r.Scenarios) {
		fmt.Fprintf(w, "  %s\n", warnStyle.Render(g))
	}
	fmt.Fprintf(w, "  %s\n", helpStyle.Render("full profile: "+path))
}

func appCPU(s profile.NamedSummary) int64 {
	if s.App == nil {
		return -1
	}
	return s.App.CPUMs
}

// waitingOn names the dependency the app waited on longest in the scenario.
func waitingOn(s profile.NamedSummary) string {
	var best string
	var bestMs int64 = -1
	var trips int
	for _, t := range s.Network {
		if t.WaitMs > bestMs {
			best, bestMs, trips = t.Container, t.WaitMs, t.RoundTrips
		}
	}
	if best == "" || bestMs <= 0 {
		return ""
	}
	return fmt.Sprintf("%s %s (%s)", best, fmtMs(bestMs), plural(trips, "round trip"))
}

// growth reports what kept growing from the first scenario to the last: the
// app's memory, its file descriptors, and the connections it holds open to
// each dependency. A pool that fills up once is fine; a number that climbs
// with every scenario is a leak.
func growth(all []profile.NamedSummary) []string {
	if len(all) < 3 {
		return nil
	}
	first, last := all[0], all[len(all)-1]
	var out []string
	if first.App != nil && last.App != nil {
		if d := int64(last.App.RSSEnd) - int64(first.App.RSSStart); d > 64<<20 {
			out = append(out, fmt.Sprintf("The app's memory grew by %s over the run (%s → %s).", fmtBytes(uint64(d)), fmtBytes(first.App.RSSStart), fmtBytes(last.App.RSSEnd)))
		}
		if d := last.App.FDsEnd - first.App.FDsStart; d > 50 {
			out = append(out, fmt.Sprintf("The app's open files grew by %d over the run (%d → %d).", d, first.App.FDsStart, last.App.FDsEnd))
		}
	}
	for name, l := range last.Network {
		f, ok := first.Network[name]
		if ok && l.Open-f.Open > 10 {
			out = append(out, fmt.Sprintf("The app's open connections to %s grew from %d to %d over the run.", l.Container, f.Open, l.Open))
		}
	}
	sort.Strings(out)
	return out
}

func fmtMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", ms)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return d.Round(time.Second).String()
	}
}

func fmtBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func fmtDelta(d int64) string {
	if d < 0 {
		return "-" + fmtBytes(uint64(-d))
	}
	return "+" + fmtBytes(uint64(d))
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
