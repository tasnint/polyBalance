package internal

import (
	"log"
)

type Logger struct {
	prefix string
}

// NewLogger creates a logger with a prefix (e.g., "HEALTH", "SERVER", etc.)
func NewLogger(prefix string) *Logger {
	return &Logger{
		prefix: "[" + prefix + "] ",
	}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	log.Printf(l.prefix+msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	log.Printf(l.prefix+"ERROR: "+msg, args...)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	log.Printf(l.prefix+"DEBUG: "+msg, args...)
}
