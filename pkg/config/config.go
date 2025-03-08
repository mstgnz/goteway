package config

import (
	"encoding/json"
	"os"
)

// Config represents the configuration for the API gateway
type Config struct {
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"server"`
	Routes []Route `json:"routes"`
}

// Route represents a route configuration
type Route struct {
	Path        string           `json:"path"`
	Target      string           `json:"target"`
	Methods     []string         `json:"methods"`
	Middlewares []string         `json:"middlewares"`
	RateLimit   *RateLimitConfig `json:"rateLimit,omitempty"`
	Auth        *AuthConfig      `json:"auth,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Limit  int `json:"limit"`
	Window int `json:"window"` // in seconds
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type   string            `json:"type"` // e.g., "jwt", "basic", "apikey"
	Config map[string]string `json:"config"`
}

// LoadConfig loads the configuration from a file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	// Set default values if not specified
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}

	return &config, nil
}
