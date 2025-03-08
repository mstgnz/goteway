package middleware

import (
	"net/http"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// LoggingMiddleware creates a middleware that logs requests
func LoggingMiddleware(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a custom response writer to capture the status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Log the request
			duration := time.Since(start)
			log.Info("%s %s %s %d %s", r.RemoteAddr, r.Method, r.URL.Path, rw.statusCode, duration)
		})
	}
}

// responseWriter is a custom response writer that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}
