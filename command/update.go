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

	tea "github.com/charmbracelet/bubbletea"
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
		&cli.BoolFlag{
			Name:    "select",
			Aliases: []string{"s"},
			Usage:   "Select from the last 5 releases",
		},
	},
	Action: func(c *cli.Context) error {
		useRC := c.Bool("rc")
		useSelect := c.Bool("select")
		return runUpdate(useRC, useSelect)
	},
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}

// Update UI model
type updateStep int

const (
	updateStepLoading updateStep = iota
	updateStepSelect
	updateStepConfirm
	updateStepUpdating
	updateStepDone
)

type updateModel struct {
	step           updateStep
	cursor         int
	releases       []githubRelease
	selectedVer    string
	currentVersion string
	platform       string
	execPath       string
	err            error
	done           bool
	cancelled      bool
	useSelect      bool
	useRC          bool
	statusMsg      string
}

type releasesLoadedMsg struct {
	releases []githubRelease
	err      error
}

type updateDoneMsg struct {
	err error
}

func initialUpdateModel(useRC, useSelect bool) updateModel {
	currentVersion := version.Version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}

	execPath, _ := os.Executable()
	execPath, _ = filepath.EvalSymlinks(execPath)
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	return updateModel{
		step:           updateStepLoading,
		currentVersion: currentVersion,
		platform:       platform,
		execPath:       execPath,
		useSelect:      useSelect,
		useRC:          useRC,
		statusMsg:      "Fetching releases...",
	}
}

func (m updateModel) Init() tea.Cmd {
	return m.fetchReleases()
}

func (m updateModel) fetchReleases() tea.Cmd {
	return func() tea.Msg {
		releases, err := getRecentReleases(m.useRC, 5)
		return releasesLoadedMsg{releases: releases, err: err}
	}
}

func (m updateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.step != updateStepUpdating {
				m.cancelled = true
				return m, tea.Quit
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := m.getMaxCursor()
			if m.cursor < max {
				m.cursor++
			}
		case "enter":
			return m.handleEnter()
		case "esc":
			if m.step == updateStepConfirm && m.useSelect {
				m.step = updateStepSelect
				m.cursor = 0
			} else if m.step == updateStepConfirm {
				m.cancelled = true
				return m, tea.Quit
			}
		}

	case releasesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.releases = msg.releases
		if len(m.releases) == 0 {
			m.err = fmt.Errorf("no releases found")
			return m, tea.Quit
		}

		if m.useSelect {
			m.step = updateStepSelect
		} else {
			m.selectedVer = m.releases[0].TagName
			if m.selectedVer == m.currentVersion {
				m.statusMsg = "Already up to date!"
				m.done = true
				return m, tea.Quit
			}
			m.step = updateStepConfirm
		}

	case updateDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.done = true
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m updateModel) getMaxCursor() int {
	switch m.step {
	case updateStepSelect:
		return len(m.releases) - 1
	case updateStepConfirm:
		return 1
	default:
		return 0
	}
}

func (m updateModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case updateStepSelect:
		m.selectedVer = m.releases[m.cursor].TagName
		if m.selectedVer == m.currentVersion {
			m.statusMsg = "Already on this version!"
			m.done = true
			return m, tea.Quit
		}
		m.step = updateStepConfirm
		m.cursor = 0

	case updateStepConfirm:
		if m.cursor == 0 {
			m.step = updateStepUpdating
			m.statusMsg = "Downloading..."
			return m, m.performUpdate()
		}
		m.cancelled = true
		return m, tea.Quit
	}

	return m, nil
}

func (m updateModel) performUpdate() tea.Cmd {
	return func() tea.Msg {
		err := downloadAndInstall(m.selectedVer, m.platform, m.execPath)
		return updateDoneMsg{err: err}
	}
}

func (m updateModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🍅 Tomato Update"))
	s.WriteString("\n\n")

	// Show platform and version info
	infoStyle := helpStyle
	s.WriteString(infoStyle.Render(fmt.Sprintf("Platform: %s", m.platform)))
	s.WriteString("\n")
	s.WriteString(infoStyle.Render(fmt.Sprintf("Current:  %s", m.currentVersion)))
	s.WriteString("\n\n")

	switch m.step {
	case updateStepLoading:
		s.WriteString(subtitleStyle.Render(m.statusMsg))

	case updateStepSelect:
		s.WriteString(subtitleStyle.Render("Select version to install:"))
		s.WriteString("\n\n")

		for i, r := range m.releases {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}

			label := r.TagName
			if r.TagName == m.currentVersion {
				label += " (current)"
			}
			if i == 0 {
				label += " (latest)"
			}

			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(label)))
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("↑/↓ navigate • ENTER select • q quit"))

	case updateStepConfirm:
		s.WriteString(subtitleStyle.Render("Confirm update"))
		s.WriteString("\n\n")

		s.WriteString(fmt.Sprintf("Update from %s to %s?\n\n",
			warnStyle.Render(m.currentVersion),
			successStyle.Render(m.selectedVer)))

		options := []string{"Yes, update", "Cancel"}
		for i, opt := range options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			style := unselectedStyle
			if i == m.cursor {
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt)))
		}

		s.WriteString("\n")
		if m.useSelect {
			s.WriteString(helpStyle.Render("ENTER confirm • ESC back • q quit"))
		} else {
			s.WriteString(helpStyle.Render("ENTER confirm • ESC/q cancel"))
		}

	case updateStepUpdating:
		s.WriteString(subtitleStyle.Render(m.statusMsg))
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("Installing %s...\n", m.selectedVer))

	case updateStepDone:
		s.WriteString(successStyle.Render("✓ Update complete!"))
	}

	return s.String()
}

func runUpdate(useRC, useSelect bool) error {
	m := initialUpdateModel(useRC, useSelect)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running update: %w", err)
	}

	finalModel := result.(updateModel)

	if finalModel.err != nil {
		return finalModel.err
	}

	if finalModel.cancelled {
		fmt.Println("\nCancelled.")
		return nil
	}

	if finalModel.done {
		if finalModel.statusMsg == "Already up to date!" || finalModel.statusMsg == "Already on this version!" {
			fmt.Println("\n" + successStyle.Render("✓ "+finalModel.statusMsg))
		} else {
			fmt.Println("\n" + successStyle.Render(fmt.Sprintf("✓ Successfully updated to %s!", finalModel.selectedVer)))
		}
	}

	return nil
}

func downloadAndInstall(version, platform, execPath string) error {
	downloadURL := fmt.Sprintf(
		"https://github.com/tomatool/tomato/releases/download/%s/tomato_%s_%s.tar.gz",
		version,
		strings.TrimPrefix(version, "v"),
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

	if err := extractTarGz(tarPath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	binaryPath := filepath.Join(tmpDir, "tomato")
	if err := replaceBinary(binaryPath, execPath); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	return nil
}

func getRecentReleases(includeRC bool, limit int) ([]githubRelease, error) {
	resp, err := http.Get("https://api.github.com/repos/tomatool/tomato/releases")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	var filtered []githubRelease
	for _, r := range releases {
		isRC := strings.Contains(r.TagName, "-rc.")
		if includeRC || !isRC {
			filtered = append(filtered, r)
		}
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
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
