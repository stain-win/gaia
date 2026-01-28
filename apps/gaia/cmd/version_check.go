package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
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
	defer resp.Body.Close()

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
		fmt.Sscanf(parts[i], "%d", &result[i])
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
