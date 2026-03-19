// Package gaiatest provides test helpers for starting an ephemeral Gaia daemon
// in unit and integration tests.
//
// Usage:
//
//	func TestMyFeature(t *testing.T) {
//	    srv := gaiatest.NewServer(t)
//	    // srv.Addr  — gRPC address ("127.0.0.1:<port>")
//	    // srv.CertDir — directory containing ca.crt, gaia_client.crt, gaia_client.key
//	}
//
// The server is automatically stopped and cleaned up when the test ends.
// The gaia binary must be in PATH or pointed to by the GAIA_BIN environment variable.
package gaiatest

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Server is an ephemeral Gaia daemon started for testing.
// Call Cleanup (or rely on t.Cleanup) to stop the process and remove temp files.
type Server struct {
	// Addr is the gRPC listen address, e.g. "127.0.0.1:50123".
	Addr string
	// CertDir is the directory holding ca.crt, gaia_client.crt, gaia_client.key.
	CertDir string

	cmd     *exec.Cmd
	tmpDir  string
	cleanup func()
}

// Option configures a Server before startup.
type Option func(*serverConfig)

type serverConfig struct {
	passphrase string
	bin        string
}

// WithPassphrase sets the master passphrase for the ephemeral daemon.
// Defaults to "gaiatest-dev".
func WithPassphrase(p string) Option {
	return func(c *serverConfig) { c.passphrase = p }
}

// WithBin sets the path to the gaia binary explicitly.
func WithBin(path string) Option {
	return func(c *serverConfig) { c.bin = path }
}

// NewServer starts an ephemeral Gaia daemon and registers t.Cleanup to stop it.
// It panics with a descriptive message if the binary cannot be found or the
// daemon does not become reachable within 10 seconds.
func NewServer(t testing.TB, opts ...Option) *Server {
	t.Helper()

	cfg := &serverConfig{
		passphrase: "gaiatest-dev",
	}
	for _, o := range opts {
		o(cfg)
	}

	bin := resolveBin(cfg.bin)
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	tmpDir := t.TempDir()

	cmd := exec.Command(bin,
		"start",
		"--unsafe",
		"--ephemeral",
		"--port", fmt.Sprintf(":%d", port),
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GAIA_PASSPHRASE=%s", cfg.passphrase),
	)
	cmd.Dir = tmpDir
	// Capture stderr for diagnostics on failure.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("gaiatest: failed to start gaia binary (%s): %v", bin, err)
	}

	srv := &Server{
		Addr:    addr,
		CertDir: filepath.Join(tmpDir, "gaia-dev", "certs"),
		cmd:     cmd,
		tmpDir:  tmpDir,
	}

	// Wait for the TCP port to accept connections (up to 10 s).
	if err := waitTCP(addr, 10*time.Second); err != nil {
		cmd.Process.Kill() //nolint:errcheck
		t.Fatalf("gaiatest: daemon did not become reachable at %s within 10s: %v", addr, err)
	}

	t.Cleanup(srv.Cleanup)
	return srv
}

// Cleanup stops the daemon process and removes temporary files.
func (s *Server) Cleanup() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill() //nolint:errcheck
		s.cmd.Wait()         //nolint:errcheck
	}
}

// resolveBin finds the gaia binary via GAIA_BIN env var or PATH.
func resolveBin(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("GAIA_BIN"); env != "" {
		return env
	}
	path, err := exec.LookPath("gaia")
	if err != nil {
		panic("gaiatest: gaia binary not found in PATH; set GAIA_BIN to point to the binary")
	}
	return path
}

// freePort returns a free TCP port on localhost.
func freePort(t testing.TB) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gaiatest: failed to find a free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// waitTCP polls the given address until it accepts a TCP connection or the
// timeout elapses.
func waitTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("address %s not reachable after %s", addr, timeout)
}
