package runlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFind(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if _, err := Find(""); err == nil {
		t.Error("no runs yet should be an error")
	}
	for _, name := range []string{"2026-10-23_050220_4d586811", "2026-10-23_050417_216b61f6"} {
		if err := os.MkdirAll(filepath.Join(".tomato", "runs", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for run, want := range map[string]string{
		"":                           "2026-10-23_050417_216b61f6",
		"4d586811":                   "2026-10-23_050220_4d586811",
		"2026-10-23_050220_4d586811": "2026-10-23_050220_4d586811",
	} {
		got, err := Find(run)
		if err != nil || got.Name != want {
			t.Errorf("Find(%q) = %q, %v; want %q", run, got.Name, err, want)
		}
	}
	for _, bad := range []string{"nope", "../../etc", "..", "a/b"} {
		if _, err := Find(bad); err == nil {
			t.Errorf("Find(%q) should fail", bad)
		}
	}
	got, _ := Find("216b61f6")
	if got.ID() != "216b61f6" || !got.Started().Equal(time.Date(2026, 10, 23, 5, 4, 17, 0, time.Local)) {
		t.Errorf("ID %q started %v", got.ID(), got.Started())
	}
}
