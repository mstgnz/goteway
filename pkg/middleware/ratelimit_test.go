package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

func TestRateLimiter(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Test cases
	tests := []struct {
		name            string
		limit           int
		window          time.Duration
		requests        int
		wantAllowed     int
		wantRateLimited int
	}{
		{
			name:            "under limit",
			limit:           5,
			window:          time.Second,
			requests:        3,
			wantAllowed:     3,
			wantRateLimited: 0,
		},
		{
			name:            "at limit",
			limit:           5,
			window:          time.Second,
			requests:        5,
			wantAllowed:     5,
			wantRateLimited: 0,
		},
		{
			name:            "over limit",
			limit:           5,
			window:          time.Second,
			requests:        10,
			wantAllowed:     5,
			wantRateLimited: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a rate limiter
			limiter := NewRateLimiter(tt.limit, tt.window, log)

			// Create a handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Apply the rate limit middleware
			wrappedHandler := RateLimitMiddleware(limiter)(handler)

			// Count responses
			allowed := 0
			rateLimited := 0

			// Make requests
			for i := 0; i < tt.requests; i++ {
				// Create a request with the same IP
				req := httptest.NewRequest("GET", "http://example.com/foo", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				w := httptest.NewRecorder()

				// Call the handler
				wrappedHandler.ServeHTTP(w, req)

				// Check the response
				resp := w.Result()
				if resp.StatusCode == http.StatusOK {
					allowed++
				} else if resp.StatusCode == http.StatusTooManyRequests {
					rateLimited++
				}
			}

			// Check results
			if allowed != tt.wantAllowed {
				t.Errorf("Allowed requests = %v, want %v", allowed, tt.wantAllowed)
			}
			if rateLimited != tt.wantRateLimited {
				t.Errorf("Rate limited requests = %v, want %v", rateLimited, tt.wantRateLimited)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test cases
	tests := []struct {
		name            string
		limit           int
		window          time.Duration
		requests        int
		wantAllowed     int
		wantRateLimited int
	}{
		{
			name:            "under limit",
			limit:           3,
			window:          time.Second,
			requests:        2,
			wantAllowed:     2,
			wantRateLimited: 0,
		},
		{
			name:            "over limit",
			limit:           3,
			window:          time.Second,
			requests:        5,
			wantAllowed:     3,
			wantRateLimited: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a rate limiter
			limiter := NewRateLimiter(tt.limit, tt.window, log)

			// Apply the rate limit middleware
			wrappedHandler := RateLimitMiddleware(limiter)(handler)

			// Count responses
			allowed := 0
			rateLimited := 0

			// Make requests
			for i := 0; i < tt.requests; i++ {
				// Create a request with the same IP
				req := httptest.NewRequest("GET", "http://example.com/foo", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				w := httptest.NewRecorder()

				// Call the handler
				wrappedHandler.ServeHTTP(w, req)

				// Check the response
				resp := w.Result()
				if resp.StatusCode == http.StatusOK {
					allowed++
				} else if resp.StatusCode == http.StatusTooManyRequests {
					rateLimited++
				}
			}

			// Check results
			if allowed != tt.wantAllowed {
				t.Errorf("Allowed responses = %v, want %v", allowed, tt.wantAllowed)
			}
			if rateLimited != tt.wantRateLimited {
				t.Errorf("Rate limited responses = %v, want %v", rateLimited, tt.wantRateLimited)
			}
		})
	}
}
