// Package handler contains the HTTP handlers for the URL shortener service.
//
// All routes are registered on a chi.Router via Shortener.Router(). The router
// applies gzip compression/decompression, structured request logging, and
// cookie-based authentication middleware to every route.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/audit"
	"github.com/arsykor/go-url-shortener/internal/middleware"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// shortenRequest is the JSON body accepted by POST /api/shorten.
type shortenRequest struct {
	URL string `json:"url"`
}

// shortenResponse is the JSON body returned by POST /api/shorten.
type shortenResponse struct {
	Result string `json:"result"`
}

// batchRequestItem is a single element of the array accepted by POST /api/shorten/batch.
type batchRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// batchResponseItem is a single element of the array returned by POST /api/shorten/batch.
type batchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// userURLResponse is a single element of the array returned by GET /api/user/urls.
type userURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Shortener is the HTTP handler that exposes all URL-shortening endpoints.
type Shortener struct {
	service  *service.ShortenerService
	db       DB
	logger   *zap.SugaredLogger
	auditSvc *audit.Service
}

// DB is the minimal database interface required by the health-check endpoint.
type DB interface {
	// Ping verifies that the database connection is still alive.
	Ping() error
}

// NewShortener creates a Shortener handler.
// Pass nil for db to disable the /ping health-check endpoint.
// Pass nil for auditSvc to disable audit logging.
func NewShortener(service *service.ShortenerService, db DB, logger *zap.SugaredLogger, auditSvc *audit.Service) *Shortener {
	return &Shortener{
		service:  service,
		db:       db,
		logger:   logger,
		auditSvc: auditSvc,
	}
}

// Router builds and returns the chi router with all routes and middleware registered.
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

// handlePing handles GET /ping.
// Returns 200 OK when the database connection is healthy, 500 otherwise.
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

// handleGetEmpty handles GET / and always returns 400 Bad Request because a
// short ID is required.
func (h *Shortener) handleGetEmpty(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

// handlePost handles POST /.
// The request body must contain the original URL as plain text.
// Returns 201 Created with the short URL on success, or 409 Conflict if the
// URL was already shortened (the response body still contains the short URL).
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
	shortURL, conflict, err := h.service.ShortenURL(r.Context(), originalURL, userID)
	if err != nil {
		h.logger.Errorw("failed to build short URL", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	if conflict {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	w.Write([]byte(shortURL))

	h.auditSvc.Notify("shorten", userID, originalURL)
}

// handleGet handles GET /{id}.
// Responds with 307 Temporary Redirect to the original URL, 410 Gone when the
// URL has been deleted, or 400 Bad Request when the ID is unknown.
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

	userID, _ := middleware.GetUserID(r.Context())
	h.auditSvc.Notify("follow", userID, originalURL)
}

// handlePostJSON handles POST /api/shorten.
// Accepts {"url":"<original_url>"} and returns {"result":"<short_url>"}.
// Returns 201 Created on success or 409 Conflict if the URL already exists.
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
	shortURL, conflict, err := h.service.ShortenURL(r.Context(), originalURL, userID)
	if err != nil {
		h.logger.Errorw("failed to build short URL", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

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

	h.auditSvc.Notify("shorten", userID, originalURL)
}

// handlePostBatch handles POST /api/shorten/batch.
// Accepts a JSON array of {correlation_id, original_url} objects and returns a
// corresponding array of {correlation_id, short_url} objects with 201 Created.
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
	correlationMap := make(map[int]string)

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
		shortURL, err := url.JoinPath(baseURL, item.ShortID)
		if err != nil {
			h.logger.Errorw("failed to build short URL", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
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

// handleGetUserURLs handles GET /api/user/urls.
// Returns a JSON array of all {short_url, original_url} pairs owned by the
// authenticated user. Returns 204 No Content when the user has no URLs.
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
		shortURL, err := url.JoinPath(baseURL, u.ShortURL)
		if err != nil {
			h.logger.Errorw("failed to build short URL", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
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

// handleDeleteUserURLs handles DELETE /api/user/urls.
// Accepts a JSON array of short IDs and schedules them for async deletion.
// Returns 202 Accepted immediately without waiting for deletion to complete.
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

	h.service.DeleteUserURLs(shortIDs, userID)

	w.WriteHeader(http.StatusAccepted)
}
