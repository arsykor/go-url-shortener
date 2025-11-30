package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/service"
)

// Shortener handles HTTP requests for URL shortening
type Shortener struct {
	service *service.ShortenerService
}

func NewShortener(service *service.ShortenerService) *Shortener {
	return &Shortener{
		service: service,
	}
}

func (h *Shortener) HandlerShortener(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodGet:
		h.handleGet(w, r)
	default:
		http.Error(w, "Bad request", http.StatusBadRequest)
	}
}

func (h *Shortener) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	shortURL := h.service.ShortenURL(originalURL)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *Shortener) handleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	originalURL, exists := h.service.GetOriginalURL(id)
	if !exists {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
