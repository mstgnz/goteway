package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

func TestNew(t *testing.T) {
	// Create a temporary config file
	configContent := `{
		"server": {
			"port": 8080,
			"host": "localhost"
		},
		"routes": [
			{
				"path": "/api",
				"target": "http://localhost:3000",
				"methods": ["GET", "POST"],
				"middlewares": ["logging"]
			}
		]
	}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create a gateway
	gw, err := New(tmpfile.Name(), logger.INFO)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Check the gateway
	if gw == nil {
		t.Error("New() returned nil")
	}
	if gw.config == nil {
		t.Error("Gateway config is nil")
	}
	if gw.log == nil {
		t.Error("Gateway log is nil")
	}
	if gw.pluginManager == nil {
		t.Error("Gateway pluginManager is nil")
	}
	if gw.routes == nil {
		t.Error("Gateway routes is nil")
	}
	if len(gw.routes) != 1 {
		t.Errorf("len(Gateway routes) = %v, want %v", len(gw.routes), 1)
	}
	if _, ok := gw.routes["/api"]; !ok {
		t.Error("Gateway routes does not contain /api")
	}
}

func TestGatewayRouting(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from " + r.URL.Path))
	}))
	defer ts.Close()

	// Create a temporary config file
	configContent := `{
		"server": {
			"port": 8080,
			"host": "localhost"
		},
		"routes": [
			{
				"path": "/api",
				"target": "` + ts.URL + `",
				"methods": ["GET", "POST"],
				"middlewares": []
			}
		]
	}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create a gateway
	gw, err := New(tmpfile.Name(), logger.INFO)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Create a test server using the gateway's handler
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := gw.routes["/api"]
		if !ok {
			http.NotFound(w, r)
			return
		}
		route.Handler.ServeHTTP(w, r)
	}))
	defer gwServer.Close()

	// Test cases
	tests := []struct {
		name           string
		path           string
		method         string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "GET request",
			path:           "/api/users",
			method:         "GET",
			wantStatusCode: http.StatusOK,
			wantBody:       "Hello from /users",
		},
		{
			name:           "POST request",
			path:           "/api/users",
			method:         "POST",
			wantStatusCode: http.StatusOK,
			wantBody:       "Hello from /users",
		},
		{
			name:           "PUT request (not allowed)",
			path:           "/api/users",
			method:         "PUT",
			wantStatusCode: http.StatusMethodNotAllowed,
			wantBody:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req, err := http.NewRequest(tt.method, gwServer.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Send the request
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Check the status code
			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Status code = %v, want %v", resp.StatusCode, tt.wantStatusCode)
			}

			// Check the body
			if tt.wantStatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}
				if string(body) != tt.wantBody {
					t.Errorf("Body = %q, want %q", string(body), tt.wantBody)
				}
			}
		})
	}
}

func TestGatewayStartStop(t *testing.T) {
	// Create a temporary config file
	configContent := `{
		"server": {
			"port": 0,
			"host": "localhost"
		},
		"routes": [
			{
				"path": "/api",
				"target": "http://localhost:3000",
				"methods": ["GET"],
				"middlewares": []
			}
		]
	}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create a gateway
	gw, err := New(tmpfile.Name(), logger.INFO)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Start the gateway in a goroutine
	go func() {
		if err := gw.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Failed to start gateway: %v", err)
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the gateway
	if err := gw.Stop(); err != nil {
		t.Errorf("Failed to stop gateway: %v", err)
	}
}
