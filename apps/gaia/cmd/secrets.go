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
	overwrite       bool
	exportClient    string
	exportNamespace string
	exportFormat    string
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
		conn, err := getClientConn(cfg)
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
	secretsCmd.AddCommand(exportCmd)

	importCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing secrets with values from the file")

	exportCmd.Flags().StringVar(&exportClient, "client", "", "Export secrets for a specific client (if not specified, exports all clients)")
	exportCmd.Flags().StringVar(&exportNamespace, "namespace", "", "Export secrets from a specific namespace (requires --client)")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format: json or yaml")
}

// exportCmd represents the `secrets export` subcommand.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export secrets to JSON or YAML format",
	Long: `Exports secrets from Gaia to a structured JSON or YAML file.

Examples:
  # Export all secrets to JSON (default)
  gaia secrets export > backup.json

  # Export secrets for a specific client
  gaia secrets export --client myapp > myapp-secrets.json

  # Export secrets from a specific namespace
  gaia secrets export --client myapp --namespace production > prod-secrets.json

  # Export in YAML format
  gaia secrets export --format yaml > backup.yaml

The exported file structure:
{
  "client-app-a": {
    "production": {
      "database_url": "postgres://...",
      "api_key": "secret_prod_key"
    }
  }
}

This format is compatible with the 'gaia secrets import' command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if exportNamespace != "" && exportClient == "" {
			return fmt.Errorf("--namespace requires --client to be specified")
		}

		if exportFormat != "json" && exportFormat != "yaml" {
			return fmt.Errorf("invalid format: %s (must be json or yaml)", exportFormat)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer conn.Close()

		adminClient := pb.NewGaiaAdminClient(conn)

		// Build the export data structure
		exportData := make(map[string]map[string]map[string]string)

		if exportClient != "" {
			// Export for specific client
			if err := exportClientSecrets(ctx, adminClient, exportClient, exportNamespace, exportData); err != nil {
				return err
			}
		} else {
			// Export all clients
			listResp, err := adminClient.ListClients(ctx, &pb.ListClientsRequest{})
			if err != nil {
				return fmt.Errorf("failed to list clients: %w", err)
			}

			for _, client := range listResp.Clients {
				if err := exportClientSecrets(ctx, adminClient, client.Name, "", exportData); err != nil {
					return fmt.Errorf("failed to export client %s: %w", client.Name, err)
				}
			}
		}

		// Output in the requested format
		var output []byte
		if exportFormat == "yaml" {
			output, err = marshalYAML(exportData)
			if err != nil {
				return fmt.Errorf("failed to marshal YAML: %w", err)
			}
		} else {
			output, err = json.MarshalIndent(exportData, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
		}

		fmt.Println(string(output))
		return nil
	},
}

// exportClientSecrets exports secrets for a specific client and optional namespace
func exportClientSecrets(ctx context.Context, client pb.GaiaAdminClient, clientName, namespace string, exportData map[string]map[string]map[string]string) error {
	listReq := &pb.ListSecretsRequest{
		ClientName: clientName,
	}

	resp, err := client.ListSecrets(ctx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list secrets for client %s: %w", clientName, err)
	}

	if exportData[clientName] == nil {
		exportData[clientName] = make(map[string]map[string]string)
	}

	for _, ns := range resp.Namespaces {
		// Filter by namespace if specified
		if namespace != "" && ns.Name != namespace {
			continue
		}

		if exportData[clientName][ns.Name] == nil {
			exportData[clientName][ns.Name] = make(map[string]string)
		}

		for _, secret := range ns.Secrets {
			exportData[clientName][ns.Name][secret.Id] = secret.Value
		}
	}

	return nil
}

// marshalYAML converts the export data to YAML format
func marshalYAML(data map[string]map[string]map[string]string) ([]byte, error) {
	// Simple YAML marshaling
	var result string
	for client, namespaces := range data {
		result += fmt.Sprintf("%s:\n", client)
		for namespace, secrets := range namespaces {
			result += fmt.Sprintf("  %s:\n", namespace)
			for key, value := range secrets {
				// Escape quotes in values
				escapedValue := value
				if containsSpecialChars(value) {
					escapedValue = fmt.Sprintf("%q", value)
				}
				result += fmt.Sprintf("    %s: %s\n", key, escapedValue)
			}
		}
	}
	return []byte(result), nil
}

// containsSpecialChars checks if a string contains special YAML characters
func containsSpecialChars(s string) bool {
	specialChars := []rune{':', '#', '\'', '"', '\n', '\t'}
	for _, char := range s {
		for _, special := range specialChars {
			if char == special {
				return true
			}
		}
	}
	return false
}
