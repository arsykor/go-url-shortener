// Package audit implements the Observer pattern for audit logging.
//
// An audit event is emitted whenever a URL is shortened or followed.
// Events can be delivered to multiple sinks simultaneously: a local file
// (one JSON line per event) and/or a remote HTTP endpoint.
// Delivery is always asynchronous so it never blocks the HTTP handler.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AuditEvent represents a single auditable action performed by a user.
type AuditEvent struct {
	// TS is the Unix timestamp (seconds) when the event occurred.
	TS int64 `json:"ts"`
	// Action describes the kind of operation: "shorten" or "follow".
	Action string `json:"action"`
	// UserID is the authenticated user who performed the action.
	// Omitted from JSON when empty.
	UserID string `json:"user_id,omitempty"`
	// URL is the original (long) URL that was shortened or followed.
	URL string `json:"url"`
}

// Observer is the subscriber interface in the Observer pattern.
// Implementations must be safe for concurrent use from goroutines.
type Observer interface {
	// Notify delivers an audit event to the sink.
	// Errors are logged internally but do not propagate to callers.
	Notify(event AuditEvent) error
}

// FileObserver writes audit events as JSON lines to a local file.
type FileObserver struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileObserver creates an Observer that appends audit events to filePath.
// The file is created if it does not exist and is opened in append-only mode.
func NewFileObserver(filePath string) (*FileObserver, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	return &FileObserver{file: f}, nil
}

// Notify serialises event as a JSON line and appends it to the audit file.
func (o *FileObserver) Notify(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	data = append(data, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, err := o.file.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}
	return nil
}

// Close closes the underlying file descriptor.
func (o *FileObserver) Close() error {
	return o.file.Close()
}

// HTTPObserver posts audit events as JSON to a remote HTTP endpoint.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver creates an Observer that POSTs audit events to url.
// Requests time out after 5 seconds.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Notify serialises event as JSON and POSTs it to the configured URL.
func (o *HTTPObserver) Notify(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	resp, err := o.client.Post(o.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send audit event to %s: %w", o.url, err)
	}
	defer resp.Body.Close()
	return nil
}

// Service is the publisher in the Observer pattern. It dispatches audit events
// to all registered observers asynchronously.
type Service struct {
	observers []Observer
	logger    *zap.SugaredLogger
}

// NewService creates an audit Service with the given observers.
// Pass no observers to create a no-op service.
func NewService(logger *zap.SugaredLogger, observers ...Observer) *Service {
	return &Service{observers: observers, logger: logger}
}

// Notify builds an AuditEvent from the provided fields and dispatches it to
// all registered observers in a separate goroutine. It is safe to call on a
// nil *Service — the call is silently ignored.
//
// action must be "shorten" or "follow".
func (s *Service) Notify(action, userID, originalURL string) {
	if s == nil || len(s.observers) == 0 {
		return
	}

	event := AuditEvent{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	}

	go func() {
		for _, obs := range s.observers {
			if err := obs.Notify(event); err != nil {
				s.logger.Warnw("audit observer failed", "err", err)
			}
		}
	}()
}
