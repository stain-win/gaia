package encrypt

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	t.Run("StrongPassword", func(t *testing.T) {
		_, err := ValidatePassword("CorrectHorseBatteryStaple123!")
		if err != nil {
			t.Errorf("Expected strong password to pass validation, but got error: %v", err)
		}
	})

	t.Run("WeakPassword", func(t *testing.T) {
		_, err := ValidatePassword("password")
		if err == nil {
			t.Error("Expected weak password to fail validation, but it passed")
		}
	})
}

func TestValidatePasswordUnsafe(t *testing.T) {
	// ValidatePasswordUnsafe should always return (true, nil) regardless of input
	tests := []string{"", "a", "weak", "CorrectHorseBatteryStaple123!"}
	for _, pw := range tests {
		ok, err := ValidatePasswordUnsafe(pw)
		if !ok || err != nil {
			t.Errorf("ValidatePasswordUnsafe(%q) = (%v, %v), want (true, nil)", pw, ok, err)
		}
	}
}
