package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/mstgnz/goteway/pkg/config"
	"github.com/mstgnz/goteway/pkg/logger"
	"github.com/mstgnz/goteway/pkg/middleware"
	"github.com/mstgnz/goteway/pkg/plugin"
)

// Gateway represents an API gateway
type Gateway struct {
	config        *config.Config
	log           *logger.Logger
	pluginManager *plugin.Manager
	server        *http.Server
	routes        map[string]*Route
}

// Route represents a route
type Route struct {
	Path        string
	Target      *url.URL
	Methods     map[string]bool
	Middlewares []middleware.Middleware
	Handler     http.Handler
}

// New creates a new gateway
func New(configPath string, logLevel logger.LogLevel) (*Gateway, error) {
	// Create a logger
	log := logger.New(logLevel)

	// Load the configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create a plugin manager
	pluginManager := plugin.NewManager(log)

	// Register plugins
	pluginManager.RegisterPlugin(plugin.NewCORSPlugin())
	pluginManager.RegisterPlugin(plugin.NewExamplePlugin())

	// Create a gateway
	g := &Gateway{
		config:        cfg,
		log:           log,
		pluginManager: pluginManager,
		routes:        make(map[string]*Route),
	}

	// Initialize the gateway
	if err := g.initialize(); err != nil {
		return nil, err
	}

	return g, nil
}

// initialize initializes the gateway
func (g *Gateway) initialize() error {
	// Initialize routes
	for _, routeConfig := range g.config.Routes {
		// Parse the target URL
		targetURL, err := url.Parse(routeConfig.Target)
		if err != nil {
			return fmt.Errorf("failed to parse target URL: %w", err)
		}

		// Create a route
		route := &Route{
			Path:    routeConfig.Path,
			Target:  targetURL,
			Methods: make(map[string]bool),
		}

		// Add allowed methods
		for _, method := range routeConfig.Methods {
			route.Methods[method] = true
		}

		// Create a reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Create a handler
		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the method is allowed
			if !route.Methods[r.Method] {
				g.log.Warn("Method not allowed: %s %s", r.Method, r.URL.Path)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Modify the request URL
			r.URL.Host = targetURL.Host
			r.URL.Scheme = targetURL.Scheme
			r.Host = targetURL.Host

			// Remove the route path prefix
			if strings.HasPrefix(r.URL.Path, route.Path) {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, route.Path)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}

			// Log the proxy request
			g.log.Debug("Proxying request: %s %s -> %s", r.Method, r.URL.Path, targetURL)

			// Proxy the request
			proxy.ServeHTTP(w, r)
		})

		// Add middlewares
		for _, middlewareName := range routeConfig.Middlewares {
			// Check if the middleware is a plugin
			if _, ok := g.pluginManager.GetPlugin(middlewareName); ok {
				handler = g.pluginManager.Middleware(middlewareName)(handler)
				continue
			}

			// Add built-in middlewares
			switch middlewareName {
			case "logging":
				handler = middleware.LoggingMiddleware(g.log)(handler)
			case "ratelimit":
				if routeConfig.RateLimit != nil {
					limiter := middleware.NewRateLimiter(
						routeConfig.RateLimit.Limit,
						time.Duration(routeConfig.RateLimit.Window)*time.Second,
						g.log,
					)
					handler = middleware.RateLimitMiddleware(limiter)(handler)
				}
			case "auth":
				if routeConfig.Auth != nil {
					var authenticator middleware.Authenticator
					switch routeConfig.Auth.Type {
					case "basic":
						authenticator = middleware.NewBasicAuthenticator(
							routeConfig.Auth.Config["username"],
							routeConfig.Auth.Config["password"],
							g.log,
						)
					case "apikey":
						authenticator = middleware.NewAPIKeyAuthenticator(
							routeConfig.Auth.Config["header"],
							routeConfig.Auth.Config["key"],
							g.log,
						)
					default:
						g.log.Warn("Unsupported auth type: %s", routeConfig.Auth.Type)
						continue
					}
					handler = middleware.AuthMiddleware(authenticator, g.log)(handler)
				}
			default:
				g.log.Warn("Unknown middleware: %s", middlewareName)
			}
		}

		// Set the handler
		route.Handler = handler

		// Add the route
		g.routes[route.Path] = route
		g.log.Info("Added route: %s -> %s", route.Path, route.Target)
	}

	return nil
}

// Start starts the gateway
func (g *Gateway) Start() error {
	// Create a mux
	mux := http.NewServeMux()

	// Add routes
	for _, route := range g.routes {
		mux.Handle(route.Path, route.Handler)
	}

	// Create a server
	g.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", g.config.Server.Host, g.config.Server.Port),
		Handler: mux,
	}

	// Start the server
	g.log.Info("Starting server on %s", g.server.Addr)
	return g.server.ListenAndServe()
}

// Stop stops the gateway
func (g *Gateway) Stop() error {
	if g.server != nil {
		g.log.Info("Stopping server")
		return g.server.Close()
	}
	return nil
}
