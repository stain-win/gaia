// Package audit provides audit logging for the Gaia daemon.
package audit

import (
	"fmt"

	"go.etcd.io/bbolt"
)

// NewBackendFromConfig creates a backend based on the configuration.
// For internal backend, the BoltDB database instance must be provided.
func NewBackendFromConfig(cfg BackendConfig, db *bbolt.DB) (Backend, error) {
	switch cfg.Type {
	case "file":
		return NewFileBackend(cfg)

	case "internal":
		if db == nil {
			return nil, fmt.Errorf("database is required for internal backend")
		}
		return NewInternalBackend(db, cfg)

	case "webhook":
		return NewWebhookBackend(cfg)

	default:
		return nil, fmt.Errorf("unknown audit backend type: %s", cfg.Type)
	}
}

// NewBackendsFromConfig creates all backends from a list of configurations.
func NewBackendsFromConfig(configs []BackendConfig, db *bbolt.DB) ([]Backend, error) {
	var backends []Backend

	for i, cfg := range configs {
		backend, err := NewBackendFromConfig(cfg, db)
		if err != nil {
			// Close any already created backends
			for _, b := range backends {
				_ = b.Close()
			}
			return nil, fmt.Errorf("failed to create backend %d (%s): %w", i, cfg.Type, err)
		}
		backends = append(backends, backend)
	}

	return backends, nil
}
