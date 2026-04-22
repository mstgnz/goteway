package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/goteway/pkg/balancer"
	"github.com/mstgnz/goteway/pkg/circuitbreaker"
	"github.com/mstgnz/goteway/pkg/config"
	"github.com/mstgnz/goteway/pkg/logger"
	"github.com/mstgnz/goteway/pkg/metrics"
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
	rateLimiters  []*middleware.RateLimiter
	metrics       *gatewayMetrics
}

type gatewayMetrics struct {
	requestsTotal   *metrics.Counter
	requestDuration *metrics.Histogram
	activeRequests  *metrics.Gauge
	cbState         *metrics.Gauge
}

// Route represents a route
type Route struct {
	Path    string
	Targets []*url.URL
	Methods map[string]bool
	Handler http.Handler
}

// New creates a new gateway
func New(configPath string, logLevel logger.LogLevel) (*Gateway, error) {
	log := logger.New(logLevel)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	pluginManager := plugin.NewManager(log)
	pluginManager.RegisterPlugin(plugin.NewCORSPlugin(), cfg.Plugins["cors"])
	pluginManager.RegisterPlugin(plugin.NewExamplePlugin(), cfg.Plugins["example"])

	reg := metrics.DefaultRegistry
	m := &gatewayMetrics{
		requestsTotal: reg.NewCounter(
			"goteway_requests_total",
			"Total number of proxied requests.",
			"route", "method", "status",
		),
		requestDuration: reg.NewHistogram(
			"goteway_request_duration_seconds",
			"Request duration in seconds.",
			"route", "method",
		),
		activeRequests: reg.NewGauge(
			"goteway_active_requests",
			"Number of currently active requests.",
			"route",
		),
		cbState: reg.NewGauge(
			"goteway_circuit_breaker_state",
			"Circuit breaker state: 0=closed 1=open 2=half-open.",
			"route",
		),
	}

	g := &Gateway{
		config:        cfg,
		log:           log,
		pluginManager: pluginManager,
		routes:        make(map[string]*Route),
		metrics:       m,
	}

	if err := g.initialize(); err != nil {
		return nil, err
	}

	return g, nil
}

// initialize sets up all routes from the configuration.
func (g *Gateway) initialize() error {
	for _, routeConfig := range g.config.Routes {
		targets := make([]*url.URL, 0, len(routeConfig.Targets))
		for _, t := range routeConfig.Targets {
			u, err := url.Parse(t)
			if err != nil {
				return fmt.Errorf("failed to parse target URL %q: %w", t, err)
			}
			targets = append(targets, u)
		}

		lb := balancer.New(targets)

		route := &Route{
			Path:    routeConfig.Path,
			Targets: targets,
			Methods: make(map[string]bool),
		}
		for _, method := range routeConfig.Methods {
			route.Methods[method] = true
		}

		// Optionally configure circuit breaker for this route
		var cb *circuitbreaker.CircuitBreaker
		if routeConfig.CircuitBreaker != nil {
			cb = circuitbreaker.New(
				route.Path,
				routeConfig.CircuitBreaker.Threshold,
				time.Duration(routeConfig.CircuitBreaker.OpenTimeoutSeconds)*time.Second,
				g.log,
			)
			g.metrics.cbState.Set(0, route.Path) // start closed
		}

		maxBodyBytes := g.config.Server.MaxBodyBytes
		gm := g.metrics
		routePath := route.Path

		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !route.Methods[r.Method] {
				g.log.Warn("Method not allowed: %s %s", r.Method, r.URL.Path)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Circuit breaker check
			if cb != nil {
				if !cb.Allow() {
					state := int64(cb.State())
					gm.cbState.Set(state, routePath)
					http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
					return
				}
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

			proxy, targetURL := lb.Next()

			r.URL.Host = targetURL.Host
			r.URL.Scheme = targetURL.Scheme
			r.Host = targetURL.Host

			if after, ok := strings.CutPrefix(r.URL.Path, route.Path); ok {
				r.URL.Path = after
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}

			g.log.Debug("Proxying: %s %s -> %s", r.Method, r.URL.Path, targetURL)

			// Capture status for circuit breaker and metrics
			sw := &statusCapture{ResponseWriter: w, status: http.StatusOK}
			proxy.ServeHTTP(sw, r)

			if cb != nil {
				if sw.status >= 500 {
					cb.RecordFailure()
				} else {
					cb.RecordSuccess()
				}
				gm.cbState.Set(int64(cb.State()), routePath)
			}
		})

		// Apply configured middlewares
		for _, name := range routeConfig.Middlewares {
			if _, ok := g.pluginManager.GetPlugin(name); ok {
				handler = g.pluginManager.Middleware(name)(handler)
				continue
			}
			switch name {
			case "logging":
				handler = middleware.LoggingMiddleware(g.log)(handler)
			case "ratelimit":
				if routeConfig.RateLimit != nil {
					limiter := middleware.NewRateLimiter(
						routeConfig.RateLimit.Limit,
						time.Duration(routeConfig.RateLimit.Window)*time.Second,
						g.log,
					)
					g.rateLimiters = append(g.rateLimiters, limiter)
					handler = middleware.RateLimitMiddleware(limiter)(handler)
				}
			case "auth":
				if routeConfig.Auth != nil {
					var auth middleware.Authenticator
					switch routeConfig.Auth.Type {
					case "basic":
						auth = middleware.NewBasicAuthenticator(
							routeConfig.Auth.Config["username"],
							routeConfig.Auth.Config["password"],
							g.log,
						)
					case "apikey":
						auth = middleware.NewAPIKeyAuthenticator(
							routeConfig.Auth.Config["header"],
							routeConfig.Auth.Config["key"],
							g.log,
						)
					case "jwt":
						auth = middleware.NewJWTAuthenticator(
							routeConfig.Auth.Config["secret"],
							g.log,
						)
					default:
						g.log.Warn("Unsupported auth type: %s", routeConfig.Auth.Type)
						continue
					}
					handler = middleware.AuthMiddleware(auth, g.log)(handler)
				}
			default:
				g.log.Warn("Unknown middleware: %s", name)
			}
		}

		// Outermost layer: automatic metrics tracking for every route
		innerHandler := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			gm.activeRequests.Inc(routePath)
			defer gm.activeRequests.Dec(routePath)

			sw := &statusCapture{ResponseWriter: w, status: http.StatusOK}
			innerHandler.ServeHTTP(sw, r)

			elapsed := time.Since(start).Seconds()
			status := strconv.Itoa(sw.status)
			gm.requestsTotal.Inc(routePath, r.Method, status)
			gm.requestDuration.Observe(elapsed, routePath, r.Method)
		})

		route.Handler = handler
		g.routes[route.Path] = route
		g.log.Info("Added route: %s -> %v", route.Path, routeConfig.Targets)
	}

	return nil
}

// Start starts the gateway
func (g *Gateway) Start() error {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus-compatible metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics.DefaultRegistry.WriteText(w)
	})

	// Register routes under exact and subtree patterns for Go 1.22+ compatibility
	for _, route := range g.routes {
		mux.Handle(route.Path, route.Handler)
		if !strings.HasSuffix(route.Path, "/") {
			mux.Handle(route.Path+"/", route.Handler)
		}
	}

	g.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", g.config.Server.Host, g.config.Server.Port),
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	g.log.Info("Starting server on %s", g.server.Addr)

	if g.config.TLS != nil {
		return g.server.ListenAndServeTLS(g.config.TLS.CertFile, g.config.TLS.KeyFile)
	}
	return g.server.ListenAndServe()
}

// Stop drains in-flight requests and shuts down the server gracefully.
func (g *Gateway) Stop() error {
	for _, rl := range g.rateLimiters {
		rl.Stop()
	}
	if g.server != nil {
		g.log.Info("Stopping server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return g.server.Shutdown(ctx)
	}
	return nil
}

// statusCapture wraps http.ResponseWriter to record the written status code.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.status = code
	sc.ResponseWriter.WriteHeader(code)
}
