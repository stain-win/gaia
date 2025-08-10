package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

var (
	overwrite bool
)

// secretsCmd represents the base command for secret management.
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets in Gaia",
	Long:  `Provides subcommands to add, import, and manage secrets within Gaia's storage.`,
}

// importCmd represents the `secrets import` subcommand.
var importCmd = &cobra.Command{
	Use:   "import [json-file-path]",
	Short: "Bulk import secrets from a JSON file",
	Long: `Imports secrets from a structured JSON file into Gaia.

The JSON file should be structured with client names as top-level keys,
followed by namespaces, and then key-value pairs for the secrets.

Example JSON structure:
{
  "client-app-a": {
    "production": {
      "database_url": "postgres://...",
      "api_key": "secret_prod_key"
    }
  },
  "common": {
    "shared": {
      "global_key": "common_value"
    }
  }
}

The import is additive. By default, it will fail if any secret in the file
already exists in the database. Use the --overwrite flag to update existing
secrets with the values from the file.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		var secretsData map[string]map[string]map[string]string
		if err := json.NewDecoder(file).Decode(&secretsData); err != nil {
			return fmt.Errorf("failed to parse JSON file: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Longer timeout for potentially large files
		defer cancel()
		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(ctx, cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)

		stream, err := client.ImportSecrets(ctx)
		if err != nil {
			return fmt.Errorf("failed to open import stream: %w", err)
		}

		configReq := &pb.ImportSecretsRequest{
			Payload: &pb.ImportSecretsRequest_Config{
				Config: &pb.ImportSecretsConfig{
					Overwrite: overwrite,
				},
			},
		}
		if err := stream.Send(configReq); err != nil {
			return fmt.Errorf("failed to send import config: %w", err)
		}

		fmt.Println("Starting secret import...")

		for clientName, namespaces := range secretsData {
			for namespace, secrets := range namespaces {
				for id, value := range secrets {
					itemReq := &pb.ImportSecretsRequest{
						Payload: &pb.ImportSecretsRequest_Item{
							Item: &pb.ImportSecretItem{
								ClientName: clientName,
								Namespace:  namespace,
								Id:         id,
								Value:      value,
							},
						},
					}
					if err := stream.Send(itemReq); err != nil {
						return fmt.Errorf("failed to send secret on stream: %w", err)
					}
				}
			}
		}
		reply, err := stream.CloseAndRecv()
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Printf("\n✔ Import successful.\n")
		fmt.Printf("  Secrets Imported: %d\n", reply.SecretsImported)
		fmt.Printf("  Message: %s\n", reply.Message)

		return nil
	},
}

func init() {
	secretsCmd.AddCommand(importCmd)

	importCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing secrets with values from the file")
}
