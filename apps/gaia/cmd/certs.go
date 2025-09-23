package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
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
	Long:  `Creates a new root Certificate Authority for Gaia.\n\nThis command generates two files:\n- ca.crt: The public root certificate.\n- ca.key: The private key for the CA (keep this secure).\n\nThis is the first step in setting up Gaia's mTLS security. The generated CA\nwill be used to sign all server and client certificates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Generating new Certificate Authority...")
		cfg := config.NewDefaultConfig()
		cfg.CertsDirectory = outputDir

		if err := certs.GenerateCA(cfg, caName); err != nil {
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
	Long:  `Creates a new server certificate and private key for the Gaia daemon.\n\nThis command requires that a CA has already been created (ca.crt and ca.key).\nIt will use the CA to sign a new certificate for the specified server hostname.\nThe hostname should be the address clients will use to connect to the daemon.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName = args[0]
		fmt.Printf("Generating new server certificate for %s...\n", serverName)
		cfg := config.NewDefaultConfig()
		cfg.CertsDirectory = outputDir

		if err := certs.GenerateServerCertificate(cfg, serverName); err != nil {
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
	Long:  `Creates a new client certificate and private key.\n\nThis command requires that a CA has already been created (ca.crt and ca.key).\nIt will use the CA to sign a new certificate for the specified client name.\nThis is useful for creating generic or administrative client certificates that\nare not managed by the 'gaia clients register' command.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName := args[0]
		fmt.Printf("Generating new client certificate for %s...\n", clientName)
		cfg := config.NewDefaultConfig()
		cfg.CertsDirectory = outputDir

		if err := certs.GenerateClientCertificate(cfg, clientName); err != nil {
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
	Short: "Generate all necessary mTLS certificates (CA, server, client)",
	Long:  `Generate a new self-signed root CA, a server certificate, and a client certificate pair for Gaia.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Generating new TLS certificates...")
		cfg := config.NewDefaultConfig()
		cfg.CertsDirectory = outputDir

		fmt.Println("Step 1: Generating Root CA...")
		if err := certs.GenerateCA(cfg, caName); err != nil {
			fmt.Printf("Error generating CA: %v\n", err)
			return
		}
		fmt.Println("...CA generated.")

		fmt.Println("\nStep 2: Generating Server Certificate...")
		if err := certs.GenerateServerCertificate(cfg, serverName); err != nil {
			fmt.Printf("Error generating server certificate: %v\n", err)
			return
		}
		fmt.Println("...Server certificate generated.")

		fmt.Println("\nStep 3: Generating Client Certificate...")
		if err := certs.GenerateClientCertificate(cfg, clientName); err != nil {
			fmt.Printf("Error generating client certificate: %v\n", err)
			return
		}
		fmt.Println("...Client certificate generated.")

		fmt.Println("\nCertificates generated successfully.")
	},
}

func init() {
	rootCmd.AddCommand(certsCmd)
	certsCmd.AddCommand(generateCmd)
	certsCmd.AddCommand(createCaCmd)
	certsCmd.AddCommand(createServerCmd)
	certsCmd.AddCommand(createClientCmd)

	certsCmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "./certs", "The output directory for the certificates")

	createCaCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")

	generateCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")
	generateCmd.Flags().StringVar(&serverName, "server-name", "localhost", "The Common Name for the server certificate")
	generateCmd.Flags().StringVar(&clientName, "client-name", "gaia-cli", "The Common Name for the CLI client certificate")
}
