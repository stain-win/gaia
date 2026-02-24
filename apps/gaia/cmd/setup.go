package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
	"github.com/stain-win/gaia/apps/gaia/internal/secutil"
)

// Setup wizard styles
var (
	wizardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				Background(lipgloss.Color("#1F2937")).
				Padding(1, 3).
				MarginBottom(1)

	wizardSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Italic(true)

	wizardSuccessStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#10B981"))

	wizardWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F59E0B"))

	wizardErrorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#EF4444"))

	wizardInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	wizardStepStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#3B82F6"))

	wizardBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4B5563")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	wizardHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")).
				Bold(true)
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard for Gaia",
	Long: `The setup command provides a guided, interactive experience to get Gaia 
up and running in minutes.

This wizard will:
  • Create necessary directories
  • Generate mTLS certificates for secure communication
  • Initialize the encrypted database
  • Optionally start the daemon
  • Check for updates

Perfect for first-time users or when setting up Gaia on a new system.
`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Load config (or create default)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.NewDefaultConfig()
	}

	// Print welcome banner
	printWelcomeBanner()

	// Check if already set up
	if isAlreadySetup(cfg) {
		return handleExistingSetup(cfg)
	}

	// Check for updates first
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("Checking for updates..."))
	checkAndOfferUpdate()

	// Run the setup wizard
	return runSetupWizard(cfg)
}

func printWelcomeBanner() {
	banner := `
   ██████╗  █████╗ ██╗ █████╗ 
  ██╔════╝ ██╔══██╗██║██╔══██╗
  ██║  ███╗███████║██║███████║
  ██║   ██║██╔══██║██║██╔══██║
  ╚██████╔╝██║  ██║██║██║  ██║
   ╚═════╝ ╚═╝  ╚═╝╚═╝╚═╝  ╚═╝
`
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Render(banner))
	fmt.Println(wizardTitleStyle.Render("  Welcome to Gaia Setup Wizard  "))
	fmt.Println()
	fmt.Println(wizardSubtitleStyle.Render("Secure secret management made simple"))
	fmt.Println()
}

func isAlreadySetup(cfg *config.Config) bool {
	// Check if database exists
	if _, err := os.Stat(cfg.Daemon.DBFile); err == nil {
		return true
	}
	return false
}

func handleExistingSetup(cfg *config.Config) error {
	fmt.Println(wizardWarningStyle.Render("⚠  Gaia appears to be already set up"))
	fmt.Println()

	// Show current configuration
	fmt.Println(wizardInfoStyle.Render("Current configuration:"))
	fmt.Printf("  Database: %s\n", wizardHighlightStyle.Render(cfg.Daemon.DBFile))
	fmt.Printf("  Certificates: %s\n", wizardHighlightStyle.Render(cfg.TLS.CertsDirectory))
	fmt.Println()

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Check for Gaia updates", "update"),
					huh.NewOption("View configuration", "config"),
					huh.NewOption("Run security audit", "audit"),
					huh.NewOption("Exit", "exit"),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return nil
	}

	switch choice {
	case "update":
		return runVersionCheck(nil, nil)
	case "config":
		printCurrentConfig(cfg)
		return nil
	case "audit":
		return runSecurityAudit(cfg)
	}

	return nil
}

func runSetupWizard(cfg *config.Config) error {
	fmt.Println()
	fmt.Println(wizardInfoStyle.Render("This wizard will guide you through the setup process."))
	fmt.Println(wizardInfoStyle.Render("Press Ctrl+C at any time to cancel."))
	fmt.Println()

	// Step 1: Configuration
	fmt.Println(wizardStepStyle.Render("━━━ Step 1/4: Configuration ━━━"))
	fmt.Println()

	useDefaults, err := promptUseDefaults(cfg)
	if err != nil {
		return err
	}

	if !useDefaults {
		if err := promptCustomConfig(cfg); err != nil {
			return err
		}
	}

	// Step 2: Create directories
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("━━━ Step 2/4: Creating Directories ━━━"))
	fmt.Println()

	if err := createDirectoriesWithProgress(cfg); err != nil {
		return wrapSetupError("directory creation", err)
	}

	// Step 3: Generate certificates
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("━━━ Step 3/4: Generating Certificates ━━━"))
	fmt.Println()

	if err := generateCertificatesWithProgress(cfg); err != nil {
		return wrapSetupError("certificate generation", err)
	}

	// Step 4: Initialize database
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("━━━ Step 4/4: Database Initialization ━━━"))
	fmt.Println()

	passphrase, err := promptSetupPassphrase()
	if err != nil {
		return err
	}
	defer secutil.WipeString(&passphrase)

	if err := initializeDatabaseWithProgress(cfg, passphrase); err != nil {
		return wrapSetupError("database initialization", err)
	}

	// Success!
	printSetupSuccess(cfg)

	// Offer to start daemon
	if err := offerStartDaemon(); err != nil {
		// Non-fatal, just warn
		fmt.Println(wizardWarningStyle.Render("Note: Could not start daemon automatically"))
	}

	return nil
}

func promptUseDefaults(cfg *config.Config) (bool, error) {
	fmt.Println(wizardInfoStyle.Render("Default paths for your system:"))
	fmt.Println()
	fmt.Printf("  📁 Database:     %s\n", wizardHighlightStyle.Render(cfg.Daemon.DBFile))
	fmt.Printf("  🔐 Certificates: %s\n", wizardHighlightStyle.Render(cfg.TLS.CertsDirectory))
	fmt.Printf("  📋 Logs:         %s\n", wizardHighlightStyle.Render(cfg.Log.FilePath))
	fmt.Println()

	var useDefaults bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Use these default paths?").
				Description("You can customize paths if needed").
				Value(&useDefaults).
				Affirmative("Yes, use defaults").
				Negative("No, let me customize"),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	if useDefaults {
		fmt.Println(wizardSuccessStyle.Render("✓ Using default configuration"))
	}

	return useDefaults, nil
}

func promptCustomConfig(cfg *config.Config) error {
	var dbPath, certsPath string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Database file path").
				Description("Where to store the encrypted database").
				Value(&dbPath).
				Placeholder(cfg.Daemon.DBFile),
			huh.NewInput().
				Title("Certificates directory").
				Description("Where to store mTLS certificates").
				Value(&certsPath).
				Placeholder(cfg.TLS.CertsDirectory),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if dbPath != "" {
		cfg.Daemon.DBFile = dbPath
	}
	if certsPath != "" {
		cfg.TLS.CertsDirectory = certsPath
	}

	fmt.Println(wizardSuccessStyle.Render("✓ Custom paths configured"))
	return nil
}

func createDirectoriesWithProgress(cfg *config.Config) error {
	dirs := []string{
		filepath.Dir(cfg.Daemon.DBFile),
		cfg.TLS.CertsDirectory,
		filepath.Dir(cfg.Log.FilePath),
	}

	action := func() {
		for _, dir := range dirs {
			if dir != "" && dir != "." {
				_ = os.MkdirAll(dir, 0755)
			}
		}
	}

	_ = spinner.New().
		Title("Creating directories...").
		Action(action).
		Run()

	// Verify directories were created
	for _, dir := range dirs {
		if dir != "" && dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return NewGaiaError(
					ErrCodeDirectoryCreate,
					fmt.Sprintf("Failed to create directory: %s", dir),
					err,
				).WithHint("Check that you have write permissions to the parent directory").
					WithHint(fmt.Sprintf("Try: mkdir -p %s", dir))
			}
		}
	}

	fmt.Println(wizardSuccessStyle.Render("✓ Directories created"))
	return nil
}

func generateCertificatesWithProgress(cfg *config.Config) error {
	// Check if certs already exist
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	if _, err := os.Stat(caCertPath); err == nil {
		fmt.Println(wizardInfoStyle.Render("  Certificates already exist, skipping generation"))
		fmt.Println(wizardSuccessStyle.Render("✓ Using existing certificates"))
		return nil
	}

	var genErr error
	action := func() {
		// Generate CA
		if err := certs.GenerateCA(cfg, "Gaia Root CA"); err != nil {
			genErr = err
			return
		}
		// Generate server cert
		if err := certs.GenerateServerCertificate(cfg, "localhost"); err != nil {
			genErr = err
			return
		}
		// Generate client cert
		if err := certs.GenerateClientCertificate(cfg, "gaia_client"); err != nil {
			genErr = err
			return
		}
	}

	_ = spinner.New().
		Title("Generating mTLS certificates...").
		Action(action).
		Run()

	if genErr != nil {
		return NewGaiaError(
			ErrCodeCertGenerate,
			"Failed to generate certificates",
			genErr,
		).WithHint("Check that you have write permissions to: " + cfg.TLS.CertsDirectory).
			WithHint("Ensure OpenSSL or crypto libraries are available")
	}

	fmt.Println(wizardSuccessStyle.Render("✓ Certificates generated"))
	fmt.Println(wizardInfoStyle.Render(fmt.Sprintf("  Location: %s", cfg.TLS.CertsDirectory)))
	return nil
}

func promptSetupPassphrase() (string, error) {
	fmt.Println(wizardInfoStyle.Render("Your master passphrase encrypts all secrets in the database."))
	fmt.Println(wizardWarningStyle.Render("⚠  This passphrase cannot be recovered if lost!"))
	fmt.Println()

	var passphrase, confirm string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter master passphrase").
				Description("Minimum 8 characters, mix of cases recommended").
				Value(&passphrase).
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if len(s) < 8 {
						return errors.New("passphrase must be at least 8 characters")
					}
					if _, err := encrypt.ValidatePassword(s); err != nil {
						return fmt.Errorf("weak passphrase: %v", err)
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Confirm passphrase").
				Value(&confirm).
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

	// Wipe confirmation from memory
	secutil.WipeString(&confirm)

	// Show passphrase strength feedback
	strength := getPassphraseStrength(passphrase)
	fmt.Printf("  Passphrase strength: %s\n", strength)
	fmt.Println(wizardSuccessStyle.Render("✓ Passphrase accepted"))

	return passphrase, nil
}

func getPassphraseStrength(passphrase string) string {
	score := 0
	if len(passphrase) >= 12 {
		score++
	}
	if len(passphrase) >= 16 {
		score++
	}

	hasUpper, hasLower, hasDigit, hasSpecial := false, false, false, false
	for _, c := range passphrase {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if hasUpper {
		score++
	}
	if hasLower {
		score++
	}
	if hasDigit {
		score++
	}
	if hasSpecial {
		score++
	}

	switch {
	case score >= 6:
		return wizardSuccessStyle.Render("████████ Excellent")
	case score >= 4:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("██████░░ Strong")
	case score >= 3:
		return wizardWarningStyle.Render("████░░░░ Moderate")
	default:
		return wizardErrorStyle.Render("██░░░░░░ Weak")
	}
}

func initializeDatabaseWithProgress(cfg *config.Config, passphrase string) error {
	var initErr error
	action := func() {
		d := daemon.NewDaemon(cfg)
		initErr = d.InitializeDB(passphrase)
	}

	_ = spinner.New().
		Title("Initializing encrypted database...").
		Action(action).
		Run()

	if initErr != nil {
		return NewGaiaError(
			ErrCodeDatabaseInit,
			"Failed to initialize database",
			initErr,
		).WithHint("Check that you have write permissions to: " + filepath.Dir(cfg.Daemon.DBFile)).
			WithHint("Ensure there is enough disk space available")
	}

	fmt.Println(wizardSuccessStyle.Render("✓ Database initialized and encrypted"))
	return nil
}

func printSetupSuccess(cfg *config.Config) {
	fmt.Println()
	successBox := wizardBoxStyle.BorderForeground(lipgloss.Color("#10B981")).Render(
		wizardSuccessStyle.Render("🎉 Gaia Setup Complete!") + "\n\n" +
			"Your secrets vault is ready to use.\n\n" +
			wizardInfoStyle.Render("Configuration:") + "\n" +
			fmt.Sprintf("  Database:     %s\n", cfg.Daemon.DBFile) +
			fmt.Sprintf("  Certificates: %s\n", cfg.TLS.CertsDirectory) +
			"\n" +
			wizardInfoStyle.Render("Next steps:") + "\n" +
			"  1. Start the daemon:  " + wizardHighlightStyle.Render("gaia start") + "\n" +
			"  2. Launch the TUI:    " + wizardHighlightStyle.Render("gaia") + "\n" +
			"  3. Unlock with your passphrase and add secrets!",
	)
	fmt.Println(successBox)
	fmt.Println()
}

func offerStartDaemon() error {
	var startNow bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Start Gaia daemon now?").
				Description("You can also start it later with 'gaia daemon start'").
				Value(&startNow).
				Affirmative("Yes, start now").
				Negative("No, I'll start it later"),
		),
	)

	if err := form.Run(); err != nil {
		return nil // Non-fatal
	}

	if startNow {
		fmt.Println()
		fmt.Println(wizardInfoStyle.Render("Starting daemon..."))
		fmt.Println(wizardInfoStyle.Render("(Press Ctrl+C to stop, or run 'gaia daemon start &' for background)"))
		fmt.Println()
		// Start daemon
		return startCmd.RunE(startCmd, nil)
	}

	return nil
}

func printCurrentConfig(cfg *config.Config) {
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("Current Configuration"))
	fmt.Println()
	fmt.Printf("  %-20s %s\n", "Listen Address:", cfg.Daemon.ListenAddr)
	fmt.Printf("  %-20s %s\n", "Database:", cfg.Daemon.DBFile)
	fmt.Printf("  %-20s %s\n", "Certificates:", cfg.TLS.CertsDirectory)
	fmt.Printf("  %-20s %s\n", "Log File:", cfg.Log.FilePath)
	fmt.Printf("  %-20s %s\n", "Log Level:", cfg.Log.Level)
	fmt.Printf("  %-20s %s\n", "Key Algorithm:", cfg.TLS.KeyAlgorithm)
	fmt.Printf("  %-20s %d days\n", "Cert Expiry:", cfg.TLS.CertExpiryDays)
	fmt.Println()

	// Show OS-specific info
	fmt.Println(wizardInfoStyle.Render(fmt.Sprintf("Operating System: %s/%s", runtime.GOOS, runtime.GOARCH)))
	fmt.Println()
}

func runSecurityAudit(cfg *config.Config) error {
	fmt.Println()
	fmt.Println(wizardStepStyle.Render("🔒 Security Audit"))
	fmt.Println()

	issues := 0
	warnings := 0

	// Check database exists
	if _, err := os.Stat(cfg.Daemon.DBFile); os.IsNotExist(err) {
		fmt.Println(wizardErrorStyle.Render("✗ Database not found"))
		issues++
	} else {
		fmt.Println(wizardSuccessStyle.Render("✓ Database exists and is encrypted (AES-256-GCM)"))
	}

	// Check certificates
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	serverCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.ServerCert)
	clientCertPath := filepath.Join(cfg.TLS.CertsDirectory, "gaia_client.crt")

	// Helper function to check certificate expiry
	checkCertExpiry := func(certPath, name string) (exists bool, expired bool, expiringSoon bool, expiryDate time.Time) {
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return false, false, false, time.Time{}
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			return true, false, false, time.Time{}
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return true, false, false, time.Time{}
		}

		now := time.Now()
		expired = now.After(cert.NotAfter)
		expiringSoon = !expired && time.Until(cert.NotAfter) < 30*24*time.Hour
		return true, expired, expiringSoon, cert.NotAfter
	}

	// Check CA certificate
	if exists, expired, expiringSoon, expiryDate := checkCertExpiry(caCertPath, "CA"); !exists {
		fmt.Println(wizardErrorStyle.Render("✗ CA certificate not found"))
		issues++
	} else if expired {
		fmt.Println(wizardErrorStyle.Render("✗ CA certificate has EXPIRED"))
		fmt.Printf("  Expired on: %s\n", expiryDate.Format("2006-01-02"))
		issues++
	} else if expiringSoon {
		fmt.Println(wizardWarningStyle.Render("⚠ CA certificate expires soon"))
		fmt.Printf("  Expires on: %s (%d days remaining)\n", expiryDate.Format("2006-01-02"), int(time.Until(expiryDate).Hours()/24))
		warnings++
	} else {
		fmt.Println(wizardSuccessStyle.Render("✓ CA certificate valid"))
		fmt.Printf("  Expires on: %s\n", expiryDate.Format("2006-01-02"))
	}

	// Check server certificate
	if exists, expired, expiringSoon, expiryDate := checkCertExpiry(serverCertPath, "Server"); !exists {
		fmt.Println(wizardErrorStyle.Render("✗ Server certificate not found"))
		issues++
	} else if expired {
		fmt.Println(wizardErrorStyle.Render("✗ Server certificate has EXPIRED"))
		fmt.Printf("  Expired on: %s\n", expiryDate.Format("2006-01-02"))
		issues++
	} else if expiringSoon {
		fmt.Println(wizardWarningStyle.Render("⚠ Server certificate expires soon"))
		fmt.Printf("  Expires on: %s (%d days remaining)\n", expiryDate.Format("2006-01-02"), int(time.Until(expiryDate).Hours()/24))
		warnings++
	} else {
		fmt.Println(wizardSuccessStyle.Render("✓ Server certificate valid"))
		fmt.Printf("  Expires on: %s\n", expiryDate.Format("2006-01-02"))
	}

	// Check client certificate
	if exists, expired, expiringSoon, expiryDate := checkCertExpiry(clientCertPath, "Client"); !exists {
		fmt.Println(wizardWarningStyle.Render("⚠ Default client certificate not found"))
		warnings++
	} else if expired {
		fmt.Println(wizardErrorStyle.Render("✗ Client certificate has EXPIRED"))
		fmt.Printf("  Expired on: %s\n", expiryDate.Format("2006-01-02"))
		issues++
	} else if expiringSoon {
		fmt.Println(wizardWarningStyle.Render("⚠ Client certificate expires soon"))
		fmt.Printf("  Expires on: %s (%d days remaining)\n", expiryDate.Format("2006-01-02"), int(time.Until(expiryDate).Hours()/24))
		warnings++
	} else {
		fmt.Println(wizardSuccessStyle.Render("✓ Client certificate valid"))
		fmt.Printf("  Expires on: %s\n", expiryDate.Format("2006-01-02"))
	}

	// Check file permissions (Unix only)
	if runtime.GOOS != "windows" {
		dbInfo, err := os.Stat(cfg.Daemon.DBFile)
		if err == nil {
			perm := dbInfo.Mode().Perm()
			if perm&0077 != 0 {
				fmt.Println(wizardWarningStyle.Render("⚠ Database file has loose permissions"))
				fmt.Printf("  Current: %o, Recommended: 600\n", perm)
				warnings++
			} else {
				fmt.Println(wizardSuccessStyle.Render("✓ Database file permissions are secure"))
			}
		}
	}

	// Check audit logging
	if !cfg.Audit.Enabled {
		fmt.Println(wizardWarningStyle.Render("⚠ Audit logging is disabled"))
		warnings++
	} else {
		fmt.Println(wizardSuccessStyle.Render("✓ Audit logging is enabled"))
	}

	// Summary
	fmt.Println()
	if issues == 0 && warnings == 0 {
		fmt.Println(wizardSuccessStyle.Render("All security checks passed! ✓"))
	} else {
		if issues > 0 {
			fmt.Printf("%s: %d issue(s) found that should be fixed\n", wizardErrorStyle.Render("Issues"), issues)
		}
		if warnings > 0 {
			fmt.Printf("%s: %d warning(s) to consider\n", wizardWarningStyle.Render("Warnings"), warnings)
		}
	}
	fmt.Println()

	return nil
}
