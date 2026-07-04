package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// WebhookBackend sends audit entries to an HTTP endpoint.
type WebhookBackend struct {
	endpoint string
	client   *http.Client
	limiter  *rate.Limiter
	headers  map[string]string
	mu       sync.Mutex
	closed   bool
}

// NewWebhookBackend creates a new webhook-based audit backend.
func NewWebhookBackend(cfg BackendConfig) (*WebhookBackend, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("webhook endpoint URL is required")
	}

	opts := cfg.Options
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = 10
	}
	if opts.RateLimitPerSec <= 0 {
		opts.RateLimitPerSec = 100
	}

	// Parse custom headers if provided
	var headers map[string]string
	if opts.Headers != "" {
		if err := json.Unmarshal([]byte(opts.Headers), &headers); err != nil {
			return nil, fmt.Errorf("invalid headers JSON: %w", err)
		}
	}

	return &WebhookBackend{
		endpoint: cfg.Path,
		client: &http.Client{
			Timeout: time.Duration(opts.TimeoutSeconds) * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(opts.RateLimitPerSec), opts.RateLimitPerSec),
		headers: headers,
	}, nil
}

// Type returns the backend type identifier.
func (b *WebhookBackend) Type() string {
	return "webhook"
}

// Log sends an audit entry to the configured webhook endpoint.
func (b *WebhookBackend) Log(ctx context.Context, entry *Entry) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("backend is closed")
	}
	b.mu.Unlock()

	// Wait for rate limiter
	if err := b.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait failed: %w", err)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Gaia-Audit/1.0")

	// Add custom headers
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

// Close marks the backend as closed.
func (b *WebhookBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}
