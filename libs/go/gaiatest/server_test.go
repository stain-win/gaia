package gaiatest_test

import (
	"os"
	"testing"

	"github.com/stain-win/gaia/libs/go/gaiatest"
)

// TestNewServer_StartsAndStops verifies that NewServer starts a reachable daemon
// and Cleanup shuts it down. Requires GAIA_BIN or gaia in PATH.
func TestNewServer_StartsAndStops(t *testing.T) {
	if _, ok := os.LookupEnv("GAIA_BIN"); !ok {
		t.Skip("GAIA_BIN not set; skipping integration test")
	}

	srv := gaiatest.NewServer(t)
	if srv.Addr == "" {
		t.Error("expected non-empty Addr")
	}
	if srv.CertDir == "" {
		t.Error("expected non-empty CertDir")
	}
}
