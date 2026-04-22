package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware_generatesID(t *testing.T) {
	handler := RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Error("X-Request-ID not set on inbound request")
		}
		if len(id) != 16 {
			t.Errorf("X-Request-ID length = %d, want 16", len(id))
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID not present in response headers")
	}
}

func TestRequestIDMiddleware_forwardsExistingID(t *testing.T) {
	const clientID = "abc123def456"

	handler := RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != clientID {
			t.Errorf("inbound X-Request-ID = %q, want %q", got, clientID)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != clientID {
		t.Errorf("response X-Request-ID = %q, want %q", got, clientID)
	}
}

func TestRequestIDMiddleware_uniquePerRequest(t *testing.T) {
	seen := make(map[string]bool)
	handler := RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		id := w.Header().Get("X-Request-ID")
		if seen[id] {
			t.Fatalf("duplicate request ID generated: %q", id)
		}
		seen[id] = true
	}
}
