package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

const DaemonStopTimeout = 5 * time.Second

// startCmd is the Cobra command for `gaia start`.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Gaia daemon",
	Long: `The start command launches the Gaia daemon process in the foreground,
intended to be managed by a service manager like systemd or launchd.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Gaia daemon. Press Ctrl+C to stop.")
		err := gaiaDaemon.Start()
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

		conn, err := getClientConn(ctx)
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

		conn, err := getClientConn(ctx)
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

		err = gaiaDaemon.Start()
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

		conn, err := getClientConn(ctx)
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
