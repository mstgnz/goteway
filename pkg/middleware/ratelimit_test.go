package middleware

import (
	"fmt"
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

func TestRateLimiterIPExtraction(t *testing.T) {
	log := logger.New(logger.INFO)
	limiter := NewRateLimiter(2, time.Second, log)
	defer limiter.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := RateLimitMiddleware(limiter)(handler)

	// Requests from same IP but different ports must share the same bucket
	allowed := 0
	for port := 1000; port < 1005; port++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = fmt.Sprintf("10.0.0.1:%d", port)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		if w.Result().StatusCode == http.StatusOK {
			allowed++
		}
	}
	if allowed != 2 {
		t.Errorf("expected 2 allowed requests for same IP across different ports, got %d", allowed)
	}
}

func TestRateLimiterStop(t *testing.T) {
	log := logger.New(logger.INFO)
	limiter := NewRateLimiter(10, 100*time.Millisecond, log)
	// Stop must not panic and must be safe to call once
	limiter.Stop()
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
