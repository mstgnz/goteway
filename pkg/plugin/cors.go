package plugin

import (
	"net/http"
	"strings"

	"github.com/mstgnz/goteway/pkg/logger"
)

// CORSPlugin represents a CORS plugin
type CORSPlugin struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
	log            *logger.Logger
}

// NewCORSPlugin creates a new CORS plugin
func NewCORSPlugin() *CORSPlugin {
	return &CORSPlugin{
		allowedOrigins: []string{"*"},
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization"},
	}
}

// Name returns the name of the plugin
func (p *CORSPlugin) Name() string {
	return "cors"
}

// Initialize initializes the plugin
func (p *CORSPlugin) Initialize(config map[string]interface{}, log *logger.Logger) error {
	p.log = log

	if origins, ok := config["allowedOrigins"].([]interface{}); ok {
		p.allowedOrigins = make([]string, len(origins))
		for i, origin := range origins {
			p.allowedOrigins[i] = origin.(string)
		}
	}

	if methods, ok := config["allowedMethods"].([]interface{}); ok {
		p.allowedMethods = make([]string, len(methods))
		for i, method := range methods {
			p.allowedMethods[i] = method.(string)
		}
	}

	if headers, ok := config["allowedHeaders"].([]interface{}); ok {
		p.allowedHeaders = make([]string, len(headers))
		for i, header := range headers {
			p.allowedHeaders[i] = header.(string)
		}
	}

	return nil
}

// ProcessRequest processes a request
func (p *CORSPlugin) ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		next.ServeHTTP(w, r)
		return
	}

	// Check if the origin is allowed
	allowed := false
	for _, allowedOrigin := range p.allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if !allowed {
		p.log.Warn("CORS: Origin not allowed: %s", origin)
		next.ServeHTTP(w, r)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(p.allowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(p.allowedHeaders, ", "))

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	next.ServeHTTP(w, r)
}
