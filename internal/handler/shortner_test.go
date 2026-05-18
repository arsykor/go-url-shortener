package handler

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/arsykor/go-url-shortener/internal/urlapi"
)

func TestHandlerShortener_Post(t *testing.T) {
	type want struct {
		code        int
		response    string
		contentType string
	}
	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		want    want
		wantErr bool
	}{
		{
			name:   "positive test - valid URL",
			method: http.MethodPost,
			path:   "/",
			body:   "https://practicum.yandex.ru/",
			want: want{
				code:        http.StatusCreated,
				contentType: "text/plain",
			},
			wantErr: false,
		},
		{
			name:   "negative test - wrong path",
			method: http.MethodPost,
			path:   "/wrong",
			body:   "https://practicum.yandex.ru/",
			want: want{
				code: http.StatusMethodNotAllowed,
			},
			wantErr: true,
		},
		{
			name:   "negative test - empty body",
			method: http.MethodPost,
			path:   "/",
			body:   "",
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - whitespace only",
			method: http.MethodPost,
			path:   "/",
			body:   "   ",
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
	}

	logger := zap.NewNop().Sugar()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewInMemoryURLRepository()
			svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
			f := &urlapi.Facade{Svc: svc, Audit: nil}
			handler := NewShortener(f, nil, logger, nil)
			r := handler.Router()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if !tt.wantErr {
				assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))
				resBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.NotEmpty(t, resBody)
				assert.Contains(t, string(resBody), "http://localhost:8080/")
			}
		})
	}
}

func TestHandlerShortener_Get(t *testing.T) {
	type want struct {
		code     int
		location string
	}
	tests := []struct {
		name     string
		method   string
		path     string
		setupURL string
		setupID  string
		want     want
		wantErr  bool
	}{
		{
			name:     "positive test - valid ID",
			method:   http.MethodGet,
			path:     "/ilovegolang",
			setupURL: "https://practicum.yandex.ru/",
			setupID:  "ilovegolang",
			want: want{
				code:     http.StatusTemporaryRedirect,
				location: "https://practicum.yandex.ru/",
			},
			wantErr: false,
		},
		{
			name:   "negative test - ID not found",
			method: http.MethodGet,
			path:   "/nonexistent",
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - empty ID",
			method: http.MethodGet,
			path:   "/",
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
	}

	logger := zap.NewNop().Sugar()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Prepare data
			repo := repository.NewInMemoryURLRepository()
			ctx := context.Background()
			if tt.setupID != "" && tt.setupURL != "" {
				repo.Save(ctx, tt.setupID, tt.setupURL, "")
			}

			svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
			f := &urlapi.Facade{Svc: svc, Audit: nil}
			handler := NewShortener(f, nil, logger, nil)
			r := handler.Router()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if !tt.wantErr {
				assert.Equal(t, tt.want.location, res.Header.Get("Location"))
			}
		})
	}
}

func TestHandlerShortener_UnsupportedMethod(t *testing.T) {
	logger := zap.NewNop().Sugar()
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
	f := &urlapi.Facade{Svc: svc, Audit: nil}
	handler := NewShortener(f, nil, logger, nil)
	r := handler.Router()

	req := httptest.NewRequest(http.MethodPut, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
}

func TestHandlerShortener_PostJSON(t *testing.T) {
	type want struct {
		code        int
		contentType string
		result      string
	}
	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		want    want
		wantErr bool
	}{
		{
			name:   "positive test - valid JSON with URL",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			want: want{
				code:        http.StatusCreated,
				contentType: "application/json",
			},
			wantErr: false,
		},
		{
			name:   "negative test - invalid JSON",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":}`,
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - empty URL",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":""}`,
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - missing URL field",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{}`,
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - whitespace only URL",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"   "}`,
			want: want{
				code: http.StatusBadRequest,
			},
			wantErr: true,
		},
		{
			name:   "negative test - wrong path",
			method: http.MethodPost,
			path:   "/api/wrong",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			want: want{
				code: http.StatusNotFound,
			},
			wantErr: true,
		},
	}

	logger := zap.NewNop().Sugar()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewInMemoryURLRepository()
			svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
			f := &urlapi.Facade{Svc: svc, Audit: nil}
			handler := NewShortener(f, nil, logger, nil)
			r := handler.Router()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if !tt.wantErr {
				assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))

				resBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.NotEmpty(t, resBody)

				var response shortenResponse
				err = json.Unmarshal(resBody, &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.Result)
				assert.Contains(t, response.Result, "http://localhost:8080/")
			}
		})
	}
}

func TestHandlerShortener_InternalStats(t *testing.T) {
	logger := zap.NewNop().Sugar()
	_, tenNet, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	_, loopback, err := net.ParseCIDR("127.0.0.0/8")
	require.NoError(t, err)

	ctx := context.Background()
	tests := []struct {
		name       string
		trusted    *net.IPNet
		xRealIP    string
		setup      func(*repository.InMemoryURLRepository)
		wantCode   int
		wantStats  statsResponse
		decodeBody bool
	}{
		{
			name:     "empty trusted subnet config denies",
			trusted:  nil,
			xRealIP:  "10.0.0.1",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "missing X-Real-IP denies",
			trusted:  tenNet,
			xRealIP:  "",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "IP outside subnet denies",
			trusted:  tenNet,
			xRealIP:  "11.0.0.1",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "invalid IP in header denies",
			trusted:  tenNet,
			xRealIP:  "not-an-ip",
			wantCode: http.StatusForbidden,
		},
		{
			name:    "trusted IP returns zeros",
			trusted: tenNet,
			xRealIP: "10.1.2.3",
			wantStats: statsResponse{
				URLs:  0,
				Users: 0,
			},
			wantCode:   http.StatusOK,
			decodeBody: true,
		},
		{
			name:    "counts urls and distinct users",
			trusted: loopback,
			xRealIP: "127.0.0.1",
			setup: func(repo *repository.InMemoryURLRepository) {
				_, _ = repo.Save(ctx, "id-one", "https://example.com/a", "user-a")
				_, _ = repo.Save(ctx, "id-two", "https://example.com/b", "user-a")
				_, _ = repo.Save(ctx, "id-three", "https://example.com/c", "user-b")
			},
			wantStats: statsResponse{
				URLs:  3,
				Users: 2,
			},
			wantCode:   http.StatusOK,
			decodeBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewInMemoryURLRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}
			svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
			f := &urlapi.Facade{Svc: svc, Audit: nil}
			h := NewShortener(f, nil, logger, tt.trusted)
			req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			w := httptest.NewRecorder()
			h.Router().ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)

			if !tt.decodeBody {
				return
			}

			var got statsResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, tt.wantStats, got)
		})
	}
}
