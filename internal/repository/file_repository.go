package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sync"
)

// StorageEntry represents a single entry in the file storage
type StorageEntry struct {
	UUID        uuid.UUID `json:"uuid"`
	ShortURL    string    `json:"short_url"`
	OriginalURL string    `json:"original_url"`
}

// FileURLRepository implements service.URLRepository using file storage
type FileURLRepository struct {
	mu       sync.RWMutex
	filePath string
	urls     map[string]string    // shortID -> originalURL
	uuidMap  map[string]uuid.UUID // shortID -> uuid
}

// NewFileURLRepository creates a new file-based repository
func NewFileURLRepository(filePath string) (*FileURLRepository, error) {
	repo := &FileURLRepository{
		filePath: filePath,
		urls:     make(map[string]string),
		uuidMap:  make(map[string]uuid.UUID),
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
	}

	return nil
}

// Save stores a URL mapping and persists to file
func (r *FileURLRepository) Save(ctx context.Context, shortID, originalURL string) {
	r.mu.Lock()

	// Check if this shortID already exists
	_, exists := r.uuidMap[shortID]
	if !exists {
		r.uuidMap[shortID] = uuid.New()
	}

	r.urls[shortID] = originalURL

	entries := make([]StorageEntry, 0, len(r.urls))
	for sid, origURL := range r.urls {
		uuid := r.uuidMap[sid]
		entries = append(entries, StorageEntry{
			UUID:        uuid,
			ShortURL:    sid,
			OriginalURL: origURL,
		})
	}

	r.mu.Unlock()

	r.writeToFile(entries)
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
