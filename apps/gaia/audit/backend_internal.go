package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	// auditBucket is the name of the BoltDB bucket for audit logs.
	auditBucket = "audit_log"

	// defaultRetentionDays is the default number of days to retain audit entries.
	defaultRetentionDays = 30
)

// InternalBackend stores audit entries in BoltDB.
type InternalBackend struct {
	db            *bbolt.DB
	retentionDays int
	mu            sync.Mutex
	closed        bool
}

// NewInternalBackend creates a new BoltDB-based audit backend.
// The database should be the same instance used by the daemon.
func NewInternalBackend(db *bbolt.DB, cfg BackendConfig) (*InternalBackend, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required for internal audit backend")
	}

	retentionDays := cfg.Options.RetentionDays
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}

	// Ensure the bucket exists
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(auditBucket))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create audit bucket: %w", err)
	}

	b := &InternalBackend{
		db:            db,
		retentionDays: retentionDays,
	}

	// Start background cleanup goroutine
	go b.cleanupLoop()

	return b, nil
}

// Type returns the backend type identifier.
func (b *InternalBackend) Type() string {
	return "internal"
}

// Log writes an audit entry to BoltDB.
func (b *InternalBackend) Log(_ context.Context, entry *Entry) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("backend is closed")
	}
	b.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Use timestamp + request ID as key for natural ordering
	key := fmt.Sprintf("%s_%s", entry.Time.Format(time.RFC3339Nano), entry.Request.ID)

	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		if bucket == nil {
			return fmt.Errorf("audit bucket not found")
		}
		return bucket.Put([]byte(key), data)
	})
}

// Close marks the backend as closed.
func (b *InternalBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// cleanupLoop periodically removes old audit entries.
func (b *InternalBackend) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup
	b.cleanup()

	for range ticker.C {
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()

		b.cleanup()
	}
}

// cleanup removes audit entries older than the retention period.
func (b *InternalBackend) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -b.retentionDays)
	cutoffKey := cutoff.Format(time.RFC3339Nano)

	var keysToDelete [][]byte

	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			// Keys are formatted as timestamp_requestID
			// If the key is less than the cutoff, mark for deletion
			if string(k) < cutoffKey {
				keysToDelete = append(keysToDelete, append([]byte(nil), k...))
			} else {
				// Keys are ordered, so we can stop once we hit a key >= cutoff
				break
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	if len(keysToDelete) == 0 {
		return
	}

	err = b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		if bucket == nil {
			return nil
		}

		for _, key := range keysToDelete {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && len(keysToDelete) > 0 {
		// Log cleanup only if entries were removed
		// Using fmt.Printf to avoid circular dependency with log package
		fmt.Printf("audit cleanup: removed %d entries older than %d days\n",
			len(keysToDelete), b.retentionDays)
	}
}

// ListEntries retrieves audit entries within the given time range.
// This is useful for the TUI/CLI to display audit logs.
func (b *InternalBackend) ListEntries(start, end time.Time, limit int) ([]*Entry, error) {
	if limit <= 0 {
		limit = 100
	}

	var entries []*Entry

	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(auditBucket))
		if bucket == nil {
			return nil
		}

		startKey := start.Format(time.RFC3339Nano)
		endKey := end.Format(time.RFC3339Nano)

		cursor := bucket.Cursor()
		for k, v := cursor.Seek([]byte(startKey)); k != nil; k, v = cursor.Next() {
			if string(k) > endKey {
				break
			}

			var entry Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue // Skip malformed entries
			}

			entries = append(entries, &entry)

			if len(entries) >= limit {
				break
			}
		}
		return nil
	})

	return entries, err
}
