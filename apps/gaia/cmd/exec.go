package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

// envVarSanitizer replaces any character that's not A-Z, 0-9, or _ with underscore
var envVarSanitizer = regexp.MustCompile(`[^A-Z0-9_]`)

var execCmd = &cobra.Command{
	Use:   "exec -- <command> [args...]",
	Short: "Execute a command with Gaia secrets injected as environment variables",
	Long: `Execute a command with Gaia secrets from the common namespace injected as environment variables.

Secrets are formatted as GAIA_NAMESPACE_KEY and added to the process environment.

Example:
  gaia exec -- node server.js
  gaia exec -- python app.py
  gaia exec -- ./my-binary --flag value

You can override the daemon address:
  gaia exec --address 10.0.0.5:50051 -- node server.js`,
	RunE: runExec,
}

func init() {
	// Address override flag (host:port). Parsed before the -- separator by Cobra.
	execCmd.Flags().String("address", "", "Override Gaia daemon address (host:port)")
}

func runExec(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified after --")
	}
	cfg := gaiaDaemon.GetConfig()
	if addr, _ := cmd.Flags().GetString("address"); addr != "" {
		cfg.Daemon.ListenAddr = addr
	}
	conn, err := getClientConn(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Gaia daemon (%s): %w", cfg.Daemon.ListenAddr, err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewGaiaClientClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ListSecrets(ctx, &pb.ClientListSecretsRequest{})
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	env := os.Environ()
	for _, ns := range resp.Namespaces {
		for _, secret := range ns.Secrets {
			// Build env var name: GAIA_<namespace>_<key>
			// Sanitize to only allow A-Z, 0-9, and underscore
			envVarName := fmt.Sprintf("GAIA_%s_%s", ns.Name, secret.Id)
			envVarName = strings.ToUpper(envVarName)
			envVarName = envVarSanitizer.ReplaceAllString(envVarName, "_")
			env = append(env, fmt.Sprintf("%s=%s", envVarName, secret.Value))
		}
	}

	// Create command with inherited stdin/stdout/stderr
	command := exec.Command(args[0], args[1:]...)
	command.Env = env
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Run the command and wait for it to complete
	if err := command.Run(); err != nil {
		// Preserve exit code if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
}
