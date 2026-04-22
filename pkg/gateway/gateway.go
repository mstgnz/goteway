package gateway

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	configPath    string
	config        *config.Config
	log           *logger.Logger
	pluginManager *plugin.Manager
	server        *http.Server
	mux           atomic.Pointer[http.ServeMux] // swapped atomically on reload
	mu            sync.Mutex                    // serialises reload
	routes        map[string]*Route
	rateLimiters  []*middleware.RateLimiter
	gm            *gatewayMetrics
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
	gm := &gatewayMetrics{
		requestsTotal: reg.NewCounter(
			"goteway_requests_total",
			"Total proxied requests.",
			"route", "method", "status",
		),
		requestDuration: reg.NewHistogram(
			"goteway_request_duration_seconds",
			"Request duration in seconds.",
			"route", "method",
		),
		activeRequests: reg.NewGauge(
			"goteway_active_requests",
			"Currently active requests.",
			"route",
		),
		cbState: reg.NewGauge(
			"goteway_circuit_breaker_state",
			"Circuit breaker state: 0=closed 1=open 2=half-open.",
			"route",
		),
	}

	g := &Gateway{
		configPath:    configPath,
		config:        cfg,
		log:           log,
		pluginManager: pluginManager,
		routes:        make(map[string]*Route),
		gm:            gm,
	}

	if err := g.initialize(); err != nil {
		return nil, err
	}
	g.mux.Store(g.buildMux())

	return g, nil
}

// initialize sets up all routes from g.config. Must be called with g.mu held
// (or before the server starts).
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

		// Per-route circuit breaker
		var cb *circuitbreaker.CircuitBreaker
		if routeConfig.CircuitBreaker != nil {
			cb = circuitbreaker.New(
				route.Path,
				routeConfig.CircuitBreaker.Threshold,
				time.Duration(routeConfig.CircuitBreaker.OpenTimeoutSeconds)*time.Second,
				g.log,
			)
			g.gm.cbState.Set(0, route.Path)
		}

		// Build retry configuration (defaults when not set)
		retryCount := 0
		retryWait := time.Duration(0)
		retryOn := map[int]bool{502: true, 503: true, 504: true}
		if routeConfig.Retry != nil {
			retryCount = routeConfig.Retry.Count
			retryWait = time.Duration(routeConfig.Retry.WaitMilliseconds) * time.Millisecond
			if len(routeConfig.Retry.RetryOnStatus) > 0 {
				retryOn = make(map[int]bool, len(routeConfig.Retry.RetryOnStatus))
				for _, s := range routeConfig.Retry.RetryOnStatus {
					retryOn[s] = true
				}
			}
		}

		maxBodyBytes := g.config.Server.MaxBodyBytes
		gm := g.gm
		routePath := route.Path
		log := g.log

		// Safe HTTP methods — retry is only allowed for these to avoid
		// re-sending a non-idempotent request body.
		safeMethods := map[string]bool{
			http.MethodGet:     true,
			http.MethodHead:    true,
			http.MethodDelete:  true,
			http.MethodOptions: true,
		}

		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !route.Methods[r.Method] {
				log.Warn("Method not allowed: %s %s", r.Method, r.URL.Path)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			if cb != nil && !cb.Allow() {
				gm.cbState.Set(int64(cb.State()), routePath)
				http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

			// Strip route prefix once before the retry loop
			if after, ok := strings.CutPrefix(r.URL.Path, route.Path); ok {
				r.URL.Path = after
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}

			// Determine effective retry count (safe methods only)
			effective := 0
			if retryCount > 0 && safeMethods[r.Method] {
				effective = retryCount
			}
			maxAttempts := effective + 1

			var finalStatus int

			for attempt := 0; attempt < maxAttempts; attempt++ {
				if attempt > 0 {
					if retryWait > 0 {
						time.Sleep(retryWait)
					}
					log.Warn("Retry %d/%d: %s %s", attempt, effective, r.Method, r.URL.Path)
				}

				proxy, targetURL := lb.Next()
				r.URL.Host = targetURL.Host
				r.URL.Scheme = targetURL.Scheme
				r.Host = targetURL.Host

				isLast := attempt == maxAttempts-1

				if isLast {
					// Final attempt — write directly so streaming works
					sc := &statusCapture{ResponseWriter: w, status: http.StatusOK}
					proxy.ServeHTTP(sc, r)
					finalStatus = sc.status
				} else {
					// Intermediate attempt — buffer so we can retry on failure
					buf := newResponseBuffer()
					proxy.ServeHTTP(buf, r)
					finalStatus = buf.status
					if !retryOn[finalStatus] {
						buf.copyTo(w) // success: flush buffer
						break
					}
					// Failure: discard buffer and retry
				}

				if !retryOn[finalStatus] {
					break
				}
			}

			if cb != nil {
				if finalStatus >= 500 {
					cb.RecordFailure()
				} else {
					cb.RecordSuccess()
				}
				gm.cbState.Set(int64(cb.State()), routePath)
			}
		})

		// Apply configured middlewares (innermost to outermost)
		for _, name := range routeConfig.Middlewares {
			if _, ok := g.pluginManager.GetPlugin(name); ok {
				handler = g.pluginManager.Middleware(name)(handler)
				continue
			}
			switch name {
			case "logging":
				handler = middleware.LoggingMiddleware(g.log)(handler)
			case "requestid":
				handler = middleware.RequestIDMiddleware()(handler)
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

		// Outermost: automatic metrics tracking for every route
		inner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			gm.activeRequests.Inc(routePath)
			defer gm.activeRequests.Dec(routePath)

			sw := &statusCapture{ResponseWriter: w, status: http.StatusOK}
			inner.ServeHTTP(sw, r)

			gm.requestsTotal.Inc(routePath, r.Method, strconv.Itoa(sw.status))
			gm.requestDuration.Observe(time.Since(start).Seconds(), routePath, r.Method)
		})

		route.Handler = handler
		g.routes[route.Path] = route
		g.log.Info("Added route: %s -> %v", route.Path, routeConfig.Targets)
	}

	return nil
}

// buildMux constructs a new ServeMux from the current routes.
func (g *Gateway) buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics.DefaultRegistry.WriteText(w)
	})

	for _, route := range g.routes {
		mux.Handle(route.Path, route.Handler)
		if !strings.HasSuffix(route.Path, "/") {
			mux.Handle(route.Path+"/", route.Handler)
		}
	}

	return mux
}

// Reload re-reads the config file and atomically swaps the route mux.
// In-flight requests finish against the old mux; new requests use the new one.
func (g *Gateway) Reload() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	cfg, err := config.LoadConfig(g.configPath)
	if err != nil {
		return fmt.Errorf("reload: failed to load config: %w", err)
	}

	// Stop old rate limiters before rebuilding
	for _, rl := range g.rateLimiters {
		rl.Stop()
	}
	g.rateLimiters = nil
	g.routes = make(map[string]*Route)
	g.config = cfg

	if err := g.initialize(); err != nil {
		return fmt.Errorf("reload: failed to initialize routes: %w", err)
	}

	g.mux.Store(g.buildMux())
	g.log.Info("Configuration reloaded: %d route(s) active", len(g.routes))
	return nil
}

// Start starts the gateway
func (g *Gateway) Start() error {
	g.server = &http.Server{
		Addr: fmt.Sprintf("%s:%d", g.config.Server.Host, g.config.Server.Port),
		// Delegate to the atomically swapped mux so Reload() takes effect immediately.
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.mux.Load().ServeHTTP(w, r)
		}),
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

// --- helpers ------------------------------------------------------------------

// statusCapture wraps http.ResponseWriter to record the written status code.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.status = code
	sc.ResponseWriter.WriteHeader(code)
}

// responseBuffer is an in-memory ResponseWriter used for buffering retry
// attempts so that a failed response is never partially flushed to the client.
type responseBuffer struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newResponseBuffer() *responseBuffer {
	return &responseBuffer{header: make(http.Header), status: http.StatusOK}
}

func (rb *responseBuffer) Header() http.Header         { return rb.header }
func (rb *responseBuffer) WriteHeader(code int)        { rb.status = code }
func (rb *responseBuffer) Write(b []byte) (int, error) { return rb.body.Write(b) }

func (rb *responseBuffer) copyTo(w http.ResponseWriter) {
	for k, vs := range rb.header {
		w.Header()[k] = vs
	}
	w.WriteHeader(rb.status)
	rb.body.WriteTo(w)
}
