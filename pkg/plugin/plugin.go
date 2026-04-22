package plugin

import (
	"net/http"

	"github.com/mstgnz/goteway/pkg/logger"
)

// Plugin represents a plugin
type Plugin interface {
	Name() string
	// Initialize is called once when the plugin is registered. config may be nil.
	Initialize(config map[string]any, log *logger.Logger) error
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

// RegisterPlugin registers and initializes a plugin. An optional config map may
// be passed as the second argument; it is forwarded to Plugin.Initialize.
func (m *Manager) RegisterPlugin(p Plugin, configs ...map[string]any) {
	var cfg map[string]any
	if len(configs) > 0 {
		cfg = configs[0]
	}

	if err := p.Initialize(cfg, m.log); err != nil {
		m.log.Error("Failed to initialize plugin %s: %v", p.Name(), err)
		return
	}

	m.plugins[p.Name()] = p
	m.log.Info("Registered plugin: %s", p.Name())
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
