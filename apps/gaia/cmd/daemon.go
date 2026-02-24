package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

const DaemonStopTimeout = 5 * time.Second

var (
	grpcPort string
	dbFile   string
	certsDir string
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

		if err := gaiaDaemon.Start(cfg); err != nil {
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
		defer conn.Close()

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
		defer conn.Close()

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
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			return fmt.Errorf("failed to get daemon status: %w", err)
		}

		fmt.Printf("Gaia daemon status: %s\n", res.Status)
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&grpcPort, "port", "p", "", "The port to run the gRPC server on")
	startCmd.Flags().StringVarP(&dbFile, "db-file", "d", "", "The path to the BoltDB file")
	startCmd.Flags().StringVarP(&certsDir, "certs-dir", "C", "", "The directory containing TLS certificates")
}
