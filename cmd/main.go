package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/mstgnz/goteway/pkg/gateway"
	"github.com/mstgnz/goteway/pkg/logger"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.json", "Path to the configuration file")
	logLevelFlag := flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal)")
	flag.Parse()

	// Determine the log level
	var logLevel logger.LogLevel
	switch *logLevelFlag {
	case "debug":
		logLevel = logger.DEBUG
	case "info":
		logLevel = logger.INFO
	case "warn":
		logLevel = logger.WARN
	case "error":
		logLevel = logger.ERROR
	case "fatal":
		logLevel = logger.FATAL
	default:
		logLevel = logger.INFO
	}

	// Create a logger
	log := logger.New(logLevel)

	// Create a gateway
	gw, err := gateway.New(*configPath, logLevel)
	if err != nil {
		log.Fatal("Failed to create gateway: %v", err)
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the gateway in a goroutine
	go func() {
		if err := gw.Start(); err != nil {
			log.Fatal("Failed to start gateway: %v", err)
		}
	}()

	log.Info("Gateway started. Press Ctrl+C to stop.")

	// Wait for a signal
	<-sigChan
	log.Info("Shutting down...")

	// Stop the gateway
	if err := gw.Stop(); err != nil {
		log.Error("Failed to stop gateway: %v", err)
	}

	log.Info("Gateway stopped.")
}
