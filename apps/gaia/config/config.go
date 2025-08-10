package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config holds all configurable settings for Gaia.
type Config struct {
	GRPCPort           string `yaml:"grpc_port"`
	DBFile             string `yaml:"db_file"`
	CACertFile         string `yaml:"ca_cert_file"`
	ServerCertFile     string `yaml:"server_cert_file"`
	ServerKeyFile      string `yaml:"server_key_file"`
	GaiaClientCertFile string `yaml:"gaia_client_cert_file"`
	GaianClientKeyFile string `yaml:"gaia_client_key_file"`
}

// NewDefaultConfig returns a Config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		GRPCPort:           "50051",
		DBFile:             "gaia.db",
		CACertFile:         "./certs/ca.crt",
		ServerCertFile:     "./certs/server.crt",
		ServerKeyFile:      "./certs/server.key",
		GaiaClientCertFile: "./certs/gaia_client.crt",
		GaianClientKeyFile: "./certs/gaia_client.key",
	}
}

// GetConfigPath returns the OS-specific path for the config file.
func GetConfigPath() (string, error) {
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

// WriteConfigToFile writes the given config to the specified path.
func WriteConfigToFile(cfg *Config, path string) error {
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
