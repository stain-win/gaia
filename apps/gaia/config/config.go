// Package config manages configuration for the Gaia daemon.
// It loads settings from config.yaml and environment variables, providing defaults
// for all configuration options including server ports, file paths, and timeouts.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// DaemonConfig holds settings specific to the Gaia daemon's operation.
type DaemonConfig struct {
	ListenAddr string        `yaml:"listen_addr"` // Combines host and port (e.g., "0.0.0.0:50051")
	DBFile     string        `yaml:"db_file"`     // Path to the BoltDB database file
	Timeout    time.Duration `yaml:"timeout"`     // General server-side operation timeout
}

// LogConfig holds settings for structured logging and log rotation.
type LogConfig struct {
	FilePath   string `yaml:"file_path"`    // Path to log file
	MaxSizeMB  int    `yaml:"max_size_mb"`  // Maximum size in MB before rotation
	MaxBackups int    `yaml:"max_backups"`  // Maximum number of old log files to keep
	MaxAgeDays int    `yaml:"max_age_days"` // Maximum days to keep old log files
	Level      string `yaml:"level"`        // Log level: debug, info, warn, error
}

// TLSConfig holds settings for mTLS (mutual TLS).
type TLSConfig struct {
	CertsDirectory string `yaml:"certs_directory"`  // Base directory for all certificates
	CACert         string `yaml:"ca_cert"`          // CA certificate file (relative to CertsDirectory)
	ServerCert     string `yaml:"server_cert"`      // Server certificate file (relative to CertsDirectory)
	ServerKey      string `yaml:"server_key"`       // Server private key file (relative to CertsDirectory)
	KeyAlgorithm   string `yaml:"key_algorithm"`    // Key algorithm: "ECDSA" or "RSA"
	KeySize        string `yaml:"key_size"`         // Key size: "P256", "P384", "P521" for ECDSA; "2048", "3072", "4096" for RSA
	CertExpiryDays int    `yaml:"cert_expiry_days"` // Certificate validity period in days

	// CAExpiryDays is the validity period of the root CA certificate in days.
	// Kept explicit (rather than a hidden multiple of cert_expiry_days) so
	// operators can reason about the root-of-trust lifetime. When zero, it
	// falls back to 10x cert_expiry_days, the historical behaviour.
	CAExpiryDays int `yaml:"ca_expiry_days"`

	// AdminCommonNames lists certificate Common Names authorized for the privileged
	// GaiaAdmin service in addition to any certificate carrying the admin OU marker.
	// It exists for backward compatibility with admin certificates issued before the
	// OU marker was introduced. Empty it to require the OU marker for all admin calls.
	AdminCommonNames []string `yaml:"admin_common_names"`
}

// AuditBackendConfig holds settings for a single audit backend.
type AuditBackendConfig struct {
	Type    string              `yaml:"type"`              // Backend type: "file", "internal", "webhook"
	Path    string              `yaml:"path,omitempty"`    // Path for file backend or URL for webhook
	Options AuditBackendOptions `yaml:"options,omitempty"` // Backend-specific options
}

// AuditBackendOptions holds optional settings for audit backends.
type AuditBackendOptions struct {
	// File backend options
	MaxSizeMB  int `yaml:"max_size_mb,omitempty"`  // Maximum file size in MB before rotation
	MaxBackups int `yaml:"max_backups,omitempty"`  // Maximum number of old files to keep
	MaxAgeDays int `yaml:"max_age_days,omitempty"` // Maximum days to keep old files

	// Internal backend options
	RetentionDays int `yaml:"retention_days,omitempty"` // Days to retain entries in BoltDB

	// Webhook backend options
	RateLimitPerSec int    `yaml:"rate_limit_per_sec,omitempty"` // Max requests per second
	TimeoutSeconds  int    `yaml:"timeout_seconds,omitempty"`    // Request timeout
	Headers         string `yaml:"headers,omitempty"`            // Custom headers as JSON object
}

// AuditConfig holds settings for audit logging.
type AuditConfig struct {
	Enabled     bool                 `yaml:"enabled"`            // Enable audit logging
	HMACKey     string               `yaml:"hmac_key,omitempty"` // HMAC key for hashing sensitive values
	LogRequest  bool                 `yaml:"log_request"`        // Log incoming requests
	LogResponse bool                 `yaml:"log_response"`       // Log outgoing responses
	Backends    []AuditBackendConfig `yaml:"backends,omitempty"` // List of audit backends
}

// Config holds all configurable settings for Gaia.
type Config struct {
	Daemon DaemonConfig `yaml:"daemon"` // Daemon-specific settings
	Log    LogConfig    `yaml:"log"`    // Logging settings
	TLS    TLSConfig    `yaml:"tls"`    // TLS/mTLS settings
	Audit  AuditConfig  `yaml:"audit"`  // Audit logging settings

	GRPCClientTimeout   time.Duration `yaml:"grpc_client_timeout"`    // Timeout for gRPC client operations
	GaiaTuiTickInterval time.Duration `yaml:"gaia_tui_tick_interval"` // TUI refresh interval

	// Client certificate files for the Gaia CLI/TUI (relative to TLS.CertsDirectory)
	GaiaClientCertFile string `yaml:"gaia_client_cert_file"` // Gaia client certificate file
	GaiaClientKeyFile  string `yaml:"gaia_client_key_file"`  // Gaia client private key file

	// Unsafe local dev mode (runtime-only, never persisted to config file)
	UnsafeMode bool   `yaml:"-"` // When true, relaxes passphrase strength and uses faster key derivation
	UnsafeDir  string `yaml:"-"` // Resolved absolute path of --dir for unsafe mode

	// Ephemeral mode (runtime-only, never persisted to config file)
	EphemeralMode bool   `yaml:"-"` // When true, all data lives in a temp file unlinked at startup
	EphemeralDir  string `yaml:"-"` // Temp dir created for ephemeral certs; cleaned up after Stop
}

// NewDefaultConfig returns a Config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			// Loopback by default: a secrets daemon should not be network-reachable
			// unless the operator explicitly opts in (e.g. "0.0.0.0:50051").
			ListenAddr: "127.0.0.1:50051",
			DBFile:     getDefaultDBPath(),
			Timeout:    10 * time.Second,
		},
		Log: LogConfig{
			FilePath:   getDefaultLogPath(),
			MaxSizeMB:  10,
			MaxBackups: 5,
			MaxAgeDays: 7,
			Level:      "info",
		},
		TLS: TLSConfig{
			CertsDirectory:   getDefaultCertsDirectory(),
			CACert:           "ca.crt",
			ServerCert:       "server.crt",
			ServerKey:        "server.key",
			KeyAlgorithm:     "ECDSA",
			KeySize:          "P256",
			CertExpiryDays:   365,
			CAExpiryDays:     3650,
			AdminCommonNames: []string{"gaia_client"},
		},
		Audit: AuditConfig{
			Enabled:     false, // Disabled by default for backward compatibility
			LogRequest:  true,
			LogResponse: true,
			Backends:    nil, // No backends configured by default
		},
		GRPCClientTimeout:   5 * time.Second,
		GaiaTuiTickInterval: 2 * time.Second,
		GaiaClientCertFile:  "gaia_client.crt",
		GaiaClientKeyFile:   "gaia_client.key",
	}
}

// getDefaultDBPath returns the OS-specific default database file path.
func getDefaultDBPath() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Gaia", "gaia.db")
		}
		return "gaia.db"
	case "darwin":
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "Library", "Application Support", "Gaia", "gaia.db")
		}
		return "gaia.db"
	case "linux":
		return "/var/lib/gaia/gaia.db"
	default:
		return "gaia.db"
	}
}

// getDefaultLogPath returns the OS-specific default log file path.
func getDefaultLogPath() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Gaia", "gaia.log")
		}
		return "gaia.log"
	case "darwin":
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "Library", "Logs", "Gaia", "gaia.log")
		}
		return "gaia.log"
	case "linux":
		return "/var/log/gaia/gaia.log"
	default:
		return "gaia.log"
	}
}

// getDefaultCertsDirectory returns the OS-specific default certificates directory.
func getDefaultCertsDirectory() string {
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Gaia", "certs")
		}
		return "./certs"
	case "darwin":
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "Library", "Application Support", "Gaia", "certs")
		}
		return "./certs"
	case "linux":
		return "/etc/gaia/certs"
	default:
		return "./certs"
	}
}

// Save writes the configuration to the specified path, or the default config path if empty.
// Runtime-only fields (UnsafeMode, UnsafeDir) are excluded via yaml:"-" tags.
// Returns the resolved path that was written.
func Save(cfg *Config, path string) (string, error) {
	if path == "" {
		var err error
		path, err = getDefaultConfigPath()
		if err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write config file '%s': %w", path, err)
	}

	return path, nil
}

// Load reads the configuration from the specified path or the default path if empty.
func Load(path string) (*Config, error) {
	cfg := NewDefaultConfig()

	if path == "" {
		var err error
		path, err = getDefaultConfigPath()
		if err != nil {
			return nil, err // This would be an error like "home directory not found"
		}
	}

	if _, err := os.Stat(path); err == nil {
		if err := loadConfigFromFile(path, cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	loadConfigFromEnv(cfg)

	return cfg, nil
}

// getDefaultConfigPath returns the OS-specific path for the config file.
func getDefaultConfigPath() (string, error) {
	var path string
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		path = filepath.Join(appData, "Gaia", "gaia-config.yaml")
	case "darwin": // macOS
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		path = filepath.Join(homeDir, "Library", "Application Support", "Gaia", "gaia-config.yaml")
	case "linux":
		path = filepath.Join("/etc", "gaia", "gaia-config.yaml")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	return path, nil
}

// loadConfigFromFile reads the configuration from the specified file path and unmarshal it into the Config struct.
func loadConfigFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file '%s': %w", path, err)
	}

	// First try the new nested config format
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config from file '%s': %w", path, err)
	}

	// Check if we loaded any meaningful nested config
	// If not, try legacy flat format for backwards compatibility
	if cfg.Daemon.ListenAddr == "" || cfg.Daemon.ListenAddr == "127.0.0.1:50051" {
		var legacy legacyConfig
		if err := yaml.Unmarshal(data, &legacy); err == nil {
			applyLegacyConfig(&legacy, cfg)
		}
	}

	return nil
}

// legacyConfig represents the old flat configuration format for backwards compatibility
type legacyConfig struct {
	GRPCServerName      string `yaml:"grpc_server_name"`
	GRPCPort            string `yaml:"grpc_port"`
	DBFile              string `yaml:"db_file"`
	CertsDirectory      string `yaml:"certs_directory"`
	CACertFile          string `yaml:"ca_cert_file"`
	ServerCertFile      string `yaml:"server_cert_file"`
	ServerKeyFile       string `yaml:"server_key_file"`
	GaiaClientCertFile  string `yaml:"gaia_client_cert_file"`
	GaiaClientKeyFile   string `yaml:"gaia_client_key_file"`
	GRPCClientTimeout   string `yaml:"grpc_client_timeout"`
	GaiaTuiTickInterval string `yaml:"gaia_tui_tick_interval"`
	CertExpiryDays      int    `yaml:"cert_expiry_days"`
}

// applyLegacyConfig migrates legacy config values to the new nested format
func applyLegacyConfig(legacy *legacyConfig, cfg *Config) {
	if legacy.GRPCPort != "" {
		cfg.Daemon.ListenAddr = "0.0.0.0:" + legacy.GRPCPort
	}
	if legacy.DBFile != "" {
		cfg.Daemon.DBFile = legacy.DBFile
	}
	if legacy.CertsDirectory != "" {
		cfg.TLS.CertsDirectory = legacy.CertsDirectory
	}
	if legacy.CACertFile != "" {
		cfg.TLS.CACert = legacy.CACertFile
	}
	if legacy.ServerCertFile != "" {
		cfg.TLS.ServerCert = legacy.ServerCertFile
	}
	if legacy.ServerKeyFile != "" {
		cfg.TLS.ServerKey = legacy.ServerKeyFile
	}
	if legacy.GaiaClientCertFile != "" {
		cfg.GaiaClientCertFile = legacy.GaiaClientCertFile
	}
	if legacy.GaiaClientKeyFile != "" {
		cfg.GaiaClientKeyFile = legacy.GaiaClientKeyFile
	}
	if legacy.GRPCClientTimeout != "" {
		if d, err := time.ParseDuration(legacy.GRPCClientTimeout); err == nil {
			cfg.GRPCClientTimeout = d
		}
	}
	if legacy.GaiaTuiTickInterval != "" {
		if d, err := time.ParseDuration(legacy.GaiaTuiTickInterval); err == nil {
			cfg.GaiaTuiTickInterval = d
		}
	}
	if legacy.CertExpiryDays > 0 {
		cfg.TLS.CertExpiryDays = legacy.CertExpiryDays
	}
}

// loadConfigFromEnv populates the Config struct with values from environment variables.
// Environment variables take precedence over config file settings.
func loadConfigFromEnv(cfg *Config) {
	// Daemon configuration
	if listenAddr := os.Getenv("GAIA_LISTEN_ADDR"); listenAddr != "" {
		cfg.Daemon.ListenAddr = listenAddr
	}
	if dbFile := os.Getenv("GAIA_DB_FILE"); dbFile != "" {
		cfg.Daemon.DBFile = dbFile
	}
	if timeout := os.Getenv("GAIA_DAEMON_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.Daemon.Timeout = d
		}
	}

	// Log configuration
	if logFile := os.Getenv("GAIA_LOG_FILE"); logFile != "" {
		cfg.Log.FilePath = logFile
	}
	if logLevel := os.Getenv("GAIA_LOG_LEVEL"); logLevel != "" {
		cfg.Log.Level = logLevel
	}

	// TLS configuration
	if certsDir := os.Getenv("GAIA_CERTS_DIR"); certsDir != "" {
		cfg.TLS.CertsDirectory = certsDir
	}

	// Client timeouts
	if timeout := os.Getenv("GAIA_CLIENT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.GRPCClientTimeout = d
		}
	}
}
