package repository

import (
	"context"
	"sync"
)

type InMemoryURLRepository struct {
	mu   sync.RWMutex
	urls map[string]string
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls: make(map[string]string),
	}
}

// Save stores a URL mapping
func (r *InMemoryURLRepository) Save(ctx context.Context, shortID, originalURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls[shortID] = originalURL
}

// Get retrieves the original URL by short ID
func (r *InMemoryURLRepository) Get(ctx context.Context, shortID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, exists := r.urls[shortID]
	return url, exists
}
