// Package presets holds what tomato needs to run a preset container, starting
// with the image of the kafka preset.
package presets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// KafkaImageRepo is where release builds of tomato find the kafka preset image;
// the release workflow publishes it under each release's version.
const KafkaImageRepo = "ghcr.io/tomatool/tomato-kafka"

// kafkaImage is the kafka preset's build context: apache/kafka plus the
// AWS_MSK_IAM SASL server in kafka/src. The release workflow builds the
// published image from this same directory.
//
//go:embed kafka
var kafkaImage embed.FS

var releaseVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-rc\.\d+)?$`)

// KafkaImage returns what the kafka preset runs for a tomato of the given
// version: the published image for a release, or for any other build (which
// has no published image) a build context extracted from the binary, for the
// container manager to build. Exactly one of the two is set.
func KafkaImage(version string) (image, buildContext string, err error) {
	if releaseVersion.MatchString(version) {
		return KafkaImageRepo + ":" + strings.TrimPrefix(version, "v"), "", nil
	}
	dir, err := extract(kafkaImage, "kafka")
	if err != nil {
		return "", "", fmt.Errorf("extracting the kafka preset image: %w", err)
	}
	return "", dir, nil
}

// extract writes the embedded directory root to a cache directory named after
// its contents, so it is written once per version of the files and a build
// cache keyed on the path stays warm.
func extract(files embed.FS, root string) (string, error) {
	var paths []string
	contents := make(map[string][]byte)
	err := fs.WalkDir(files, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		paths = append(paths, path)
		contents[path] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(contents[p]))
		h.Write(contents[p])
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "tomato", "presets", root+"-"+hex.EncodeToString(h.Sum(nil))[:12])
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		return dir, nil
	}

	// Write next to the final directory and rename, so a half-written context is
	// never mistaken for a complete one.
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for _, p := range paths {
		dst := filepath.Join(tmp, filepath.FromSlash(strings.TrimPrefix(p, root+"/")))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, contents[p], 0o644); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another run got there first; theirs is identical.
		if _, statErr := os.Stat(filepath.Join(dir, "Dockerfile")); statErr == nil {
			return dir, nil
		}
		return "", err
	}
	return dir, nil
}
