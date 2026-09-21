package handler

import (
	"slices"
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
