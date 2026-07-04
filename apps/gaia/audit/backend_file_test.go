package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileBackendStdout(t *testing.T) {
	cfg := BackendConfig{
		Type: "file",
		Path: "-", // stdout
	}

	backend, err := NewFileBackend(cfg)
	if err != nil {
		t.Fatalf("failed to create file backend: %v", err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("failed to close backend: %v", err)
		}
	})

	if backend.Type() != "file" {
		t.Errorf("expected type 'file', got %s", backend.Type())
	}
}

func TestFileBackendFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := BackendConfig{
		Type: "file",
		Path: logPath,
		Options: BackendOptions{
			MaxSizeMB:  1,
			MaxBackups: 1,
			MaxAgeDays: 1,
		},
	}

	backend, err := NewFileBackend(cfg)
	if err != nil {
		t.Fatalf("failed to create file backend: %v", err)
	}

	// Log an entry
	entry := &Entry{
		Type: EntryTypeRequest,
		Time: time.Now().UTC(),
		Auth: Auth{ClientIdentity: "test-client"},
		Request: Request{
			ID:        "req-123",
			Operation: "/test/Operation",
		},
	}

	err = backend.Log(context.Background(), entry)
	if err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// Close to flush
	err = backend.Close()
	if err != nil {
		t.Fatalf("failed to close backend: %v", err)
	}

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var loggedEntry Entry
	err = json.Unmarshal(data, &loggedEntry)
	if err != nil {
		t.Fatalf("failed to parse logged entry: %v", err)
	}

	if loggedEntry.Request.ID != "req-123" {
		t.Errorf("expected request ID 'req-123', got %s", loggedEntry.Request.ID)
	}
}

func TestFileBackendMultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	cfg := BackendConfig{
		Type: "file",
		Path: logPath,
	}

	backend, err := NewFileBackend(cfg)
	if err != nil {
		t.Fatalf("failed to create file backend: %v", err)
	}

	// Log multiple entries
	for i := 0; i < 3; i++ {
		entry := &Entry{
			Type: EntryTypeRequest,
			Time: time.Now().UTC(),
			Request: Request{
				ID: "req-" + string(rune('A'+i)),
			},
		}
		err = backend.Log(context.Background(), entry)
		if err != nil {
			t.Fatalf("failed to log entry %d: %v", i, err)
		}
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("failed to close backend: %v", err)
	}

	// Verify file contains 3 lines (JSON lines format)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Count newlines (each entry ends with \n)
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}

	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}
}
