package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/arsykor/go-url-shortener/internal/urlapi"
	"go.uber.org/zap"
)

// newExampleHandler creates a Shortener backed by an in-memory repository,
// suitable for use in examples and tests that do not require a database.
func newExampleHandler() *Shortener {
	repo := repository.NewInMemoryURLRepository()
	logger := zap.NewNop().Sugar()
	svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
	f := &urlapi.Facade{Svc: svc, Audit: nil}
	return NewShortener(f, nil, logger, nil)
}

// ExampleShortener_handlePost demonstrates shortening a URL via the plain-text
// POST / endpoint. The response body contains the full short URL and the status
// is 201 Created.
func ExampleShortener_handlePost() {
	r := newExampleHandler().Router()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))
	// Output:
	// 201
	// text/plain
}

// ExampleShortener_handlePost_conflict demonstrates that shortening the same
// URL twice returns 409 Conflict with the existing short URL in the body.
func ExampleShortener_handlePost_conflict() {
	r := newExampleHandler().Router()

	first := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	r.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, second)

	fmt.Println(w.Code)
	fmt.Println(strings.HasPrefix(strings.TrimSpace(w.Body.String()), "http://localhost:8080/"))
	// Output:
	// 409
	// true
}

// ExampleShortener_handleGet demonstrates following a short URL. The handler
// responds with 307 Temporary Redirect and sets the Location header to the
// original URL.
func ExampleShortener_handleGet() {
	r := newExampleHandler().Router()

	// Shorten the target URL first.
	postReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	// Derive the short path from the response body.
	shortURL := strings.TrimSpace(postW.Body.String())
	path := strings.TrimPrefix(shortURL, "http://localhost:8080")

	// Follow the short link.
	getReq := httptest.NewRequest(http.MethodGet, path, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	fmt.Println(getW.Code)
	fmt.Println(getW.Header().Get("Location"))
	// Output:
	// 307
	// https://practicum.yandex.ru
}

// ExampleShortener_handlePostJSON demonstrates the JSON API endpoint. The
// request body must be {"url":"..."} and the response body is {"result":"..."}.
func ExampleShortener_handlePostJSON() {
	r := newExampleHandler().Router()

	body := `{"url":"https://practicum.yandex.ru"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))

	var resp shortenResponse
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	fmt.Println(strings.HasPrefix(resp.Result, "http://localhost:8080/"))
	// Output:
	// 201
	// application/json
	// true
}

// ExampleShortener_handlePostBatch demonstrates the batch shortening endpoint.
// Each element in the request array must have correlation_id and original_url.
// The response mirrors correlation IDs alongside the generated short URLs.
func ExampleShortener_handlePostBatch() {
	r := newExampleHandler().Router()

	body := `[
		{"correlation_id":"a","original_url":"https://practicum.yandex.ru"},
		{"correlation_id":"b","original_url":"https://go.dev"}
	]`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))

	var items []batchResponseItem
	json.NewDecoder(w.Body).Decode(&items) //nolint:errcheck
	fmt.Println(len(items))
	fmt.Println(items[0].CorrelationID)
	fmt.Println(strings.HasPrefix(items[0].ShortURL, "http://localhost:8080/"))
	// Output:
	// 201
	// application/json
	// 2
	// a
	// true
}

// ExampleShortener_handleGetUserURLs_noContent demonstrates that GET /api/user/urls
// returns 204 No Content when the authenticated user has not yet shortened any URLs.
func ExampleShortener_handleGetUserURLs_noContent() {
	r := newExampleHandler().Router()

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	// Output:
	// 204
}

// ExampleShortener_handleGetUserURLs demonstrates retrieving all URLs shortened
// by the authenticated user. The cookie obtained from a POST request is reused
// to identify the same user on the subsequent GET.
func ExampleShortener_handleGetUserURLs() {
	r := newExampleHandler().Router()

	// Shorten a URL and capture the auth cookie.
	postReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	postResult := postW.Result()
	defer postResult.Body.Close()
	authCookie := postResult.Cookies()

	// List URLs using the same identity.
	listReq := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	for _, c := range authCookie {
		listReq.AddCookie(c)
	}
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)

	fmt.Println(listW.Code)
	fmt.Println(listW.Header().Get("Content-Type"))

	var items []userURLResponse
	json.NewDecoder(listW.Body).Decode(&items) //nolint:errcheck
	fmt.Println(len(items))
	fmt.Println(items[0].OriginalURL)
	fmt.Println(strings.HasPrefix(items[0].ShortURL, "http://localhost:8080/"))
	// Output:
	// 200
	// application/json
	// 1
	// https://practicum.yandex.ru
	// true
}

// ExampleShortener_handleDeleteUserURLs demonstrates scheduling async deletion
// of short URLs. The endpoint accepts a JSON array of short IDs and returns
// 202 Accepted immediately.
func ExampleShortener_handleDeleteUserURLs() {
	r := newExampleHandler().Router()

	// Shorten a URL and note the short ID.
	postReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	shortURL := strings.TrimSpace(postW.Body.String())
	shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	postResult := postW.Result()
	defer postResult.Body.Close()
	authCookie := postResult.Cookies()

	// Request deletion of the short URL.
	body := fmt.Sprintf(`["%s"]`, shortID)
	delReq := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(body))
	delReq.Header.Set("Content-Type", "application/json")
	for _, c := range authCookie {
		delReq.AddCookie(c)
	}
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)

	fmt.Println(delW.Code)
	// Output:
	// 202
}
