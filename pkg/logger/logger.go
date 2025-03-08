package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the level of logging
type LogLevel int

const (
	// DEBUG level
	DEBUG LogLevel = iota
	// INFO level
	INFO
	// WARN level
	WARN
	// ERROR level
	ERROR
	// FATAL level
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// For testing
var exitFunc = os.Exit

// Logger represents a logger
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// New creates a new logger
func New(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...any) {
	if l.level <= DEBUG {
		l.log(DEBUG, format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...any) {
	if l.level <= INFO {
		l.log(INFO, format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...any) {
	if l.level <= WARN {
		l.log(WARN, format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...any) {
	if l.level <= ERROR {
		l.log(ERROR, format, v...)
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, v ...any) {
	if l.level <= FATAL {
		l.log(FATAL, format, v...)
		exitFunc(1)
	}
}

// log logs a message with the given level
func (l *Logger) log(level LogLevel, format string, v ...any) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	message := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] [%s] %s", timestamp, levelName, message)
}
