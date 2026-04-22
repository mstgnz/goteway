package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

func makeJWT(secret string, exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"sub": "test", "exp": exp})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%s.%s", header, payload)))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s.%s", header, payload, sig)
}

func TestJWTAuthenticator(t *testing.T) {
	log := logger.New(logger.INFO)
	secret := "test-secret"
	auth := NewJWTAuthenticator(secret, log)

	tests := []struct {
		name        string
		token       string
		wantSuccess bool
	}{
		{
			name:        "valid token",
			token:       makeJWT(secret, time.Now().Add(time.Hour).Unix()),
			wantSuccess: true,
		},
		{
			name:        "expired token",
			token:       makeJWT(secret, time.Now().Add(-time.Hour).Unix()),
			wantSuccess: false,
		},
		{
			name:        "wrong secret",
			token:       makeJWT("wrong-secret", time.Now().Add(time.Hour).Unix()),
			wantSuccess: false,
		},
		{
			name:        "no bearer prefix",
			token:       "",
			wantSuccess: false,
		},
		{
			name:        "malformed token",
			token:       "not.a.jwt",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			if got := auth.Authenticate(req); got != tt.wantSuccess {
				t.Errorf("Authenticate() = %v, want %v", got, tt.wantSuccess)
			}
		})
	}
}

func TestBasicAuthenticator(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create an authenticator
	auth := NewBasicAuthenticator("admin", "password", log)

	// Test cases
	tests := []struct {
		name        string
		username    string
		password    string
		wantSuccess bool
	}{
		{
			name:        "valid credentials",
			username:    "admin",
			password:    "password",
			wantSuccess: true,
		},
		{
			name:        "invalid username",
			username:    "invalid",
			password:    "password",
			wantSuccess: false,
		},
		{
			name:        "invalid password",
			username:    "admin",
			password:    "invalid",
			wantSuccess: false,
		},
		{
			name:        "empty credentials",
			username:    "",
			password:    "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			if tt.username != "" || tt.password != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}

			// Authenticate
			success := auth.Authenticate(req)

			// Check result
			if success != tt.wantSuccess {
				t.Errorf("Authenticate() = %v, want %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestAPIKeyAuthenticator(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create an authenticator
	auth := NewAPIKeyAuthenticator("X-API-Key", "secret-key", log)

	// Test cases
	tests := []struct {
		name        string
		headerName  string
		headerValue string
		wantSuccess bool
	}{
		{
			name:        "valid key",
			headerName:  "X-API-Key",
			headerValue: "secret-key",
			wantSuccess: true,
		},
		{
			name:        "invalid key",
			headerName:  "X-API-Key",
			headerValue: "invalid-key",
			wantSuccess: false,
		},
		{
			name:        "wrong header",
			headerName:  "X-Wrong-Header",
			headerValue: "secret-key",
			wantSuccess: false,
		},
		{
			name:        "empty header",
			headerName:  "",
			headerValue: "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.headerValue)
			}

			// Authenticate
			success := auth.Authenticate(req)

			// Check result
			if success != tt.wantSuccess {
				t.Errorf("Authenticate() = %v, want %v", success, tt.wantSuccess)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test cases
	tests := []struct {
		name           string
		authenticator  Authenticator
		setAuth        func(*http.Request)
		wantStatusCode int
	}{
		{
			name:          "basic auth success",
			authenticator: NewBasicAuthenticator("admin", "password", log),
			setAuth: func(r *http.Request) {
				r.SetBasicAuth("admin", "password")
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:          "basic auth failure",
			authenticator: NewBasicAuthenticator("admin", "password", log),
			setAuth: func(r *http.Request) {
				r.SetBasicAuth("admin", "wrong")
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:          "api key success",
			authenticator: NewAPIKeyAuthenticator("X-API-Key", "secret-key", log),
			setAuth: func(r *http.Request) {
				r.Header.Set("X-API-Key", "secret-key")
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:          "api key failure",
			authenticator: NewAPIKeyAuthenticator("X-API-Key", "secret-key", log),
			setAuth: func(r *http.Request) {
				r.Header.Set("X-API-Key", "wrong-key")
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "no auth",
			authenticator:  NewBasicAuthenticator("admin", "password", log),
			setAuth:        func(r *http.Request) {},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the auth middleware
			wrappedHandler := AuthMiddleware(tt.authenticator, log)(handler)

			// Create a request
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			tt.setAuth(req)
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
