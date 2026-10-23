package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/tomatool/tomato/internal/report"
	"github.com/tomatool/tomato/internal/runlog"
	"github.com/tomatool/tomato/internal/version"
	"github.com/urfave/cli/v2"
)

var reportCommand = &cli.Command{
	Name:      "report",
	Usage:     "Write a report of a run in a standard format",
	ArgsUsage: "[run]",
	Description: `Writes a run's report from the events it recorded in .tomato/runs: the
most recent run, or the one named by its id (as "📋 run:" prints it) or its
directory name. Any run can be reported, in any format, whatever its output
setting was.

Formats:
  ctrf    Common Test Report Format JSON (https://ctrf.io), with the failed
          step, screenshots and the scenario's profile in each test's extra
  junit   JUnit XML, with the failed step, the steps and screenshots in
          system-out, and the profile as properties`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "ctrf",
			Usage:   "Report format: " + strings.Join(report.Formats, ", "),
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "File to write, instead of stdout",
		},
	},
	Action: writeReport,
}

func writeReport(c *cli.Context) error {
	runArg, format, path, err := reportArgs(c.Args().Slice(), c.String("format"), c.String("output"))
	if err != nil {
		return err
	}
	info, err := runlog.Find(runArg)
	if err != nil {
		return err
	}
	run, err := report.Load(info.Dir, info.ID(), info.Started())
	if err != nil {
		return fmt.Errorf("run %s: %w", info.ID(), err)
	}

	if path == "" {
		return report.Write(os.Stdout, format, run, version.Version)
	}
	if err := writeReportFile(path, format, run); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote the %s report of run %s to %s\n", format, info.ID(), path)
	return nil
}

// reportArgs reads the run and the flags written after it: the flag parser
// stops at the first argument, and `tomato report 216b61f6 -f junit` is how
// it is naturally written.
func reportArgs(args []string, format, path string) (run, outFormat, outPath string, err error) {
	outFormat, outPath = format, path
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		switch {
		case !strings.HasPrefix(a, "-"):
			if run != "" {
				return "", "", "", fmt.Errorf("report takes one run, got %q and %q", run, a)
			}
			run = a
			continue
		case name != "f" && name != "format" && name != "o" && name != "output":
			return "", "", "", fmt.Errorf("unknown flag %s", a)
		case !hasValue:
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("flag %s needs a value", a)
			}
			i++
			value = args[i]
		}
		if name == "f" || name == "format" {
			outFormat = value
		} else {
			outPath = value
		}
	}
	return run, outFormat, outPath, nil
}
