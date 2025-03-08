package plugin

import (
	"net/http"

	"github.com/mstgnz/goteway/pkg/logger"
)

// Plugin represents a plugin
type Plugin interface {
	// Name returns the name of the plugin
	Name() string
	// Initialize initializes the plugin
	Initialize(config map[string]interface{}, log *logger.Logger) error
	// ProcessRequest processes a request
	ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler)
}

// Manager represents a plugin manager
type Manager struct {
	plugins map[string]Plugin
	log     *logger.Logger
}

// NewManager creates a new plugin manager
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
		log:     log,
	}
}

// RegisterPlugin registers a plugin
func (m *Manager) RegisterPlugin(plugin Plugin) {
	m.plugins[plugin.Name()] = plugin
	m.log.Info("Registered plugin: %s", plugin.Name())
}

// GetPlugin returns a plugin by name
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	plugin, ok := m.plugins[name]
	return plugin, ok
}

// Middleware creates a middleware that processes requests using a plugin
func (m *Manager) Middleware(pluginName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plugin, ok := m.GetPlugin(pluginName)
			if !ok {
				m.log.Error("Plugin not found: %s", pluginName)
				next.ServeHTTP(w, r)
				return
			}

			plugin.ProcessRequest(w, r, next)
		})
	}
}
