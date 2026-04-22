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
	configPath := flag.String("config", "config.json", "Path to the configuration file")
	logLevelFlag := flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal)")
	flag.Parse()

	var logLevel logger.LogLevel
	switch *logLevelFlag {
	case "debug":
		logLevel = logger.DEBUG
	case "warn":
		logLevel = logger.WARN
	case "error":
		logLevel = logger.ERROR
	case "fatal":
		logLevel = logger.FATAL
	default:
		logLevel = logger.INFO
	}

	log := logger.New(logLevel)

	gw, err := gateway.New(*configPath, logLevel)
	if err != nil {
		log.Fatal("Failed to create gateway: %v", err)
	}

	// SIGINT / SIGTERM → graceful shutdown
	// SIGHUP           → hot-reload config without downtime
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	errChan := make(chan error, 1)
	go func() {
		if err := gw.Start(); err != nil {
			errChan <- err
		}
	}()

	log.Info("Gateway started. SIGINT/SIGTERM to stop, SIGHUP to reload config.")

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				log.Info("SIGHUP received — reloading configuration")
				if err := gw.Reload(); err != nil {
					log.Error("Config reload failed: %v", err)
				}
			default:
				log.Info("Signal %s received, shutting down", sig)
				if err := gw.Stop(); err != nil {
					log.Error("Failed to stop gateway: %v", err)
					os.Exit(1)
				}
				log.Info("Gateway stopped.")
				return
			}
		case err := <-errChan:
			log.Error("Gateway error: %v", err)
			return
		}
	}
}
