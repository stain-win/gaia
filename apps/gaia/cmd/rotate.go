package cmd

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
	"github.com/stain-win/gaia/apps/gaia/internal/secutil"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"golang.org/x/term"
)

// rotatePasswordCmd represents the `rotate-password` command.
var rotatePasswordCmd = &cobra.Command{
	Use:   "rotate-password",
	Short: "Rotate the master passphrase",
	Long: `Rotates the master passphrase used to encrypt secrets in the Gaia database.

This operation:
  1. Validates your current passphrase
  2. Creates a backup of the database
  3. Re-encrypts ALL secrets with a new key derived from the new passphrase
  4. Updates the salt and key hash atomically

The daemon must be running and unlocked to perform this operation.
A database backup is created before any changes are made.`,
	RunE: runRotatePassword,
}

func runRotatePassword(cmd *cobra.Command, args []string) error {
	// Step 1: Prompt for the current passphrase
	fmt.Print("Enter current master passphrase: ")
	currentPassphrase, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}
	fmt.Println()
	defer secutil.WipeBytes(currentPassphrase)

	// Step 2: Prompt for a new passphrase
	fmt.Print("Enter new master passphrase: ")
	newPassphrase, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read new passphrase: %w", err)
	}
	fmt.Println()
	defer secutil.WipeBytes(newPassphrase)

	// Step 3: Confirm a new passphrase
	fmt.Print("Confirm new master passphrase: ")
	confirmPassphrase, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read confirmation passphrase: %w", err)
	}
	fmt.Println()
	defer secutil.WipeBytes(confirmPassphrase)

	if string(newPassphrase) != string(confirmPassphrase) {
		return fmt.Errorf("new passphrases do not match")
	}

	// Step 4: Client-side password strength validation for fast feedback
	if _, err := encrypt.ValidatePassword(string(newPassphrase)); err != nil {
		return fmt.Errorf("new passphrase too weak: %w", err)
	}

	// Step 5: Make gRPC call with an extended timeout (scrypt + re-encryption can be slow)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := gaiaDaemon.GetConfig()
	conn, err := getClientConn(cfg)
	if err != nil {
		return fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer closeConn(conn)

	fmt.Println("Rotating master passphrase... (this may take a moment)")

	client := pb.NewGaiaAdminClient(conn)
	resp, err := client.RotatePassword(ctx, &pb.RotatePasswordRequest{
		CurrentPassphrase: string(currentPassphrase),
		NewPassphrase:     string(newPassphrase),
	})
	if err != nil {
		return fmt.Errorf("password rotation failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("password rotation failed: %s", resp.Message)
	}

	fmt.Println("Password rotated successfully.")
	fmt.Printf("  Secrets re-encrypted: %d\n", resp.SecretsRotated)
	fmt.Printf("  Database backup: %s\n", resp.BackupPath)

	// Check if GAIA_PASSPHRASE is set and warn the user to update it
	if os.Getenv("GAIA_PASSPHRASE") != "" {
		fmt.Println("\n  WARNING: GAIA_PASSPHRASE environment variable is set.")
		fmt.Println("  Remember to update it with the new passphrase.")
	}

	return nil
}
