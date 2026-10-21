package command

import (
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/registry"
)

// `tomato steps` and the generated docs fill {resource} with a name per step
// category. A new resource type without one would print "<resource>" in its
// examples, so every registered category has to have a name.
func TestEveryStepCategoryHasAnExampleName(t *testing.T) {
	for _, cat := range registry.AllStepCategories() {
		if _, ok := exampleResourceNames[cat.Name]; !ok {
			t.Errorf("no example resource name for step category %q: add one to exampleResourceNames", cat.Name)
		}
		for _, step := range cat.Steps {
			if got := exampleFor(cat.Name, step.Example); strings.Contains(got, "{resource}") {
				t.Errorf("%s: example still has a placeholder: %s", cat.Name, got)
			}
		}
	}
}
