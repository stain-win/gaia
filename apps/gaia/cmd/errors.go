package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Error codes for categorized error handling
const (
	ErrCodeUnknown         = "UNKNOWN"
	ErrCodeConfigLoad      = "CONFIG_LOAD"
	ErrCodeConfigParse     = "CONFIG_PARSE"
	ErrCodeDatabaseInit    = "DB_INIT"
	ErrCodeDatabaseOpen    = "DB_OPEN"
	ErrCodeDatabaseLocked  = "DB_LOCKED"
	ErrCodeCertNotFound    = "CERT_NOT_FOUND"
	ErrCodeCertGenerate    = "CERT_GENERATE"
	ErrCodeCertExpired     = "CERT_EXPIRED"
	ErrCodeCertInvalid     = "CERT_INVALID"
	ErrCodeDaemonConnect   = "DAEMON_CONNECT"
	ErrCodeDaemonOffline   = "DAEMON_OFFLINE"
	ErrCodeDaemonLocked    = "DAEMON_LOCKED"
	ErrCodeAuthFailed      = "AUTH_FAILED"
	ErrCodePermission      = "PERMISSION"
	ErrCodeDirectoryCreate = "DIR_CREATE"
	ErrCodeSecretNotFound  = "SECRET_NOT_FOUND"
	ErrCodeClientNotFound  = "CLIENT_NOT_FOUND"
	ErrCodePassphrase      = "PASSPHRASE"
	ErrCodeNetwork         = "NETWORK"
	ErrCodeTimeout         = "TIMEOUT"
)

// GaiaError is an enhanced error type with actionable hints
type GaiaError struct {
	Code    string
	Message string
	Cause   error
	Hints   []string
	DocsURL string
}

// NewGaiaError creates a new GaiaError
func NewGaiaError(code, message string, cause error) *GaiaError {
	return &GaiaError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Hints:   []string{},
	}
}

// WithHint adds a hint to the error
func (e *GaiaError) WithHint(hint string) *GaiaError {
	e.Hints = append(e.Hints, hint)
	return e
}

// WithDocs adds a documentation URL
func (e *GaiaError) WithDocs(url string) *GaiaError {
	e.DocsURL = url
	return e
}

// Error implements the error interface
func (e *GaiaError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error
func (e *GaiaError) Unwrap() error {
	return e.Cause
}

// Styles for error display
var (
	errTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444"))

	errCodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)

	errMessageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171"))

	errHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA"))

	errCauseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true)

	errDocsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Underline(true)

	errBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#EF4444")).
			Padding(1, 2).
			MarginTop(1)
)

// FormatError formats a GaiaError for display
func FormatError(err error) string {
	gaiaErr, ok := err.(*GaiaError)
	if !ok {
		// Wrap regular errors
		gaiaErr = wrapError(err)
	}

	var sb strings.Builder

	// Error title and code
	sb.WriteString(errTitleStyle.Render("❌ Error"))
	if gaiaErr.Code != "" && gaiaErr.Code != ErrCodeUnknown {
		sb.WriteString(" ")
		sb.WriteString(errCodeStyle.Render(fmt.Sprintf("[%s]", gaiaErr.Code)))
	}
	sb.WriteString("\n\n")

	// Main message
	sb.WriteString(errMessageStyle.Render(gaiaErr.Message))
	sb.WriteString("\n")

	// Original cause (if different from message)
	if gaiaErr.Cause != nil && gaiaErr.Cause.Error() != gaiaErr.Message {
		sb.WriteString("\n")
		sb.WriteString(errCauseStyle.Render(fmt.Sprintf("Cause: %s", gaiaErr.Cause.Error())))
		sb.WriteString("\n")
	}

	// Hints
	if len(gaiaErr.Hints) > 0 {
		sb.WriteString("\n")
		sb.WriteString(errHintStyle.Render("💡 To fix this:\n"))
		for i, hint := range gaiaErr.Hints {
			fmt.Fprintf(&sb, "   %d. %s\n", i+1, hint)
		}
	}

	// Documentation link
	if gaiaErr.DocsURL != "" {
		sb.WriteString("\n")
		sb.WriteString(errDocsStyle.Render(fmt.Sprintf("📚 See: %s", gaiaErr.DocsURL)))
		sb.WriteString("\n")
	}

	return errBoxStyle.Render(sb.String())
}

// PrintError prints a formatted error to stdout
func PrintError(err error) {
	fmt.Println(FormatError(err))
}

// wrapError wraps a regular error with contextual hints based on error message patterns
func wrapError(err error) *GaiaError {
	if err == nil {
		return nil
	}

	msg := err.Error()
	lowerMsg := strings.ToLower(msg)

	// Pattern matching for common errors
	switch {
	// Certificate errors
	case strings.Contains(lowerMsg, "certificate") && strings.Contains(lowerMsg, "not found"):
		return NewGaiaError(ErrCodeCertNotFound, "Certificate file not found", err).
			WithHint("Generate certificates with: gaia certs generate").
			WithHint("Or run the setup wizard: gaia setup").
			WithDocs("https://github.com/stain-win/gaia#certificate-management")

	case strings.Contains(lowerMsg, "ca.crt") || strings.Contains(lowerMsg, "ca.key"):
		return NewGaiaError(ErrCodeCertNotFound, "CA certificate not found", err).
			WithHint("Generate a new CA: gaia certs create-ca").
			WithHint("Or run full setup: gaia setup")

	case strings.Contains(lowerMsg, "server.crt") || strings.Contains(lowerMsg, "server.key"):
		return NewGaiaError(ErrCodeCertNotFound, "Server certificate not found", err).
			WithHint("Generate server certificate: gaia certs create-server localhost").
			WithHint("Make sure CA exists first (ca.crt, ca.key)")

	case strings.Contains(lowerMsg, "x509") || strings.Contains(lowerMsg, "certificate has expired"):
		return NewGaiaError(ErrCodeCertExpired, "Certificate has expired", err).
			WithHint("Regenerate certificates: gaia certs generate").
			WithHint("Consider increasing cert_expiry_days in config")

	case strings.Contains(lowerMsg, "certificate verify") || strings.Contains(lowerMsg, "unknown authority"):
		return NewGaiaError(ErrCodeCertInvalid, "Certificate validation failed", err).
			WithHint("Ensure client certificate is signed by the same CA").
			WithHint("Check that ca.crt matches on both client and server")

	// Connection errors
	case strings.Contains(lowerMsg, "connection refused"):
		return NewGaiaError(ErrCodeDaemonOffline, "Cannot connect to Gaia daemon", err).
			WithHint("Start the daemon: gaia daemon start").
			WithHint("Check if daemon is running: gaia daemon status").
			WithHint("Verify the address in config matches daemon's listen address")

	case strings.Contains(lowerMsg, "dial"):
		return NewGaiaError(ErrCodeNetwork, "Network connection failed", err).
			WithHint("Check that the daemon is running").
			WithHint("Verify network connectivity to the daemon host").
			WithHint("Check firewall rules if connecting remotely")

	case strings.Contains(lowerMsg, "timeout") || strings.Contains(lowerMsg, "deadline exceeded"):
		return NewGaiaError(ErrCodeTimeout, "Operation timed out", err).
			WithHint("The daemon may be overloaded or unresponsive").
			WithHint("Try increasing the timeout in config").
			WithHint("Check system resources (CPU, memory, disk)")

	// Database errors
	case strings.Contains(lowerMsg, "database") && strings.Contains(lowerMsg, "locked"):
		return NewGaiaError(ErrCodeDatabaseLocked, "Database is locked", err).
			WithHint("Unlock the daemon: gaia daemon unlock").
			WithHint("Or use the TUI to unlock: gaia")

	case strings.Contains(lowerMsg, "database") && strings.Contains(lowerMsg, "not found"):
		return NewGaiaError(ErrCodeDatabaseOpen, "Database file not found", err).
			WithHint("Initialize Gaia first: gaia init").
			WithHint("Or run the setup wizard: gaia setup").
			WithHint("Check the db_file path in your config")

	case strings.Contains(lowerMsg, "passphrase") || strings.Contains(lowerMsg, "password"):
		return NewGaiaError(ErrCodePassphrase, "Passphrase error", err).
			WithHint("Make sure you're using the correct master passphrase").
			WithHint("The passphrase was set during 'gaia init' or 'gaia setup'")

	// Permission errors
	case strings.Contains(lowerMsg, "permission denied"):
		return NewGaiaError(ErrCodePermission, "Permission denied", err).
			WithHint("Check file/directory permissions").
			WithHint("On Linux, you may need to run as root or use sudo").
			WithHint("Ensure the user running Gaia owns the data directories")

	case strings.Contains(lowerMsg, "access denied"):
		return NewGaiaError(ErrCodePermission, "Access denied", err).
			WithHint("Check that your client certificate is valid").
			WithHint("Ensure the client is registered: gaia clients list")

	// Config errors
	case strings.Contains(lowerMsg, "config") && strings.Contains(lowerMsg, "not found"):
		return NewGaiaError(ErrCodeConfigLoad, "Configuration file not found", err).
			WithHint("Run setup to create config: gaia setup").
			WithHint("Or specify config path: gaia --config /path/to/config.yaml")

	case strings.Contains(lowerMsg, "yaml") || strings.Contains(lowerMsg, "unmarshal"):
		return NewGaiaError(ErrCodeConfigParse, "Configuration file is invalid", err).
			WithHint("Check YAML syntax in your config file").
			WithHint("Use a YAML validator to find issues").
			WithHint("See template: gaia-config.template.yaml")

	// Secret errors
	case strings.Contains(lowerMsg, "secret") && strings.Contains(lowerMsg, "not found"):
		return NewGaiaError(ErrCodeSecretNotFound, "Secret not found", err).
			WithHint("Check that the secret key exists").
			WithHint("List secrets: gaia secrets list").
			WithHint("Secret names are case-sensitive")

	// Client errors
	case strings.Contains(lowerMsg, "client") && strings.Contains(lowerMsg, "not found"):
		return NewGaiaError(ErrCodeClientNotFound, "Client not found", err).
			WithHint("Register the client: gaia clients register <name>").
			WithHint("List clients: gaia clients list")

	// Daemon state errors
	case strings.Contains(lowerMsg, "daemon") && strings.Contains(lowerMsg, "locked"):
		return NewGaiaError(ErrCodeDaemonLocked, "Gaia daemon is locked", err).
			WithHint("Unlock with: gaia daemon unlock").
			WithHint("Or use the TUI: gaia → Unlock Gaia")

	default:
		return NewGaiaError(ErrCodeUnknown, msg, err).
			WithHint("Check the daemon logs for more details").
			WithHint("Run 'gaia daemon status' to check daemon state").
			WithHint("See documentation: https://github.com/stain-win/gaia")
	}
}

// wrapSetupError creates an error for the setup wizard
func wrapSetupError(step string, err error) error {
	return NewGaiaError(
		ErrCodeUnknown,
		fmt.Sprintf("Setup failed during %s", step),
		err,
	).WithHint("Check the error message above for details").
		WithHint("You can retry the setup with: gaia setup").
		WithHint("For manual setup, see: gaia init --help")
}

// Common error constructors for use throughout the codebase

// ErrCertificateNotFound creates a certificate not found error
func ErrCertificateNotFound(certType, path string) *GaiaError {
	return NewGaiaError(
		ErrCodeCertNotFound,
		fmt.Sprintf("%s certificate not found: %s", certType, path),
		nil,
	).WithHint("Generate certificates with: gaia certs generate").
		WithHint("Or run the setup wizard: gaia setup")
}

// ErrDaemonOffline creates a daemon offline error
func ErrDaemonOffline() *GaiaError {
	return NewGaiaError(
		ErrCodeDaemonOffline,
		"Cannot connect to Gaia daemon - it appears to be offline",
		nil,
	).WithHint("Start the daemon: gaia daemon start").
		WithHint("Check status: gaia daemon status")
}

// ErrDaemonLocked creates a daemon locked error
func ErrDaemonLocked() *GaiaError {
	return NewGaiaError(
		ErrCodeDaemonLocked,
		"Gaia daemon is locked - secrets are not accessible",
		nil,
	).WithHint("Unlock with: gaia daemon unlock").
		WithHint("Or use the TUI: gaia → select 'Unlock Gaia'")
}

// ErrDatabaseNotInitialized creates a database not initialized error
func ErrDatabaseNotInitialized() *GaiaError {
	return NewGaiaError(
		ErrCodeDatabaseOpen,
		"Gaia is not initialized - no database found",
		nil,
	).WithHint("Run the setup wizard: gaia setup").
		WithHint("Or initialize manually: gaia init")
}

// ErrInvalidPassphrase creates an invalid passphrase error
func ErrInvalidPassphrase() *GaiaError {
	return NewGaiaError(
		ErrCodePassphrase,
		"Invalid passphrase",
		nil,
	).WithHint("Make sure you're entering the correct master passphrase").
		WithHint("The passphrase was set during initial setup").
		WithHint("If forgotten, you'll need to reinitialize (data will be lost)")
}
