package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
)

var (
	outputDir  string
	caName     string
	serverName string
	clientName string
)

// certsCmd represents the base command for certificate management
var certsCmd = &cobra.Command{
	Use:   "certs",
	Short: "Manage Gaia's mTLS certificates",
	Long:  `The certs command provides subcommands to manage the TLS certificates used by Gaia and its clients.`,
}

// createCaCmd represents the `certs create-ca` subcommand.
var createCaCmd = &cobra.Command{
	Use:   "create-ca",
	Short: "Create a new self-signed Certificate Authority (CA)",
	Long: `Creates a new root Certificate Authority for Gaia.

This command generates two files:
- ca.crt: The public root certificate.
- ca.key: The private key for the CA (keep this secure).

This is the first step in setting up Gaia's mTLS security. The generated CA
will be used to sign all server and client certificates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Generating new Certificate Authority...")

		if err := certs.GenerateCA(outputDir, caName); err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}

		fmt.Printf("\n✔ Certificate Authority created successfully in %s/\n", outputDir)
		fmt.Println("  - ca.crt (public certificate)")
		fmt.Println("  - ca.key (private key - KEEP SAFE!)")
		return nil
	},
}

// createServerCmd represents the `certs create-server` subcommand.
var createServerCmd = &cobra.Command{
	Use:   "create-server [hostname]",
	Short: "Create a new server certificate signed by the CA",
	Long: `Creates a new server certificate and private key for the Gaia daemon.

This command requires that a CA has already been created (ca.crt and ca.key).
It will use the CA to sign a new certificate for the specified server hostname.
The hostname should be the address clients will use to connect to the daemon.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName = args[0]
		fmt.Printf("Generating new server certificate for %s...\n", serverName)

		// This function will need to be added to your certs package.
		if err := certs.GenerateServerCertificate(outputDir, serverName); err != nil {
			return fmt.Errorf("failed to generate server certificate: %w", err)
		}

		fmt.Printf("\n✔ Server certificate created successfully in %s/\n", outputDir)
		fmt.Println("  - server.crt (public certificate)")
		fmt.Println("  - server.key (private key)")
		return nil
	},
}

// createClientCmd represents the `certs create-client` subcommand.
var createClientCmd = &cobra.Command{
	Use:   "create-client [client-name]",
	Short: "Create a new client certificate signed by the CA",
	Long: `Creates a new client certificate and private key.

This command requires that a CA has already been created (ca.crt and ca.key).
It will use the CA to sign a new certificate for the specified client name.
This is useful for creating generic or administrative client certificates that
are not managed by the 'gaia clients register' command.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName := args[0]
		fmt.Printf("Generating new client certificate for %s...\n", clientName)

		if err := certs.GenerateClientCertificate(outputDir, clientName); err != nil {
			return fmt.Errorf("failed to generate client certificate: %w", err)
		}

		fmt.Printf("\n✔ Client certificate created successfully in %s/\n", outputDir)
		fmt.Printf("  - %s.crt (public certificate)\n", clientName)
		fmt.Printf("  - %s.key (private key)\n", clientName)
		return nil
	},
}

// generateCmd represents the `certs generate` subcommand
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate new mTLS certificates",
	Long:  `Generate a new self-signed root CA, a server certificate, and a client certificate pair for Gaia.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Generating new TLS certificates...")

		err := certs.GenerateTLSCertificates(outputDir, caName, serverName, clientName)
		if err != nil {
			fmt.Printf("Error generating certificates: %v\n", err)
			return
		}
		fmt.Println("Certificates generated successfully.")
	},
}

func init() {
	certsCmd.AddCommand(generateCmd)
	certsCmd.AddCommand(createCaCmd)
	certsCmd.AddCommand(createServerCmd)
	certsCmd.AddCommand(createClientCmd)

	generateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "./certs", "The output directory for the certificates")
	generateCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")
	generateCmd.Flags().StringVar(&serverName, "server-name", "localhost", "The Common Name for the server certificate")
	generateCmd.Flags().StringVar(&clientName, "client-name", "gaia-cli", "The Common Name for the CLI client certificate")
}
