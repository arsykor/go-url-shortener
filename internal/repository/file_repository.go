package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/google/uuid"
)

// StorageEntry represents a single entry in the file storage
type StorageEntry struct {
	UUID        uuid.UUID `json:"uuid"`
	ShortURL    string    `json:"short_url"`
	OriginalURL string    `json:"original_url"`
	UserID      string    `json:"user_id,omitempty"`
}

// FileURLRepository implements service.URLRepository using file storage
type FileURLRepository struct {
	mu          sync.RWMutex
	filePath    string
	urls        map[string]string    // shortID -> originalURL
	uuidMap     map[string]uuid.UUID // shortID -> uuid
	reverseUrls map[string]string    // originalURL -> shortID
	userURLs    map[string][]string  // userID -> []shortID
}

// NewFileURLRepository creates a new file-based repository
func NewFileURLRepository(filePath string) (*FileURLRepository, error) {
	repo := &FileURLRepository{
		filePath:    filePath,
		urls:        make(map[string]string),
		uuidMap:     make(map[string]uuid.UUID),
		reverseUrls: make(map[string]string),
		userURLs:    make(map[string][]string),
	}

	// Load existing data from file
	if err := repo.loadFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load from file: %w", err)
	}

	return repo, nil
}

// loadFromFile reads data from the JSON file
func (r *FileURLRepository) loadFromFile() error {
	fileInfo, err := os.Stat(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty data
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", r.filePath)
	}

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if fileInfo != nil && fileInfo.IsDir() {
			return fmt.Errorf("cannot read directory as file: %s", r.filePath)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// If file is empty, start with empty data
	if len(data) == 0 {
		return nil
	}

	var entries []StorageEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Load data into memory
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		r.urls[entry.ShortURL] = entry.OriginalURL
		r.uuidMap[entry.ShortURL] = entry.UUID
		r.reverseUrls[entry.OriginalURL] = entry.ShortURL
		if entry.UserID != "" {
			r.userURLs[entry.UserID] = append(r.userURLs[entry.UserID], entry.ShortURL)
		}
	}

	return nil
}

// Save stores a URL mapping and persists to file
// Returns existing shortID and true if originalURL already exists
func (r *FileURLRepository) Save(ctx context.Context, shortID, originalURL, userID string) (existingShortID string, conflict bool) {
	r.mu.Lock()

	if existingShortID, exists := r.reverseUrls[originalURL]; exists {
		r.mu.Unlock()
		return existingShortID, true
	}

	_, exists := r.uuidMap[shortID]
	if !exists {
		r.uuidMap[shortID] = uuid.New()
	}

	r.urls[shortID] = originalURL
	r.reverseUrls[originalURL] = shortID
	if userID != "" {
		r.userURLs[userID] = append(r.userURLs[userID], shortID)
	}

	entries := r.buildEntries()

	r.mu.Unlock()

	r.writeToFile(entries)
	return "", false
}

// buildEntries creates storage entries from current state
func (r *FileURLRepository) buildEntries() []StorageEntry {
	// Build reverse map: shortID -> userID
	shortToUser := make(map[string]string)
	for uid, shortIDs := range r.userURLs {
		for _, sid := range shortIDs {
			shortToUser[sid] = uid
		}
	}

	entries := make([]StorageEntry, 0, len(r.urls))
	for sid, origURL := range r.urls {
		entries = append(entries, StorageEntry{
			UUID:        r.uuidMap[sid],
			ShortURL:    sid,
			OriginalURL: origURL,
			UserID:      shortToUser[sid],
		})
	}
	return entries
}

// writeToFile writes entries to file (called without lock)
func (r *FileURLRepository) writeToFile(entries []StorageEntry) error {
	// Ensure the directory exists
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(r.filePath, data, 0666); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves the original URL by short ID
func (r *FileURLRepository) Get(ctx context.Context, shortID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, exists := r.urls[shortID]
	return url, exists
}

// SaveBatch stores multiple URL mappings in a single operation
func (r *FileURLRepository) SaveBatch(ctx context.Context, items []service.BatchItem, userID string) error {
	if len(items) == 0 {
		return nil
	}

	r.mu.Lock()

	for _, item := range items {
		_, exists := r.uuidMap[item.ShortID]
		if !exists {
			r.uuidMap[item.ShortID] = uuid.New()
		}
		r.urls[item.ShortID] = item.OriginalURL
		if userID != "" {
			r.userURLs[userID] = append(r.userURLs[userID], item.ShortID)
		}
	}

	entries := r.buildEntries()

	r.mu.Unlock()

	return r.writeToFile(entries)
}

// GetURLsByUser returns all URLs shortened by a specific user
func (r *FileURLRepository) GetURLsByUser(ctx context.Context, userID string) ([]service.UserURL, error) {
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
