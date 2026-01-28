package audit

import (
	"testing"
	"time"
)

func TestNewRequestEntry(t *testing.T) {
	auth := Auth{
		ClientIdentity: "test-client",
		ClientType:     "admin",
	}

	entry := NewRequestEntry("req-123", "/gaia.GaiaAdmin/AddSecret", "127.0.0.1:50000", auth)

	if entry.Type != EntryTypeRequest {
		t.Errorf("expected type %s, got %s", EntryTypeRequest, entry.Type)
	}
	if entry.Request.ID != "req-123" {
		t.Errorf("expected request ID req-123, got %s", entry.Request.ID)
	}
	if entry.Request.Operation != "/gaia.GaiaAdmin/AddSecret" {
		t.Errorf("expected operation /gaia.GaiaAdmin/AddSecret, got %s", entry.Request.Operation)
	}
	if entry.Request.RemoteAddr != "127.0.0.1:50000" {
		t.Errorf("expected remote addr 127.0.0.1:50000, got %s", entry.Request.RemoteAddr)
	}
	if entry.Auth.ClientIdentity != "test-client" {
		t.Errorf("expected client identity test-client, got %s", entry.Auth.ClientIdentity)
	}
	if entry.Time.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestNewResponseEntry(t *testing.T) {
	auth := Auth{
		ClientIdentity: "test-client",
		ClientType:     "client",
	}

	entry := NewResponseEntry("req-456", "/gaia.GaiaClient/GetSecret", "192.168.1.1:45000", auth, true, 0)

	if entry.Type != EntryTypeResponse {
		t.Errorf("expected type %s, got %s", EntryTypeResponse, entry.Type)
	}
	if entry.Response == nil {
		t.Fatal("expected non-nil response")
	}
	if !entry.Response.Success {
		t.Error("expected success true")
	}
	if entry.Response.StatusCode != 0 {
		t.Errorf("expected status code 0, got %d", entry.Response.StatusCode)
	}
}

func TestEntryWithMethods(t *testing.T) {
	auth := Auth{ClientIdentity: "test"}
	entry := NewRequestEntry("req-1", "/test", "127.0.0.1:1234", auth)

	entry.WithPath("client-a/production/db_password").
		WithNamespace("production").
		WithClientName("client-a").
		WithData(map[string]any{"key": "value"})

	if entry.Request.Path != "client-a/production/db_password" {
		t.Errorf("expected path client-a/production/db_password, got %s", entry.Request.Path)
	}
	if entry.Request.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", entry.Request.Namespace)
	}
	if entry.Request.ClientName != "client-a" {
		t.Errorf("expected client name client-a, got %s", entry.Request.ClientName)
	}
	if entry.Request.Data["key"] != "value" {
		t.Errorf("expected data key=value, got %v", entry.Request.Data)
	}
}

func TestEntryWithError(t *testing.T) {
	auth := Auth{ClientIdentity: "test"}
	entry := NewResponseEntry("req-1", "/test", "127.0.0.1:1234", auth, false, 2)
	entry.WithError("secret not found", "NOT_FOUND")

	if entry.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if entry.Error.Message != "secret not found" {
		t.Errorf("expected error message 'secret not found', got %s", entry.Error.Message)
	}
	if entry.Error.Code != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %s", entry.Error.Code)
	}
}

func TestEntryTimeIsUTC(t *testing.T) {
	auth := Auth{ClientIdentity: "test"}
	entry := NewRequestEntry("req-1", "/test", "127.0.0.1:1234", auth)

	if entry.Time.Location() != time.UTC {
		t.Errorf("expected UTC time, got %s", entry.Time.Location())
	}
}
