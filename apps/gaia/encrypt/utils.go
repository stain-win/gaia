package encrypt

import (
	passwordvalidator "github.com/wagslane/go-password-validator"
)

func ValidatePassword(password string) (bool, error) {
	minEntropy := 60.0
	err := passwordvalidator.Validate(password, minEntropy)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ValidatePasswordUnsafe skips entropy checking. Only call from unsafe dev mode paths.
func ValidatePasswordUnsafe(_ string) (bool, error) { return true, nil }
