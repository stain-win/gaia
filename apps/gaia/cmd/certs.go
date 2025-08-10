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

// generateCmd represents the `certs generate` subcommand
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate new mTLS certificates",
	Long:  `Generate a new self-signed root CA, a server certificate, and a client certificate pair for Gaia.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Generating new TLS certificates...")

		// Use the flags to call the certificate generation function.
		err := certs.GenerateTLSCertificates(outputDir, caName, serverName, clientName)
		if err != nil {
			fmt.Printf("Error generating certificates: %v\n", err)
			return
		}
		fmt.Println("Certificates generated successfully.")
	},
}

func init() {
	// Add the `generate` subcommand to the `certs` command.
	certsCmd.AddCommand(generateCmd)

	// Define flags for the `generate` subcommand.
	generateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "./certs", "The output directory for the certificates")
	generateCmd.Flags().StringVar(&caName, "ca-name", "Gaia Root CA", "The Common Name for the Root CA")
	generateCmd.Flags().StringVar(&serverName, "server-name", "localhost", "The Common Name for the server certificate")
	generateCmd.Flags().StringVar(&clientName, "client-name", "gaia-cli", "The Common Name for the CLI client certificate")
}
