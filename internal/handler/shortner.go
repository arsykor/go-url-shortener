package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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

// userURLResponse represents a single URL pair in GET /api/user/urls response
type userURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Shortener handles HTTP requests for URL shortening
type Shortener struct {
	service *service.ShortenerService
	db      DB
	logger  *zap.SugaredLogger
}

// DB interface for database operations
type DB interface {
	Ping() error
}

func NewShortener(service *service.ShortenerService, db DB, logger *zap.SugaredLogger) *Shortener {
	return &Shortener{
		service: service,
		db:      db,
		logger:  logger,
	}
}

func (h *Shortener) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.WithGzipDecompression)
	r.Use(middleware.WithGzipCompression)
	r.Use(middleware.WithLogging(h.logger))
	r.Use(middleware.WithAuth)
	r.Get("/ping", h.handlePing)
	r.Post("/", h.handlePost)
	r.Post("/api/shorten", h.handlePostJSON)
	r.Post("/api/shorten/batch", h.handlePostBatch)
	r.Get("/api/user/urls", h.handleGetUserURLs)
	r.Delete("/api/user/urls", h.handleDeleteUserURLs)
	r.Get("/", h.handleGetEmpty)
	r.Get("/{id}", h.handleGet)
	return r
}

func (h *Shortener) handlePing(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		h.logger.Error("ping failed: database not configured")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := h.db.Ping(); err != nil {
		h.logger.Errorw("ping failed: database connection error", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Shortener) handleGetEmpty(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

func (h *Shortener) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.logger.Errorw("failed to get user ID", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	shortURL, conflict := h.service.ShortenURL(r.Context(), originalURL, userID)

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
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.GetOriginalURL(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrURLDeleted) {
			w.WriteHeader(http.StatusGone)
			return
		}
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (h *Shortener) handlePostJSON(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(req.URL)
	if originalURL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.logger.Errorw("failed to get user ID", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	shortURL, conflict := h.service.ShortenURL(r.Context(), originalURL, userID)

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
		h.logger.Errorw("failed to encode response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (h *Shortener) handlePostBatch(w http.ResponseWriter, r *http.Request) {
	var req []batchRequestItem

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(req) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originalURLs := make([]string, 0, len(req))
	correlationMap := make(map[int]string) // index -> correlation_id

	for i, item := range req {
		originalURL := strings.TrimSpace(item.OriginalURL)
		if originalURL == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		originalURLs = append(originalURLs, originalURL)
		correlationMap[i] = item.CorrelationID
	}

	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.logger.Errorw("failed to get user ID", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	batchItems, err := h.service.ShortenURLBatch(r.Context(), originalURLs, userID)
	if err != nil {
		h.logger.Errorw("failed to shorten URL batch", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := h.service.BaseURL()
	response := make([]batchResponseItem, 0, len(batchItems))
	for i, item := range batchItems {
		shortURL, _ := url.JoinPath(baseURL, item.ShortID)
		response = append(response, batchResponseItem{
			CorrelationID: correlationMap[i],
			ShortURL:      shortURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Errorw("failed to encode response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (h *Shortener) handleGetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	urls, err := h.service.GetURLsByUser(r.Context(), userID)
	if err != nil {
		h.logger.Errorw("failed to get URLs by user", "error", err, "userID", userID)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	baseURL := h.service.BaseURL()
	response := make([]userURLResponse, 0, len(urls))
	for _, u := range urls {
		shortURL, _ := url.JoinPath(baseURL, u.ShortURL)
		response = append(response, userURLResponse{
			ShortURL:    shortURL,
			OriginalURL: u.OriginalURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Errorw("failed to encode response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// handleDeleteUserURLs handles DELETE /api/user/urls
func (h *Shortener) handleDeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var shortIDs []string
	if err := json.Unmarshal(body, &shortIDs); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(shortIDs) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Schedule async deletion
	h.service.DeleteUserURLs(shortIDs, userID)

	w.WriteHeader(http.StatusAccepted)
}
