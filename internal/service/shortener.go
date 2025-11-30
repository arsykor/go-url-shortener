package service

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/arsykor/go-url-shortener/internal/repository"
)

const baseURL = "http://localhost:8080"

type ShortenerService struct {
	repo repository.URLRepository
}

func NewShortenerService(repo repository.URLRepository) *ShortenerService {
	return &ShortenerService{
		repo: repo,
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
func (s *ShortenerService) ShortenURL(originalURL string) string {
	shortID := generateShortID()
	s.repo.Save(shortID, originalURL)
	return baseURL + "/" + shortID
}

// GetOriginalURL retrieves the original URL by short ID
func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	return s.repo.Get(shortID)
}
