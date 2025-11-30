package repository

type URLRepository interface {
	Save(shortID, originalURL string)
	Get(shortID string) (string, bool)
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
func (r *InMemoryURLRepository) Save(shortID, originalURL string) {
	r.urls[shortID] = originalURL
}

// Get retrieves the original URL by short ID
func (r *InMemoryURLRepository) Get(shortID string) (string, bool) {
	url, exists := r.urls[shortID]
	return url, exists
}
