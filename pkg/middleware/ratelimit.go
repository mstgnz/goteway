package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// RateLimiter implements a sliding-window rate limiter per client IP.
type RateLimiter struct {
	limit   int
	window  time.Duration
	clients map[string][]time.Time
	mu      sync.Mutex
	log     *logger.Logger
	stopCh  chan struct{}
}

// NewRateLimiter creates a new rate limiter and starts a background cleanup goroutine.
func NewRateLimiter(limit int, window time.Duration, log *logger.Logger) *RateLimiter {
	rl := &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string][]time.Time),
		log:     log,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Stop shuts down the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanup periodically evicts stale entries to prevent unbounded memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, times := range rl.clients {
				var valid []time.Time
				for _, t := range times {
					if now.Sub(t) <= rl.window {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.clients, ip)
				} else {
					rl.clients[ip] = valid
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// RateLimitMiddleware creates a middleware that limits the rate of requests per client IP.
func RateLimitMiddleware(limiter *RateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := extractIP(r)

			limiter.mu.Lock()

			now := time.Now()
			var requests []time.Time
			for _, t := range limiter.clients[clientIP] {
				if now.Sub(t) <= limiter.window {
					requests = append(requests, t)
				}
			}

			if len(requests) >= limiter.limit {
				limiter.mu.Unlock()
				limiter.log.Warn("Rate limit exceeded for %s", clientIP)
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			limiter.clients[clientIP] = append(requests, now)
			limiter.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP returns the client IP address, stripping the port from RemoteAddr.
func extractIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
