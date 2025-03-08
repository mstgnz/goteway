package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	// Create test middlewares
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Test-1", "value1")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Test-2", "value2")
			next.ServeHTTP(w, r)
		})
	}

	middleware3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Test-3", "value3")
			next.ServeHTTP(w, r)
		})
	}

	// Create a final handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test cases
	tests := []struct {
		name        string
		middlewares []Middleware
		wantHeaders map[string]string
	}{
		{
			name:        "no middleware",
			middlewares: []Middleware{},
			wantHeaders: map[string]string{},
		},
		{
			name:        "single middleware",
			middlewares: []Middleware{middleware1},
			wantHeaders: map[string]string{
				"X-Test-1": "value1",
			},
		},
		{
			name:        "multiple middlewares",
			middlewares: []Middleware{middleware1, middleware2, middleware3},
			wantHeaders: map[string]string{
				"X-Test-1": "value1",
				"X-Test-2": "value2",
				"X-Test-3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Chain the middlewares
			handler := Chain(tt.middlewares...)(finalHandler)

			// Create a request
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			w := httptest.NewRecorder()

			// Call the handler
			handler.ServeHTTP(w, req)

			// Check the response
			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code = %v, want %v", resp.StatusCode, http.StatusOK)
			}

			// Check the headers
			for key, want := range tt.wantHeaders {
				if got := resp.Header.Get(key); got != want {
					t.Errorf("Header %q = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestApply(t *testing.T) {
	// Create a middleware
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Test", "value")
			next.ServeHTTP(w, r)
		})
	}

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply the middleware
	wrappedHandler := Apply(handler, middleware)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()

	// Call the handler
	wrappedHandler.ServeHTTP(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check the header
	if got := resp.Header.Get("X-Test"); got != "value" {
		t.Errorf("Header X-Test = %q, want %q", got, "value")
	}
}
