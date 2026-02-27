package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

// policyCmd represents the base command for policy management.
var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage client authorization policies",
	Long: `Manage authorization policies that control what clients can do.

Policies define fine-grained access control rules using:
- Paths: Resource patterns (e.g., "common/*", "myapp/production/*")
- Capabilities: Actions allowed (read, write, delete, list)

Examples:
  gaia policy list                    # List all policies
  gaia policy get myapp               # Show policy for myapp
  gaia policy set myapp policy.json   # Set policy from file
  gaia policy delete myapp            # Remove policy
  gaia policy validate policy.json    # Validate policy file`,
}

// policyListCmd lists all policies.
var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all client policies",
	Long: `Lists all authorization policies currently configured.

Shows a summary of each policy including:
- Client name
- Number of rules
- Brief description of access granted

Example:
  gaia policy list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		resp, err := client.ListPolicies(ctx, &pb.ListPoliciesRequest{})
		if err != nil {
			return fmt.Errorf("failed to list policies: %w", err)
		}

		if len(resp.Policies) == 0 {
			fmt.Println("No policies configured.")
			return nil
		}

		// Print in table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "CLIENT\tRULES\tACCESS SUMMARY")
		_, _ = fmt.Fprintln(w, "------\t-----\t--------------")

		for _, p := range resp.Policies {
			summary := buildAccessSummary(p)
			_, _ = fmt.Fprintf(w, "%s\t%d\t%s\n", p.ClientName, len(p.Rules), summary)
		}
		return w.Flush()
	},
}

// policyGetCmd gets a specific policy.
var policyGetCmd = &cobra.Command{
	Use:   "get <client-name>",
	Short: "Get policy for a specific client",
	Long: `Displays the complete authorization policy for a client.

Shows all rules with their paths, capabilities, and descriptions.

Example:
  gaia policy get myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		resp, err := client.GetPolicy(ctx, &pb.GetPolicyRequest{ClientName: clientName})
		if err != nil {
			return fmt.Errorf("failed to get policy: %w", err)
		}

		// Pretty print the policy
		printPolicy(resp.Policy)

		return nil
	},
}

// policySetCmd sets a policy from a file.
var policySetCmd = &cobra.Command{
	Use:   "set <client-name> <policy-file>",
	Short: "Set policy for a client from a JSON file",
	Long: `Sets or updates the authorization policy for a client.

The policy file should be a JSON file with the following structure:
{
  "client_name": "myapp",
  "rules": [
    {
      "path": "common/*",
      "capabilities": ["read"],
      "description": "Read common secrets"
    },
    {
      "path": "myapp/*",
      "capabilities": ["read", "write", "delete", "list"],
      "description": "Full access to own namespace"
    }
  ]
}

Valid capabilities: read, write, delete, list

Example:
  gaia policy set myapp policy.json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName := args[0]
		policyFile := args[1]

		// Read and parse policy file
		data, err := os.ReadFile(policyFile)
		if err != nil {
			return fmt.Errorf("failed to read policy file: %w", err)
		}

		var pol policy.Policy
		if err := json.Unmarshal(data, &pol); err != nil {
			return fmt.Errorf("failed to parse policy file: %w", err)
		}

		// Ensure client name matches
		if pol.ClientName != clientName {
			return fmt.Errorf("client name mismatch: file has '%s', command specified '%s'", pol.ClientName, clientName)
		}

		// Validate policy
		if err := policy.ValidatePolicy(pol); err != nil {
			return fmt.Errorf("invalid policy: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)

		// Convert to protobuf policy
		pbPolicy := policyToProto(&pol)

		resp, err := client.SetPolicy(ctx, &pb.SetPolicyRequest{Policy: pbPolicy})
		if err != nil {
			return fmt.Errorf("failed to set policy: %w", err)
		}

		if resp.Success {
			fmt.Printf("✔ Policy set successfully for client '%s'\n", clientName)
			fmt.Printf("  Rules: %d\n", len(pol.Rules))
		} else {
			fmt.Printf("Failed to set policy: %s\n", resp.Message)
		}

		return nil
	},
}

// policyDeleteCmd deletes a policy.
var policyDeleteCmd = &cobra.Command{
	Use:   "delete <client-name>",
	Short: "Delete policy for a client",
	Long: `Removes the authorization policy for a client.

WARNING: After deletion, the client will be denied access to all resources
until a new policy is created. Consider updating the policy instead of deleting it.

Example:
  gaia policy delete myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientName := args[0]

		// Confirm deletion
		fmt.Printf("⚠ Warning: This will remove the policy for '%s' and deny all access.\n", clientName)
		fmt.Print("Continue? (yes/no): ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		if confirm != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := gaiaDaemon.GetConfig()
		conn, err := getClientConn(cfg)
		if err != nil {
			return fmt.Errorf("could not connect to daemon: %w", err)
		}
		defer closeConn(conn)

		client := pb.NewGaiaAdminClient(conn)
		resp, err := client.DeletePolicy(ctx, &pb.DeletePolicyRequest{ClientName: clientName})
		if err != nil {
			return fmt.Errorf("failed to delete policy: %w", err)
		}

		if resp.Success {
			fmt.Printf("✔ Policy deleted for client '%s'\n", clientName)
		} else {
			fmt.Printf("Failed to delete policy: %s\n", resp.Message)
		}

		return nil
	},
}

// policyValidateCmd validates a policy file.
var policyValidateCmd = &cobra.Command{
	Use:   "validate <policy-file>",
	Short: "Validate a policy file",
	Long: `Validates a policy JSON file without applying it.

Checks for:
- Valid JSON syntax
- Required fields present
- Valid capabilities
- Path format correctness

Example:
  gaia policy validate policy.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		policyFile := args[0]

		// Read and parse policy file
		data, err := os.ReadFile(policyFile)
		if err != nil {
			return fmt.Errorf("failed to read policy file: %w", err)
		}

		var pol policy.Policy
		if err := json.Unmarshal(data, &pol); err != nil {
			return fmt.Errorf("failed to parse policy file: %w", err)
		}

		// Validate policy
		if err := policy.ValidatePolicy(pol); err != nil {
			fmt.Printf("❌ Policy validation failed:\n")
			fmt.Printf("   %s\n", err)
			return nil
		}

		fmt.Printf("✔ Policy is valid\n")
		fmt.Printf("  Client: %s\n", pol.ClientName)
		fmt.Printf("  Rules: %d\n", len(pol.Rules))
		fmt.Println("\nRules summary:")
		for i, rule := range pol.Rules {
			fmt.Printf("  %d. %s → %v\n", i+1, rule.Path, rule.Capabilities)
		}

		return nil
	},
}

func init() {
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyGetCmd)
	policyCmd.AddCommand(policySetCmd)
	policyCmd.AddCommand(policyDeleteCmd)
	policyCmd.AddCommand(policyValidateCmd)
}

// Helper functions

func buildAccessSummary(p *pb.Policy) string {
	if len(p.Rules) == 0 {
		return "No access"
	}

	// Find key patterns
	hasCommon := false
	hasOwn := false
	crossAccess := 0

	for _, rule := range p.Rules {
		if rule.Path == "common/*" || rule.Path == "common/"+p.ClientName+"/*" {
			hasCommon = true
		} else if rule.Path == p.ClientName+"/*" {
			hasOwn = true
		} else {
			crossAccess++
		}
	}

	summary := ""
	if hasOwn {
		summary += "Own namespace"
	}
	if hasCommon {
		if summary != "" {
			summary += ", "
		}
		summary += "common"
	}
	if crossAccess > 0 {
		if summary != "" {
			summary += ", "
		}
		summary += fmt.Sprintf("+%d cross-namespace", crossAccess)
	}

	return summary
}

func printPolicy(p *pb.Policy) {
	fmt.Printf("Policy for client: %s\n", p.ClientName)
	fmt.Printf("Rules: %d\n\n", len(p.Rules))

	for i, rule := range p.Rules {
		fmt.Printf("Rule %d:\n", i+1)
		fmt.Printf("  Path: %s\n", rule.Path)
		fmt.Printf("  Capabilities: %v\n", rule.Capabilities)
		if rule.Description != "" {
			fmt.Printf("  Description: %s\n", rule.Description)
		}
		fmt.Println()
	}
}

func policyToProto(p *policy.Policy) *pb.Policy {
	pbRules := make([]*pb.PolicyRule, len(p.Rules))
	for i, rule := range p.Rules {
		pbRules[i] = &pb.PolicyRule{
			Path:         rule.Path,
			Capabilities: capabilitiesToStrings(rule.Capabilities),
			Description:  rule.Description,
		}
	}

	return &pb.Policy{
		ClientName: p.ClientName,
		Rules:      pbRules,
	}
}

func capabilitiesToStrings(caps []policy.Capability) []string {
	strs := make([]string, len(caps))
	for i, capability := range caps {
		strs[i] = string(capability)
	}
	return strs
}
