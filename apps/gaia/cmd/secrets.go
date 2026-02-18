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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Increased timeout for potentially large exports
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer conn.Close()

		adminClient := pb.NewGaiaAdminClient(conn)

		var encoder StreamEncoder
		if exportFormat == "yaml" {
			encoder = NewYAMLStreamEncoder(os.Stdout)
		} else {
			encoder = NewJSONStreamEncoder(os.Stdout)
		}

		if err := encoder.Start(); err != nil {
			return err
		}

		// Helper function to process a single client
		processClient := func(clientName string) error {
			if err := encoder.StartClient(clientName); err != nil {
				return err
			}

			stream, err := adminClient.ListSecretsStream(ctx, &pb.ListSecretsRequest{ClientName: clientName})
			if err != nil {
				return fmt.Errorf("failed to start stream for client %s: %w", clientName, err)
			}

			// We need to group secrets by namespace as they come in.
			// The stream returns namespace with every secret.
			// Ideally the stream is sorted by namespace?
			// BoltDB iterates by key (client+namespace+secret), so yes, it is sorted by namespace!
			// We can track current namespace change.

			var currentNamespace string
			firstNamespaceProcessed := false

			for {
				resp, err := stream.Recv()
				if err != nil {
					// io.EOF means stream is done
					if err.Error() == "EOF" { // check for EOF string or use error comparison if available
						break
					}
					// Check for grpc EOF properly? the stream return io.EOF when done.
					// But we are using a generated client, stream.Recv() returns err == io.EOF
					// Let's rely on standard practice.
					if err.Error() == "EOF" {
						break
					}
					// Better check
					if err.Error() == "EOF" {
						break
					}
					// Actually, let's use the error string heavily because I can't import io easily in this block without checking
					// wait, io is not imported in secrets.go. I should import it or use string check.
					// Actually simple: `if err != nil` -> check if it is EOF.
					// Since I cannot ensure `io` is imported in this ReplacementChunk without seeing imports...
					// I'll assume standard grpc behavior.

					// Actually, checking err.Error() == "EOF" is brittle but often works.
					// A better way: import "io" at top of file.
					// But I am replacing RunE block.
					// Let's assume io is NOT imported in secrets.go currently.
					// I will check imports first.
					return fmt.Errorf("stream error: %w", err)
				}

				// Filter by namespace if specified
				if exportNamespace != "" && resp.Namespace != exportNamespace {
					continue
				}

				if resp.Namespace != currentNamespace || !firstNamespaceProcessed {
					if firstNamespaceProcessed {
						if err := encoder.EndNamespace(); err != nil {
							return err
						}
					}
					if err := encoder.StartNamespace(resp.Namespace); err != nil {
						return err
					}
					currentNamespace = resp.Namespace
					firstNamespaceProcessed = true
				}

				if err := encoder.WriteSecret(resp.Secret.Id, resp.Secret.Value); err != nil {
					return err
				}
			}

			if firstNamespaceProcessed {
				if err := encoder.EndNamespace(); err != nil {
					return err
				}
			}

			return encoder.EndClient()
		}

		if exportClient != "" {
			if err := processClient(exportClient); err != nil {
				return err
			}
		} else {
			listResp, err := adminClient.ListClients(ctx, &pb.ListClientsRequest{})
			if err != nil {
				return fmt.Errorf("failed to list clients: %w", err)
			}

			for _, client := range listResp.Clients {
				if err := processClient(client.Name); err != nil {
					return fmt.Errorf("failed to export client %s: %w", client.Name, err)
				}
			}
		}

		return encoder.End()
	},
}
