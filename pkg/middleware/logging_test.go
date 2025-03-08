package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/goteway/pkg/logger"
)

type testResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        *bytes.Buffer
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *testResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *testResponseWriter) Header() http.Header {
	return http.Header{}
}

func TestLoggingMiddleware(t *testing.T) {
	// Create a logger
	testLogger := logger.New(logger.DEBUG)

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply the logging middleware
	wrappedHandler := LoggingMiddleware(testLogger)(handler)

	// Test cases
	tests := []struct {
		name           string
		method         string
		path           string
		wantStatusCode int
	}{
		{
			name:           "GET request",
			method:         "GET",
			path:           "/test",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "POST request",
			method:         "POST",
			path:           "/api/data",
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Call the handler
			wrappedHandler.ServeHTTP(w, req)

			// Check the response
			resp := w.Result()
			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Status code = %v, want %v", resp.StatusCode, tt.wantStatusCode)
			}
		})
	}
}
