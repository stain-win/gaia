package audit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileBackend writes audit entries to a file or stdout in JSON lines format.
type FileBackend struct {
	writer io.WriteCloser
	mu     sync.Mutex
	path   string
}

// NewFileBackend creates a new file-based audit backend.
// If path is "-" or empty, it writes to stdout.
func NewFileBackend(cfg BackendConfig) (*FileBackend, error) {
	opts := cfg.Options
	if opts.MaxSizeMB == 0 {
		opts.MaxSizeMB = 100
	}
	if opts.MaxBackups == 0 {
		opts.MaxBackups = 5
	}
	if opts.MaxAgeDays == 0 {
		opts.MaxAgeDays = 30
	}

	var writer io.WriteCloser

	if cfg.Path == "-" || cfg.Path == "" {
		// Write to stdout
		writer = &nopCloser{os.Stdout}
	} else {
		// Write to rotating file
		writer = &lumberjack.Logger{
			Filename:   cfg.Path,
			MaxSize:    opts.MaxSizeMB,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAgeDays,
			Compress:   true,
			LocalTime:  true,
		}
	}

	return &FileBackend{
		writer: writer,
		path:   cfg.Path,
	}, nil
}

// Type returns the backend type identifier.
func (b *FileBackend) Type() string {
	return "file"
}

// Log writes an audit entry as a JSON line.
func (b *FileBackend) Log(_ context.Context, entry *Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Append newline for JSON lines format
	data = append(data, '\n')

	b.mu.Lock()
	defer b.mu.Unlock()

	_, err = b.writer.Write(data)
	return err
}

// Close closes the underlying writer.
func (b *FileBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.writer.Close()
}

// nopCloser wraps an io.Writer to implement io.WriteCloser with a no-op Close.
type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error {
	return nil
}
