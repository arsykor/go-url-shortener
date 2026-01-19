package repository

import (
	"context"
	"sync"

	"github.com/arsykor/go-url-shortener/internal/service"
)

type InMemoryURLRepository struct {
	mu   sync.RWMutex
	urls map[string]string // shortID -> originalURL
	reverseUrls map[string]string // originalURL -> shortID
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls: make(map[string]string),
		reverseUrls: make(map[string]string),
	}
}

// Save stores a URL mapping
// Returns existing shortID and true if originalURL already exists
func (r *InMemoryURLRepository) Save(ctx context.Context, shortID, originalURL string) (existingShortID string, conflict bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check if originalURL already exists
	if existingShortID, exists := r.reverseUrls[originalURL]; exists {
		return existingShortID, true
	}
	
	r.urls[shortID] = originalURL
	r.reverseUrls[originalURL] = shortID
	return "", false
}

// Get retrieves the original URL by short ID
func (r *InMemoryURLRepository) Get(ctx context.Context, shortID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, exists := r.urls[shortID]
	return url, exists
}

// SaveBatch stores multiple URL mappings in a single operation
func (r *InMemoryURLRepository) SaveBatch(ctx context.Context, items []service.BatchItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		r.urls[item.ShortID] = item.OriginalURL
	}
}