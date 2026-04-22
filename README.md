# Goteway

A lightweight, production-ready API Gateway written in pure Go with zero external dependencies.

[![Go Report Card](https://goreportcard.com/badge/github.com/mstgnz/goteway)](https://goreportcard.com/report/github.com/mstgnz/goteway)
[![GoDoc](https://godoc.org/github.com/mstgnz/goteway?status.svg)](https://godoc.org/github.com/mstgnz/goteway)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

Goteway is a production-ready API Gateway for microservices architectures. It focuses on security, reliability, and observability while maintaining zero external dependencies. All features are implemented using the Go standard library.

## Features

- **Zero external dependencies:** only the Go standard library
- **Round-robin load balancing** across multiple backend targets
- **Circuit breaker:** Closed/Open/Half-Open state machine per route
- **Automatic retries** with response buffering (safe methods only)
- **Config hot-reload:** send SIGHUP, no downtime
- **Authentication:** Basic Auth, API Key, JWT (HS256)
- **Rate limiting:** sliding-window, per-IP, with background cleanup
- **CORS:** per-route origin allowlist with `Vary: Origin`
- **Request ID:** generates or forwards `X-Request-ID` for log correlation
- **Structured JSON logging,** compatible with any log aggregation stack
- **Prometheus-compatible metrics** at `/metrics`
- **Health check** at `/healthz`
- **Graceful shutdown:** in-flight requests drain before the process exits
- **TLS termination:** configure `certFile` / `keyFile`
- **Plugin system:** extend with custom middleware plugins
- **Environment variable expansion** in config (`${MY_SECRET}`)

## Installation

### From source

```bash
git clone https://github.com/mstgnz/goteway.git
cd goteway
make build
```

### Docker

```bash
docker build -t goteway .
docker run -p 8080:8080 -v $(pwd)/config.json:/app/config.json goteway
```

## Quick Start

1. Set credentials as environment variables:

```bash
export GATEWAY_ADMIN_USERNAME=admin
export GATEWAY_ADMIN_PASSWORD=supersecret
```

2. Create `config.json`:

```json
{
  "server": {
    "port": 8080,
    "host": "0.0.0.0"
  },
  "routes": [
    {
      "path": "/api/users",
      "targets": [
        "http://users-service-1:8081",
        "http://users-service-2:8082"
      ],
      "methods": ["GET", "POST", "PUT", "DELETE"],
      "middlewares": ["requestid", "logging", "ratelimit", "auth", "cors"],
      "rateLimit": { "limit": 100, "window": 60 },
      "auth": {
        "type": "basic",
        "config": {
          "username": "${GATEWAY_ADMIN_USERNAME}",
          "password": "${GATEWAY_ADMIN_PASSWORD}"
        }
      },
      "circuitBreaker": { "threshold": 5, "openTimeoutSeconds": 30 },
      "retry": { "count": 3, "waitMilliseconds": 100 }
    }
  ]
}
```

3. Start the gateway:

```bash
./goteway -config config.json -log-level info
```

4. Make a request:

```bash
curl -u admin:supersecret http://localhost:8080/api/users
```

## Configuration Reference

### Server

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `port` | int | Listening port | `8080` |
| `host` | string | Bind address | `"0.0.0.0"` |
| `maxBodyBytes` | int | Maximum request body size in bytes | `33554432` (32 MB) |

### TLS (optional)

```json
"tls": {
  "certFile": "/path/to/cert.pem",
  "keyFile": "/path/to/key.pem"
}
```

### Route

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `path` | string | URL prefix to match | Yes |
| `target` | string | Single backend URL | One of `target` or `targets` |
| `targets` | array | Multiple backend URLs (round-robin) | One of `target` or `targets` |
| `methods` | array | Allowed HTTP methods (case-insensitive) | Yes |
| `middlewares` | array | Ordered list of middlewares to apply | No |
| `rateLimit` | object | Rate limiting settings | No |
| `auth` | object | Authentication settings | No |
| `circuitBreaker` | object | Circuit breaker settings | No |
| `retry` | object | Retry settings | No |

### Rate Limiting

```json
"rateLimit": {
  "limit": 100,
  "window": 60
}
```

| Field | Type | Description |
|-------|------|-------------|
| `limit` | int | Max requests per window |
| `window` | int | Window duration in seconds |

### Authentication

#### Basic Auth

```json
"auth": {
  "type": "basic",
  "config": {
    "username": "${GATEWAY_USER}",
    "password": "${GATEWAY_PASS}"
  }
}
```

#### API Key

```json
"auth": {
  "type": "apikey",
  "config": {
    "header": "X-API-Key",
    "key": "${GATEWAY_API_KEY}"
  }
}
```

#### JWT (HS256)

```json
"auth": {
  "type": "jwt",
  "config": {
    "secret": "${GATEWAY_JWT_SECRET}"
  }
}
```

Validates the `Authorization: Bearer <token>` header. Checks signature (HMAC-SHA256) and `exp` claim.

### Circuit Breaker

```json
"circuitBreaker": {
  "threshold": 5,
  "openTimeoutSeconds": 30
}
```

| Field | Description |
|-------|-------------|
| `threshold` | Consecutive failures before opening the circuit |
| `openTimeoutSeconds` | Seconds to stay open before probing (half-open) |

State transitions: **Closed** (normal) → **Open** (rejecting) → **Half-Open** (probing) → **Closed**.

### Retry

Only applied to safe, idempotent methods: `GET`, `HEAD`, `DELETE`, `OPTIONS`. POST and PUT are never retried.

```json
"retry": {
  "count": 3,
  "waitMilliseconds": 100,
  "retryOnStatus": [502, 503, 504]
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `count` | Maximum retry attempts | (none) |
| `waitMilliseconds` | Delay between retries | `0` |
| `retryOnStatus` | HTTP status codes that trigger a retry | `[502, 503, 504]` |

Intermediate attempts are fully buffered in memory so a failed response is never partially flushed to the client.

### Plugin Config

Global plugin configuration lives under `"plugins"`:

```json
"plugins": {
  "cors": {
    "allowedOrigins": ["https://app.example.com"],
    "allowedMethods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    "allowedHeaders": ["Content-Type", "Authorization"]
  }
}
```

## Middlewares

Add middleware names to a route's `"middlewares"` array. Order matters; they are applied left to right (outermost first).

| Name | Description |
|------|-------------|
| `requestid` | Generates or forwards `X-Request-ID` for log correlation |
| `logging` | Logs method, path, status, and duration per request |
| `ratelimit` | Sliding-window rate limiter per client IP |
| `auth` | Authenticates via Basic Auth, API Key, or JWT |
| `cors` | Sets CORS headers; enforces origin allowlist |
| `example` | Sample plugin, adds `X-Example-Plugin` header |

Metrics are tracked automatically for every route regardless of which middlewares are enabled.

## Built-in Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Returns `{"status":"ok"}`, use for liveness probes |
| `GET /metrics` | Prometheus-compatible text format metrics |

### Available Metrics

```
goteway_requests_total{route, method, status}        counter
goteway_request_duration_seconds{route, method}      histogram
goteway_active_requests{route}                       gauge
goteway_circuit_breaker_state{route}                 gauge  # 0=closed 1=open 2=half-open
```

## Log Format

All log lines are JSON for easy ingestion by Loki, CloudWatch, Datadog, etc.:

```json
{"time":"2026-04-22T10:00:00Z","level":"INFO","msg":"Added route: /api/users -> [http://...]"}
{"time":"2026-04-22T10:00:01Z","level":"WARN","msg":"Circuit breaker \"/api/users\": open after 5 failure(s)"}
```

## Config Hot-Reload

Reload routes without restarting the process:

```bash
kill -HUP $(pgrep goteway)
```

In-flight requests complete against the old configuration. New requests are served by the updated routes immediately after the swap.

## Plugin System

Implement the `Plugin` interface to create custom middleware plugins:

```go
type Plugin interface {
    Name() string
    Initialize(config map[string]any, log *logger.Logger) error
    ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler)
}
```

Register your plugin in `gateway.go`:

```go
pluginManager.RegisterPlugin(mypkg.NewMyPlugin(), cfg.Plugins["myplugin"])
```

Then add it to any route's `"middlewares"` list in config.

## Security

- All credential comparisons use `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
- Credentials must be provided via environment variables. Never hardcode secrets in `config.json`
- HTTP server enforces `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `ReadHeaderTimeout` to prevent Slowloris attacks
- Request bodies are limited to `maxBodyBytes` (default 32 MB)
- CORS plugin sets `Vary: Origin` to prevent cache poisoning
- Graceful shutdown drains in-flight requests before the process exits; connections are never abruptly closed

## Project Structure

```
goteway/
├── cmd/
│   └── main.go                  # Entry point, signal handling
├── pkg/
│   ├── balancer/                # Round-robin load balancer
│   ├── circuitbreaker/          # Circuit breaker state machine
│   ├── config/                  # Config loading, validation, env expansion
│   ├── gateway/                 # Core: routing, proxy, retry, reload
│   ├── logger/                  # Structured JSON logger
│   ├── metrics/                 # Prometheus-compatible metrics registry
│   ├── middleware/              # auth, ratelimit, logging, requestid, cors
│   └── plugin/                  # Plugin interface and manager
├── config.json                  # Example configuration
├── Dockerfile
└── Makefile
```

## Development

```bash
make build    # Build binary
make test     # Run tests
make fmt      # Format code
make vet      # Vet code
make run      # Run with default config
make clean    # Remove build artifacts
```

## Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to config file | `config.json` |
| `-log-level` | Log level: debug, info, warn, error, fatal | `info` |

## License

MIT License. See [LICENSE](LICENSE) for details.
