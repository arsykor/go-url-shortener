package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/middleware"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// shortenRequest represents the JSON request body for /api/shorten
type shortenRequest struct {
	URL string `json:"url"`
}

// shortenResponse represents the JSON response body for /api/shorten
type shortenResponse struct {
	Result string `json:"result"`
}

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
	r.Use(middleware.WithGzipDecompression)
	r.Use(middleware.WithGzipCompression)
	r.Use(middleware.WithLogging(logger))
	r.Post("/", h.handlePost)
	r.Post("/api/shorten", h.handlePostJSON)
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

func (h *Shortener) handlePostJSON(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(req.URL)
	if originalURL == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	shortURL := h.service.ShortenURL(r.Context(), originalURL)

	response := shortenResponse{
		Result: shortURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
