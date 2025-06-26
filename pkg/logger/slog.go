package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// SlogConfig holds configuration for structured logging
type SlogConfig struct {
	Level     slog.Level
	Format    string // "json" or "text"
	AddSource bool
	Stdout    io.Writer
	Stderr    io.Writer
}

// DefaultSlogConfig returns a sensible default configuration
func DefaultSlogConfig() SlogConfig {
	return SlogConfig{
		Level:     slog.LevelInfo,
		Format:    "text",
		AddSource: false,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}
}

// NewSlogLogger creates a new structured logger with level-based output routing
func NewSlogLogger(config SlogConfig) *slog.Logger {
	// Create a level-aware writer that routes to stdout/stderr appropriately
	writer := &LevelAwareWriter{
		Stdout: config.Stdout,
		Stderr: config.Stderr,
	}

	opts := &slog.HandlerOptions{
		Level:     config.Level,
		AddSource: config.AddSource,
	}

	var handler slog.Handler
	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	return slog.New(handler)
}

// LevelAwareWriter routes log messages to stdout or stderr based on level
type LevelAwareWriter struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (w *LevelAwareWriter) Write(p []byte) (n int, err error) {
	// Parse the log level from the message to determine routing
	// For simplicity, we'll route ERROR and higher to stderr, others to stdout
	msg := string(p)

	// Simple heuristic: if the message contains "level=ERROR" or "level=WARN", route to stderr
	if containsLogLevel(msg, "ERROR", "FATAL", "PANIC") {
		return w.Stderr.Write(p)
	}

	return w.Stdout.Write(p)
}

// containsLogLevel checks if the message contains any of the specified log levels
func containsLogLevel(msg string, levels ...string) bool {
	for _, level := range levels {
		if containsLevel(msg, level) {
			return true
		}
	}
	return false
}

func containsLevel(msg, level string) bool {
	// Check both slog text format and JSON format patterns
	textPattern := "level=" + level
	jsonPattern := `"level":"` + level + `"`
	return contains(msg, textPattern) || contains(msg, jsonPattern)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Global structured logger instance
var globalSlogger *slog.Logger

// InitGlobalSlogger initializes the global structured logger
func InitGlobalSlogger(config SlogConfig) {
	globalSlogger = NewSlogLogger(config)
	slog.SetDefault(globalSlogger)
}

// GetGlobalSlogger returns the global structured logger
func GetGlobalSlogger() *slog.Logger {
	if globalSlogger == nil {
		InitGlobalSlogger(DefaultSlogConfig())
	}
	return globalSlogger
}

// Convenience functions for structured logging
func InfoS(ctx context.Context, msg string, args ...any) {
	GetGlobalSlogger().InfoContext(ctx, msg, args...)
}

func WarnS(ctx context.Context, msg string, args ...any) {
	GetGlobalSlogger().WarnContext(ctx, msg, args...)
}

func ErrorS(ctx context.Context, msg string, args ...any) {
	GetGlobalSlogger().ErrorContext(ctx, msg, args...)
}

func DebugS(ctx context.Context, msg string, args ...any) {
	GetGlobalSlogger().DebugContext(ctx, msg, args...)
}
