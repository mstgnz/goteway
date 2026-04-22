package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// LoggingMiddleware creates a middleware that logs requests
func LoggingMiddleware(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			log.Info("%s %s %s %d %s", r.RemoteAddr, r.Method, r.URL.Path, rw.statusCode, duration)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code and forward
// optional interfaces (Flusher, Hijacker) so streaming and WebSocket proxying work.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Flush forwards to the underlying ResponseWriter if it implements http.Flusher.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying ResponseWriter if it implements http.Hijacker.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported by underlying ResponseWriter")
}
