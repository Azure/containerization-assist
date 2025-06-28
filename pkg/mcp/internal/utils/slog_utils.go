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
	// Simplified log capture - just store the text with minimal parsing
	logText := string(p)

	// Simple level detection
	level := "info"
	if strings.Contains(logText, "level=ERROR") {
		level = "error"
	} else if strings.Contains(logText, "level=WARN") {
		level = "warn"
	} else if strings.Contains(logText, "level=DEBUG") {
		level = "debug"
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   logText,
		Fields:    make(map[string]interface{}),
	}

	w.buffer.Add(entry)

	// Also write to the original writer
	return w.writer.Write(p)
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
