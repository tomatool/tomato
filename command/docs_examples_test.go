package command

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/registry"
)

var (
	fencedBlock = regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)```")
	gherkinStep = regexp.MustCompile(`^\s*(?:Given|When|Then|And|But)\s+(.+?)\s*$`)
)

// Every Gherkin step printed in the documentation has to be a step tomato
// actually registers. Three in docs/configuration/s3.md were not — they used
// `"db" table "x" with values:` and `response code should be`, wording from
// before a rename — and they had been published that way for as long as the
// page existed, because nothing read the docs against the code.
//
// docs/resources/ is skipped: `tomato docs` generates it from the same step
// definitions, so it cannot disagree with them.
func TestDocsExamplesUseRealSteps(t *testing.T) {
	patterns := stepPatterns(t)

	var checked int
	err := filepath.Walk("../docs", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		if strings.Contains(filepath.ToSlash(path), "/docs/resources/") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, block := range fencedBlock.FindAllStringSubmatch(string(body), -1) {
			if !strings.Contains(block[1], "Scenario") && !strings.Contains(block[1], "Feature:") {
				continue
			}
			for _, line := range strings.Split(block[1], "\n") {
				m := gherkinStep.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				step := m[1]
				// table rows and docstring fences are not steps
				if strings.HasPrefix(step, "|") || strings.HasSuffix(step, `"""`) {
					continue
				}
				checked++
				if !matchesAny(patterns, step) {
					t.Errorf("%s: no step definition matches\n    %s\n  run `tomato steps` for the real wording", path, step)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs: %v", err)
	}
	if checked == 0 {
		t.Fatal("no Gherkin steps found in docs/; this test would pass vacuously")
	}
	t.Logf("checked %d steps", checked)
}

func stepPatterns(t *testing.T) []*regexp.Regexp {
	t.Helper()
	var out []*regexp.Regexp
	for _, cat := range registry.AllStepCategories() {
		for _, step := range cat.Steps {
			p := strings.ReplaceAll(step.Pattern, "{resource}", `[^"]+`)
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				continue
			}
			out = append(out, re)
		}
	}
	if len(out) == 0 {
		t.Fatal("no step patterns; this test would pass vacuously")
	}
	return out
}

func matchesAny(patterns []*regexp.Regexp, s string) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

var mdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// A link to a page that no longer exists ships silently: the docs workflow
// regenerates and deploys rather than validating, so nothing notices. Deleting
// `examples/` left Getting Started pointing at it, and deleting the agent
// skills left the nav pointing at a removed page — both published.
func TestDocsHaveNoDeadInternalLinks(t *testing.T) {
	var checked, dead int
	err := filepath.Walk("../docs", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range mdLink.FindAllStringSubmatch(string(body), -1) {
			target := m[2]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if i := strings.Index(target, "#"); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			checked++
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
				dead++
				t.Errorf("%s: link [%s](%s) points at nothing", path, m[1], m[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs: %v", err)
	}
	if checked == 0 {
		t.Fatal("no internal links found; this test would pass vacuously")
	}
	t.Logf("checked %d internal links, %d dead", checked, dead)
}

// Every page has to be reachable from the nav, and every nav entry has to
// exist. A page outside the nav is invisible on the site; a nav entry without
// a page breaks the build.
func TestDocsNavMatchesPages(t *testing.T) {
	nav, err := os.ReadFile("../mkdocs.yml")
	if err != nil {
		t.Fatalf("reading mkdocs.yml: %v", err)
	}
	listed := map[string]bool{}
	for _, m := range regexp.MustCompile(`([A-Za-z0-9/._-]+\.md)`).FindAllStringSubmatch(string(nav), -1) {
		listed[m[1]] = true
	}
	if len(listed) == 0 {
		t.Fatal("no pages found in the nav; this test would pass vacuously")
	}

	onDisk := map[string]bool{}
	_ = filepath.Walk("../docs", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		rel, _ := filepath.Rel("../docs", path)
		onDisk[filepath.ToSlash(rel)] = true
		return nil
	})

	for page := range listed {
		if !onDisk[page] {
			t.Errorf("mkdocs.yml lists %s, which does not exist", page)
		}
	}
	for page := range onDisk {
		if !listed[page] {
			t.Errorf("docs/%s is not in the mkdocs nav, so it is invisible on the site", page)
		}
	}
}
