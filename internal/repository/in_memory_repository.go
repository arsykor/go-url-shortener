// Package repository provides implementations of service.URLRepository (memory, file, Postgres).
package repository

import (
	"context"
	"sync"

	"github.com/arsykor/go-url-shortener/internal/service"
)

type InMemoryURLRepository struct {
	mu          sync.RWMutex
	urls        map[string]string   // shortID -> originalURL
	reverseUrls map[string]string   // originalURL -> shortID
	userURLs    map[string][]string // userID -> []shortID
	deleted     map[string]bool     // shortID -> isDeleted
}

// NewInMemoryURLRepository creates a new in-memory repository.
func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls:        make(map[string]string),
		reverseUrls: make(map[string]string),
		userURLs:    make(map[string][]string),
		deleted:     make(map[string]bool),
	}
}

// Save stores a URL mapping.
// Returns existing shortID and true if originalURL already exists.
func (r *InMemoryURLRepository) Save(ctx context.Context, shortID, originalURL, userID string) (existingShortID string, conflict bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if originalURL already exists
	if existingShortID, exists := r.reverseUrls[originalURL]; exists {
		return existingShortID, true
	}

	r.urls[shortID] = originalURL
	r.reverseUrls[originalURL] = shortID
	if userID != "" {
		r.userURLs[userID] = append(r.userURLs[userID], shortID)
	}
	return "", false
}

// Get retrieves the original URL by short ID.
// Returns originalURL, isDeleted flag, and whether the record exists.
func (r *InMemoryURLRepository) Get(ctx context.Context, shortID string) (string, bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, exists := r.urls[shortID]
	if !exists {
		return "", false, false
	}
	return url, r.deleted[shortID], true
}

// SaveBatch stores multiple URL mappings in a single operation.
func (r *InMemoryURLRepository) SaveBatch(ctx context.Context, items []service.BatchItem, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		r.urls[item.ShortID] = item.OriginalURL
		if userID != "" {
			r.userURLs[userID] = append(r.userURLs[userID], item.ShortID)
		}
	}
	return nil
}

// GetURLsByUser returns all URLs shortened by a specific user.
func (r *InMemoryURLRepository) GetURLsByUser(ctx context.Context, userID string) ([]service.UserURL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	shortIDs := r.userURLs[userID]
	if len(shortIDs) == 0 {
		return nil, nil
	}

	result := make([]service.UserURL, 0, len(shortIDs))
	for _, shortID := range shortIDs {
		if originalURL, exists := r.urls[shortID]; exists {
			result = append(result, service.UserURL{
				ShortURL:    shortID,
				OriginalURL: originalURL,
			})
		}
	}
	return result, nil
}

// DeleteURLs marks URLs as deleted.
// Only URLs belonging to the given userID are affected.
func (r *InMemoryURLRepository) DeleteURLs(ctx context.Context, shortIDs []string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Build a set of shortIDs owned by this user
	owned := make(map[string]bool)
	for _, sid := range r.userURLs[userID] {
		owned[sid] = true
	}

	for _, sid := range shortIDs {
		if owned[sid] {
			r.deleted[sid] = true
		}
	}
	return nil
}

// Stats returns totals of shortened URL records and users with saved URLs (non-empty user_id).
func (r *InMemoryURLRepository) Stats(_ context.Context) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	urls := len(r.urls)
	users := 0
	for uid := range r.userURLs {
		if uid != "" {
			users++
		}
	}
	return urls, users, nil
}
