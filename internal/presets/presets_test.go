package presets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKafkaImage_ReleaseUsesThePublishedImage(t *testing.T) {
	for version, want := range map[string]string{
		"2.2.0":      KafkaImageRepo + ":2.2.0",
		"v2.2.0":     KafkaImageRepo + ":2.2.0",
		"2.2.0-rc.1": KafkaImageRepo + ":2.2.0-rc.1",
	} {
		image, buildContext, err := KafkaImage(version)
		if err != nil {
			t.Fatalf("%s: %v", version, err)
		}
		if image != want || buildContext != "" {
			t.Errorf("%s: got image %q, context %q; want image %q", version, image, buildContext, want)
		}
	}
}

func TestKafkaImage_DevelopmentBuildGetsABuildContext(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // os.UserCacheDir on Linux
	t.Setenv("HOME", t.TempDir())           // and on macOS

	for _, version := range []string{"dev", "2.1.3-next", ""} {
		image, dir, err := KafkaImage(version)
		if err != nil {
			t.Fatalf("%s: %v", version, err)
		}
		if image != "" {
			t.Errorf("%s: got image %q, want a build context", version, image)
		}
		for _, file := range []string{
			"Dockerfile",
			"src/io/github/tomatool/mskiam/MskIamSaslServer.java",
			"src/io/github/tomatool/mskiam/MskIamLoginModule.java",
			"src/io/github/tomatool/mskiam/MskIamSaslServerProvider.java",
		} {
			if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
				t.Errorf("%s: build context lacks %s: %v", version, file, err)
			}
		}
	}

	// The context is written once per content and then reused.
	_, first, _ := KafkaImage("dev")
	_, second, _ := KafkaImage("dev")
	if first != second {
		t.Errorf("the build context moved between calls: %s then %s", first, second)
	}
}
