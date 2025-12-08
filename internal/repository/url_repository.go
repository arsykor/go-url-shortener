package repository

import (
	"context"
)

// TODO: интерфейс временно здесь, позже перенесу в нужное место
type URLRepository interface {
	Save(ctx context.Context, shortID, originalURL string)
	Get(ctx context.Context, shortID string) (string, bool)
}

type InMemoryURLRepository struct {
	urls map[string]string
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls: make(map[string]string),
	}
}

// Save stores a URL mapping
func (r *InMemoryURLRepository) Save(ctx context.Context, shortID, originalURL string) {
	r.urls[shortID] = originalURL
}

// Get retrieves the original URL by short ID
func (r *InMemoryURLRepository) Get(ctx context.Context, shortID string) (string, bool) {
	url, exists := r.urls[shortID]
	return url, exists
}
