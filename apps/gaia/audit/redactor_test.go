package audit

import (
	"testing"
)

func TestRedactorHash(t *testing.T) {
	redactor := NewRedactor("test-key")

	hash1 := redactor.Hash("secret-value")
	hash2 := redactor.Hash("secret-value")
	hash3 := redactor.Hash("different-value")

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Errorf("expected same hash for same input, got %s and %s", hash1, hash2)
	}

	// Different input should produce different hash
	if hash1 == hash3 {
		t.Error("expected different hash for different input")
	}

	// Hash should have proper prefix
	if len(hash1) < 12 || hash1[:12] != "hmac-sha256:" {
		t.Errorf("expected hash to start with 'hmac-sha256:', got %s", hash1)
	}
}

func TestRedactorHashEmpty(t *testing.T) {
	redactor := NewRedactor("test-key")

	hash := redactor.Hash("")
	if hash != "" {
		t.Errorf("expected empty string for empty input, got %s", hash)
	}
}

func TestRedactorRedactEntry(t *testing.T) {
	redactor := NewRedactor("test-key")

	entry := &Entry{
		Type: EntryTypeRequest,
		Auth: Auth{ClientIdentity: "test-client"},
		Request: Request{
			ID:        "req-123",
			Operation: "/gaia.GaiaAdmin/AddSecret",
			Data: map[string]any{
				"namespace":   "production",
				"secret_id":   "db_password",
				"value":       "super-secret-password", // Should be redacted
				"client_name": "test-client",
			},
		},
	}

	redacted := redactor.RedactEntry(entry)

	// Original should not be modified
	if entry.Request.Data["value"] != "super-secret-password" {
		t.Error("original entry was modified")
	}

	// Redacted value should be hashed
	redactedValue, ok := redacted.Request.Data["value"].(string)
	if !ok {
		t.Fatal("expected string value in redacted data")
	}
	if len(redactedValue) < 12 || redactedValue[:12] != "hmac-sha256:" {
		t.Errorf("expected hashed value, got %s", redactedValue)
	}

	// Non-sensitive fields should remain unchanged
	if redacted.Request.Data["namespace"] != "production" {
		t.Errorf("expected namespace to remain unchanged, got %v", redacted.Request.Data["namespace"])
	}
}

func TestRedactorRedactSensitiveKeys(t *testing.T) {
	redactor := NewRedactor("test-key")

	sensitiveFields := []string{
		"value", "secret", "password", "passphrase", "private_key",
		"api_key", "token", "access_token", "authorization",
	}

	for _, field := range sensitiveFields {
		entry := &Entry{
			Type: EntryTypeRequest,
			Request: Request{
				Data: map[string]any{
					field: "sensitive-data",
				},
			},
		}

		redacted := redactor.RedactEntry(entry)
		val := redacted.Request.Data[field].(string)

		if val == "sensitive-data" {
			t.Errorf("field %s should have been redacted", field)
		}
		if len(val) < 12 || val[:12] != "hmac-sha256:" {
			t.Errorf("field %s should have HMAC prefix, got %s", field, val)
		}
	}
}

func TestRedactorRedactNilEntry(t *testing.T) {
	redactor := NewRedactor("test-key")

	redacted := redactor.RedactEntry(nil)
	if redacted != nil {
		t.Error("expected nil for nil input")
	}
}

func TestRedactorWithDefaultKey(t *testing.T) {
	// Empty key should use default
	redactor := NewRedactor("")

	hash := redactor.Hash("test-value")
	if hash == "" {
		t.Error("expected non-empty hash with default key")
	}
}
