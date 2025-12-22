package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/middleware"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
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

func (h *Shortener) Router(logger *zap.SugaredLogger) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.WithLogging(logger))
	r.Post("/", h.handlePost)
	r.Get("/", h.handleGetEmpty)
	r.Get("/{id}", h.handleGet)
	return r
}

func (h *Shortener) handleGetEmpty(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Bad request", http.StatusBadRequest)
}

func (h *Shortener) handlePost(w http.ResponseWriter, r *http.Request) {
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

	shortURL := h.service.ShortenURL(r.Context(), originalURL)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *Shortener) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	originalURL, exists := h.service.GetOriginalURL(r.Context(), id)
	if !exists {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
