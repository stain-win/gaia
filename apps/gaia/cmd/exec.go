package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

var execCmd = &cobra.Command{
	Use:   "exec -- <command> [args...]",
	Short: "Execute a command with Gaia secrets injected as environment variables",
	Long: `Execute a command with Gaia secrets from the common namespace injected as environment variables.

Secrets are formatted as GAIA_NAMESPACE_KEY and added to the process environment.

Example:
  gaia exec -- node server.js
  gaia exec -- python app.py
  gaia exec -- ./my-binary --flag value`,
	DisableFlagParsing: true, // We handle flags manually to support -- separator
	RunE:               runExec,
}

func runExec(_ *cobra.Command, args []string) error {
	// Find the -- separator
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	// Everything after -- is the command to execute
	var cmdArgs []string
	if separatorIndex == -1 {
		// No separator found, treat all args as the command
		cmdArgs = args
	} else {
		cmdArgs = args[separatorIndex+1:]
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified after --")
	}

	// Get the config
	cfg := gaiaDaemon.GetConfig()

	// Connect to the Gaia daemon using internal gRPC client
	conn, err := getClientConn(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Gaia daemon: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := pb.NewGaiaAdminClient(conn)

	// Fetch secrets from the "common" client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ListSecrets(ctx, &pb.ListSecretsRequest{
		ClientName: "common",
	})
	if err != nil {
		return fmt.Errorf("failed to fetch common secrets: %w", err)
	}

	// Build environment variables from secrets
	env := os.Environ()
	for _, ns := range resp.Namespaces {
		for _, secret := range ns.Secrets {
			// Format: GAIA_<NAMESPACE>_<KEY>
			envVarName := fmt.Sprintf("GAIA_%s_%s", ns.Name, secret.Id)
			// Convert to uppercase and replace hyphens with underscores
			envVarName = strings.ToUpper(envVarName)
			envVarName = strings.ReplaceAll(envVarName, "-", "_")
			env = append(env, fmt.Sprintf("%s=%s", envVarName, secret.Value))
		}
	}

	// Find the binary
	binary, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", cmdArgs[0])
	}

	// Execute and replace the current process
	// Note: syscall.Exec replaces the current process, so this function never returns
	err = syscall.Exec(binary, cmdArgs, env)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	// This line is never reached if syscall.Exec succeeds
	return nil
}
