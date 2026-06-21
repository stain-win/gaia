package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{name: "latest newer than current", v1: "1.2.3", v2: "1.2.4", want: -1},
		{name: "current equals latest", v1: "v1.2.3", v2: "1.2.3", want: 0},
		{name: "prerelease sorts before release", v1: "1.2.3-rc.1", v2: "1.2.3", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(normalizeVersion(tt.v1), normalizeVersion(tt.v2)); got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestFindReleaseAssetMatchesSupportedPlatforms(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.2.3",
		Assets: []GitHubAsset{
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
			{Name: "gaia_v1.2.3_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.test/linux-amd64.tar.gz"},
			{Name: "gaia_v1.2.3_Linux_arm64.tar.gz", BrowserDownloadURL: "https://example.test/linux-arm64.tar.gz"},
			{Name: "gaia_v1.2.3_Darwin_x86_64.tar.gz", BrowserDownloadURL: "https://example.test/darwin-amd64.tar.gz"},
			{Name: "gaia_v1.2.3_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.test/darwin-arm64.tar.gz"},
		},
	}

	tests := []struct {
		name    string
		goos    string
		goarch  string
		wantURL string
	}{
		{name: "linux amd64", goos: "linux", goarch: "amd64", wantURL: "https://example.test/linux-amd64.tar.gz"},
		{name: "linux arm64", goos: "linux", goarch: "arm64", wantURL: "https://example.test/linux-arm64.tar.gz"},
		{name: "darwin amd64", goos: "darwin", goarch: "amd64", wantURL: "https://example.test/darwin-amd64.tar.gz"},
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", wantURL: "https://example.test/darwin-arm64.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := findReleaseAsset(release, tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("findReleaseAsset returned error: %v", err)
			}
			if asset.BrowserDownloadURL != tt.wantURL {
				t.Fatalf("asset URL = %q, want %q", asset.BrowserDownloadURL, tt.wantURL)
			}
		})
	}
}

func TestFindReleaseAssetRejectsUnsupportedPlatform(t *testing.T) {
	release := &GitHubRelease{TagName: "v1.2.3"}

	_, err := findReleaseAsset(release, "freebsd", "amd64")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if !strings.Contains(err.Error(), "manual update") {
		t.Fatalf("error = %q, want manual update guidance", err.Error())
	}
}

func TestInstallReleaseArchiveFailsChecksumMismatchBeforeReplacement(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "gaia_v1.2.3_Linux_x86_64.tar.gz")
	writeGaiaArchive(t, archivePath, []byte("new-binary"))

	targetPath := filepath.Join(dir, "gaia")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}

	err := installReleaseArchive(archivePath, []byte("0000000000000000000000000000000000000000000000000000000000000000  "+filepath.Base(archivePath)+"\n"), targetPath)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %q, want checksum failure", err.Error())
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("target was replaced despite checksum failure: %q", got)
	}
}

func TestInstallReleaseArchiveReplacesExecutableWith0755Mode(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "gaia_v1.2.3_Darwin_arm64.tar.gz")
	writeGaiaArchive(t, archivePath, []byte("new-binary"))

	targetPath := filepath.Join(dir, "gaia")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	sum := sha256.Sum256(mustReadFile(t, archivePath))
	checksums := []byte(fmt.Sprintf("%x  %s\n", sum, filepath.Base(archivePath)))

	if err := installReleaseArchive(archivePath, checksums, targetPath); err != nil {
		t.Fatalf("installReleaseArchive returned error: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("target content = %q, want new binary", got)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o755 {
		t.Fatalf("target mode = %o, want 0755", gotMode)
	}
}

func writeGaiaArchive(t *testing.T, archivePath string, content []byte) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name:    "gaia",
		Mode:    0o755,
		Size:    int64(len(content)),
		ModTime: time.Unix(0, 0),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
