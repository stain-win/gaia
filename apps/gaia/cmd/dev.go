package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	"gopkg.in/yaml.v3"
)

var (
	devDir        string
	devPassphrase string
	devSeedFile   string
)

// devSeedConfig is the schema for the optional --seed file.
type devSeedConfig struct {
	Secrets []devSeedSecret `yaml:"secrets"`
}

type devSeedSecret struct {
	Namespace string `yaml:"namespace"`
	Key       string `yaml:"key"`
	Value     string `yaml:"value"`
}

// devCmd is the Cobra command for `gaia dev`.
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start Gaia in unsafe dev mode (auto-init + auto-unlock)",
	Long: `The dev command starts Gaia in unsafe local dev mode with automatic
initialization and unlock — suitable for local development and CI pipelines.

  • Creates ./gaia-dev/ (or --dir) if it does not exist
  • Initializes the database on first run (passphrase via --passphrase, GAIA_PASSPHRASE env, or prompt)
  • Starts the daemon, auto-unlocks, and optionally seeds secrets from a YAML file
  • Prints the gRPC address and cert paths for connecting

WARNING: NOT for production. Passphrase strength is NOT enforced.`,
	RunE: runDev,
}

func runDev(_ *cobra.Command, _ []string) error {
	cfg := gaiaDaemon.GetConfig()

	// Configure unsafe mode (resolves --dir, overrides paths)
	savedUnsafeDir := unsafeDir
	unsafeDir = devDir
	if err := configureUnsafeMode(cfg); err != nil {
		return err
	}
	unsafeDir = savedUnsafeDir

	// Resolve passphrase
	pass, err := resolveDevPassphrase(cfg.UnsafeMode)
	if err != nil {
		return err
	}

	// Auto-init if the database does not yet exist
	if _, statErr := os.Stat(cfg.Daemon.DBFile); os.IsNotExist(statErr) {
		fmt.Println("Initializing new Gaia dev database...")

		if err := os.MkdirAll(cfg.TLS.CertsDirectory, 0700); err != nil {
			return fmt.Errorf("failed to create certs directory: %w", err)
		}
		if err := certs.GenerateCA(cfg, "Gaia Dev CA"); err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}
		if err := certs.GenerateServerCertificate(cfg, "localhost"); err != nil {
			return fmt.Errorf("failed to generate server cert: %w", err)
		}
		if err := certs.GenerateClientCertificate(cfg, "gaia_client"); err != nil {
			return fmt.Errorf("failed to generate client cert: %w", err)
		}

		d := daemon.NewDaemon(cfg)
		if err := d.InitializeDB(pass); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		fmt.Println("Database initialized.")
	}

	// Create the daemon instance
	gaiaDaemon = daemon.NewDaemon(cfg)

	// Start daemon in the background
	startErr := make(chan error, 1)
	go func() {
		startErr <- gaiaDaemon.Start(cfg)
	}()

	// Wait until the daemon is running (in locked state)
	if err := waitForDaemon(gaiaDaemon, 30*time.Second, startErr); err != nil {
		return err
	}

	// Auto-unlock
	if err := gaiaDaemon.UnlockDB(pass); err != nil {
		return fmt.Errorf("failed to auto-unlock daemon: %w", err)
	}
	fmt.Println("Daemon unlocked.")

	// Seed secrets if --seed provided
	if devSeedFile != "" {
		if err := seedSecrets(gaiaDaemon, devSeedFile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: seed failed: %v\n", err)
		} else {
			fmt.Println("Secrets seeded.")
		}
	}

	printDevConnectionInfo(cfg)

	// Block until daemon stops
	return <-startErr
}

// resolveDevPassphrase returns the passphrase from flag, env var, or prompts the user.
func resolveDevPassphrase(unsafeMode bool) (string, error) {
	if devPassphrase != "" {
		return devPassphrase, nil
	}
	if p := os.Getenv("GAIA_PASSPHRASE"); p != "" {
		os.Unsetenv("GAIA_PASSPHRASE")
		return p, nil
	}
	return promptForPassphrase(unsafeMode)
}

// waitForDaemon polls until the daemon reaches "locked" or "unlocked" running state.
func waitForDaemon(d *daemon.Daemon, timeout time.Duration, startErr <-chan error) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case err := <-startErr:
			return fmt.Errorf("daemon failed to start: %w", err)
		case <-ticker.C:
			s := d.Status()
			if s == "locked" || s == "unlocked" {
				return nil
			}
		}
	}
	return fmt.Errorf("timeout waiting for daemon to start")
}

// seedSecrets imports secrets from a YAML seed file into the daemon.
func seedSecrets(d *daemon.Daemon, seedPath string) error {
	data, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("failed to read seed file: %w", err)
	}

	var sc devSeedConfig
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return fmt.Errorf("failed to parse seed file: %w", err)
	}

	for _, s := range sc.Secrets {
		if err := d.AddSecret(s.Namespace, s.Namespace, s.Key, s.Value); err != nil {
			return fmt.Errorf("failed to seed %s/%s: %w", s.Namespace, s.Key, err)
		}
	}
	return nil
}

func printDevConnectionInfo(cfg *config.Config) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║  Gaia dev daemon is running              ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Address:   %s\n", cfg.Daemon.ListenAddr)
	fmt.Printf("  Certs:     %s/\n", cfg.TLS.CertsDirectory)
	fmt.Printf("  CA cert:   %s/%s\n", cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	fmt.Println()
	fmt.Println("  Example client connection:")
	fmt.Printf("    gaia --config %s/gaia-config.yaml secrets list\n", cfg.UnsafeDir)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(devCmd)
	devCmd.Flags().StringVar(&devDir, "dir", "", "Working directory for dev mode (default: ./gaia-dev)")
	devCmd.Flags().StringVar(&devPassphrase, "passphrase", "", "Master passphrase (or set GAIA_PASSPHRASE env var)")
	devCmd.Flags().StringVar(&devSeedFile, "seed", "", "Path to a YAML file with secrets to seed on startup")
}
