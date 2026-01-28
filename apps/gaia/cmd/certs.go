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

// getCertsConfig loads the config and applies output-dir override if specified
func getCertsConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Check if output-dir was explicitly set
	// For persistent flags inherited from parent, we need to check both the flag's Changed property
	// and whether the outputDir variable has been set to a non-empty value
	flag := cmd.Flags().Lookup("output-dir")
	if (flag != nil && flag.Changed) || outputDir != "" {
		cfg.TLS.CertsDirectory = outputDir
	}

	return cfg, nil
}

// createCaCmd represents the `certs create-ca` subcommand.
var createCaCmd = &cobra.Command{
	Use:   "create-ca",
	Short: "Create a new self-signed Certificate Authority (CA)",
	Long:  `Creates a new root Certificate Authority for Gaia.\n\nThis command generates two files:\n- ca.crt: The public root certificate.\n- ca.key: The private key for the CA (keep this secure).\n\nThis is the first step in setting up Gaia's mTLS security. The generated CA\nwill be used to sign all server and client certificates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getCertsConfig(cmd)
		if err != nil {
			return err
		}

		fmt.Printf("Generating new Certificate Authority in %s...\n", cfg.TLS.CertsDirectory)

		if err := certs.GenerateCA(cfg, caName); err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}

		fmt.Printf("\n✔ Certificate Authority created successfully in %s/\n", cfg.TLS.CertsDirectory)
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
		cfg, err := getCertsConfig(cmd)
		if err != nil {
			return err
		}

		serverName = args[0]
		fmt.Printf("Generating new server certificate for %s in %s...\n", serverName, cfg.TLS.CertsDirectory)

		if err := certs.GenerateServerCertificate(cfg, serverName); err != nil {
			return fmt.Errorf("failed to generate server certificate: %w", err)
		}

		fmt.Printf("\n✔ Server certificate created successfully in %s/\n", cfg.TLS.CertsDirectory)
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
		cfg, err := getCertsConfig(cmd)
		if err != nil {
			return err
		}

		clientName := args[0]
		fmt.Printf("Generating new client certificate for %s in %s...\n", clientName, cfg.TLS.CertsDirectory)

		if err := certs.GenerateClientCertificate(cfg, clientName); err != nil {
			return fmt.Errorf("failed to generate client certificate: %w", err)
		}

		fmt.Printf("\n✔ Client certificate created successfully in %s/\n", cfg.TLS.CertsDirectory)
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getCertsConfig(cmd)
		if err != nil {
			return err
		}

		fmt.Printf("Generating new TLS certificates in %s...\n", cfg.TLS.CertsDirectory)

		fmt.Println("\nStep 1: Generating Root CA...")
		if err := certs.GenerateCA(cfg, caName); err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}
		fmt.Println("  ✔ CA generated.")

		fmt.Println("\nStep 2: Generating Server Certificate...")
		if err := certs.GenerateServerCertificate(cfg, serverName); err != nil {
			return fmt.Errorf("failed to generate server certificate: %w", err)
		}
		fmt.Println("  ✔ Server certificate generated.")

		fmt.Println("\nStep 3: Generating Client Certificate...")
		if err := certs.GenerateClientCertificate(cfg, clientName); err != nil {
			return fmt.Errorf("failed to generate client certificate: %w", err)
		}
		fmt.Println("  ✔ Client certificate generated.")

		fmt.Printf("\n✔ All certificates generated successfully in %s/\n", cfg.TLS.CertsDirectory)
		fmt.Println("\nGenerated files:")
		fmt.Println("  - ca.crt, ca.key (Certificate Authority)")
		fmt.Println("  - server.crt, server.key (Server certificate)")
		fmt.Printf("  - %s.crt, %s.key (Client certificate)\n", clientName, clientName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(certsCmd)
	certsCmd.AddCommand(generateCmd)
	certsCmd.AddCommand(createCaCmd)
	certsCmd.AddCommand(createServerCmd)
	certsCmd.AddCommand(createClientCmd)

	// Use empty default so we can detect if flag was explicitly set
	certsCmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "", "Override the certificates directory from config")

	createCaCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")

	generateCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")
	generateCmd.Flags().StringVar(&serverName, "server-name", "localhost", "The Common Name for the server certificate")
	generateCmd.Flags().StringVar(&clientName, "client-name", "gaia_client", "The Common Name for the CLI client certificate")
}
