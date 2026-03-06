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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// envVarSanitizer replaces any character that's not A-Z, 0-9, or _ with underscore
var envVarSanitizer = regexp.MustCompile(`[^A-Z0-9_]`)

var execCmd = &cobra.Command{
	Use:   "exec -- <command> [args...]",
	Short: "Execute a command with Gaia secrets injected as environment variables",
	Long: `Execute a command with Gaia secrets injected as environment variables.

Secrets are formatted as GAIA_NAMESPACE_KEY and added to the process environment.

Flags:
  --address    Override Gaia daemon address (host:port)
  --namespace  Only inject secrets from a specific namespace
  --dry-run    Preview injected variable names without running the command
  --strip-prefix  Remove the GAIA_ prefix from variable names

Example:
  gaia exec -- node server.js
  gaia exec -- python app.py
  gaia exec -n production -- ./my-binary --flag value
  gaia exec --dry-run -- echo hello
  gaia exec --strip-prefix -- docker compose up

You can override the daemon address:
  gaia exec --address 10.0.0.5:50051 -- node server.js`,
	RunE: runExec,
}

func init() {
	execCmd.Flags().String("address", "", "Override Gaia daemon address (host:port)")
	execCmd.Flags().StringP("namespace", "n", "", "Only inject secrets from this namespace")
	execCmd.Flags().Bool("dry-run", false, "Preview injected variable names without running the command")
	execCmd.Flags().Bool("strip-prefix", false, "Remove the GAIA_ prefix from variable names")
}

func runExec(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if len(args) == 0 && !dryRun {
		return fmt.Errorf("no command specified after --")
	}

	cfg := gaiaDaemon.GetConfig()
	if addr, _ := cmd.Flags().GetString("address"); addr != "" {
		cfg.Daemon.ListenAddr = addr
	}

	conn, err := getClientConn(cfg)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
			PrintError(ErrDaemonOffline())
			return fmt.Errorf("daemon is not reachable")
		}
		return fmt.Errorf("failed to connect to Gaia daemon (%s): %w", cfg.Daemon.ListenAddr, err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewGaiaClientClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build request with optional namespace filter
	req := &pb.ClientListSecretsRequest{}
	if ns, _ := cmd.Flags().GetString("namespace"); ns != "" {
		req.Namespace = ns
	}

	resp, err := client.ListSecrets(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition:
				PrintError(ErrDaemonLocked())
				return fmt.Errorf("daemon is locked")
			case codes.Unavailable:
				PrintError(ErrDaemonOffline())
				return fmt.Errorf("daemon is not reachable")
			}
		}
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	stripPrefix, _ := cmd.Flags().GetBool("strip-prefix")

	// Build environment variables from secrets
	env := os.Environ()
	secretCount := 0
	namespaceCount := 0

	for _, ns := range resp.Namespaces {
		if len(ns.Secrets) == 0 {
			continue
		}
		namespaceCount++
		for _, secret := range ns.Secrets {
			var envVarName string
			if stripPrefix {
				envVarName = fmt.Sprintf("%s_%s", ns.Name, secret.Id)
			} else {
				envVarName = fmt.Sprintf("GAIA_%s_%s", ns.Name, secret.Id)
			}
			envVarName = strings.ToUpper(envVarName)
			envVarName = envVarSanitizer.ReplaceAllString(envVarName, "_")

			if dryRun {
				fmt.Fprintf(os.Stderr, "  %s\n", envVarName)
			} else {
				env = append(env, fmt.Sprintf("%s=%s", envVarName, secret.Value))
			}
			secretCount++
		}
	}

	// Dry-run: print summary and exit
	if dryRun {
		fmt.Fprintf(os.Stderr, "\n🔍 Dry run: %d secret(s) from %d namespace(s) would be injected\n", secretCount, namespaceCount)
		return nil
	}

	// Print injection summary to stderr (doesn't interfere with stdout)
	fmt.Fprintf(os.Stderr, "✓ Injected %d secret(s) from %d namespace(s)\n", secretCount, namespaceCount)

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
