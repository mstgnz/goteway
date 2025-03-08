package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/mstgnz/goteway/pkg/logger"
)

// AuthType represents the type of authentication
type AuthType string

const (
	// BasicAuth represents basic authentication
	BasicAuth AuthType = "basic"
	// APIKeyAuth represents API key authentication
	APIKeyAuth AuthType = "apikey"
	// JWTAuth represents JWT authentication
	JWTAuth AuthType = "jwt"
)

// Authenticator represents an authenticator
type Authenticator interface {
	Authenticate(r *http.Request) bool
}

// BasicAuthenticator represents a basic authenticator
type BasicAuthenticator struct {
	username string
	password string
	log      *logger.Logger
}

// NewBasicAuthenticator creates a new basic authenticator
func NewBasicAuthenticator(username, password string, log *logger.Logger) *BasicAuthenticator {
	return &BasicAuthenticator{
		username: username,
		password: password,
		log:      log,
	}
}

// Authenticate authenticates a request using basic authentication
func (a *BasicAuthenticator) Authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}

	payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		a.log.Error("Failed to decode basic auth: %v", err)
		return false
	}

	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return false
	}

	return pair[0] == a.username && pair[1] == a.password
}

// APIKeyAuthenticator represents an API key authenticator
type APIKeyAuthenticator struct {
	header string
	key    string
	log    *logger.Logger
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(header, key string, log *logger.Logger) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		header: header,
		key:    key,
		log:    log,
	}
}

// Authenticate authenticates a request using an API key
func (a *APIKeyAuthenticator) Authenticate(r *http.Request) bool {
	key := r.Header.Get(a.header)
	return key == a.key
}

// AuthMiddleware creates a middleware that authenticates requests
func AuthMiddleware(authenticator Authenticator, log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authenticator.Authenticate(r) {
				log.Warn("Authentication failed for %s", r.RemoteAddr)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
