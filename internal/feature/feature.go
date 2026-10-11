// Package feature finds .feature files and the scenarios inside them.
//
// It exists so the runner can turn a scenario filter into the exact set of
// scenarios godog should run, instead of starting every scenario and skipping
// the ones that do not match.
package feature

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/gherkin/go/v26"
	messages "github.com/cucumber/messages/go/v21"
)

// Files returns the .feature files under root, or root itself when it is one.
// A root that does not exist yields no files rather than an error: godog
// reports a bad path on its own, and a filter should fail with what it was
// looking for, not with what it was looking in.
func Files(root string) ([]string, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if strings.HasSuffix(root, ".feature") {
			return []string{root}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".feature") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// Match is one scenario a filter selected, addressed the way godog wants it.
type Match struct {
	Name string
	// Path is "file.feature:12", which godog resolves to the scenario
	// declared on that line. For a Scenario Outline that is the outline
	// itself, so every example row runs.
	Path string
}

// Select returns the scenarios under paths whose name matches re.
//
// It reports no match as an error. Returning "nothing to run" as success is
// how a renamed scenario turns into a green run that tested nothing.
func Select(paths []string, re *regexp.Regexp) ([]Match, error) {
	var matches []Match
	var scanned int

	for _, root := range paths {
		files, err := Files(root)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", root, err)
		}
		for _, file := range files {
			found, err := scenariosIn(file, re)
			if err != nil {
				return nil, err
			}
			scanned++
			matches = append(matches, found...)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no scenario matches %q (searched %d feature file(s) under %s)",
			re.String(), scanned, strings.Join(paths, ", "))
	}
	return matches, nil
}

func scenariosIn(file string, re *regexp.Regexp) ([]Match, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", file, err)
	}
	defer f.Close()

	doc, err := gherkin.ParseGherkinDocument(f, (&messages.Incrementing{}).NewId)
	if err != nil {
		// A file being edited into an invalid state should not stop a run
		// that does not target it.
		return nil, nil
	}
	if doc.Feature == nil {
		return nil, nil
	}

	var matches []Match
	for _, child := range doc.Feature.Children {
		if child.Scenario == nil || !re.MatchString(child.Scenario.Name) {
			continue
		}
		matches = append(matches, Match{
			Name: child.Scenario.Name,
			Path: fmt.Sprintf("%s:%d", file, child.Scenario.Location.Line),
		})
	}
	return matches, nil
}
