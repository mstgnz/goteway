package plugin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/goteway/pkg/logger"
)

// MockPlugin is a mock plugin for testing
type MockPlugin struct {
	name            string
	initializeCount int
	processCount    int
}

func (p *MockPlugin) Name() string {
	return p.name
}

func (p *MockPlugin) Initialize(config map[string]any, log *logger.Logger) error {
	p.initializeCount++
	return nil
}

func (p *MockPlugin) ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	p.processCount++
	w.Header().Add("X-Plugin-Processed", p.name)
	next.ServeHTTP(w, r)
}

func TestPluginManager(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create a plugin manager
	manager := NewManager(log)

	// Check the manager
	if manager == nil {
		t.Error("NewManager() returned nil")
		return
	}
	if manager.plugins == nil {
		t.Error("Manager plugins is nil")
	}
	if manager.log == nil {
		t.Error("Manager log is nil")
	}

	// Create mock plugins
	plugin1 := &MockPlugin{name: "plugin1"}
	plugin2 := &MockPlugin{name: "plugin2"}

	// Register plugins
	manager.RegisterPlugin(plugin1)
	manager.RegisterPlugin(plugin2)

	// Check registered plugins
	if len(manager.plugins) != 2 {
		t.Errorf("len(Manager plugins) = %v, want %v", len(manager.plugins), 2)
	}

	// Get plugins
	p1, ok := manager.GetPlugin("plugin1")
	if !ok {
		t.Error("GetPlugin(plugin1) returned false")
	}
	if p1 != plugin1 {
		t.Errorf("GetPlugin(plugin1) = %v, want %v", p1, plugin1)
	}

	p2, ok := manager.GetPlugin("plugin2")
	if !ok {
		t.Error("GetPlugin(plugin2) returned false")
	}
	if p2 != plugin2 {
		t.Errorf("GetPlugin(plugin2) = %v, want %v", p2, plugin2)
	}

	// Get non-existent plugin
	_, ok = manager.GetPlugin("non-existent")
	if ok {
		t.Error("GetPlugin(non-existent) returned true")
	}
}

func TestPluginMiddleware(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create a plugin manager
	manager := NewManager(log)

	// Create mock plugins
	plugin1 := &MockPlugin{name: "plugin1"}
	plugin2 := &MockPlugin{name: "plugin2"}

	// Register plugins
	manager.RegisterPlugin(plugin1)
	manager.RegisterPlugin(plugin2)

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test cases
	tests := []struct {
		name           string
		pluginName     string
		wantProcessed  bool
		wantStatusCode int
		wantHeader     string
	}{
		{
			name:           "existing plugin",
			pluginName:     "plugin1",
			wantProcessed:  true,
			wantStatusCode: http.StatusOK,
			wantHeader:     "plugin1",
		},
		{
			name:           "non-existent plugin",
			pluginName:     "non-existent",
			wantProcessed:  false,
			wantStatusCode: http.StatusOK,
			wantHeader:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset process count
			plugin1.processCount = 0
			plugin2.processCount = 0

			// Apply the plugin middleware
			wrappedHandler := manager.Middleware(tt.pluginName)(handler)

			// Create a request
			req := httptest.NewRequest("GET", "http://example.com/foo", nil)
			w := httptest.NewRecorder()

			// Call the handler
			wrappedHandler.ServeHTTP(w, req)

			// Check the response
			resp := w.Result()
			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Status code = %v, want %v", resp.StatusCode, tt.wantStatusCode)
			}

			// Check if the plugin was processed
			if tt.wantProcessed {
				var expectedCount1, expectedCount2 int
				if tt.pluginName == "plugin1" {
					expectedCount1 = 1
				}
				if tt.pluginName == "plugin2" {
					expectedCount2 = 1
				}

				if plugin1.processCount != expectedCount1 {
					t.Errorf("plugin1.processCount = %v, want %v", plugin1.processCount, expectedCount1)
				}
				if plugin2.processCount != expectedCount2 {
					t.Errorf("plugin2.processCount = %v, want %v", plugin2.processCount, expectedCount2)
				}
			}

			// Check the header
			if tt.wantHeader != "" {
				if got := resp.Header.Get("X-Plugin-Processed"); got != tt.wantHeader {
					t.Errorf("Header X-Plugin-Processed = %q, want %q", got, tt.wantHeader)
				}
			}
		})
	}
}

func TestCORSPlugin(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create a CORS plugin
	corsPlugin := NewCORSPlugin()

	// Initialize the plugin
	err := corsPlugin.Initialize(nil, log)
	if err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test cases
	tests := []struct {
		name           string
		method         string
		headers        map[string]string
		wantStatusCode int
		wantHeaders    map[string]string
	}{
		{
			name:   "OPTIONS request",
			method: "OPTIONS",
			headers: map[string]string{
				"Origin":                         "http://example.com",
				"Access-Control-Request-Method":  "POST",
				"Access-Control-Request-Headers": "Content-Type",
			},
			wantStatusCode: http.StatusOK,
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "http://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
		},
		{
			name:   "GET request",
			method: "GET",
			headers: map[string]string{
				"Origin": "http://example.com",
			},
			wantStatusCode: http.StatusOK,
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "http://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest(tt.method, "http://example.com/foo", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			// Process the request
			corsPlugin.ProcessRequest(w, req, handler)

			// Check the response
			resp := w.Result()
			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Status code = %v, want %v", resp.StatusCode, tt.wantStatusCode)
			}

			// Check the headers
			for k, want := range tt.wantHeaders {
				if got := resp.Header.Get(k); got != want {
					t.Errorf("Header %q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestExamplePlugin(t *testing.T) {
	// Create a logger
	log := logger.New(logger.INFO)

	// Create an example plugin
	examplePlugin := NewExamplePlugin()

	// Initialize the plugin
	err := examplePlugin.Initialize(nil, log)
	if err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()

	// Process the request
	examplePlugin.ProcessRequest(w, req, handler)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Check the header
	if got := resp.Header.Get("X-Example-Plugin"); got != "Hello from example plugin!" {
		t.Errorf("Header X-Example-Plugin = %q, want %q", got, "Hello from example plugin!")
	}
}
