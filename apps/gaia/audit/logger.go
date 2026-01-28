package audit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

const (
	// defaultBufferSize is the default size of the async log buffer.
	defaultBufferSize = 1000

	// defaultWorkers is the number of worker goroutines per backend.
	defaultWorkers = 2
)

// Logger is the main audit logging orchestrator.
// It manages multiple backends and provides asynchronous, non-blocking logging.
type Logger struct {
	enabled     bool
	backends    []Backend
	redactor    *Redactor
	logRequest  bool
	logResponse bool

	// Async processing
	entryChan chan *Entry
	wg        sync.WaitGroup
	closeOnce sync.Once
	closed    bool
	mu        sync.RWMutex

	// Metrics
	entriesLogged  int64
	entriesDropped int64
	backendErrors  int64
	metricsMu      sync.Mutex
}

// LoggerConfig holds configuration for the audit logger.
type LoggerConfig struct {
	Enabled     bool
	HMACKey     string
	LogRequest  bool
	LogResponse bool
	BufferSize  int
	WorkerCount int
}

// DefaultLoggerConfig returns sensible default configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Enabled:     false,
		LogRequest:  true,
		LogResponse: true,
		BufferSize:  defaultBufferSize,
		WorkerCount: defaultWorkers,
	}
}

// NewLogger creates a new audit logger with the given backends.
func NewLogger(cfg LoggerConfig, backends ...Backend) *Logger {
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = defaultWorkers
	}

	l := &Logger{
		enabled:     cfg.Enabled,
		backends:    backends,
		redactor:    NewRedactor(cfg.HMACKey),
		logRequest:  cfg.LogRequest,
		logResponse: cfg.LogResponse,
		entryChan:   make(chan *Entry, bufferSize),
	}

	if l.enabled && len(backends) > 0 {
		// Start worker goroutines
		for i := 0; i < workerCount; i++ {
			l.wg.Add(1)
			go l.worker()
		}
		slog.Info("audit logger initialized",
			slog.Int("backends", len(backends)),
			slog.Int("workers", workerCount),
			slog.Int("buffer_size", bufferSize))
	}

	return l
}

// worker processes entries from the channel and writes to all backends.
func (l *Logger) worker() {
	defer l.wg.Done()

	for entry := range l.entryChan {
		// Redact sensitive data before logging
		redactedEntry := l.redactor.RedactEntry(entry)

		// Write to all backends
		for _, backend := range l.backends {
			if err := backend.Log(context.Background(), redactedEntry); err != nil {
				l.metricsMu.Lock()
				l.backendErrors++
				l.metricsMu.Unlock()

				slog.Error("audit backend error",
					slog.String("backend", backend.Type()),
					slog.String("error", err.Error()))
			}
		}

		l.metricsMu.Lock()
		l.entriesLogged++
		l.metricsMu.Unlock()
	}
}

// Log sends an entry to all configured backends asynchronously.
// This method is non-blocking; entries are buffered and processed by workers.
// Returns immediately even if backends are slow or unavailable.
func (l *Logger) Log(entry *Entry) {
	if !l.enabled || entry == nil {
		return
	}

	// Check entry type filter
	if entry.Type == EntryTypeRequest && !l.logRequest {
		return
	}
	if entry.Type == EntryTypeResponse && !l.logResponse {
		return
	}

	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return
	}
	l.mu.RUnlock()

	// Non-blocking send with drop if buffer is full
	select {
	case l.entryChan <- entry:
		// Entry queued successfully
	default:
		// Buffer full, drop the entry
		l.metricsMu.Lock()
		l.entriesDropped++
		l.metricsMu.Unlock()

		slog.Warn("audit buffer full, entry dropped",
			slog.String("operation", entry.Request.Operation))
	}
}

// LogRequest is a convenience method for logging request entries.
func (l *Logger) LogRequest(entry *Entry) {
	if entry != nil {
		entry.Type = EntryTypeRequest
		l.Log(entry)
	}
}

// LogResponse is a convenience method for logging response entries.
func (l *Logger) LogResponse(entry *Entry) {
	if entry != nil {
		entry.Type = EntryTypeResponse
		l.Log(entry)
	}
}

// Close gracefully shuts down the logger, flushing all pending entries.
// It blocks until all entries are processed or context is canceled.
func (l *Logger) Close() error {
	// If logger is disabled or has no channel, nothing to close
	if !l.enabled || l.entryChan == nil {
		return nil
	}

	var closeErr error

	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.mu.Unlock()

		// Close the channel to signal workers to stop
		close(l.entryChan)

		// Wait for workers to finish processing
		l.wg.Wait()

		// Close all backends
		var errs []error
		for _, backend := range l.backends {
			if err := backend.Close(); err != nil {
				errs = append(errs, fmt.Errorf("backend %s: %w", backend.Type(), err))
			}
		}

		if len(errs) > 0 {
			closeErr = fmt.Errorf("errors closing backends: %v", errs)
		}

		l.metricsMu.Lock()
		slog.Info("audit logger closed",
			slog.Int64("entries_logged", l.entriesLogged),
			slog.Int64("entries_dropped", l.entriesDropped),
			slog.Int64("backend_errors", l.backendErrors))
		l.metricsMu.Unlock()
	})

	return closeErr
}

// Stats returns current audit logger statistics.
func (l *Logger) Stats() (logged, dropped, errors int64) {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()
	return l.entriesLogged, l.entriesDropped, l.backendErrors
}

// IsEnabled returns whether audit logging is enabled.
func (l *Logger) IsEnabled() bool {
	return l.enabled
}

// NoopLogger returns a disabled logger for testing or when audit is disabled.
func NoopLogger() *Logger {
	return &Logger{
		enabled: false,
	}
}
