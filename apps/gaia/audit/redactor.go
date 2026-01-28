package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Redactor handles HMAC-based redaction of sensitive values in audit entries.
// This ensures that sensitive data is never stored in plain text in audit logs.
type Redactor struct {
	key []byte
}

// NewRedactor creates a new Redactor with the given HMAC key.
// If the key is empty, a default key will be used (not recommended for production).
func NewRedactor(key string) *Redactor {
	k := []byte(key)
	if len(k) == 0 {
		// Use a default key if none provided - this is not secure but allows
		// the system to function. A warning should be logged when this happens.
		k = []byte("gaia-default-hmac-key-change-me")
	}
	return &Redactor{key: k}
}

// Hash computes an HMAC-SHA256 hash of the input and returns it as a hex string.
// The result is prefixed with "hmac-sha256:" to indicate the hashing algorithm.
func (r *Redactor) Hash(value string) string {
	if value == "" {
		return ""
	}
	h := hmac.New(sha256.New, r.key)
	h.Write([]byte(value))
	return "hmac-sha256:" + hex.EncodeToString(h.Sum(nil))
}

// RedactEntry creates a copy of the entry with sensitive data redacted.
// Sensitive fields are replaced with HMAC hashes to allow correlation
// without exposing the actual values.
func (r *Redactor) RedactEntry(entry *Entry) *Entry {
	if entry == nil {
		return nil
	}

	// Create a shallow copy
	redacted := *entry

	// Redact request data if present
	if entry.Request.Data != nil {
		redacted.Request.Data = r.redactMap(entry.Request.Data)
	}

	// Redact response data if present
	if entry.Response != nil && entry.Response.Data != nil {
		responseCopy := *entry.Response
		responseCopy.Data = r.redactMap(entry.Response.Data)
		redacted.Response = &responseCopy
	}

	// Redact error messages that might contain sensitive data
	if entry.Error != nil {
		errorCopy := *entry.Error
		errorCopy.Message = r.redactErrorMessage(entry.Error.Message)
		redacted.Error = &errorCopy
	}

	return &redacted
}

// redactMap redacts sensitive fields in a map.
func (r *Redactor) redactMap(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}

	redacted := make(map[string]any, len(data))
	for k, v := range data {
		redacted[k] = r.redactValue(k, v)
	}
	return redacted
}

// sensitiveKeys are field names that should always be redacted.
var sensitiveKeys = map[string]bool{
	"value":           true,
	"secret":          true,
	"secret_value":    true,
	"password":        true,
	"passphrase":      true,
	"private_key":     true,
	"privatekey":      true,
	"api_key":         true,
	"apikey":          true,
	"token":           true,
	"access_token":    true,
	"refresh_token":   true,
	"authorization":   true,
	"credential":      true,
	"credentials":     true,
	"certificate_key": true,
}

// redactValue redacts a value if the key indicates it's sensitive.
func (r *Redactor) redactValue(key string, value any) any {
	lowerKey := strings.ToLower(key)

	// Check if this is a sensitive key
	if sensitiveKeys[lowerKey] {
		if s, ok := value.(string); ok && s != "" {
			return r.Hash(s)
		}
		// For non-string sensitive values, just mark as redacted
		return "<redacted>"
	}

	// Handle nested maps
	if m, ok := value.(map[string]any); ok {
		return r.redactMap(m)
	}

	// Handle slices
	if s, ok := value.([]any); ok {
		redacted := make([]any, len(s))
		for i, item := range s {
			if m, ok := item.(map[string]any); ok {
				redacted[i] = r.redactMap(m)
			} else {
				redacted[i] = item
			}
		}
		return redacted
	}

	return value
}

// redactErrorMessage redacts potential secrets from error messages.
func (r *Redactor) redactErrorMessage(msg string) string {
	// Common patterns that might contain sensitive data
	// We keep the error structure but redact potential values

	// This is a simple implementation - in production you might want
	// more sophisticated regex-based redaction
	if strings.Contains(strings.ToLower(msg), "secret") && strings.Contains(msg, ":") {
		// Split on common delimiters and redact potential values
		parts := strings.SplitN(msg, ":", 2)
		if len(parts) == 2 && len(strings.TrimSpace(parts[1])) > 20 {
			// If the value part is long, it might be a secret
			return parts[0] + ": <redacted>"
		}
	}
	return msg
}
