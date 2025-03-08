package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// RateLimiter represents a rate limiter
type RateLimiter struct {
	limit   int
	window  time.Duration
	clients map[string][]time.Time
	mu      sync.Mutex
	log     *logger.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration, log *logger.Logger) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string][]time.Time),
		log:     log,
	}
}

// RateLimitMiddleware creates a middleware that limits the rate of requests
func RateLimitMiddleware(limiter *RateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.RemoteAddr

			limiter.mu.Lock()

			// Remove old requests
			now := time.Now()
			var requests []time.Time
			for _, t := range limiter.clients[clientIP] {
				if now.Sub(t) <= limiter.window {
					requests = append(requests, t)
				}
			}

			// Check if the client has exceeded the limit
			if len(requests) >= limiter.limit {
				limiter.mu.Unlock()
				limiter.log.Warn("Rate limit exceeded for %s", clientIP)
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add the current request
			limiter.clients[clientIP] = append(requests, now)
			limiter.mu.Unlock()

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}
