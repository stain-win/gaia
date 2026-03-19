package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

const DaemonStopTimeout = 5 * time.Second

var (
	grpcPort      string
	dbFile        string
	certsDir      string
	unsafeMode    bool
	unsafeDir     string
	ephemeralMode bool
)

// startCmd is the Cobra command for `gaia start`.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Gaia daemon",
	Long: `The start command launches the Gaia daemon process.

The daemon runs in the foreground and is designed to be managed by a service
manager like systemd or launchd. It will start, open its database, and begin
listening for secure gRPC connections from authorized clients.

By default, the daemon starts in a locked state and must be explicitly unlocked
with the 'gaia unlock' command before it will serve secrets. This ensures that
even after a system reboot, secrets are not exposed until an operator
intervenes.

Configuration values can be overridden from the config file using flags.
For example:
  gaia start --db-file /var/lib/gaia/data.db
  gaia start --grpc-port :60051`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Gaia daemon. Press Ctrl+C to stop.")

		cfg := gaiaDaemon.GetConfig()

		// Override with flags if set
		if grpcPort != "" {
			cfg.Daemon.ListenAddr = grpcPort
		}
		if dbFile != "" {
			cfg.Daemon.DBFile = dbFile
		}
		if certsDir != "" {
			cfg.TLS.CertsDirectory = certsDir
			cfg.TLS.CACert = "ca.crt"
			cfg.TLS.ServerCert = "server.crt"
			cfg.TLS.ServerKey = "server.key"
		}

		// --ephemeral requires --unsafe
		if ephemeralMode && !unsafeMode {
			return fmt.Errorf("--ephemeral requires --unsafe; ephemeral mode is for local development only")
		}

		// Handle unsafe local dev mode
		if unsafeMode {
			if err := configureUnsafeMode(cfg); err != nil {
				return err
			}
			gaialog.Get().Warn("daemon running in UNSAFE LOCAL DEV MODE",
				"dir", cfg.UnsafeDir,
				"mode", "unsafe",
			)
		}

		// Handle ephemeral mode (requires --unsafe)
		if ephemeralMode {
			if err := configureEphemeralMode(cfg); err != nil {
				return err
			}
		}

		// Re-create daemon with updated config
		gaiaDaemon = daemon.NewDaemon(cfg)

		err := gaiaDaemon.Start(cfg)

		// Clean up ephemeral temp dir after daemon stops
		if ephemeralMode && cfg.EphemeralDir != "" {
			os.RemoveAll(cfg.EphemeralDir)
		}

		if err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
		return nil
	},
}

// stopCmd is the Cobra command for `gaia stop`.
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Gaia daemon",
	Long:  `The stop command gracefully shuts down the running Gaia daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Sending stop command to Gaia daemon...")
		ctx, cancel := context.WithTimeout(context.Background(), DaemonStopTimeout)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon (is it running?): %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		if _, err = client.Stop(ctx, &pb.StopRequest{}); err != nil {
			return fmt.Errorf("failed to send stop command: %w", err)
		}

		fmt.Println("Gaia daemon stop command sent successfully.")
		return nil
	},
}

// restartCmd is the Cobra command for `gaia restart`.
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Gaia daemon",
	Long: `The restart command stops the running Gaia daemon.

Since the daemon runs in the foreground, you will need to start it again
manually after it stops. This is equivalent to running 'gaia stop' followed
by 'gaia start'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stopping Gaia daemon for restart...")
		ctx, cancel := context.WithTimeout(context.Background(), DaemonStopTimeout)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon (is it running?): %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		if _, err = client.Stop(ctx, &pb.StopRequest{}); err != nil {
			return fmt.Errorf("failed to send stop command: %w", err)
		}

		fmt.Println("Gaia daemon stopped successfully.")
		fmt.Println("To complete the restart, run: gaia start")
		return nil
	},
}

// statusCmd is the Cobra command for `gaia status`.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the Gaia daemon",
	Long:  `The status command returns the current operational status of the Gaia daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := gaiaDaemon.GetConfig()
		ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
		defer cancel()

		conn, err := getClientConn(cfg)
		if err != nil {
			fmt.Printf("Gaia daemon status: %s\n", daemon.StatusStopped)
			return nil
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			return fmt.Errorf("failed to get daemon status: %w", err)
		}

		fmt.Printf("Gaia daemon status: %s\n", res.Status)
		return nil
	},
}

// configureUnsafeMode sets up the config for unsafe local dev mode.
// It resolves the --dir path, creates directories, overrides all config paths,
// and prints a warning banner.
func configureUnsafeMode(cfg *config.Config) error {
	dir := unsafeDir
	if dir == "" {
		dir = "gaia-dev"
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve unsafe dir path: %w", err)
	}

	// Create directories
	if err := os.MkdirAll(absDir, 0700); err != nil {
		return fmt.Errorf("failed to create unsafe dir: %w", err)
	}
	certsPath := filepath.Join(absDir, "certs")
	if err := os.MkdirAll(certsPath, 0700); err != nil {
		return fmt.Errorf("failed to create unsafe certs dir: %w", err)
	}

	// Override all config paths to point inside the unsafe dir
	cfg.Daemon.DBFile = filepath.Join(absDir, "gaia.db")
	cfg.TLS.CertsDirectory = certsPath
	cfg.TLS.CACert = "ca.crt"
	cfg.TLS.ServerCert = "server.crt"
	cfg.TLS.ServerKey = "server.key"
	cfg.Log.FilePath = filepath.Join(absDir, "gaia.log")
	cfg.UnsafeMode = true
	cfg.UnsafeDir = absDir

	// Override audit file backend paths if configured
	for i := range cfg.Audit.Backends {
		if cfg.Audit.Backends[i].Type == "file" {
			cfg.Audit.Backends[i].Path = filepath.Join(absDir, "audit.log")
		}
	}

	printUnsafeWarning(absDir)
	return nil
}

// configureEphemeralMode sets up the config for ephemeral (in-memory) mode.
// It creates a temp directory for certs, auto-generates them, and sets EphemeralMode=true.
// The DB file path is set by daemon.InitializeDB at startup (new temp file each run).
func configureEphemeralMode(cfg *config.Config) error {
	tmpDir, err := os.MkdirTemp("", "gaia-ephemeral-*")
	if err != nil {
		return fmt.Errorf("failed to create ephemeral temp dir: %w", err)
	}

	certsPath := filepath.Join(tmpDir, "certs")
	if err := os.MkdirAll(certsPath, 0700); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to create ephemeral certs dir: %w", err)
	}

	cfg.TLS.CertsDirectory = certsPath
	cfg.TLS.CACert = "ca.crt"
	cfg.TLS.ServerCert = "server.crt"
	cfg.TLS.ServerKey = "server.key"
	cfg.EphemeralMode = true
	cfg.EphemeralDir = tmpDir

	// Auto-generate certs for the ephemeral session (mTLS still active)
	if err := certs.GenerateCA(cfg, "Gaia Ephemeral CA"); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to generate ephemeral CA: %w", err)
	}
	if err := certs.GenerateServerCertificate(cfg, "localhost"); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to generate ephemeral server cert: %w", err)
	}
	if err := certs.GenerateClientCertificate(cfg, "gaia_client"); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to generate ephemeral client cert: %w", err)
	}

	printEphemeralWarning()
	return nil
}

func printEphemeralWarning() {
	fmt.Fprintf(os.Stderr, "\n"+
		"╔══════════════════════════════════════════════════════════════════╗\n"+
		"║  ⚠  UNSAFE LOCAL DEV MODE — EPHEMERAL (NO DATA PERSISTENCE) ⚠  ║\n"+
		"║                                                                  ║\n"+
		"║  All data is IN-MEMORY ONLY. Nothing survives process exit.     ║\n"+
		"║  mTLS is still active. Do NOT store real secrets here.          ║\n"+
		"╚══════════════════════════════════════════════════════════════════╝\n\n")
}

func printUnsafeWarning(absDir string) {
	fmt.Fprintf(os.Stderr, "\n"+
		"╔══════════════════════════════════════════════════════════════╗\n"+
		"║  WARNING: UNSAFE LOCAL DEV MODE — NOT FOR PRODUCTION USE    ║\n"+
		"║                                                              ║\n"+
		"║  All data is stored in:                                      ║\n"+
		"║    %s\n"+
		"║                                                              ║\n"+
		"║  Passphrase strength is NOT enforced.                        ║\n"+
		"║  This database CANNOT be migrated to safe mode.             ║\n"+
		"║  Do NOT store real secrets here.                             ║\n"+
		"╚══════════════════════════════════════════════════════════════╝\n\n",
		absDir)
}

func init() {
	startCmd.Flags().StringVarP(&grpcPort, "port", "p", "", "The port to run the gRPC server on")
	startCmd.Flags().StringVarP(&dbFile, "db-file", "d", "", "The path to the BoltDB file")
	startCmd.Flags().StringVarP(&certsDir, "certs-dir", "C", "", "The directory containing TLS certificates")
	startCmd.Flags().BoolVar(&unsafeMode, "unsafe", false, "Run in unsafe local dev mode (relaxed passphrase, local directory)")
	startCmd.Flags().StringVar(&unsafeDir, "dir", "", "Working directory for unsafe mode (default: ./gaia-dev)")
	startCmd.Flags().BoolVar(&ephemeralMode, "ephemeral", false, "Run in ephemeral mode: all data is in-memory only and lost on exit (requires --unsafe)")
}
