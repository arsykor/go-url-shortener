package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewInMemoryURLRepository()
			svc := service.NewShortenerService(repo, "http://localhost:8080")
			handler := NewShortener(svc)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Prepare data
			repo := repository.NewInMemoryURLRepository()
			if tt.setupID != "" && tt.setupURL != "" {
				repo.Save(tt.setupID, tt.setupURL)
			}

			svc := service.NewShortenerService(repo, "http://localhost:8080")
			handler := NewShortener(svc)
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
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewShortenerService(repo, "http://localhost:8080")
	handler := NewShortener(svc)
	r := handler.Router()

	req := httptest.NewRequest(http.MethodPut, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	// chi router returns 405 Method Not Allowed for unsupported methods
	assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
}
