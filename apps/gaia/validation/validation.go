package validation

import (
	"fmt"
	"regexp"
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
		return fmt.Errorf("name '%s' is invalid: must be 1-63 characters, start/end with a letter/number, and contain only lowercase letters, numbers, '-', or '_'", name)
	}
	return nil
}
