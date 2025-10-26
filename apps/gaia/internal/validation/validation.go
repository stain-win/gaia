// Package validation provides input validation for client names, namespace names, and secret keys.
// It enforces Gaia's naming rules: names must be 1-63 characters, consist of lowercase letters,
// numbers, hyphens, and underscores, and must start and end with a letter or number.
package validation

import (
	"fmt"
	"regexp"

	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
)

// nameValidationRegex defines the allowed format for client, namespace, and key names.
// Rules:
// - Must be between 1 and 63 characters.
// - Must consist of lowercase letters, numbers, and hyphens/underscores.
// - Must start and end with a letter or number.
var nameValidationRegex = regexp.MustCompile(`^[a-z0-9]([-_a-z0-9]{0,61}[a-z0-9])?$`)

// ValidateName checks if a given name conforms to the standard naming rules.
func ValidateName(name string) error {
	if !nameValidationRegex.MatchString(name) {
		return gaiaerrors.NewValidationError(
			"name",
			name,
			"must be 1-63 characters, start/end with a letter/number, and contain only lowercase letters, numbers, '-', or '_'",
		)
	}
	return nil
}

// ValidateClient validates a client name.
func ValidateClient(name string) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid client name: %w", err)
	}
	return nil
}

// ValidateNamespace validates a namespace name and ensures it's not reserved.
func ValidateNamespace(name string) error {
	if name == "common" {
		return fmt.Errorf("%w: 'common' is a reserved namespace", gaiaerrors.ErrReservedName)
	}
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid namespace name: %w", err)
	}
	return nil
}

// ValidateKey validates a secret key name.
func ValidateKey(name string) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid key name: %w", err)
	}
	return nil
}
