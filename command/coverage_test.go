package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestExcludedTags(t *testing.T) {
	got := excludedTags("~@skip-dynamic-port and not @wip, @smoke")
	for _, tag := range []string{"@skip-dynamic-port", "@wip"} {
		if !got[tag] {
			t.Errorf("%s should be excluded, got %v", tag, got)
		}
	}
	if got["@smoke"] {
		t.Error("@smoke is included, not excluded")
	}
	if len(excludedTags("")) != 0 {
		t.Error("an empty expression excludes nothing")
	}
}

func writeFeature(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageCountsStepsPerResourceType(t *testing.T) {
	dir := t.TempDir()
	writeFeature(t, dir, "a.feature", `Feature: a
  Scenario: set and check a key
    Given "cache" key "k" is "v"
    Then "cache" key "k" exists

  Scenario Outline: outlines are expanded
    Then "shell" runs "<cmd>"
    Examples:
      | cmd  |
      | true |

  @skip
  Scenario: excluded by tags
    Then "cache" key "k" does not exist

  Scenario: a step no resource defines
    Then nothing matches this
`)
	cfg := &config.Config{
		Resources: map[string]config.Resource{
			"cache": {Type: "redis"},
			"shell": {Type: "shell"},
		},
		Features: config.Features{Paths: []string{dir}, Tags: "~@skip"},
	}
	steps, err := featureSteps(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 {
		t.Fatalf("expected 4 runnable steps (outline expanded, @skip dropped), got %d: %v", len(steps), steps)
	}

	report := computeCoverage(cfg, steps, false)
	byType := map[string]TypeCoverage{}
	for _, tc := range report.Types {
		byType[tc.Type] = tc
	}
	if len(byType) != 2 {
		t.Fatalf("only declared types are reported without --all-types, got %v", report.Types)
	}
	if byType["redis"].Covered != 2 || byType["shell"].Covered != 1 {
		t.Errorf("covered: redis=%d shell=%d, want 2 and 1", byType["redis"].Covered, byType["shell"].Covered)
	}
	for _, u := range byType["redis"].Uncovered {
		if strings.Contains(u, "{resource}") {
			t.Errorf("uncovered patterns should show <resource>, got %q", u)
		}
	}
	if len(report.Unmatched) != 1 || report.Unmatched[0] != "nothing matches this" {
		t.Errorf("unmatched steps = %v", report.Unmatched)
	}
	if report.Percent() <= 0 || report.Percent() >= 100 {
		t.Errorf("total percent = %.1f", report.Percent())
	}

	all := computeCoverage(cfg, steps, true)
	if len(all.Types) <= 2 {
		t.Error("--all-types should include resource types the config doesn't declare")
	}
}

func TestCoverageAliasesCountOnce(t *testing.T) {
	canonical := canonicalTypes()
	if canonical["postgresql"] != canonical["postgres"] || canonical["http-client"] != "http" {
		t.Errorf("aliases should map to one canonical type: %v", canonical)
	}
	if _, ok := canonical["mysql"]; ok {
		t.Error("unimplemented types are not resource types")
	}
}

func TestCoverageWriters(t *testing.T) {
	r := CoverageReport{
		Types: []TypeCoverage{
			{Type: "redis", Name: "Redis", Total: 2, Covered: 2},
			{Type: "shell", Name: "Shell", Total: 2, Covered: 1, Uncovered: []string{`^"<resource>" stderr is empty$`}},
		},
		Total: 4, Covered: 3,
	}
	var text, md strings.Builder
	writeCoverageText(&text, r)
	writeCoverageMarkdown(&md, r)
	if !strings.Contains(text.String(), "3/4 (75.0%)") || !strings.Contains(text.String(), "missing:") {
		t.Errorf("text report:\n%s", text.String())
	}
	if !strings.Contains(md.String(), "| Shell `shell` | 1 / 2 | ❌ 50.0% |") || !strings.Contains(md.String(), "<details>") {
		t.Errorf("markdown report:\n%s", md.String())
	}
	if (TypeCoverage{}).Percent() != 100 || (CoverageReport{}).Percent() != 100 {
		t.Error("nothing to cover counts as fully covered")
	}
}
