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

// batchRequestItem represents a single item in batch request
type batchRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// batchResponseItem represents a single item in batch response
type batchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// Shortener handles HTTP requests for URL shortening
type Shortener struct {
	service *service.ShortenerService
	db      DB
}

// DB interface for database operations
type DB interface {
	Ping() error
}

func NewShortener(service *service.ShortenerService, db DB) *Shortener {
	return &Shortener{
		service: service,
		db:      db,
	}
}

func (h *Shortener) Router(logger *zap.SugaredLogger) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.WithGzipDecompression)
	r.Use(middleware.WithGzipCompression)
	r.Use(middleware.WithLogging(logger))
	r.Get("/ping", h.handlePing)
	r.Post("/", h.handlePost)
	r.Post("/api/shorten", h.handlePostJSON)
	r.Post("/api/shorten/batch", h.handlePostBatch)
	r.Get("/", h.handleGetEmpty)
	r.Get("/{id}", h.handleGet)
	return r
}

func (h *Shortener) handlePing(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, "Database not configured", http.StatusInternalServerError)
		return
	}

	if err := h.db.Ping(); err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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

	shortURL, conflict := h.service.ShortenURL(r.Context(), originalURL)

	w.Header().Set("Content-Type", "text/plain")
	if conflict {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
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

	shortURL, conflict := h.service.ShortenURL(r.Context(), originalURL)

	response := shortenResponse{
		Result: shortURL,
	}

	w.Header().Set("Content-Type", "application/json")
	if conflict {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Shortener) handlePostBatch(w http.ResponseWriter, r *http.Request) {
	var req []batchRequestItem

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

	if len(req) == 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	originalURLs := make([]string, 0, len(req))
	correlationMap := make(map[int]string) // index -> correlation_id

	for i, item := range req {
		originalURL := strings.TrimSpace(item.OriginalURL)
		if originalURL == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		originalURLs = append(originalURLs, originalURL)
		correlationMap[i] = item.CorrelationID
	}

	batchItems := h.service.ShortenURLBatch(r.Context(), originalURLs)

	response := make([]batchResponseItem, 0, len(batchItems))
	baseURL := h.service.BaseURL()
	for i, item := range batchItems {
		response = append(response, batchResponseItem{
			CorrelationID: correlationMap[i],
			ShortURL:      baseURL + "/" + item.ShortID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
