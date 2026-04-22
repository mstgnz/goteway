package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the level of logging
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
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

// Logger writes structured JSON log lines.
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// New creates a new Logger that writes JSON to stdout.
func New(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

func (l *Logger) Debug(format string, v ...any) {
	if l.level <= DEBUG {
		l.log(DEBUG, format, v...)
	}
}

func (l *Logger) Info(format string, v ...any) {
	if l.level <= INFO {
		l.log(INFO, format, v...)
	}
}

func (l *Logger) Warn(format string, v ...any) {
	if l.level <= WARN {
		l.log(WARN, format, v...)
	}
}

func (l *Logger) Error(format string, v ...any) {
	if l.level <= ERROR {
		l.log(ERROR, format, v...)
	}
}

func (l *Logger) Fatal(format string, v ...any) {
	if l.level <= FATAL {
		l.log(FATAL, format, v...)
		exitFunc(1)
	}
}

type logEntry struct {
	Time  string `json:"time"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func (l *Logger) log(level LogLevel, format string, v ...any) {
	var msg string
	if len(v) > 0 {
		msg = fmt.Sprintf(format, v...)
	} else {
		msg = format
	}

	entry := logEntry{
		Time:  time.Now().UTC().Format(time.RFC3339),
		Level: levelNames[level],
		Msg:   msg,
	}
	data, _ := json.Marshal(entry)
	l.logger.Print(string(data))
}
