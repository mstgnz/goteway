package middleware

import (
	"net/http"
)

// Middleware represents a middleware function
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middlewares together
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Apply applies a middleware to a handler
func Apply(h http.Handler, m Middleware) http.Handler {
	return m(h)
}
