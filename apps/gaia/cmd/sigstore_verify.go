package cmd

import (
	"bytes"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	// sigstoreBundleAssetName is the release asset holding the Sigstore bundle
	// for checksums.txt (certificate + signature + transparency-log proof).
	sigstoreBundleAssetName = "checksums.txt.sigstore.json"

	// Release signatures must come from this repository's GitHub Actions
	// release workflow, authenticated through GitHub's OIDC issuer. Anything
	// else — even a validly signed bundle from another repository — is rejected.
	sigstoreExpectedIssuer  = "https://token.actions.githubusercontent.com"
	sigstoreExpectedSANRegx = `^https://github\.com/stain-win/gaia/`
)

// findSignatureBundleAsset locates the Sigstore bundle for checksums.txt in a
// release. Releases published before signing was introduced do not have one;
// callers decide how to treat that (warn for now, since those releases exist).
func findSignatureBundleAsset(release *GitHubRelease) (GitHubAsset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == sigstoreBundleAssetName {
			return asset, true
		}
	}
	return GitHubAsset{}, false
}

// verifyChecksumSignature authenticates the checksums.txt content against its
// Sigstore bundle. It verifies, using the public Sigstore trust root (fetched
// via TUF and cached locally):
//   - the signature over the artifact matches the certificate's public key,
//   - the certificate chains to Fulcio and was logged in the Rekor
//     transparency log while valid (observer timestamp from the log entry),
//   - the certificate identity is this repository's GitHub Actions workflow.
//
// Only after this returns nil should the checksums be trusted to validate the
// downloaded archive.
func verifyChecksumSignature(bundlePath string, artifact []byte) error {
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to parse signature bundle: %w", err)
	}

	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("failed to fetch Sigstore trust root (TUF): %w", err)
	}

	verifier, err := verify.NewVerifier(
		trustedRoot,
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("failed to construct signature verifier: %w", err)
	}

	identity, err := verify.NewShortCertificateIdentity(
		sigstoreExpectedIssuer, "", "", sigstoreExpectedSANRegx,
	)
	if err != nil {
		return fmt.Errorf("failed to build expected certificate identity: %w", err)
	}

	_, err = verifier.Verify(b, verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(artifact)),
		verify.WithCertificateIdentity(identity),
	))
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}
