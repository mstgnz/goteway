package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const requestIDHeader = "X-Request-ID"

// RequestIDMiddleware ensures every request carries an X-Request-ID header.
// If the incoming request already has one it is forwarded as-is; otherwise a
// cryptographically random 16-character hex ID is generated and attached to
// both the inbound request (visible to the backend) and the response.
func RequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestIDHeader)
			if id == "" {
				id = newRequestID()
				r.Header.Set(requestIDHeader, id)
			}
			w.Header().Set(requestIDHeader, id)
			next.ServeHTTP(w, r)
		})
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a zero-filled ID rather than panic.
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}
