package audit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockBackend is a test backend that records logged entries.
type mockBackend struct {
	mu      sync.Mutex
	entries []*Entry
	closed  bool
	logErr  error
}

func (m *mockBackend) Type() string { return "mock" }

func (m *mockBackend) Log(_ context.Context, entry *Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logErr != nil {
		return m.logErr
	}
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockBackend) getEntries() []*Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*Entry(nil), m.entries...)
}

func TestLoggerDisabled(t *testing.T) {
	backend := &mockBackend{}
	logger := NewLogger(LoggerConfig{Enabled: false}, backend)

	entry := &Entry{Type: EntryTypeRequest}
	logger.Log(entry)

	// Wait a bit for any async processing
	time.Sleep(50 * time.Millisecond)

	if len(backend.getEntries()) > 0 {
		t.Error("disabled logger should not log entries")
	}
}

func TestLoggerEnabled(t *testing.T) {
	backend := &mockBackend{}
	cfg := LoggerConfig{
		Enabled:     true,
		LogRequest:  true,
		LogResponse: true,
	}
	logger := NewLogger(cfg, backend)

	entry := &Entry{
		Type: EntryTypeRequest,
		Request: Request{
			ID:        "test-1",
			Operation: "/test",
		},
	}
	logger.Log(entry)

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	entries := backend.getEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestLoggerFiltersRequestEntries(t *testing.T) {
	backend := &mockBackend{}
	cfg := LoggerConfig{
		Enabled:     true,
		LogRequest:  false, // Disable request logging
		LogResponse: true,
	}
	logger := NewLogger(cfg, backend)

	reqEntry := &Entry{Type: EntryTypeRequest, Request: Request{ID: "1"}}
	respEntry := &Entry{Type: EntryTypeResponse, Request: Request{ID: "2"}}

	logger.Log(reqEntry)
	logger.Log(respEntry)

	time.Sleep(100 * time.Millisecond)
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	entries := backend.getEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (response only), got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Type != EntryTypeResponse {
		t.Errorf("expected response entry, got %s", entries[0].Type)
	}
}

func TestLoggerMultipleBackends(t *testing.T) {
	backend1 := &mockBackend{}
	backend2 := &mockBackend{}
	cfg := LoggerConfig{
		Enabled:     true,
		LogRequest:  true,
		LogResponse: true,
	}
	logger := NewLogger(cfg, backend1, backend2)

	entry := &Entry{
		Type:    EntryTypeRequest,
		Request: Request{ID: "test-1"},
	}
	logger.Log(entry)

	time.Sleep(100 * time.Millisecond)
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	if len(backend1.getEntries()) != 1 {
		t.Errorf("backend1 expected 1 entry, got %d", len(backend1.getEntries()))
	}
	if len(backend2.getEntries()) != 1 {
		t.Errorf("backend2 expected 1 entry, got %d", len(backend2.getEntries()))
	}
}

func TestLoggerRedactsSensitiveData(t *testing.T) {
	backend := &mockBackend{}
	cfg := LoggerConfig{
		Enabled:     true,
		HMACKey:     "test-key",
		LogRequest:  true,
		LogResponse: true,
	}
	logger := NewLogger(cfg, backend)

	entry := &Entry{
		Type: EntryTypeRequest,
		Request: Request{
			ID: "test-1",
			Data: map[string]any{
				"password": "super-secret",
				"username": "test-user",
			},
		},
	}
	logger.Log(entry)

	time.Sleep(100 * time.Millisecond)
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	entries := backend.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Password should be hashed
	loggedEntry := entries[0]
	password := loggedEntry.Request.Data["password"].(string)
	if password == "super-secret" {
		t.Error("password should have been redacted")
	}

	// Username should remain unchanged
	username := loggedEntry.Request.Data["username"]
	if username != "test-user" {
		t.Errorf("username should remain unchanged, got %v", username)
	}
}

func TestLoggerClose(t *testing.T) {
	backend := &mockBackend{}
	cfg := LoggerConfig{Enabled: true, LogRequest: true}
	logger := NewLogger(cfg, backend)

	// Log some entries
	for i := 0; i < 5; i++ {
		logger.Log(&Entry{Type: EntryTypeRequest, Request: Request{ID: "test"}})
	}

	// Close should wait for all entries to be processed
	err := logger.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	if !backend.closed {
		t.Error("backend should be closed")
	}

	entries := backend.getEntries()
	if len(entries) != 5 {
		t.Errorf("expected 5 entries after close, got %d", len(entries))
	}
}

func TestNoopLogger(t *testing.T) {
	logger := NoopLogger()

	if logger.IsEnabled() {
		t.Error("noop logger should not be enabled")
	}

	// Should not panic
	logger.Log(&Entry{})
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}
}

func TestLoggerStats(t *testing.T) {
	backend := &mockBackend{}
	cfg := LoggerConfig{Enabled: true, LogRequest: true}
	logger := NewLogger(cfg, backend)

	for i := 0; i < 3; i++ {
		logger.Log(&Entry{Type: EntryTypeRequest, Request: Request{ID: "test"}})
	}

	time.Sleep(100 * time.Millisecond)
	if err := logger.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	logged, dropped, errors := logger.Stats()
	if logged != 3 {
		t.Errorf("expected 3 logged, got %d", logged)
	}
	if dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", dropped)
	}
	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
}
