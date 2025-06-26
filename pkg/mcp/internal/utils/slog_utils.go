package utils

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// MCPSlogConfig holds slog configuration for MCP components
type MCPSlogConfig struct {
	Level     slog.Level
	Component string
	AddSource bool
	Writer    io.Writer
}

// NewMCPSlogger creates a slog logger configured for MCP components
func NewMCPSlogger(config MCPSlogConfig) *slog.Logger {
	if config.Writer == nil {
		config.Writer = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     config.Level,
		AddSource: config.AddSource,
	}

	handler := slog.NewTextHandler(config.Writer, opts)
	logger := slog.New(handler)

	// Add component context if specified
	if config.Component != "" {
		logger = logger.With("component", config.Component)
	}

	return logger
}

// ParseSlogLevel converts a string level to slog.Level
func ParseSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// CreateMCPLoggerWithCapture creates an slog logger with log capture capability
func CreateMCPLoggerWithCapture(logBuffer *RingBuffer, output io.Writer, level slog.Level, component string) *slog.Logger {
	// Create a multi-writer that writes to both the output and captures logs
	captureWriter := NewLogCaptureWriterSlog(logBuffer, output)

	config := MCPSlogConfig{
		Level:     level,
		Component: component,
		AddSource: true,
		Writer:    captureWriter,
	}

	return NewMCPSlogger(config)
}

// LogCaptureWriterSlog captures slog output to a ring buffer
type LogCaptureWriterSlog struct {
	buffer *RingBuffer
	writer io.Writer
}

// NewLogCaptureWriterSlog creates a new slog log capture writer
func NewLogCaptureWriterSlog(buffer *RingBuffer, writer io.Writer) *LogCaptureWriterSlog {
	return &LogCaptureWriterSlog{
		buffer: buffer,
		writer: writer,
	}
}

// Write implements io.Writer and captures log entries
func (w *LogCaptureWriterSlog) Write(p []byte) (n int, err error) {
	// Parse the slog output and capture to buffer
	logText := string(p)
	entry := LogEntry{
		Timestamp: time.Now(), // parseTimestampFromSlog returns interface{}, use current time
		Level:     parseLevelFromSlog(logText),
		Message:   parseMessageFromSlog(logText),
		Fields:    parseFieldsFromSlog(logText),
	}

	w.buffer.Add(entry)

	// Also write to the original writer
	return w.writer.Write(p)
}

// Helper functions to parse slog text format
func parseTimestampFromSlog(logText string) interface{} {
	// Simple parsing - in practice you'd want more robust parsing
	return "now" // Simplified
}

func parseLevelFromSlog(logText string) string {
	if contains(logText, "level=ERROR") {
		return "error"
	}
	if contains(logText, "level=WARN") {
		return "warn"
	}
	if contains(logText, "level=INFO") {
		return "info"
	}
	if contains(logText, "level=DEBUG") {
		return "debug"
	}
	return "info"
}

func parseMessageFromSlog(logText string) string {
	// Extract message from slog text format - simplified implementation
	return logText
}

func parseFieldsFromSlog(logText string) map[string]interface{} {
	// Parse structured fields from slog text format - simplified implementation
	return make(map[string]interface{})
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Convenience functions for MCP logging
func InfoMCP(ctx context.Context, logger *slog.Logger, msg string, args ...any) {
	logger.InfoContext(ctx, msg, args...)
}

func WarnMCP(ctx context.Context, logger *slog.Logger, msg string, args ...any) {
	logger.WarnContext(ctx, msg, args...)
}

func ErrorMCP(ctx context.Context, logger *slog.Logger, msg string, args ...any) {
	logger.ErrorContext(ctx, msg, args...)
}

func DebugMCP(ctx context.Context, logger *slog.Logger, msg string, args ...any) {
	logger.DebugContext(ctx, msg, args...)
}
