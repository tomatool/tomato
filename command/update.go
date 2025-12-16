package command

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/version"
	"github.com/urfave/cli/v2"
)

// checkForUpdate checks if a newer version is available and prints a warning
// Returns silently if check fails or is disabled via TOMATO_SKIP_UPDATE_CHECK=true
func checkForUpdate() {
	if os.Getenv("TOMATO_SKIP_UPDATE_CHECK") == "true" || os.Getenv("TOMATO_SKIP_UPDATE_CHECK") == "1" {
		return
	}

	// Use a short timeout to avoid slowing down the command
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/tomatool/tomato/releases/latest")
	if err != nil {
		return // Silently fail
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	latestVersion := release.TagName
	currentVersion := version.Version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}

	// Skip if current version is a dev build or RC
	if currentVersion == "vdev" || strings.Contains(currentVersion, "-rc.") {
		return
	}

	if latestVersion != currentVersion && latestVersion > currentVersion {
		fmt.Printf("\n%s New version available: %s (current: %s)\n", warnStyle.Render("⚠"), latestVersion, currentVersion)
		fmt.Printf("  Run 'tomato update' to upgrade\n")
		fmt.Printf("  Set TOMATO_SKIP_UPDATE_CHECK=true to disable this check\n\n")
	}
}

var updateCommand = &cli.Command{
	Name:  "update",
	Usage: "Update tomato to the latest version",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "rc",
			Usage: "Update to latest release candidate",
		},
	},
	Action: func(c *cli.Context) error {
		useRC := c.Bool("rc")
		return runUpdate(useRC)
	},
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
}

func runUpdate(useRC bool) error {
	fmt.Println("Checking for updates...")

	// Get current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Detect platform
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Platform: %s\n", platform)
	fmt.Printf("Current version: %s\n", version.Version)

	// Get latest version
	latestVersion, err := getLatestVersion(useRC)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	if useRC {
		fmt.Printf("Latest RC version: %s\n", latestVersion)
	} else {
		fmt.Printf("Latest version: %s\n", latestVersion)
	}

	// Check if update is needed
	currentVersion := version.Version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}
	if currentVersion == latestVersion {
		fmt.Println("Already up to date!")
		return nil
	}

	// Download new version
	fmt.Printf("Downloading %s...\n", latestVersion)
	downloadURL := fmt.Sprintf(
		"https://github.com/tomatool/tomato/releases/download/%s/tomato_%s_%s.tar.gz",
		latestVersion,
		strings.TrimPrefix(latestVersion, "v"),
		platform,
	)

	tmpDir, err := os.MkdirTemp("", "tomato-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "tomato.tar.gz")
	if err := downloadFile(downloadURL, tarPath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Extract binary
	fmt.Println("Extracting...")
	binaryPath := filepath.Join(tmpDir, "tomato")
	if err := extractTarGz(tarPath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Replace current binary
	fmt.Println("Installing...")
	if err := replaceBinary(binaryPath, execPath); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	fmt.Printf("Successfully updated to %s!\n", latestVersion)
	return nil
}

func getLatestVersion(useRC bool) (string, error) {
	if useRC {
		return getLatestRCVersion()
	}
	return getLatestStableVersion()
}

func getLatestStableVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/tomatool/tomato/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

func getLatestRCVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/tomatool/tomato/releases")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}

	// Filter RC versions and sort by semver
	var rcVersions []string
	for _, r := range releases {
		if strings.Contains(r.TagName, "-rc.") {
			rcVersions = append(rcVersions, r.TagName)
		}
	}

	if len(rcVersions) == 0 {
		return "", fmt.Errorf("no release candidates found")
	}

	// Sort by semver (simple string sort works for vX.Y.Z-rc.N format)
	sort.Slice(rcVersions, func(i, j int) bool {
		return compareVersions(rcVersions[i], rcVersions[j]) < 0
	})

	return rcVersions[len(rcVersions)-1], nil
}

func compareVersions(a, b string) int {
	// Simple version comparison for vX.Y.Z-rc.N format
	return strings.Compare(a, b)
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

func replaceBinary(newBinary, oldBinary string) error {
	// Read new binary
	newData, err := os.ReadFile(newBinary)
	if err != nil {
		return err
	}

	// Get permissions of old binary
	info, err := os.Stat(oldBinary)
	if err != nil {
		return err
	}

	// Write new binary to old path
	if err := os.WriteFile(oldBinary, newData, info.Mode()); err != nil {
		return err
	}

	return nil
}
