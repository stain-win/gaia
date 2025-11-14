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
	"github.com/stain-win/gaia/libs/go/client"
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

	// Build paths to certificates
	caCertFile := fmt.Sprintf("%s/%s", cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	clientCertFile := fmt.Sprintf("%s/%s", cfg.TLS.CertsDirectory, cfg.GaiaClientCertFile)
	clientKeyFile := fmt.Sprintf("%s/%s", cfg.TLS.CertsDirectory, cfg.GaiaClientKeyFile)

	// Create a Gaia client
	gaiaClient, err := client.NewClient(client.Config{
		Address:        cfg.Daemon.ListenAddr,
		CACertFile:     caCertFile,
		ClientCertFile: clientCertFile,
		ClientKeyFile:  clientKeyFile,
		Timeout:        cfg.GRPCClientTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gaia client: %w", err)
	}
	defer func() {
		if closeErr := gaiaClient.Close(); closeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close Gaia client: %v\n", closeErr)
		}
	}()

	// Fetch common secrets
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secrets, err := gaiaClient.GetCommonSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch common secrets: %w", err)
	}

	// Build environment variables
	env := os.Environ()
	for namespace, kv := range secrets {
		for key, value := range kv {
			envVarName := fmt.Sprintf("GAIA_%s_%s", namespace, key)
			// Convert to uppercase and replace hyphens with underscores
			envVarName = strings.ToUpper(envVarName)
			envVarName = strings.ReplaceAll(envVarName, "-", "_")
			env = append(env, fmt.Sprintf("%s=%s", envVarName, value))
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
