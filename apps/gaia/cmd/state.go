package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"golang.org/x/term"
)

// lockCmd represents the `lock` command.
var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock the Gaia daemon",
	Long: `Sends a command to the running Gaia daemon to immediately lock its storage.

Once locked, the daemon will no longer serve secrets to clients until it is
unlocked again with the master passphrase. This is a crucial security measure
to take before performing maintenance or if a potential threat is detected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		_, err = client.Lock(ctx, &pb.LockRequest{})
		if err != nil {
			return fmt.Errorf("gRPC Lock failed: %w", err)
		}

		fmt.Println("Daemon locked successfully.")
		return nil
	},
}

// unlockCmd represents the `unlock` command.
var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock the Gaia daemon with the master passphrase",
	Long: `Sends the master passphrase to the running Gaia daemon to unlock its storage.

The daemon must be unlocked before it can serve secrets to clients. You will be
prompted to enter the master passphrase securely.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Enter master passphrase: ")
		passphrase, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read passphrase: %w", err)
		}
		fmt.Println() // Newline after password input

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		_, err = client.Unlock(ctx, &pb.UnlockRequest{Passphrase: string(passphrase)})
		if err != nil {
			return fmt.Errorf("gRPC Unlock failed: %w", err)
		}

		fmt.Println("Daemon unlocked successfully.")
		return nil
	},
}
