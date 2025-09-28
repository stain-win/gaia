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
