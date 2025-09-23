package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configurable settings for Gaia.
type Config struct {
	GRPCServerName      string        `yaml:"grpc_server_name"`
	GRPCPort            string        `yaml:"grpc_port"`
	DBFile              string        `yaml:"db_file"`
	CertsDirectory      string        `yaml:"certs_directory"`
	CACertFile          string        `yaml:"ca_cert_file"`
	ServerCertFile      string        `yaml:"server_cert_file"`
	ServerKeyFile       string        `yaml:"server_key_file"`
	GaiaClientCertFile  string        `yaml:"gaia_client_cert_file"`
	GaianClientKeyFile  string        `yaml:"gaia_client_key_file"`
	GRPCClientTimeout   time.Duration `yaml:"grpc_client_timeout"`
	GaiaTuiTickInterval time.Duration `yaml:"gaia_tui_tick_interval"`
	CertExpiryDays      int           `yaml:"cert_expiry_days"`
}

// NewDefaultConfig returns a Config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		GRPCServerName:      "localhost",
		GRPCPort:            "50051",
		DBFile:              "gaia.db",
		CertsDirectory:      "./certs",
		CACertFile:          "ca.crt",
		ServerCertFile:      "server.crt",
		ServerKeyFile:       "server.key",
		GaiaClientCertFile:  "gaia_client.crt",
		GaianClientKeyFile:  "gaia_client.key",
		GRPCClientTimeout:   5 * time.Second,
		GaiaTuiTickInterval: 2 * time.Second,
		CertExpiryDays:      365, // Default to 365 days
	}
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
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config from file '%s': %w", path, err)
	}
	return nil
}

// loadConfigFromEnv populates the Config struct with values from environment variables.
func loadConfigFromEnv(cfg *Config) {
	if dbFile := os.Getenv("GAIA_DB_FILE"); dbFile != "" {
		cfg.DBFile = dbFile
	}
	if grpcPort := os.Getenv("GAIA_GRPC_PORT"); grpcPort != "" {
		cfg.GRPCPort = grpcPort
	}
}

// WriteConfigToFile writes the given config to the specified path.
func WriteConfigToFile(cfg *Config) error {
	path, err := getDefaultConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get default config path: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
