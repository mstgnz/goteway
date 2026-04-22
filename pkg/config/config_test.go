package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test cases
	tests := []struct {
		name          string
		configContent string
		wantErr       bool
		checkFunc     func(*Config) bool
	}{
		{
			name: "valid config",
			configContent: `{
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
			}`,
			wantErr: false,
			checkFunc: func(c *Config) bool {
				return c.Server.Port == 8080 &&
					c.Server.Host == "localhost" &&
					len(c.Routes) == 1 &&
					c.Routes[0].Path == "/api" &&
					c.Routes[0].Target == "http://localhost:3000" &&
					len(c.Routes[0].Methods) == 2 &&
					c.Routes[0].Methods[0] == "GET" &&
					c.Routes[0].Methods[1] == "POST" &&
					len(c.Routes[0].Middlewares) == 1 &&
					c.Routes[0].Middlewares[0] == "logging"
			},
		},
		{
			name: "invalid json",
			configContent: `{
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
			`,
			wantErr: true,
			checkFunc: func(c *Config) bool {
				return c == nil
			},
		},
		{
			name: "default values",
			configContent: `{
				"routes": [
					{
						"path": "/api",
						"target": "http://localhost:3000",
						"methods": ["GET"],
						"middlewares": []
					}
				]
			}`,
			wantErr: false,
			checkFunc: func(c *Config) bool {
				return c.Server.Port == 8080 &&
					c.Server.Host == "0.0.0.0" &&
					len(c.Routes) == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tmpfile, err := os.CreateTemp("", "config-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			// Write the config content
			if _, err := tmpfile.Write([]byte(tt.configContent)); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatalf("Failed to close temp file: %v", err)
			}

			// Load the config
			got, err := LoadConfig(tmpfile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check the config
			if !tt.wantErr && !tt.checkFunc(got) {
				t.Errorf("LoadConfig() got = %v, want to satisfy checkFunc", got)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		wantErr       bool
	}{
		{
			name: "missing path",
			configContent: `{"routes":[{"target":"http://localhost:3000","methods":["GET"],"middlewares":[]}]}`,
			wantErr: true,
		},
		{
			name: "missing target",
			configContent: `{"routes":[{"path":"/api","methods":["GET"],"middlewares":[]}]}`,
			wantErr: true,
		},
		{
			name: "invalid target url",
			configContent: `{"routes":[{"path":"/api","target":"not a url","methods":["GET"],"middlewares":[]}]}`,
			wantErr: true,
		},
		{
			name: "rateLimit limit zero",
			configContent: `{"routes":[{"path":"/api","target":"http://localhost:3000","methods":["GET"],"middlewares":[],"rateLimit":{"limit":0,"window":60}}]}`,
			wantErr: true,
		},
		{
			name: "rateLimit window zero",
			configContent: `{"routes":[{"path":"/api","target":"http://localhost:3000","methods":["GET"],"middlewares":[],"rateLimit":{"limit":10,"window":0}}]}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "config-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())
			tmpfile.Write([]byte(tt.configContent))
			tmpfile.Close()

			_, err = LoadConfig(tmpfile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("TEST_SECRET_KEY", "mysecret")
	defer os.Unsetenv("TEST_SECRET_KEY")

	configContent := `{"routes":[{"path":"/api","target":"http://localhost:3000","methods":["GET"],"middlewares":[],"auth":{"type":"apikey","config":{"header":"X-Key","key":"${TEST_SECRET_KEY}"}}}]}`
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Write([]byte(configContent))
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.Routes[0].Auth.Config["key"]; got != "mysecret" {
		t.Errorf("env var not expanded: got %q, want %q", got, "mysecret")
	}
}

func TestMethodNormalization(t *testing.T) {
	configContent := `{"routes":[{"path":"/api","target":"http://localhost:3000","methods":["get","post"],"middlewares":[]}]}`
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Write([]byte(configContent))
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	for _, m := range cfg.Routes[0].Methods {
		if m != "GET" && m != "POST" {
			t.Errorf("method not normalized to uppercase: %q", m)
		}
	}
}

func TestConfigStructs(t *testing.T) {
	// Test Route struct
	route := Route{
		Path:        "/api",
		Target:      "http://localhost:3000",
		Methods:     []string{"GET", "POST"},
		Middlewares: []string{"logging"},
		RateLimit: &RateLimitConfig{
			Limit:  100,
			Window: 60,
		},
		Auth: &AuthConfig{
			Type: "basic",
			Config: map[string]string{
				"username": "admin",
				"password": "password",
			},
		},
	}

	if route.Path != "/api" {
		t.Errorf("Route.Path = %v, want %v", route.Path, "/api")
	}
	if route.Target != "http://localhost:3000" {
		t.Errorf("Route.Target = %v, want %v", route.Target, "http://localhost:3000")
	}
	if len(route.Methods) != 2 {
		t.Errorf("len(Route.Methods) = %v, want %v", len(route.Methods), 2)
	}
	if route.RateLimit.Limit != 100 {
		t.Errorf("Route.RateLimit.Limit = %v, want %v", route.RateLimit.Limit, 100)
	}
	if route.Auth.Type != "basic" {
		t.Errorf("Route.Auth.Type = %v, want %v", route.Auth.Type, "basic")
	}
}
