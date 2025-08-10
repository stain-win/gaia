package encrypt

import (
	passwordvalidator "github.com/wagslane/go-password-validator"
)

func ValidatePassword(password string) (bool, error) {

	minEntropy := 60. // Minimum entropy in bits
	validationError := passwordvalidator.Validate(password, minEntropy)
	if validationError != nil {
		return false, validationError
	} else {
		return true, nil
	}
}
