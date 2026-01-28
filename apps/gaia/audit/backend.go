package audit

import (
	"context"
)

// Backend defines the interface for audit log storage backends.
// Each backend is responsible for persisting audit entries in its own format and location.
type Backend interface {
	// Type returns the backend type identifier (e.g., "file", "internal", "webhook").
	Type() string

	// Log writes an audit entry to the backend.
	// The entry has already been redacted by the time it reaches the backend.
	Log(ctx context.Context, entry *Entry) error

	// Close gracefully shuts down the backend, flushing any buffered entries.
	Close() error
}

// BackendConfig contains configuration for a specific audit backend.
type BackendConfig struct {
	// Type is the backend type: "internal", "file", or "webhook".
	Type string `yaml:"type"`

	// Path is the destination path:
	// - For "file": file path, or "-" for stdout
	// - For "internal": unused (stored in BoltDB)
	// - For "webhook": HTTP/gRPC endpoint URL
	Path string `yaml:"path,omitempty"`

	// Options contains backend-specific configuration.
	Options BackendOptions `yaml:"options,omitempty"`
}

// BackendOptions contains optional settings for backends.
type BackendOptions struct {
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

	// Common options
	Format string `yaml:"format,omitempty"` // Output format: "json" (default), "jsonl"
}

// DefaultBackendOptions returns sensible default options for backends.
func DefaultBackendOptions() BackendOptions {
	return BackendOptions{
		MaxSizeMB:       100,
		MaxBackups:      5,
		MaxAgeDays:      30,
		RetentionDays:   30,
		RateLimitPerSec: 100,
		TimeoutSeconds:  10,
		Format:          "json",
	}
}
