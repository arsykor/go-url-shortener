package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type AuditEvent struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}

// Subscriber interface
type Observer interface {
	Notify(event AuditEvent) error
}

type FileObserver struct {
	filePath string
}

func NewFileObserver(filePath string) *FileObserver {
	return &FileObserver{filePath: filePath}
}

func (o *FileObserver) Notify(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}
	return nil
}

type HTTPObserver struct {
	url    string
	client *http.Client
}

func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

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

type Service struct {
	observers []Observer
}

func NewService(observers ...Observer) *Service {
	return &Service{observers: observers}
}

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
			obs.Notify(event)
		}
	}()
}
