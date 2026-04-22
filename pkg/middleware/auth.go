package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// AuthType represents the type of authentication
type AuthType string

const (
	BasicAuth  AuthType = "basic"
	APIKeyAuth AuthType = "apikey"
	JWTAuth    AuthType = "jwt"
)

// Authenticator represents an authenticator
type Authenticator interface {
	Authenticate(r *http.Request) bool
}

// BasicAuthenticator authenticates via HTTP Basic Auth
type BasicAuthenticator struct {
	username []byte
	password []byte
	log      *logger.Logger
}

// NewBasicAuthenticator creates a new basic authenticator
func NewBasicAuthenticator(username, password string, log *logger.Logger) *BasicAuthenticator {
	return &BasicAuthenticator{
		username: []byte(username),
		password: []byte(password),
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

	// Constant-time comparison to prevent timing attacks
	userMatch := subtle.ConstantTimeCompare([]byte(pair[0]), a.username)
	passMatch := subtle.ConstantTimeCompare([]byte(pair[1]), a.password)
	return userMatch&passMatch == 1
}

// APIKeyAuthenticator authenticates via a header-based API key
type APIKeyAuthenticator struct {
	header string
	key    []byte
	log    *logger.Logger
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(header, key string, log *logger.Logger) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		header: header,
		key:    []byte(key),
		log:    log,
	}
}

// Authenticate authenticates a request using an API key
func (a *APIKeyAuthenticator) Authenticate(r *http.Request) bool {
	key := r.Header.Get(a.header)
	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(key), a.key) == 1
}

// JWTAuthenticator authenticates requests using HS256-signed JWT tokens.
type JWTAuthenticator struct {
	secret []byte
	log    *logger.Logger
}

// NewJWTAuthenticator creates a new JWT authenticator with an HMAC-SHA256 secret.
func NewJWTAuthenticator(secret string, log *logger.Logger) *JWTAuthenticator {
	return &JWTAuthenticator{
		secret: []byte(secret),
		log:    log,
	}
}

// Authenticate validates the Bearer JWT in the Authorization header.
func (a *JWTAuthenticator) Authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	return a.verifyHS256(strings.TrimPrefix(auth, "Bearer "))
}

func (a *JWTAuthenticator) verifyHS256(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		a.log.Warn("JWT: invalid signature")
		return false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		a.log.Warn("JWT: failed to decode payload: %v", err)
		return false
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		a.log.Warn("JWT: failed to parse claims: %v", err)
		return false
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			a.log.Warn("JWT: token expired")
			return false
		}
	}

	return true
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
