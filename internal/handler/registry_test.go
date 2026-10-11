package handler

import (
	"slices"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

// TestValidResourceTypes_MatchesWhatCanBeBuilt pins the invariant that broke
// once already: every type `tomato validate` accepts must be a type
// `tomato run` can actually construct, and vice versa.
//
// When these were two hand-maintained lists, the grpc resource was added to
// the constructor and not to the valid-types list, so `run` worked and
// `validate` rejected the config as unknown — a failure that only showed up
// in CI, because the GitHub Action validates before running.
func TestValidResourceTypes_MatchesWhatCanBeBuilt(t *testing.T) {
	r := &Registry{}
	for _, typ := range ValidResourceTypes() {
		if _, err := r.createHandler("res", config.Resource{Type: typ}); err != nil {
			t.Errorf("type %q is advertised as valid but cannot be constructed: %v", typ, err)
		}
	}

	if _, err := r.createHandler("res", config.Resource{Type: "definitely-not-a-resource"}); err == nil {
		t.Error("an unknown type must not construct")
	}
}

// TestValidResourceTypes_CoversEveryHandler guards the other direction: a
// resource that can be built but is not advertised is invisible to
// validation and to the error message that lists the options.
func TestValidResourceTypes_CoversEveryHandler(t *testing.T) {
	valid := ValidResourceTypes()
	for typ := range handlerFactories {
		if !slices.Contains(valid, typ) {
			t.Errorf("type %q can be constructed but is not in ValidResourceTypes", typ)
		}
	}
	if len(valid) != len(handlerFactories) {
		t.Errorf("ValidResourceTypes has %d entries, the factory table has %d", len(valid), len(handlerFactories))
	}
}

// TestValidResourceTypes_IsSorted keeps the "Valid types: ..." suggestion
// stable, rather than reshuffling with Go's map iteration order every run.
func TestValidResourceTypes_IsSorted(t *testing.T) {
	got := ValidResourceTypes()
	if !slices.IsSorted(got) {
		t.Errorf("ValidResourceTypes must be sorted for a stable error message, got %v", got)
	}
}

// TestContainerBasedTypes_AreValid stops the container-requirement list
// naming a type that no longer exists.
func TestContainerBasedTypes_AreValid(t *testing.T) {
	valid := ValidResourceTypes()
	for _, typ := range ContainerBasedTypes() {
		if !slices.Contains(valid, typ) {
			t.Errorf("ContainerBasedTypes names %q, which is not a valid resource type", typ)
		}
	}
}

// TestGRPCIsRegistered is the specific regression: the grpc resource must be
// both constructible and advertised.
func TestGRPCIsRegistered(t *testing.T) {
	for _, typ := range []string{"grpc", "grpc-client"} {
		if !slices.Contains(ValidResourceTypes(), typ) {
			t.Errorf("%q must be advertised as a valid resource type", typ)
		}
		h, err := (&Registry{}).createHandler("grpc", config.Resource{Type: typ})
		if err != nil {
			t.Errorf("%q must be constructible: %v", typ, err)
			continue
		}
		if _, ok := h.(*GRPC); !ok {
			t.Errorf("%q built a %T, want *GRPC", typ, h)
		}
	}
}

// TestUnimplementedTypes_FailLoudly guards against the old no-op stubs coming
// back: a type tomato can't drive must be rejected with a hint, never
// constructed as a handler that silently does nothing.
func TestUnimplementedTypes_FailLoudly(t *testing.T) {
	r := &Registry{}
	for typ := range unimplementedTypes {
		if slices.Contains(ValidResourceTypes(), typ) {
			t.Errorf("unimplemented type %q must not be advertised as valid", typ)
		}
		_, err := r.createHandler("res", config.Resource{Type: typ})
		if err == nil {
			t.Errorf("unimplemented type %q must not construct", typ)
			continue
		}
		hint, _ := UnimplementedTypeHint(typ)
		if !strings.Contains(err.Error(), hint) {
			t.Errorf("error for %q should carry the hint %q, got %q", typ, hint, err)
		}
	}
}

// TestCanonicalTypesCoverEveryResource pins the second half of the same
// invariant. handlerFactories decides what can be built; canonicalTypes
// decides what gets documented and listed by `tomato steps`. A resource added
// to the first and not the second builds and runs fine, so nothing fails —
// it just has no documentation page and cannot be found with `tomato steps`,
// which is the kind of gap that survives review.
func TestCanonicalTypesCoverEveryResource(t *testing.T) {
	documented := map[string]bool{}
	for _, cat := range AllStepCategories() {
		documented[cat.Name] = true
	}

	for typ := range handlerFactories {
		cat, ok := StepCategoryForType(typ)
		if !ok {
			continue // not a step provider; nothing to document
		}
		if !documented[cat.Name] {
			t.Errorf("resource type %q (%q) is buildable but absent from canonicalTypes, "+
				"so it has no docs page and `tomato steps` cannot find it", typ, cat.Name)
		}
	}
}

// TestCanonicalTypesAreBuildableAndUnique guards the other direction: a typo
// in canonicalTypes would silently drop a resource from the docs, because
// AllStepCategories skips a type it cannot build.
func TestCanonicalTypesAreBuildableAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, typ := range canonicalTypes {
		if _, ok := handlerFactories[typ]; !ok {
			t.Errorf("canonicalTypes has %q, which is not a buildable resource type", typ)
		}
		if seen[typ] {
			t.Errorf("canonicalTypes lists %q twice", typ)
		}
		seen[typ] = true
	}
	if got, want := len(canonicalTypes), len(AllStepCategories()); got != want {
		t.Errorf("canonicalTypes has %d entries but only %d produced a step category", got, want)
	}
}
