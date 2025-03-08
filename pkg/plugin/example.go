package plugin

import (
	"net/http"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// ExamplePlugin represents an example plugin
type ExamplePlugin struct {
	message string
	log     *logger.Logger
}

// NewExamplePlugin creates a new example plugin
func NewExamplePlugin() *ExamplePlugin {
	return &ExamplePlugin{
		message: "Hello from example plugin!",
	}
}

// Name returns the name of the plugin
func (p *ExamplePlugin) Name() string {
	return "example"
}

// Initialize initializes the plugin
func (p *ExamplePlugin) Initialize(config map[string]any, log *logger.Logger) error {
	p.log = log

	if message, ok := config["message"].(string); ok {
		p.message = message
	}

	p.log.Info("Example plugin initialized with message: %s", p.message)
	return nil
}

// ProcessRequest processes a request
func (p *ExamplePlugin) ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Add a custom header
	w.Header().Set("X-Example-Plugin", p.message)

	// Add a timestamp header
	w.Header().Set("X-Example-Timestamp", time.Now().Format(time.RFC3339))

	// Log the request
	p.log.Debug("Example plugin processing request: %s %s", r.Method, r.URL.Path)

	// Call the next handler
	next.ServeHTTP(w, r)
}
