package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// JUnit XML, as Jenkins, GitLab, CircleCI and Azure DevOps read it: a
// testsuite per feature and a testcase per scenario. Beyond pass and fail it
// carries the failed step and its error in <failure>, the steps in
// <system-out> with each screenshot as [[ATTACHMENT|path]] (which Jenkins
// and GitLab show), and what the scenario cost the app as <properties>.
type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string          `xml:"name,attr"`
	File      string          `xml:"file,attr,omitempty"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	Timestamp string          `xml:"timestamp,attr,omitempty"`
	Props     *junitPropsList `xml:"properties,omitempty"`
	Cases     []junitCase     `xml:"testcase"`
}

type junitCase struct {
	Name      string          `xml:"name,attr"`
	Classname string          `xml:"classname,attr"`
	File      string          `xml:"file,attr,omitempty"`
	Time      string          `xml:"time,attr"`
	Props     *junitPropsList `xml:"properties,omitempty"`
	Failure   *junitProblem   `xml:"failure,omitempty"`
	Error     *junitProblem   `xml:"error,omitempty"`
	Skipped   *struct{}       `xml:"skipped,omitempty"`
	SystemOut string          `xml:"system-out,omitempty"`
}

type junitProblem struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

type junitPropsList struct {
	Props []junitProp `xml:"property"`
}

type junitProp struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func secs(ms int64) string { return fmt.Sprintf("%.3f", float64(ms)/1000) }

// WriteJUnit writes the run as JUnit XML. A run that failed before any
// scenario is a suite named tomato with one errored case for the phase.
func WriteJUnit(w io.Writer, run *Run) error {
	all := junitSuites{Name: "tomato"}
	var totalMs int64
	for _, f := range run.Features {
		suite := junitSuite{Name: f.Name, File: f.File}
		var ms int64
		for _, s := range f.Scenarios {
			c := junitCase{Name: s.Name, Classname: f.Name, File: f.File, Time: secs(s.DurationMs)}
			if suite.Timestamp == "" && !s.Start.IsZero() {
				suite.Timestamp = s.Start.UTC().Format("2006-01-02T15:04:05")
			}
			switch {
			case s.Status == "failed" || s.FailedStep() != nil:
				st := s.FailedStep()
				text, msg := "", "scenario failed"
				if st != nil {
					msg = firstLine(st.Error)
					text = fmt.Sprintf("step %d: %s\n\n%s", st.Index+1, st.Text, st.Error)
				}
				c.Failure = &junitProblem{Message: msg, Type: "step", Text: text}
				suite.Failures++
			case s.Status == "skipped":
				c.Skipped = &struct{}{}
				suite.Skipped++
			case s.Status == "other":
				c.Error = &junitProblem{Message: "the run stopped while this scenario was running", Type: "interrupted"}
				suite.Errors++
			}
			c.SystemOut = stepsOut(s)
			c.Props = profileProps(s)
			suite.Cases = append(suite.Cases, c)
			suite.Tests++
			ms += s.DurationMs
		}
		suite.Time = secs(ms)
		totalMs += ms
		all.Suites = append(all.Suites, suite)
	}

	if run.Error != nil {
		all.Suites = append(all.Suites, junitSuite{
			Name: "tomato", Tests: 1, Errors: 1, Time: "0.000",
			Cases: []junitCase{{
				Name: phaseName(run.Error.Phase), Classname: "tomato", Time: "0.000",
				Error:     &junitProblem{Message: firstLine(run.Error.Message), Type: run.Error.Phase, Text: run.Error.Message},
				SystemOut: strings.Join(run.Error.Logs, "\n"),
			}},
		})
	}

	for _, s := range all.Suites {
		all.Tests += s.Tests
		all.Failures += s.Failures
		all.Errors += s.Errors
		all.Skipped += s.Skipped
	}
	all.Time = secs(totalMs)

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(all); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// stepsOut is a scenario's steps as its <system-out>: one line each with
// its status and time, the error under a failed one, and the screenshots in
// the attachment syntax CI servers pick up.
func stepsOut(s *Scenario) string {
	var b strings.Builder
	for _, st := range s.Steps {
		fmt.Fprintf(&b, "%-9s %s (%dms)\n", st.Status, st.Text, st.DurationMs)
		if st.Error != "" && st.Status != "passed" {
			for _, line := range strings.Split(st.Error, "\n") {
				fmt.Fprintf(&b, "          %s\n", line)
			}
		}
		for _, shot := range st.Screenshots {
			fmt.Fprintf(&b, "[[ATTACHMENT|%s]]\n", shot.Path)
		}
	}
	return b.String()
}

// profileProps is what the scenario cost the app, as properties.
func profileProps(s *Scenario) *junitPropsList {
	p := s.Profile
	if p == nil {
		return nil
	}
	var props []junitProp
	add := func(name string, v any) { props = append(props, junitProp{Name: name, Value: fmt.Sprint(v)}) }
	if p.App != nil {
		add("app.cpuMs", p.App.CPUMs)
		add("app.peakRssBytes", p.App.PeakRSS)
		add("app.rssGrowthBytes", int64(p.App.RSSEnd)-int64(p.App.RSSStart))
	}
	for name, t := range p.Network {
		add("dependency."+name+".waitMs", t.WaitMs)
		add("dependency."+name+".roundTrips", t.RoundTrips)
	}
	if p.Logs != nil {
		add("app.loggedErrors", p.Logs.Errors)
		add("app.loggedWarnings", p.Logs.Warnings)
	}
	if len(props) == 0 {
		return nil
	}
	return &junitPropsList{Props: props}
}
