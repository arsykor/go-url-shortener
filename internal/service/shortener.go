// Package service implements URL shortening over a pluggable URLRepository.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrURLNotFound = errors.New("url not found")
	ErrURLDeleted  = errors.New("url has been deleted")
)

// ShortenerService creates and resolves short links.
type ShortenerService struct {
	repo     URLRepository
	baseURL  string
	logger   *zap.SugaredLogger
	deleteCh chan DeleteTask
}

// URLRepository is the storage interface; implementations must be goroutine-safe.
type URLRepository interface {
	Save(ctx context.Context, shortID, originalURL, userID string) (existingShortID string, conflict bool)
	Get(ctx context.Context, shortID string) (originalURL string, isDeleted bool, exists bool)
	SaveBatch(ctx context.Context, items []BatchItem, userID string) error
	GetURLsByUser(ctx context.Context, userID string) ([]UserURL, error)
	DeleteURLs(ctx context.Context, shortIDs []string, userID string) error
}

// BatchItem represents a single item in a batch operation
//
//generate:reset
type BatchItem struct {
	ShortID     string
	OriginalURL string
}

// UserURL represents a URL pair belonging to a user
//
//generate:reset
type UserURL struct {
	ShortURL    string
	OriginalURL string
}

// DeleteTask represents a single URL deletion task
//
//generate:reset
type DeleteTask struct {
	ShortID string
	UserID  string
}

// NewShortenerService starts background delete flushing and returns the service.
func NewShortenerService(repo URLRepository, baseURL string, logger *zap.SugaredLogger) *ShortenerService {
	s := &ShortenerService{
		repo:     repo,
		baseURL:  baseURL,
		logger:   logger,
		deleteCh: make(chan DeleteTask, 1024),
	}
	go s.flushDeletes()
	return s
}

// generateShortID generates a random short ID (8 characters)
func generateShortID() string {
	var b [6]byte
	rand.Read(b[:])
	return base64.URLEncoding.EncodeToString(b[:])[:8]
}

// ShortenURL creates a shortened URL for the given original URL.
func (s *ShortenerService) ShortenURL(ctx context.Context, originalURL, userID string) (shortURL string, conflict bool, err error) {
	shortID := generateShortID()
	existingShortID, conflict := s.repo.Save(ctx, shortID, originalURL, userID)
	if conflict {
		result, err := url.JoinPath(s.baseURL, existingShortID)
		return result, true, err
	}
	result, err := url.JoinPath(s.baseURL, shortID)
	return result, false, err
}

// GetOriginalURL retrieves the original URL by short ID.
// Returns the original URL or a typed error: ErrURLNotFound / ErrURLDeleted.
func (s *ShortenerService) GetOriginalURL(ctx context.Context, shortID string) (string, error) {
	originalURL, isDeleted, exists := s.repo.Get(ctx, shortID)
	if !exists {
		return "", ErrURLNotFound
	}
	if isDeleted {
		return "", ErrURLDeleted
	}
	return originalURL, nil
}

// BaseURL returns the base URL for shortened URLs
func (s *ShortenerService) BaseURL() string {
	return s.baseURL
}

// ShortenURLBatch creates shortened URLs for multiple URLs in a single operation
func (s *ShortenerService) ShortenURLBatch(ctx context.Context, originalURLs []string, userID string) ([]BatchItem, error) {
	if len(originalURLs) == 0 {
		return nil, nil
	}

	items := make([]BatchItem, 0, len(originalURLs))
	for _, originalURL := range originalURLs {
		items = append(items, BatchItem{
			ShortID:     generateShortID(),
			OriginalURL: originalURL,
		})
	}

	if err := s.repo.SaveBatch(ctx, items, userID); err != nil {
		return nil, err
	}

	return items, nil
}

// GetURLsByUser returns all URLs shortened by a specific user
func (s *ShortenerService) GetURLsByUser(ctx context.Context, userID string) ([]UserURL, error) {
	return s.repo.GetURLsByUser(ctx, userID)
}

// DeleteUserURLs accepts a list of short IDs and schedules them for async deletion.
func (s *ShortenerService) DeleteUserURLs(shortIDs []string, userID string) {
	// Create a channel for this request's delete tasks
	inputCh := make(chan DeleteTask)
	go func() {
		defer close(inputCh)
		for _, id := range shortIDs {
			inputCh <- DeleteTask{ShortID: id, UserID: userID}
		}
	}()

	// Drain this request's channel into the main delete channel
	go func() {
		for task := range inputCh {
			s.deleteCh <- task
		}
	}()
}

// fanIn merges multiple input channels into a single output channel.
func fanIn(doneCh chan struct{}, channels ...chan DeleteTask) chan DeleteTask {
	finalCh := make(chan DeleteTask)
	var wg sync.WaitGroup

	for _, ch := range channels {
		chClosure := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range chClosure {
				select {
				case <-doneCh:
					return
				case finalCh <- data:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(finalCh)
	}()

	return finalCh
}

// flushDeletes runs in the background, reads from deleteCh and batch-deletes URLs.
func (s *ShortenerService) flushDeletes() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var buffer []DeleteTask

	for {
		select {
		case task := <-s.deleteCh:
			buffer = append(buffer, task)
		case <-ticker.C:
			if len(buffer) == 0 {
				continue
			}
			// Group by userID for batch update
			grouped := make(map[string][]string)
			for _, t := range buffer {
				grouped[t.UserID] = append(grouped[t.UserID], t.ShortID)
			}
			for userID, shortIDs := range grouped {
				if err := s.repo.DeleteURLs(context.Background(), shortIDs, userID); err != nil {
					s.logger.Errorw("failed to delete URLs", "error", err, "userID", userID)
				}
			}
			buffer = buffer[:0]
		}
	}
}
