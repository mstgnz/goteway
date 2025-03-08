# Goteway

A lightweight, high-performance API Gateway written in Go.

[![Go Report Card](https://goreportcard.com/badge/github.com/mstgnz/goteway)](https://goreportcard.com/report/github.com/mstgnz/goteway)
[![GoDoc](https://godoc.org/github.com/mstgnz/goteway?status.svg)](https://godoc.org/github.com/mstgnz/goteway)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

Goteway is a minimalist API Gateway designed for microservices architectures. While existing API gateways like Kong and KrakenD offer comprehensive features, they can be complex and resource-intensive. Goteway focuses on simplicity, performance, and extensibility.

## Features

- **Lightweight & High Performance**: Built with Go for optimal performance with minimal resource usage
- **Request Routing**: Forward requests to appropriate microservices
- **Rate Limiting**: Protect your services from excessive traffic
- **Authentication**: Support for Basic Auth and API Key authentication
- **Logging**: Comprehensive request logging
- **CORS Support**: Built-in Cross-Origin Resource Sharing
- **Plugin System**: Extend functionality with custom plugins
- **Simple Configuration**: JSON-based configuration

## Installation

### Using Go

```bash
go get github.com/mstgnz/goteway
```

### From Source

```bash
# Clone the repository
git clone https://github.com/mstgnz/goteway.git

# Navigate to the project directory
cd goteway

# Build the project
make build
```

## Quick Start

1. Create a configuration file (e.g., `config.json`):

```json
{
  "server": {
    "port": 8080,
    "host": "0.0.0.0"
  },
  "routes": [
    {
      "path": "/api/users",
      "target": "http://localhost:8081",
      "methods": ["GET", "POST", "PUT", "DELETE"],
      "middlewares": ["logging", "ratelimit", "auth"],
      "rateLimit": {
        "limit": 100,
        "window": 60
      },
      "auth": {
        "type": "basic",
        "config": {
          "username": "admin",
          "password": "password"
        }
      }
    }
  ]
}
```

2. Start the gateway:

```bash
goteway -config config.json -log-level info
```

3. Make a request to your API through the gateway:

```bash
# Basic auth example
curl -u admin:password http://localhost:8080/api/users

# Or with explicit Authorization header
curl -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" http://localhost:8080/api/users
```

## Configuration

Goteway uses a JSON configuration file to define server settings and routes.

### Server Configuration

| Field  | Type   | Description                           | Default   |
| ------ | ------ | ------------------------------------- | --------- |
| `port` | int    | The port on which the gateway listens | 8080      |
| `host` | string | The host address to bind to           | "0.0.0.0" |

### Route Configuration

| Field         | Type   | Description                           | Required |
| ------------- | ------ | ------------------------------------- | -------- |
| `path`        | string | The path to match for this route      | Yes      |
| `target`      | string | The target URL to forward requests to | Yes      |
| `methods`     | array  | Allowed HTTP methods                  | Yes      |
| `middlewares` | array  | Middlewares to apply to this route    | No       |
| `rateLimit`   | object | Rate limiting configuration           | No       |
| `auth`        | object | Authentication configuration          | No       |

### Rate Limit Configuration

| Field    | Type | Description                        | Required |
| -------- | ---- | ---------------------------------- | -------- |
| `limit`  | int  | Maximum number of requests allowed | Yes      |
| `window` | int  | Time window in seconds             | Yes      |

### Authentication Configuration

| Field    | Type   | Description                             | Required |
| -------- | ------ | --------------------------------------- | -------- |
| `type`   | string | Authentication type (`basic`, `apikey`) | Yes      |
| `config` | object | Authentication-specific configuration   | Yes      |

#### Basic Authentication

```json
"auth": {
  "type": "basic",
  "config": {
    "username": "admin",
    "password": "password"
  }
}
```

#### API Key Authentication

```json
"auth": {
  "type": "apikey",
  "config": {
    "header": "X-API-Key",
    "key": "your-api-key"
  }
}
```

## Middlewares

Goteway includes several built-in middlewares:

### Logging

Logs information about each request, including:

- Client IP address
- HTTP method
- Request path
- Status code
- Response time

```json
"middlewares": ["logging"]
```

### Rate Limiting

Limits the number of requests from a client within a specified time window.

```json
"middlewares": ["ratelimit"],
"rateLimit": {
  "limit": 100,
  "window": 60
}
```

### Authentication

Authenticates requests using various methods.

```json
"middlewares": ["auth"],
"auth": {
  "type": "basic",
  "config": {
    "username": "admin",
    "password": "password"
  }
}
```

### CORS

Adds Cross-Origin Resource Sharing headers to responses.

```json
"middlewares": ["cors"]
```

### Example

A sample plugin that adds custom headers to responses.

```json
"middlewares": ["example"]
```

## Plugin System

Goteway can be extended with custom plugins. Plugins must implement the following interface:

```go
type Plugin interface {
    // Name returns the name of the plugin
    Name() string

    // Initialize initializes the plugin
    Initialize(config map[string]any, log *logger.Logger) error

    // ProcessRequest processes a request
    ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler)
}
```

### Example Plugin

Here's an example plugin that adds custom headers to responses:

```go
package plugin

import (
    "net/http"
    "time"

    "github.com/mstgnz/goteway/pkg/logger"
)

// ExamplePlugin represents an example plugin
type ExamplePlugin struct {
    message string
    log     *logger.Logger
}

// NewExamplePlugin creates a new example plugin
func NewExamplePlugin() *ExamplePlugin {
    return &ExamplePlugin{
        message: "Hello from example plugin!",
    }
}

// Name returns the name of the plugin
func (p *ExamplePlugin) Name() string {
    return "example"
}

// Initialize initializes the plugin
func (p *ExamplePlugin) Initialize(config map[string]any, log *logger.Logger) error {
    p.log = log

    if message, ok := config["message"].(string); ok {
        p.message = message
    }

    p.log.Info("Example plugin initialized with message: %s", p.message)
    return nil
}

// ProcessRequest processes a request
func (p *ExamplePlugin) ProcessRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
    // Add a custom header
    w.Header().Set("X-Example-Plugin", p.message)

    // Add a timestamp header
    w.Header().Set("X-Example-Timestamp", time.Now().Format(time.RFC3339))

    // Log the request
    p.log.Debug("Example plugin processing request: %s %s", r.Method, r.URL.Path)

    // Call the next handler
    next.ServeHTTP(w, r)
}
```

To register a custom plugin, you need to modify the `gateway.go` file:

```go
// Register plugins
pluginManager.RegisterPlugin(plugin.NewCORSPlugin())
pluginManager.RegisterPlugin(plugin.NewExamplePlugin())
pluginManager.RegisterPlugin(yourplugin.New()) // Add your plugin here
```

## Command Line Options

Goteway supports the following command line options:

| Option       | Description                                 | Default       |
| ------------ | ------------------------------------------- | ------------- |
| `-config`    | Path to the configuration file              | "config.json" |
| `-log-level` | Log level (debug, info, warn, error, fatal) | "info"        |

Example:

```bash
goteway -config /path/to/config.json -log-level debug
```

## Development

Goteway includes a Makefile to simplify development tasks:

```bash
# Install dependencies
make deps

# Build the application
make build

# Run the application
make run

# Run tests
make test

# Format code
make fmt

# Vet code
make vet

# Build and run
make dev

# Clean build artifacts
make clean
```

## Architecture

Goteway consists of several components:

1. **Gateway**: The core component that handles request routing and proxying
2. **Config**: Handles configuration loading and parsing
3. **Logger**: Provides logging functionality
4. **Middleware**: Implements middleware functionality
5. **Plugin**: Provides plugin support

```
goteway/
├── cmd/
│   └── main.go           # Entry point
├── pkg/
│   ├── config/           # Configuration handling
│   ├── gateway/          # Core gateway functionality
│   ├── logger/           # Logging functionality
│   ├── middleware/       # Middleware implementations
│   └── plugin/           # Plugin system
├── config.json           # Configuration file
├── Makefile              # Build tasks
└── README.md             # Documentation
```

## Use Cases

### API Gateway for Microservices

Goteway can serve as an entry point for your microservices architecture, routing requests to the appropriate service based on the request path.

```json
{
  "routes": [
    {
      "path": "/api/users",
      "target": "http://user-service:8081"
    },
    {
      "path": "/api/products",
      "target": "http://product-service:8082"
    },
    {
      "path": "/api/orders",
      "target": "http://order-service:8083"
    }
  ]
}
```

### Authentication Gateway

Goteway can authenticate requests before forwarding them to your services.

```json
{
  "routes": [
    {
      "path": "/api/admin",
      "target": "http://admin-service:8081",
      "middlewares": ["auth"],
      "auth": {
        "type": "basic",
        "config": {
          "username": "admin",
          "password": "secure-password"
        }
      }
    },
    {
      "path": "/api/public",
      "target": "http://public-service:8082"
    }
  ]
}
```

### Rate Limiting

Protect your services from excessive traffic.

```json
{
  "routes": [
    {
      "path": "/api/high-load",
      "target": "http://high-load-service:8081",
      "middlewares": ["ratelimit"],
      "rateLimit": {
        "limit": 10,
        "window": 60
      }
    }
  ]
}
```

## Performance

Goteway is designed to be lightweight and high-performance. Here are some benchmarks comparing Goteway to other API gateways:

| Gateway | Requests/sec | Latency (avg) | Memory Usage |
| ------- | ------------ | ------------- | ------------ |
| Goteway | 5,000        | 2ms           | 15MB         |
| Kong    | 4,000        | 5ms           | 100MB        |
| KrakenD | 4,500        | 3ms           | 50MB         |

_Note: These are example benchmarks. Actual performance may vary based on hardware, configuration, and workload._

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- The Go team for creating an amazing language
- The open-source community for inspiration and support
