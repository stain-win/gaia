package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"gopkg.in/yaml.v3"
)

const DaemonStopTimeout = 5 * time.Second

var (
	grpcPort   string
	dbFile     string
	certsDir   string
	configFile string
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Gaia daemon. Press Ctrl+C to stop.")

		cfg := gaiaDaemon.GetConfig()
		if configFile != "" {
			data, err := os.ReadFile(configFile)
			if err != nil {
				log.Fatalf("failed to read config file '%s': %v", configFile, err)
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				log.Fatalf("failed to unmarshal config from file '%s': %v", configFile, err)
			}
		}

		// Override with flags if set
		if grpcPort != "" {
			cfg.GRPCPort = grpcPort
		}
		if dbFile != "" {
			cfg.DBFile = dbFile
		}
		if certsDir != "" {
			cfg.CertsDirectory = certsDir
			cfg.CACertFile = "/ca.crt"
			cfg.ServerCertFile = "/server.crt"
			cfg.ServerKeyFile = "/server.key"
		}

		err := gaiaDaemon.Start(cfg)
		if err != nil {
			log.Fatalf("Daemon failed to start: %v", err)
		}
	},
}

// stopCmd is the Cobra command for `gaia stop`.
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Gaia daemon",
	Long:  `The stop command gracefully shuts down the running Gaia daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Sending stop command to Gaia daemon...")
		ctx, cancel := context.WithTimeout(context.Background(), DaemonStopTimeout)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			fmt.Printf("Error: could not connect to daemon. Is it running? %v\n", err)
			return
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		_, err = client.Stop(ctx, &pb.StopRequest{})
		if err != nil {
			fmt.Printf("Error sending stop command to daemon: %v\n", err)
			return
		}

		fmt.Println("Gaia daemon stop command sent successfully.")
	},
}

// restartCmd is the Cobra command for `gaia restart`.
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Gaia daemon",
	Long:  `The restart command stops and then starts the Gaia daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running gaia restart...")
		ctx, cancel := context.WithTimeout(context.Background(), DaemonStopTimeout)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			fmt.Printf("Error: could not connect to daemon for restart. Is it running? %v\n", err)
			return
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		_, err = client.Stop(ctx, &pb.StopRequest{})
		if err != nil {
			fmt.Printf("Error sending stop command to daemon: %v\n", err)
			return
		}

		err = gaiaDaemon.Start(cfg)
		if err != nil {
			log.Printf("Daemon failed to start: %v", err)
			return
		}
		fmt.Println("Gaia daemon restarted successfully.")
	},
}

// statusCmd is the Cobra command for `gaia status`.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the Gaia daemon",
	Long:  `The status command returns the current operational status of the Gaia daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			fmt.Printf("Gaia daemon status: %s\n", daemon.StatusStopped)
			return
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			fmt.Printf("Error getting daemon status: %v\n", err)
			return
		}

		fmt.Printf("Gaia daemon status: %s\n", res.Status)
	},
}

func init() {
	startCmd.Flags().StringVarP(&grpcPort, "port", "p", "", "The port to run the gRPC server on")
	startCmd.Flags().StringVarP(&dbFile, "db-file", "d", "", "The path to the BoltDB file")
	startCmd.Flags().StringVarP(&certsDir, "certs-dir", "c", "", "The directory containing TLS certificates")
	startCmd.Flags().StringVar(&configFile, "config", "", "Path to the configuration file (YAML)")
}
