package logger

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	// Test log level constants
	if DEBUG != 0 {
		t.Errorf("DEBUG = %v, want %v", DEBUG, 0)
	}
	if INFO != 1 {
		t.Errorf("INFO = %v, want %v", INFO, 1)
	}
	if WARN != 2 {
		t.Errorf("WARN = %v, want %v", WARN, 2)
	}
	if ERROR != 3 {
		t.Errorf("ERROR = %v, want %v", ERROR, 3)
	}
	if FATAL != 4 {
		t.Errorf("FATAL = %v, want %v", FATAL, 4)
	}

	// Test level names
	if levelNames[DEBUG] != "DEBUG" {
		t.Errorf("levelNames[DEBUG] = %v, want %v", levelNames[DEBUG], "DEBUG")
	}
	if levelNames[INFO] != "INFO" {
		t.Errorf("levelNames[INFO] = %v, want %v", levelNames[INFO], "INFO")
	}
	if levelNames[WARN] != "WARN" {
		t.Errorf("levelNames[WARN] = %v, want %v", levelNames[WARN], "WARN")
	}
	if levelNames[ERROR] != "ERROR" {
		t.Errorf("levelNames[ERROR] = %v, want %v", levelNames[ERROR], "ERROR")
	}
	if levelNames[FATAL] != "FATAL" {
		t.Errorf("levelNames[FATAL] = %v, want %v", levelNames[FATAL], "FATAL")
	}
}

func TestNew(t *testing.T) {
	logger := New(INFO)
	if logger == nil {
		t.Error("New() returned nil")
	}
	if logger.level != INFO {
		t.Errorf("logger.level = %v, want %v", logger.level, INFO)
	}
	if logger.logger == nil {
		t.Error("logger.logger is nil")
	}
}

func TestLogMethods(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	origLogger := log.New(&buf, "", 0)

	// Test cases
	tests := []struct {
		name      string
		level     LogLevel
		logFunc   func(*Logger, string, ...any)
		message   string
		args      []any
		wantLevel string
		shouldLog bool
	}{
		{
			name:      "debug at debug level",
			level:     DEBUG,
			logFunc:   func(l *Logger, f string, v ...any) { l.Debug(f, v...) },
			message:   "test message %s",
			args:      []any{"arg"},
			wantLevel: "DEBUG",
			shouldLog: true,
		},
		{
			name:      "debug at info level",
			level:     INFO,
			logFunc:   func(l *Logger, f string, v ...any) { l.Debug(f, v...) },
			message:   "test message",
			args:      []any{},
			wantLevel: "DEBUG",
			shouldLog: false,
		},
		{
			name:      "info at info level",
			level:     INFO,
			logFunc:   func(l *Logger, f string, v ...any) { l.Info(f, v...) },
			message:   "test message",
			args:      []any{},
			wantLevel: "INFO",
			shouldLog: true,
		},
		{
			name:      "warn at warn level",
			level:     WARN,
			logFunc:   func(l *Logger, f string, v ...any) { l.Warn(f, v...) },
			message:   "test message",
			args:      []any{},
			wantLevel: "WARN",
			shouldLog: true,
		},
		{
			name:      "error at error level",
			level:     ERROR,
			logFunc:   func(l *Logger, f string, v ...any) { l.Error(f, v...) },
			message:   "test message",
			args:      []any{},
			wantLevel: "ERROR",
			shouldLog: true,
		},
		{
			name:      "info at error level",
			level:     ERROR,
			logFunc:   func(l *Logger, f string, v ...any) { l.Info(f, v...) },
			message:   "test message",
			args:      []any{},
			wantLevel: "INFO",
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear buffer
			buf.Reset()

			// Create logger with custom level
			logger := &Logger{
				level:  tt.level,
				logger: origLogger,
			}

			// Call log method
			tt.logFunc(logger, tt.message, tt.args...)

			// Check output
			output := buf.String()
			if tt.shouldLog {
				if !strings.Contains(output, tt.wantLevel) {
					t.Errorf("Log output does not contain level %q: %q", tt.wantLevel, output)
				}
				if !strings.Contains(output, "test message") {
					t.Errorf("Log output does not contain message: %q", output)
				}
				if len(tt.args) > 0 && !strings.Contains(output, "test message arg") {
					t.Errorf("Log output does not contain formatted message: %q", output)
				}
			} else {
				if output != "" {
					t.Errorf("Expected no log output, got: %q", output)
				}
			}
		})
	}
}

// TestFatal tests the Fatal method without actually exiting
func TestFatal(t *testing.T) {
	// Save original exitFunc and restore it after the test
	origExit := exitFunc
	defer func() { exitFunc = origExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
		// Don't actually exit
	}

	// Capture log output
	var buf bytes.Buffer
	origLogger := log.New(&buf, "", 0)

	// Create logger
	logger := &Logger{
		level:  FATAL,
		logger: origLogger,
	}

	// Call Fatal
	logger.Fatal("fatal message")

	// Check output
	output := buf.String()
	if !strings.Contains(output, "FATAL") {
		t.Errorf("Log output does not contain level FATAL: %q", output)
	}
	if !strings.Contains(output, "fatal message") {
		t.Errorf("Log output does not contain message: %q", output)
	}

	// Check exit code
	if exitCode != 1 {
		t.Errorf("Exit code = %v, want %v", exitCode, 1)
	}
}
