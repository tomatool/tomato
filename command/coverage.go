package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v26"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/handler"
	"github.com/urfave/cli/v2"
)

var coverageCommand = &cli.Command{
	Name:  "coverage",
	Usage: "Report which resource steps your feature files use",
	Description: `Matches every step in the feature files against the step definitions of
each resource in tomato.yml, and reports how many of each resource type's
steps are used at least once. Scenarios excluded by features.tags are skipped.

Use --min to fail when coverage drops below a percentage, and --all-types to
include resource types the config does not declare (they count as 0%).`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "tomato.yml",
			Usage:   "path to config file",
		},
		&cli.StringFlag{
			Name:  "format",
			Value: "text",
			Usage: "output format: text, markdown, json",
		},
		&cli.Float64Flag{
			Name:  "min",
			Usage: "fail if any resource type (or the total, with --min-total) is below this percentage",
		},
		&cli.BoolFlag{
			Name:  "all-types",
			Usage: "include every resource type tomato supports, even those not declared in the config",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "write the report to a file instead of stdout",
		},
	},
	Action: runCoverage,
}

// TypeCoverage is the step coverage of one resource type.
type TypeCoverage struct {
	Type      string   `json:"type"`
	Name      string   `json:"name"`
	Resources []string `json:"resources"`
	Total     int      `json:"total"`
	Covered   int      `json:"covered"`
	Uncovered []string `json:"uncovered"`
}

func (t TypeCoverage) Percent() float64 {
	if t.Total == 0 {
		return 100
	}
	return 100 * float64(t.Covered) / float64(t.Total)
}

// CoverageReport is the result of `tomato coverage`.
type CoverageReport struct {
	Types     []TypeCoverage `json:"types"`
	Total     int            `json:"total"`
	Covered   int            `json:"covered"`
	Unmatched []string       `json:"unmatched_steps"`
}

func (r CoverageReport) Percent() float64 {
	if r.Total == 0 {
		return 100
	}
	return 100 * float64(r.Covered) / float64(r.Total)
}

func runCoverage(c *cli.Context) error {
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return err
	}
	steps, err := featureSteps(cfg, filepath.Dir(c.String("config")))
	if err != nil {
		return err
	}
	report := computeCoverage(cfg, steps, c.Bool("all-types"))

	var out io.Writer = os.Stdout
	if path := c.String("output"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	switch c.String("format") {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	case "markdown":
		writeCoverageMarkdown(out, report)
	default:
		writeCoverageText(out, report)
	}

	if min := c.Float64("min"); min > 0 {
		var below []string
		for _, t := range report.Types {
			if t.Percent() < min {
				below = append(below, fmt.Sprintf("%s %.1f%%", t.Type, t.Percent()))
			}
		}
		if len(below) > 0 {
			return fmt.Errorf("step coverage below %.0f%%: %s", min, strings.Join(below, ", "))
		}
	}
	return nil
}

// featureSteps returns the text of every step that will run: outlines are
// expanded, and scenarios excluded by features.tags are dropped.
func featureSteps(cfg *config.Config, configDir string) ([]string, error) {
	excluded := excludedTags(cfg.Features.Tags)
	var steps []string
	for _, root := range cfg.Features.Paths {
		if _, err := os.Stat(root); err != nil && !filepath.IsAbs(root) {
			root = filepath.Join(configDir, root)
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".feature") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			ids := &messages.Incrementing{}
			doc, err := gherkin.ParseGherkinDocument(f, ids.NewId)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
			for _, pickle := range gherkin.Pickles(*doc, path, ids.NewId) {
				if pickleExcluded(pickle, excluded) {
					continue
				}
				for _, s := range pickle.Steps {
					steps = append(steps, s.Text)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return steps, nil
}

// excludedTags reads the tags a features.tags expression excludes: "~@tag",
// "not @tag". Other expressions include everything.
func excludedTags(expr string) map[string]bool {
	out := map[string]bool{}
	fields := strings.Fields(strings.NewReplacer(",", " ", "(", " ", ")", " ").Replace(expr))
	for i, f := range fields {
		switch {
		case strings.HasPrefix(f, "~@"):
			out[f[1:]] = true
		case f == "not" && i+1 < len(fields) && strings.HasPrefix(fields[i+1], "@"):
			out[fields[i+1]] = true
		}
	}
	return out
}

func pickleExcluded(p *messages.Pickle, excluded map[string]bool) bool {
	for _, t := range p.Tags {
		if excluded[t.Name] {
			return true
		}
	}
	return false
}

// canonicalTypes maps every resource type to one name per step category, so
// aliases (postgresql, http-client, minio, ...) are counted once.
func canonicalTypes() map[string]string {
	byCategory := map[string]string{}
	canonical := map[string]string{}
	for _, typ := range handler.ValidResourceTypes() {
		cat, ok := handler.StepCategoryForType(typ)
		if !ok {
			continue
		}
		// The shortest name is the canonical one (http over http-client).
		if cur, seen := byCategory[cat.Name]; !seen || len(typ) < len(cur) {
			byCategory[cat.Name] = typ
		}
	}
	for _, typ := range handler.ValidResourceTypes() {
		if cat, ok := handler.StepCategoryForType(typ); ok {
			canonical[typ] = byCategory[cat.Name]
		}
	}
	return canonical
}

func computeCoverage(cfg *config.Config, steps []string, allTypes bool) CoverageReport {
	canonical := canonicalTypes()

	resourcesByType := map[string][]string{}
	for name, res := range cfg.Resources {
		if typ, ok := canonical[res.Type]; ok {
			resourcesByType[typ] = append(resourcesByType[typ], name)
		}
	}
	types := map[string]bool{}
	for typ := range resourcesByType {
		types[typ] = true
	}
	if allTypes {
		for _, typ := range canonical {
			types[typ] = true
		}
	}

	matched := make([]bool, len(steps))
	var report CoverageReport
	for typ := range types {
		cat, _ := handler.StepCategoryForType(typ)
		names := resourcesByType[typ]
		sort.Strings(names)
		tc := TypeCoverage{Type: typ, Name: cat.Name, Resources: names, Total: len(cat.Steps)}
		for _, def := range cat.Steps {
			covered := false
			for _, name := range names {
				re, err := regexp.Compile(strings.ReplaceAll(def.Pattern, "{resource}", regexp.QuoteMeta(name)))
				if err != nil {
					continue
				}
				for i, text := range steps {
					if re.MatchString(text) {
						covered = true
						matched[i] = true
					}
				}
			}
			if covered {
				tc.Covered++
			} else {
				tc.Uncovered = append(tc.Uncovered, strings.ReplaceAll(def.Pattern, "{resource}", "<resource>"))
			}
		}
		report.Types = append(report.Types, tc)
		report.Total += tc.Total
		report.Covered += tc.Covered
	}
	sort.Slice(report.Types, func(i, j int) bool { return report.Types[i].Type < report.Types[j].Type })

	seen := map[string]bool{}
	for i, text := range steps {
		if !matched[i] && !seen[text] {
			seen[text] = true
			report.Unmatched = append(report.Unmatched, text)
		}
	}
	return report
}

func writeCoverageText(w io.Writer, r CoverageReport) {
	fmt.Fprintf(w, "Step coverage: %d/%d (%.1f%%)\n\n", r.Covered, r.Total, r.Percent())
	for _, t := range r.Types {
		fmt.Fprintf(w, "  %-18s %3d/%-3d %6.1f%%\n", t.Type, t.Covered, t.Total, t.Percent())
		for _, u := range t.Uncovered {
			fmt.Fprintf(w, "      missing: %s\n", u)
		}
	}
}

func writeCoverageMarkdown(w io.Writer, r CoverageReport) {
	fmt.Fprintf(w, "| Resource | Steps covered | Coverage |\n|---|---|---|\n")
	for _, t := range r.Types {
		mark := "✅"
		if t.Covered < t.Total {
			mark = "❌"
		}
		fmt.Fprintf(w, "| %s `%s` | %d / %d | %s %.1f%% |\n", t.Name, t.Type, t.Covered, t.Total, mark, t.Percent())
	}
	fmt.Fprintf(w, "| **Total** | **%d / %d** | **%.1f%%** |\n", r.Covered, r.Total, r.Percent())

	var missing []string
	for _, t := range r.Types {
		for _, u := range t.Uncovered {
			missing = append(missing, fmt.Sprintf("- `%s`: `%s`", t.Type, u))
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(w, "\n<details><summary>%d steps not used by any scenario</summary>\n\n%s\n\n</details>\n", len(missing), strings.Join(missing, "\n"))
	}
}
