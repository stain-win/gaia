package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSignatureBundleAsset(t *testing.T) {
	tests := []struct {
		name      string
		assets    []GitHubAsset
		wantFound bool
	}{
		{
			name: "bundle present among other assets",
			assets: []GitHubAsset{
				{Name: "gaia_1.0.0_Linux_x86_64.tar.gz"},
				{Name: "checksums.txt"},
				{Name: sigstoreBundleAssetName},
			},
			wantFound: true,
		},
		{
			name: "pre-signing release has no bundle",
			assets: []GitHubAsset{
				{Name: "gaia_1.0.0_Linux_x86_64.tar.gz"},
				{Name: "checksums.txt"},
			},
			wantFound: false,
		},
		{
			name:      "no assets at all",
			assets:    nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &GitHubRelease{Assets: tt.assets}
			asset, found := findSignatureBundleAsset(release)
			if found != tt.wantFound {
				t.Fatalf("findSignatureBundleAsset() found = %v, want %v", found, tt.wantFound)
			}
			if found && asset.Name != sigstoreBundleAssetName {
				t.Fatalf("unexpected asset name %q", asset.Name)
			}
		})
	}
}

// The verification failure paths below must not require network access:
// a bundle that cannot even be parsed has to be rejected before any
// trust-root fetching happens.

func TestVerifyChecksumSignature_MissingFile(t *testing.T) {
	err := verifyChecksumSignature(filepath.Join(t.TempDir(), "nope.sigstore.json"), []byte("data"))
	if err == nil {
		t.Fatal("expected error for missing bundle file")
	}
}

func TestVerifyChecksumSignature_GarbageBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "checksums.txt.sigstore.json")
	if err := os.WriteFile(bundlePath, []byte("not a sigstore bundle"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := verifyChecksumSignature(bundlePath, []byte("data"))
	if err == nil {
		t.Fatal("expected error for malformed bundle")
	}
}

func TestVerifyChecksumSignature_EmptyJSONBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "checksums.txt.sigstore.json")
	if err := os.WriteFile(bundlePath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := verifyChecksumSignature(bundlePath, []byte("data"))
	if err == nil {
		t.Fatal("expected error for empty JSON bundle")
	}
}
