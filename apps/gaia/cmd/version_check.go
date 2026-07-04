package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	// GitHub API endpoint for releases
	githubReleasesAPI = "https://api.github.com/repos/stain-win/gaia/releases/latest"
	// GitHub releases download URL pattern
	githubDownloadURL = "https://github.com/stain-win/gaia/releases/download/%s/gaia_%s_%s_%s.tar.gz"
	// Check timeout
	versionCheckTimeout = 10 * time.Second
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	PublishedAt time.Time     `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a downloadable release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Version styles
var (
	versionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED"))

	versionCurrentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF"))

	versionNewStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981"))

	versionUpToDateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981"))

	versionBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#10B981")).
			Padding(1, 2).
			MarginTop(1)

	versionUpdateBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#F59E0B")).
				Padding(1, 2).
				MarginTop(1)
)

func runVersionCheck(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println(versionTitleStyle.Render("🔍 Checking for Gaia updates..."))
	fmt.Println()

	release, err := getLatestRelease()
	if err != nil {
		return NewGaiaError(
			ErrCodeNetwork,
			"Failed to check for updates",
			err,
		).WithHint("Check your internet connection").
			WithHint("You can manually check: https://github.com/stain-win/gaia/releases")
	}

	currentVersion := normalizeVersion(version)
	latestVersion := normalizeVersion(release.TagName)

	fmt.Printf("  Current version: %s\n", versionCurrentStyle.Render(currentVersion))
	fmt.Printf("  Latest version:  %s\n", versionNewStyle.Render(latestVersion))
	fmt.Println()

	comparison := compareVersions(currentVersion, latestVersion)
	if updateInstall {
		return runVersionInstall(release, currentVersion, latestVersion, comparison)
	}

	switch comparison {
	case 0:
		// Up to date
		upToDateBox := versionBoxStyle.Render(
			versionUpToDateStyle.Render("✓ You're running the latest version of Gaia!"),
		)
		fmt.Println(upToDateBox)

	case -1:
		// Update available
		printUpdateAvailable(release, currentVersion)
		offerUpdateInstructions(release)

	case 1:
		// Running newer than release (dev version)
		fmt.Println(versionCurrentStyle.Render("ℹ You're running a development version"))
	}

	fmt.Println()
	return nil
}

func runVersionInstall(release *GitHubRelease, currentVersion, latestVersion string, comparison int) error {
	if version == "dev" && !updateForce {
		return NewGaiaError(
			ErrCodeUnknown,
			"Refusing to self-update a development build",
			nil,
		).WithHint("Run 'gaia update --install --force' if you intentionally want to replace this dev build").
			WithHint("For server installs, prefer: ansible-playbook -i inventories/production/hosts.yml update.yml")
	}

	if comparison == 0 && !updateForce {
		fmt.Println(versionBoxStyle.Render(versionUpToDateStyle.Render("✓ You're running the latest version of Gaia.")))
		fmt.Println("Use --force to reinstall the current release.")
		return nil
	}

	if comparison > 0 && !updateForce {
		return NewGaiaError(
			ErrCodeUnknown,
			"Refusing to replace a newer local version",
			nil,
		).WithHint("Use --force to install the latest GitHub release anyway").
			WithHint(fmt.Sprintf("Manual release page: %s", release.HTMLURL))
	}

	asset, err := findReleaseAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	checksumAsset, err := findChecksumAsset(release)
	if err != nil {
		return err
	}

	executablePath, err := os.Executable()
	if err != nil {
		return NewGaiaError(ErrCodeUnknown, "Could not determine current Gaia executable path", err)
	}

	if runtime.GOOS == "linux" && isGaiaSystemdServiceActive() {
		fmt.Println(versionCurrentStyle.Render("⚠ Gaia systemd service is active; Ansible is preferred for service-managed installs."))
		fmt.Println(versionCurrentStyle.Render("  ansible-playbook -i inventories/production/hosts.yml update.yml"))
		fmt.Println()
	}

	if !updateYes {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Install Gaia %s over %s?", release.TagName, currentVersion)).
					Value(&confirmed).
					Affirmative("Install").
					Negative("Cancel"),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Update cancelled.")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "gaia-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, asset.Name)
	checksumsPath := filepath.Join(tmpDir, checksumAsset.Name)

	fmt.Printf("Downloading %s...\n", asset.Name)
	if err := downloadFile(ctx, asset.BrowserDownloadURL, archivePath); err != nil {
		return NewGaiaError(ErrCodeNetwork, "Failed to download Gaia release asset", err).
			WithHint(fmt.Sprintf("Manual download: %s", asset.BrowserDownloadURL))
	}
	if err := downloadFile(ctx, checksumAsset.BrowserDownloadURL, checksumsPath); err != nil {
		return NewGaiaError(ErrCodeNetwork, "Failed to download Gaia checksums", err).
			WithHint(fmt.Sprintf("Manual download: %s", checksumAsset.BrowserDownloadURL))
	}

	checksums, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}

	// Authenticate checksums.txt before trusting it to validate the archive.
	// Without this, the checksum only protects against corruption, not tampering.
	if bundleAsset, ok := findSignatureBundleAsset(release); ok {
		if updateSkipSignature {
			fmt.Println(versionCurrentStyle.Render("⚠ Signature verification SKIPPED (--skip-signature)."))
		} else {
			bundlePath := filepath.Join(tmpDir, bundleAsset.Name)
			if err := downloadFile(ctx, bundleAsset.BrowserDownloadURL, bundlePath); err != nil {
				return NewGaiaError(ErrCodeNetwork, "Failed to download release signature bundle", err).
					WithHint(fmt.Sprintf("Manual download: %s", bundleAsset.BrowserDownloadURL))
			}
			if err := verifyChecksumSignature(bundlePath, checksums); err != nil {
				return NewGaiaError(
					ErrCodeUnknown,
					"Release signature verification FAILED — refusing to install",
					err,
				).WithHint("The release may have been tampered with, or Sigstore infrastructure is unreachable").
					WithHint("If you have independently verified this release, re-run with --skip-signature")
			}
			fmt.Println(versionUpToDateStyle.Render("✓ Release signature verified (sigstore)"))
		}
	} else {
		fmt.Println(versionCurrentStyle.Render("⚠ This release is not signed (published before signing was introduced)."))
		fmt.Println(versionCurrentStyle.Render("  Proceeding with checksum-only verification."))
	}

	if err := installReleaseArchive(archivePath, checksums, executablePath); err != nil {
		if isPermissionError(err) {
			return NewGaiaError(ErrCodePermission, "Permission denied while replacing Gaia executable", err).
				WithHint(fmt.Sprintf("Run with elevated permissions: sudo %s update --install --yes", shellQuote(executablePath))).
				WithHint(fmt.Sprintf("For Ansible-managed servers: ansible-playbook -i inventories/production/hosts.yml update.yml --extra-vars \"gaia_version=%s\"", release.TagName))
		}
		return err
	}

	fmt.Println(versionBoxStyle.Render(versionUpToDateStyle.Render(fmt.Sprintf("✓ Gaia updated to %s", release.TagName))))
	return nil
}

func getLatestRelease() (*GitHubRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesAPI, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("Gaia/%s", version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func normalizeVersion(v string) string {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")
	// Remove any build metadata after +
	if idx := strings.Index(v, "+"); idx != -1 {
		v = v[:idx]
	}
	return v
}

// compareVersions compares two semver strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	for i := 0; i < 3; i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	// Check pre-release (e.g., -rc.1)
	pre1 := getPreRelease(v1)
	pre2 := getPreRelease(v2)

	if pre1 == "" && pre2 != "" {
		return 1 // Release > pre-release
	}
	if pre1 != "" && pre2 == "" {
		return -1 // Pre-release < release
	}

	return 0
}

func parseVersion(v string) [3]int {
	// Remove pre-release suffix
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		if _, err := fmt.Sscanf(parts[i], "%d", &result[i]); err != nil {
			result[i] = 0
		}
	}
	return result
}

func getPreRelease(v string) string {
	if idx := strings.Index(v, "-"); idx != -1 {
		return v[idx+1:]
	}
	return ""
}

func printUpdateAvailable(release *GitHubRelease, currentVersion string) {
	updateContent := fmt.Sprintf(
		"%s\n\n"+
			"A new version of Gaia is available!\n\n"+
			"  Current: %s\n"+
			"  Latest:  %s\n"+
			"  Released: %s\n\n"+
			"%s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Render("⬆ Update Available"),
		currentVersion,
		versionNewStyle.Render(release.TagName),
		release.PublishedAt.Format("January 2, 2006"),
		versionCurrentStyle.Render("Release notes: "+release.HTMLURL),
	)

	fmt.Println(versionUpdateBoxStyle.Render(updateContent))
}

func offerUpdateInstructions(release *GitHubRelease) {
	fmt.Println()

	var showInstructions bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Show update instructions?").
				Value(&showInstructions).
				Affirmative("Yes").
				Negative("No"),
		),
	)

	if err := form.Run(); err != nil {
		return
	}

	if !showInstructions {
		return
	}

	fmt.Println()
	fmt.Println(versionTitleStyle.Render("📦 Update Instructions"))
	fmt.Println()

	// Determine the right asset name for this OS/arch
	osName := getOSName()
	archName := getArchName()
	assetName := fmt.Sprintf("gaia_%s_%s_%s", release.TagName, osName, archName)

	// Find download URL
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, osName) && strings.Contains(asset.Name, archName) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		downloadURL = fmt.Sprintf(githubDownloadURL, release.TagName, release.TagName, osName, archName)
	}

	switch runtime.GOOS {
	case "darwin":
		printMacOSUpdateInstructions(release.TagName, downloadURL, assetName)
	case "linux":
		printLinuxUpdateInstructions(release.TagName, downloadURL, assetName)
	case "windows":
		printWindowsUpdateInstructions(release.TagName)
	default:
		printGenericUpdateInstructions(release.TagName, downloadURL)
	}
}

func getOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func getArchName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

func findReleaseAsset(release *GitHubRelease, goos, goarch string) (GitHubAsset, error) {
	osName, archName, err := platformReleaseNames(goos, goarch)
	if err != nil {
		return GitHubAsset{}, err
	}

	expectedName := fmt.Sprintf("gaia_%s_%s_%s.tar.gz", release.TagName, osName, archName)
	for _, asset := range release.Assets {
		if asset.Name == expectedName {
			return asset, nil
		}
	}

	return GitHubAsset{}, NewGaiaError(
		ErrCodeUnknown,
		fmt.Sprintf("No Gaia release asset found for %s/%s", goos, goarch),
		nil,
	).WithHint("Use a manual update from https://github.com/stain-win/gaia/releases").
		WithHint(fmt.Sprintf("Expected asset: %s", expectedName))
}

func findChecksumAsset(release *GitHubRelease) (GitHubAsset, error) {
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			return asset, nil
		}
	}

	return GitHubAsset{}, NewGaiaError(
		ErrCodeUnknown,
		"Release checksums.txt asset was not found",
		nil,
	).WithHint("Use a manual update from https://github.com/stain-win/gaia/releases")
}

func platformReleaseNames(goos, goarch string) (string, string, error) {
	var osName string
	switch goos {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	default:
		return "", "", NewGaiaError(
			ErrCodeUnknown,
			fmt.Sprintf("Self-update is not supported on %s/%s; please use a manual update", goos, goarch),
			nil,
		).WithHint("Please use a manual update from https://github.com/stain-win/gaia/releases")
	}

	var archName string
	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		return "", "", NewGaiaError(
			ErrCodeUnknown,
			fmt.Sprintf("Self-update is not supported on %s/%s; please use a manual update", goos, goarch),
			nil,
		).WithHint("Please use a manual update from https://github.com/stain-win/gaia/releases")
	}

	return osName, archName, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Gaia/%s", version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}

func installReleaseArchive(archivePath string, checksums []byte, targetPath string) error {
	if err := verifyArchiveChecksum(archivePath, checksums); err != nil {
		return err
	}

	targetDir := filepath.Dir(targetPath)
	tmpDir, err := os.MkdirTemp(targetDir, ".gaia-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpBinary := filepath.Join(tmpDir, filepath.Base(targetPath))
	if err := extractGaiaBinary(archivePath, tmpBinary); err != nil {
		return err
	}
	if err := os.Chmod(tmpBinary, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpBinary, targetPath); err != nil {
		return err
	}

	return os.Chmod(targetPath, 0o755)
}

func verifyArchiveChecksum(archivePath string, checksums []byte) error {
	archiveName := filepath.Base(archivePath)
	want, err := checksumForAsset(checksums, archiveName)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", archiveName, got, want)
	}

	return nil
}

func checksumForAsset(checksums []byte, assetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(checksums)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(name) == assetName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("checksum for %s not found", assetName)
}

func extractGaiaBinary(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.FileInfo().IsDir() || filepath.Base(header.Name) != "gaia" {
			continue
		}

		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	return fmt.Errorf("gaia binary not found in %s", archivePath)
}

func isGaiaSystemdServiceActive() bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", "gaia")
	return cmd.Run() == nil
}

func isPermissionError(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func printMacOSUpdateInstructions(version, downloadURL, assetName string) {
	instructions := fmt.Sprintf(`
%s

# Option 1: Download and install manually
curl -LO %s.tar.gz
tar -xzf %s.tar.gz
sudo mv gaia /usr/local/bin/

# Option 2: If installed via Homebrew (future)
# brew upgrade gaia

# Verify the update
gaia --version
`,
		wizardInfoStyle.Render("For macOS:"),
		downloadURL,
		assetName,
	)
	fmt.Println(instructions)
}

func printLinuxUpdateInstructions(version, downloadURL, assetName string) {
	instructions := fmt.Sprintf(`
%s

# Download the latest release
wget %s.tar.gz

# Extract
tar -xzf %s.tar.gz

# Install (may require sudo)
sudo mv gaia /usr/local/bin/

# Verify the update
gaia --version

# If running as a service, restart it
sudo systemctl restart gaia
`,
		wizardInfoStyle.Render("For Linux:"),
		downloadURL,
		assetName,
	)
	fmt.Println(instructions)
}

func printWindowsUpdateInstructions(version string) {
	instructions := fmt.Sprintf(`
%s

1. Download the latest release from:
   https://github.com/stain-win/gaia/releases/latest

2. Extract the ZIP file

3. Replace the existing gaia.exe in your PATH

4. Verify the update:
   gaia --version
`,
		wizardInfoStyle.Render("For Windows:"),
	)
	fmt.Println(instructions)
}

func printGenericUpdateInstructions(version, downloadURL string) {
	instructions := fmt.Sprintf(`
%s

1. Download the latest release for your platform:
   https://github.com/stain-win/gaia/releases/latest

2. Extract and replace your existing gaia binary

3. Verify the update:
   gaia --version
`,
		wizardInfoStyle.Render("Generic instructions:"),
	)
	fmt.Println(instructions)
}

// checkAndOfferUpdate is called during setup to check for updates
func checkAndOfferUpdate() {
	release, err := getLatestRelease()
	if err != nil {
		fmt.Println(wizardInfoStyle.Render("  (Could not check for updates)"))
		return
	}

	currentVersion := normalizeVersion(version)
	latestVersion := normalizeVersion(release.TagName)

	if compareVersions(currentVersion, latestVersion) < 0 {
		fmt.Println(wizardWarningStyle.Render(fmt.Sprintf("  ⬆ Update available: %s → %s", currentVersion, latestVersion)))
		fmt.Println(wizardInfoStyle.Render("    Run 'gaia version check' after setup to update"))
	} else {
		fmt.Println(wizardSuccessStyle.Render("  ✓ You're running the latest version"))
	}
}
