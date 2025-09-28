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
