package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
	"github.com/stain-win/gaia/apps/gaia/internal/secutil"
)

var (
	autoGenCerts bool
	skipCerts    bool
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42"))

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))

	stepStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))
)

// initCmd is the Cobra command for `gaia init`.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Gaia secure storage",
	Long: `The init command guides you through the process of setting up Gaia's encrypted database and mTLS certificates.

This is a one-time operation that:
  • Creates an encrypted database with your master passphrase
  • Generates mTLS certificates for secure communication (optional)
  • Sets up the necessary directory structure

Once initialized, this command will not run again unless the database file is deleted.
`,
	RunE: runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	cfg := gaiaDaemon.GetConfig()
	if dbFile != "" {
		cfg.Daemon.DBFile = dbFile
	}

	// Print welcome banner
	fmt.Println()
	fmt.Println(titleStyle.Render("🔐 Welcome to Gaia - Secure Secret Management"))
	fmt.Println()
	fmt.Println(infoStyle.Render("This wizard will guide you through the initial setup."))
	fmt.Println()

	// Check if already initialized
	if _, err := os.Stat(cfg.Daemon.DBFile); err == nil {
		fmt.Println(errorStyle.Render("✗ Gaia is already initialized"))
		fmt.Printf("\n  Database file found at: %s\n", cfg.Daemon.DBFile)
		fmt.Println("\n  To re-initialize, please delete the existing database file first:")
		fmt.Printf("    rm %s\n", cfg.Daemon.DBFile)
		fmt.Println()
		return errors.New("database already exists")
	}

	// Step 1: Setup passphrase
	fmt.Println(stepStyle.Render("Step 1/3: Master Passphrase Setup"))
	fmt.Println()

	passphrase, err := promptForPassphrase()
	if err != nil {
		fmt.Println()
		fmt.Println(warningStyle.Render("Initialization cancelled."))
		return err
	}
	// Ensure passphrase is wiped from memory when function exits
	defer secutil.WipeString(&passphrase)

	// Step 2: Certificate setup
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 2/3: mTLS Certificate Setup"))
	fmt.Println()

	certsGenerated := false
	if !skipCerts {
		var err error
		certsGenerated, err = handleCertificateSetup(cfg)
		if err != nil {
			fmt.Println()
			fmt.Println(errorStyle.Render("✗ Certificate setup failed"))
			return err
		}
	} else {
		fmt.Println(infoStyle.Render("Certificate generation skipped (--skip-certs flag)"))
	}

	// Step 3: Initialize database
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 3/3: Database Initialization"))
	fmt.Println()

	if err := initializeDatabase(cfg, passphrase); err != nil {
		fmt.Println()
		fmt.Println(errorStyle.Render("✗ Database initialization failed"))
		return err
	}

	// Success summary
	printSuccessSummary(cfg, certsGenerated)

	return nil
}

func promptForPassphrase() (string, error) {
	if envPass := os.Getenv("GAIA_PASSPHRASE"); envPass != "" {
		// IMPORTANT: Clear the environment variable immediately so it doesn't linger
		// in the process environment block or leak to child processes.
		os.Unsetenv("GAIA_PASSPHRASE")

		fmt.Println("Using master passphrase from GAIA_PASSPHRASE environment variable.")
		// We still validate the passphrase strength
		if len(envPass) < 8 {
			return "", errors.New("passphrase from env var must be at least 8 characters")
		}
		if _, err := encrypt.ValidatePassword(envPass); err != nil {
			return "", fmt.Errorf("weak passphrase in env var: %v", err)
		}
		return envPass, nil
	}

	var passphrase string
	var passphraseConfirm string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Secure Your Data").
				Description("Your master passphrase encrypts all secrets in the database.\n"+
					"Requirements:\n"+
					"  • At least 8 characters\n"+
					"  • Mix of uppercase and lowercase recommended\n"+
					"  • Include numbers and special characters\n"+
					"\n⚠️  This passphrase cannot be recovered if lost!"),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Master passphrase").
				Description("Choose a strong passphrase").
				Value(&passphrase).
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if len(s) < 8 {
						return errors.New("passphrase must be at least 8 characters")
					}
					_, err := encrypt.ValidatePassword(s)
					if err != nil {
						return fmt.Errorf("weak passphrase: %v", err)
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Confirm passphrase").
				Description("Enter the same passphrase again").
				Value(&passphraseConfirm).
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if s != passphrase {
						return errors.New("passphrases do not match")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	// Wipe the confirmation passphrase from memory
	defer secutil.WipeString(&passphraseConfirm)

	fmt.Println(successStyle.Render("✓ Passphrase validated"))
	return passphrase, nil
}

func handleCertificateSetup(cfg *config.Config) (bool, error) {
	// Check if certificates already exist
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	serverCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.ServerCert)

	certsExist := false
	if _, err := os.Stat(caCertPath); err == nil {
		if _, err := os.Stat(serverCertPath); err == nil {
			certsExist = true
		}
	}

	if certsExist {
		fmt.Println(infoStyle.Render("Existing certificates found"))

		if autoGenCerts {
			fmt.Println(infoStyle.Render("Automated mode: relying on existing certs..."))
			return false, nil
		}

		var useCerts bool
		useForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Use existing certificates?").
					Description(fmt.Sprintf("Found certificates in %s/\nDo you want to use them?", cfg.TLS.CertsDirectory)).
					Value(&useCerts),
			),
		)

		if err := useForm.Run(); err != nil {
			return false, err
		}

		if useCerts {
			fmt.Println(successStyle.Render("✓ Using existing certificates"))
			return false, nil
		}
	}

	// Prompt for certificate generation
	var generateCerts bool

	if autoGenCerts {
		generateCerts = true
	} else {
		certForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("mTLS Certificates").
					Description("Gaia uses mutual TLS (mTLS) for secure communication.\n"+
						"This requires:\n"+
						"  • A Certificate Authority (CA)\n"+
						"  • A server certificate\n"+
						"  • Client certificates (generated when registering clients)\n"),
			),
			huh.NewGroup(
				huh.NewConfirm().
					Title("Generate certificates now?").
					Description("Recommended for new installations").
					Value(&generateCerts).
					Affirmative("Yes, generate them").
					Negative("No, I'll do it later"),
			),
		)

		if err := certForm.Run(); err != nil {
			return false, err
		}
	}

	if !generateCerts {
		fmt.Println(warningStyle.Render("⚠ Certificates not generated"))
		fmt.Println(infoStyle.Render("  You can generate them later with: gaia certs generate"))
		return false, nil
	}

	// Generate certificates
	fmt.Println()
	fmt.Println(infoStyle.Render("Generating mTLS certificates..."))

	// Create certs directory
	if err := os.MkdirAll(cfg.TLS.CertsDirectory, 0755); err != nil {
		return false, fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Generate CA
	fmt.Print(infoStyle.Render("  • Creating Certificate Authority... "))
	if err := certs.GenerateCA(cfg, "Gaia Root CA"); err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return false, fmt.Errorf("failed to generate CA: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))

	// Generate server certificate
	fmt.Print(infoStyle.Render("  • Creating server certificate... "))
	if err := certs.GenerateServerCertificate(cfg, "localhost"); err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return false, fmt.Errorf("failed to generate server certificate: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))

	// Generate admin client certificate
	fmt.Print(infoStyle.Render("  • Creating admin client certificate... "))
	if err := certs.GenerateClientCertificate(cfg, "gaia_client"); err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return false, fmt.Errorf("failed to generate client certificate: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))

	fmt.Println()
	fmt.Println(successStyle.Render("✓ Certificates generated successfully"))

	return true, nil
}

func initializeDatabase(cfg *config.Config, passphrase string) error {
	fmt.Println(infoStyle.Render("Creating encrypted database..."))

	// Ensure the directory exists
	dbDir := filepath.Dir(cfg.Daemon.DBFile)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Initialize the database
	d := daemon.NewDaemon(cfg)
	if err := d.InitializeDB(passphrase); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	fmt.Println(successStyle.Render("✓ Database initialized"))
	return nil
}

func printSuccessSummary(cfg *config.Config, certsGenerated bool) {
	fmt.Println()
	fmt.Println(titleStyle.Render("🎉 Gaia Initialization Complete!"))
	fmt.Println()

	// Database info
	fmt.Println(successStyle.Render("Database"))
	fmt.Printf("  Location: %s\n", cfg.Daemon.DBFile)
	fmt.Printf("  Status:   %s\n", successStyle.Render("Encrypted and ready"))
	fmt.Println()

	// Certificates info
	if certsGenerated {
		fmt.Println(successStyle.Render("Certificates"))
		fmt.Printf("  Location: %s/\n", cfg.TLS.CertsDirectory)
		fmt.Println("  Files:")
		fmt.Println("    • ca.crt, ca.key          (Certificate Authority)")
		fmt.Println("    • server.crt, server.key  (Server certificate)")
		fmt.Println("    • gaia_client.crt, .key    (Admin client)")
		fmt.Println()
		fmt.Println(warningStyle.Render("  ⚠  Keep ca.key and *.key files secure!"))
		fmt.Println()
	} else {
		fmt.Println(warningStyle.Render("Certificates"))
		fmt.Println("  Status: Not generated")
		fmt.Println("  Run:    gaia certs generate")
		fmt.Println()
	}

	// Next steps
	fmt.Println(stepStyle.Render("Next Steps"))
	fmt.Println()
	fmt.Println("  1. Start the daemon:")
	fmt.Println(infoStyle.Render("     gaia start"))
	fmt.Println()
	fmt.Println("  2. Unlock with your passphrase:")
	fmt.Println(infoStyle.Render("     gaia unlock"))
	fmt.Println()
	fmt.Println("  3. Register your first client:")
	fmt.Println(infoStyle.Render("     gaia clients register my-app"))
	fmt.Println()
	fmt.Println("  4. Add your first secret:")
	fmt.Println(infoStyle.Render("     gaia secrets add my-app production database-url"))
	fmt.Println()

	// Optional: TUI
	fmt.Println(infoStyle.Render("  Or use the interactive TUI:"))
	fmt.Println(infoStyle.Render("     gaia"))
	fmt.Println()

	// Documentation
	fmt.Println(stepStyle.Render("📚 Documentation"))
	fmt.Println(infoStyle.Render("  For more information, see the README or run:"))
	fmt.Println(infoStyle.Render("     gaia --help"))
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&dbFile, "db-file", "d", "", "Path to the database file")
	initCmd.Flags().BoolVar(&autoGenCerts, "auto-certs", false, "Automatically generate certificates without prompting")
	initCmd.Flags().BoolVar(&skipCerts, "skip-certs", false, "Skip certificate generation entirely")
}
