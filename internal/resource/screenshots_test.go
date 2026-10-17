package resource

import (
	"reflect"
	"testing"
)

func TestScreenshotsAreTakenOncePerScenario(t *testing.T) {
	RecordScreenshot("A", Screenshot{Resource: "alice", Path: "a/alice.png"})
	RecordScreenshot("A", Screenshot{Resource: "bob", Path: "a/bob.png"})
	RecordScreenshot("B", Screenshot{Resource: "alice", Path: "b/alice.png"})

	got := TakeScreenshots("A")
	want := []Screenshot{{Resource: "alice", Path: "a/alice.png"}, {Resource: "bob", Path: "a/bob.png"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TakeScreenshots(A) = %v, want %v", got, want)
	}
	// Taking forgets: the next step of A starts with none, and B's are
	// untouched by it.
	if again := TakeScreenshots("A"); again != nil {
		t.Errorf("a second take returned %v, want nothing", again)
	}
	if got := TakeScreenshots("B"); len(got) != 1 || got[0].Path != "b/alice.png" {
		t.Errorf("TakeScreenshots(B) = %v", got)
	}
	if got := TakeScreenshots("never ran"); got != nil {
		t.Errorf("an unknown scenario returned %v", got)
	}
}
