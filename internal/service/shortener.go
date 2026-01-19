package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
)

type ShortenerService struct {
	repo    URLRepository
	baseURL string
}

type URLRepository interface {
	Save(ctx context.Context, shortID, originalURL string)
	Get(ctx context.Context, shortID string) (string, bool)
	SaveBatch(ctx context.Context, items []BatchItem)
}

// BatchItem represents a single item in a batch operation
type BatchItem struct {
	ShortID     string
	OriginalURL string
}

func NewShortenerService(repo URLRepository, baseURL string) *ShortenerService {
	return &ShortenerService{
		repo:    repo,
		baseURL: baseURL,
	}
}

// generateShortID generates a random short ID (8 characters)
func generateShortID() string {
	b := make([]byte, 6)
	rand.Read(b)
	encoded := base64.URLEncoding.EncodeToString(b)
	// Take first 8 characters
	if len(encoded) >= 8 {
		return encoded[:8]
	}
	return encoded
}

// ShortenURL creates a shortened URL for the given original URL
func (s *ShortenerService) ShortenURL(ctx context.Context, originalURL string) string {
	shortID := generateShortID()
	s.repo.Save(ctx, shortID, originalURL)
	return s.baseURL + "/" + shortID
}

// GetOriginalURL retrieves the original URL by short ID
func (s *ShortenerService) GetOriginalURL(ctx context.Context, shortID string) (string, bool) {
	return s.repo.Get(ctx, shortID)
}

// BaseURL returns the base URL for shortened URLs
func (s *ShortenerService) BaseURL() string {
	return s.baseURL
}

// ShortenURLBatch creates shortened URLs for multiple URLs in a single operation
func (s *ShortenerService) ShortenURLBatch(ctx context.Context, originalURLs []string) []BatchItem {
	if len(originalURLs) == 0 {
		return nil
	}

	items := make([]BatchItem, 0, len(originalURLs))
	for _, originalURL := range originalURLs {
		items = append(items, BatchItem{
			ShortID:     generateShortID(),
			OriginalURL: originalURL,
		})
	}

	s.repo.SaveBatch(ctx, items)

	return items
}
