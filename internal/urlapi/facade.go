// Package urlapi hosts shared URL operations used by HTTP and gRPC handlers.
package urlapi

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/service"
)

// Audit dispatches follow/shorten audit events.
type Audit interface {
	Notify(action, userID, originalURL string)
}

// Facade centralizes shorten/expand/list logic for multiple transports.
type Facade struct {
	Svc   *service.ShortenerService
	Audit Audit
}

var ErrEmptyURL = errors.New("url is empty")

// Shorten creates a short link for the given URL and records audit on success.
func (f *Facade) Shorten(ctx context.Context, userID, rawURL string) (shortURL string, conflict bool, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false, ErrEmptyURL
	}
	shortURL, conflict, err = f.Svc.ShortenURL(ctx, rawURL, userID)
	if err != nil {
		return "", false, err
	}
	if f.Audit != nil {
		f.Audit.Notify("shorten", userID, rawURL)
	}
	return shortURL, conflict, nil
}

// Expand resolves a short id to the original URL.
func (f *Facade) Expand(ctx context.Context, shortID string) (string, error) {
	shortID = strings.TrimSpace(shortID)
	if shortID == "" {
		return "", service.ErrURLNotFound
	}
	return f.Svc.GetOriginalURL(ctx, shortID)
}

// ListedURL is a full short URL plus its target.
type ListedURL struct {
	ShortURL    string
	OriginalURL string
}

// ListUserURLsDisplay returns all URLs for a user with absolute short links.
func (f *Facade) ListUserURLsDisplay(ctx context.Context, userID string) ([]ListedURL, error) {
	pairs, err := f.Svc.GetURLsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, nil
	}
	base := f.Svc.BaseURL()
	out := make([]ListedURL, 0, len(pairs))
	for _, u := range pairs {
		su, err := url.JoinPath(base, u.ShortURL)
		if err != nil {
			return nil, err
		}
		out = append(out, ListedURL{ShortURL: su, OriginalURL: u.OriginalURL})
	}
	return out, nil
}
