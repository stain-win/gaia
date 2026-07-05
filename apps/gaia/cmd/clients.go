package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

// clientOutputDir is the output directory for registered client certificates.
// This is separate from the certs command's outputDir to avoid conflicts.
var clientOutputDir string

// clientsCmd represents the base command for client management.
var clientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Manage Gaia clients and their certificates",
	Long:  `The clients command provides subcommands to register new clients, list existing ones, and manage their lifecycle.`,
}

// registerClientCmd represents the `clients register` subcommand.
var registerClientCmd = &cobra.Command{
	Use:   "register [name]",
	Short: "Register a new client and generate its certificate",
	Long: `Registers a new client with the Gaia daemon.

This command communicates with the daemon to:
1. Create a new client certificate signed by Gaia's Certificate Authority.
2. Register the client's name in the daemon's database.

The generated client certificate and private key will be saved to the specified
output directory. This certificate is required for the client to authenticate
with the Gaia daemon.`,
	Args: cobra.ExactArgs(1), // Enforce that the client name is provided as an argument.
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName = args[0]
		fmt.Printf("Registering new client: %s\n", clientName)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer closeConn(conn)

		c := pb.NewGaiaAdminClient(conn)

		res, err := c.RegisterClient(ctx, &pb.RegisterClientRequest{ClientName: clientName})
		if err != nil {
			return fmt.Errorf("gRPC RegisterClient failed: %w", err)
		}

		if err := os.MkdirAll(clientOutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		certPath := filepath.Join(clientOutputDir, clientName+".crt")
		if err := os.WriteFile(certPath, []byte(res.Certificate), 0644); err != nil {
			return fmt.Errorf("failed to write certificate file: %w", err)
		}
		fmt.Printf("  ✓ Certificate saved to: %s\n", certPath)

		keyPath := filepath.Join(clientOutputDir, clientName+".key")
		if err := os.WriteFile(keyPath, []byte(res.PrivateKey), 0600); err != nil {
			return fmt.Errorf("failed to write private key file: %w", err)
		}
		// Best-effort: if clientOutputDir has an operator-configured default ACL
		// (e.g. granting a docker/app group read access), unblock its effective
		// permissions without widening this file's own mode bits. See
		// certs.RelaxKeyACLMask for why this is needed and why it's safe.
		certs.RelaxKeyACLMask(keyPath)
		fmt.Printf("  ✓ Private key saved to: %s\n", keyPath)
		fmt.Println("\nClient registered successfully.")

		return nil
	},
}

func init() {
	clientsCmd.AddCommand(registerClientCmd)

	registerClientCmd.Flags().StringVarP(&clientOutputDir, "output-dir", "o", "./certs", "Output directory for the new client certificate and key")
}
