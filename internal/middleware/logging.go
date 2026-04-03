package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	// responseData stores information about the response
	responseData struct {
		status int
		size   int
	}

	// loggingResponseWriter wraps http.ResponseWriter to capture status and size
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

// WithLogging is a middleware that logs request and response information
func WithLogging(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}
			lw := loggingResponseWriter{
				ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
				responseData:   responseData,
			}

			next.ServeHTTP(&lw, r) // внедряем реализацию http.ResponseWriter

			duration := time.Since(start)

			status := responseData.status
			if status == 0 {
				status = http.StatusOK
			}

			logger.Infow(
				"Request processed",
				"uri", r.RequestURI,
				"method", r.Method,
				"status", status,
				"duration", duration,
				"size", responseData.size,
			)
		})
	}
}
