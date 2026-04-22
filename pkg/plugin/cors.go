package plugin

import (
	"net/http"
	"strings"

	"github.com/mstgnz/goteway/pkg/logger"
)

// CORSPlugin handles Cross-Origin Resource Sharing headers.
type CORSPlugin struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
	log            *logger.Logger
}

// NewCORSPlugin creates a new CORS plugin. The default allowedOrigins is ["*"];
// configure explicit origins via the "cors" entry in config.json plugins.
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

// Initialize sets up the plugin with optional config overrides.
func (p *CORSPlugin) Initialize(config map[string]any, log *logger.Logger) error {
	p.log = log

	if config == nil {
		return nil
	}

	if origins, ok := config["allowedOrigins"].([]any); ok {
		p.allowedOrigins = make([]string, len(origins))
		for i, origin := range origins {
			p.allowedOrigins[i] = origin.(string)
		}
	}

	if methods, ok := config["allowedMethods"].([]any); ok {
		p.allowedMethods = make([]string, len(methods))
		for i, method := range methods {
			p.allowedMethods[i] = method.(string)
		}
	}

	if headers, ok := config["allowedHeaders"].([]any); ok {
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

	allowed := false
	for _, allowedOrigin := range p.allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if !allowed {
		p.log.Warn("CORS: origin not allowed: %s", origin)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Vary: Origin must be set whenever the response varies by Origin so that
	// caches do not serve one client's preflight response to another.
	w.Header().Add("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(p.allowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(p.allowedHeaders, ", "))

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	next.ServeHTTP(w, r)
}
