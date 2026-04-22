package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config represents the configuration for the API gateway
type Config struct {
	Server struct {
		Port         int    `json:"port"`
		Host         string `json:"host"`
		MaxBodyBytes int64  `json:"maxBodyBytes,omitempty"`
	} `json:"server"`
	TLS     *TLSConfig                 `json:"tls,omitempty"`
	Plugins map[string]map[string]any  `json:"plugins,omitempty"`
	Routes  []Route                    `json:"routes"`
}

// TLSConfig holds TLS certificate configuration
type TLSConfig struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

// Route represents a route configuration
type Route struct {
	Path           string               `json:"path"`
	Target         string               `json:"target"`
	Targets        []string             `json:"targets,omitempty"` // multiple targets for load balancing
	Methods        []string             `json:"methods"`
	Middlewares    []string             `json:"middlewares"`
	RateLimit      *RateLimitConfig     `json:"rateLimit,omitempty"`
	Auth           *AuthConfig          `json:"auth,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Limit  int `json:"limit"`
	Window int `json:"window"` // in seconds
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type   string            `json:"type"` // "basic", "apikey", "jwt"
	Config map[string]string `json:"config"`
}

// CircuitBreakerConfig configures the per-route circuit breaker.
type CircuitBreakerConfig struct {
	// Threshold is the number of consecutive failures that opens the circuit.
	Threshold int `json:"threshold"`
	// OpenTimeoutSeconds is how long (in seconds) the circuit stays open before
	// transitioning to half-open and allowing a probe request.
	OpenTimeoutSeconds int `json:"openTimeoutSeconds"`
}

// LoadConfig loads the configuration from a file, expanding ${ENV_VAR} references.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := json.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	// Defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.MaxBodyBytes == 0 {
		cfg.Server.MaxBodyBytes = 32 << 20 // 32 MB
	}

	// Normalize HTTP methods to uppercase
	for i := range cfg.Routes {
		for j, m := range cfg.Routes[i].Methods {
			cfg.Routes[i].Methods[j] = strings.ToUpper(m)
		}
		// If no multi-target list, copy single Target into Targets for uniform handling
		if len(cfg.Routes[i].Targets) == 0 && cfg.Routes[i].Target != "" {
			cfg.Routes[i].Targets = []string{cfg.Routes[i].Target}
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks the configuration for required fields and valid values.
func (c *Config) Validate() error {
	for i, route := range c.Routes {
		if route.Path == "" {
			return fmt.Errorf("route %d: path is required", i)
		}
		if len(route.Targets) == 0 {
			return fmt.Errorf("route %d: target or targets is required", i)
		}
		for _, t := range route.Targets {
			if _, err := url.ParseRequestURI(t); err != nil {
				return fmt.Errorf("route %d: invalid target URL %q: %w", i, t, err)
			}
		}
		if route.RateLimit != nil {
			if route.RateLimit.Limit <= 0 {
				return fmt.Errorf("route %d: rateLimit.limit must be positive", i)
			}
			if route.RateLimit.Window <= 0 {
				return fmt.Errorf("route %d: rateLimit.window must be positive", i)
			}
		}
		if route.CircuitBreaker != nil {
			if route.CircuitBreaker.Threshold <= 0 {
				return fmt.Errorf("route %d: circuitBreaker.threshold must be positive", i)
			}
			if route.CircuitBreaker.OpenTimeoutSeconds <= 0 {
				return fmt.Errorf("route %d: circuitBreaker.openTimeoutSeconds must be positive", i)
			}
		}
	}
	return nil
}
