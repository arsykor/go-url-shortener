package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// gzipWriter wraps http.ResponseWriter to compress responses
type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	// w.Writer будет отвечать за gzip-сжатие, поэтому пишем в него
	return w.Writer.Write(b)
}

// shouldCompressContentType проверяет, нужно ли сжимать данный Content-Type
func shouldCompressContentType(contentType string) bool {
	compressibleTypes := []string{
		"application/json",
		"text/html",
	}

	for _, t := range compressibleTypes {
		if strings.HasPrefix(contentType, t) {
			return true
		}
	}
	return false
}

// contentTypeWrapper перехватывает заголовки для проверки Content-Type
type contentTypeWrapper struct {
	http.ResponseWriter
	headerWritten  bool
	shouldCompress bool
	gzipWriter     *gzip.Writer
}

func (w *contentTypeWrapper) WriteHeader(statusCode int) {
	if w.headerWritten {
		return
	}
	w.headerWritten = true

	// проверяем Content-Type
	contentType := w.Header().Get("Content-Type")
	w.shouldCompress = shouldCompressContentType(contentType)

	if w.shouldCompress {
		// создаём gzip.Writer поверх оригинального ResponseWriter
		gz, err := gzip.NewWriterLevel(w.ResponseWriter, gzip.BestSpeed)
		if err != nil {
			w.ResponseWriter.WriteHeader(statusCode)
			return
		}
		w.gzipWriter = gz
		w.Header().Set("Content-Encoding", "gzip")
		w.ResponseWriter.WriteHeader(statusCode)
	} else {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *contentTypeWrapper) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}

	if w.shouldCompress && w.gzipWriter != nil {
		// используем gzipWriter для сжатия
		return w.gzipWriter.Write(b)
	}
	// если не нужно сжимать, пишем напрямую
	return w.ResponseWriter.Write(b)
}

func (w *contentTypeWrapper) Close() error {
	if w.gzipWriter != nil {
		return w.gzipWriter.Close()
	}
	return nil
}

// WithGzipCompression is a middleware that compresses responses using gzip.
// It only compresses content types: application/json and text/html.
func WithGzipCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// создаём обёртку для проверки Content-Type
		ctw := &contentTypeWrapper{
			ResponseWriter: w,
		}
		defer ctw.Close()

		next.ServeHTTP(ctw, r)
	})
}

// WithGzipDecompression is a middleware that decompresses gzip-compressed requests.
func WithGzipDecompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// создаём gzip.Reader для распаковки тела запроса
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer gz.Close()

		// подменяем тело запроса на распакованное
		r.Body = io.NopCloser(gz)

		next.ServeHTTP(w, r)
	})
}
