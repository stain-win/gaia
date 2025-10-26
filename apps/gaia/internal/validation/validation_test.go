package validation

import (
	"errors"
	"testing"

	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		// Valid names
		{"valid lowercase", "myapp", false},
		{"valid with numbers", "app123", false},
		{"valid with hyphens", "my-app", false},
		{"valid with underscores", "my_app", false},
		{"valid mixed", "app-name_123", false},
		{"single character", "a", false},
		{"max length 63", "a12345678901234567890123456789012345678901234567890123456789012", false},

		// Invalid names
		{"empty string", "", true},
		{"starts with hyphen", "-app", true},
		{"ends with hyphen", "app-", true},
		{"starts with underscore", "_app", true},
		{"ends with underscore", "app_", true},
		{"uppercase letters", "MyApp", true},
		{"contains spaces", "my app", true},
		{"contains special chars", "my@app", true},
		{"too long", "a1234567890123456789012345678901234567890123456789012345678901234", true},
		{"only hyphen", "-", true},
		{"only underscore", "_", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateName(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}

			if err != nil {
				var valErr *gaiaerrors.ValidationError
				if !errors.As(err, &valErr) {
					t.Errorf("Expected ValidationError, got %T", err)
				}
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid client name", "web-app", false},
		{"valid with numbers", "client123", false},
		{"invalid uppercase", "WebApp", true},
		{"invalid special chars", "web@app", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClient(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateClient(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		checkErr  error
	}{
		{"valid namespace", "production", false, nil},
		{"valid with hyphens", "prod-env", false, nil},
		{"reserved common", "common", true, gaiaerrors.ErrReservedName},
		{"invalid uppercase", "Production", true, nil},
		{"empty", "", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNamespace(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateNamespace(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}

			if tt.checkErr != nil && !errors.Is(err, tt.checkErr) {
				t.Errorf("ValidateNamespace(%q) expected error %v, got %v", tt.input, tt.checkErr, err)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid key", "database-url", false},
		{"valid with underscores", "api_key", false},
		{"valid alphanumeric", "key123", false},
		{"invalid uppercase", "API_KEY", true},
		{"invalid special chars", "key@name", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateKey(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestValidationErrorType(t *testing.T) {
	err := ValidateName("Invalid@Name")
	if err == nil {
		t.Fatal("Expected error for invalid name")
	}

	var valErr *gaiaerrors.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("Expected ValidationError, got %T", err)
	}

	if valErr.Field != "name" {
		t.Errorf("Expected field 'name', got %q", valErr.Field)
	}

	if valErr.Value != "Invalid@Name" {
		t.Errorf("Expected value 'Invalid@Name', got %q", valErr.Value)
	}
}
