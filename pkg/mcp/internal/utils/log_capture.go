package utils

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// LogCaptureHook is a zerolog hook that captures logs to a ring buffer
type LogCaptureHook struct {
	buffer *RingBuffer
}

// NewLogCaptureHook creates a new log capture hook
func NewLogCaptureHook(capacity int) *LogCaptureHook {
	return &LogCaptureHook{
		buffer: NewRingBuffer(capacity),
	}
}

// Run implements zerolog.Hook interface
func (h *LogCaptureHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// Extract fields from the event (this is a bit hacky but zerolog doesn't expose fields directly)
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Message:   msg,
		Fields:    make(map[string]interface{}),
	}

	h.buffer.Add(entry)
}

// GetBuffer returns the underlying ring buffer
func (h *LogCaptureHook) GetBuffer() *RingBuffer {
	return h.buffer
}

// LogCaptureWriter is an io.Writer that captures structured logs
type LogCaptureWriter struct {
	buffer *RingBuffer
	writer io.Writer // Original writer to pass through
}

// NewLogCaptureWriter creates a new log capture writer
func NewLogCaptureWriter(buffer *RingBuffer, writer io.Writer) *LogCaptureWriter {
	return &LogCaptureWriter{
		buffer: buffer,
		writer: writer,
	}
}

// Write implements io.Writer interface
func (w *LogCaptureWriter) Write(p []byte) (n int, err error) {
	// Pass through to original writer first
	if w.writer != nil {
		n, err = w.writer.Write(p)
		if err != nil {
			return n, err
		}
	} else {
		n = len(p)
	}

	// Parse the log line and capture it
	line := string(p)
	entry := parseZerologLine(line)
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	w.buffer.Add(entry)

	return n, nil
}

// parseZerologLine attempts to parse a zerolog formatted line
func parseZerologLine(line string) LogEntry {
	entry := LogEntry{
		Fields: make(map[string]interface{}),
	}

	// Simple parsing - in production, you'd want more robust parsing
	parts := strings.Fields(line)
	if len(parts) == 0 {
		entry.Message = line
		return entry
	}

	// Look for common patterns
	for i, part := range parts {
		// Level detection
		if isLogLevel(part) {
			entry.Level = strings.ToLower(part)
			continue
		}

		// Time detection (ISO format)
		if strings.Contains(part, "T") && strings.Contains(part, ":") {
			if t, err := time.Parse(time.RFC3339, part); err == nil {
				entry.Timestamp = t
				continue
			}
		}

		// Key=value pairs
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				entry.Fields[kv[0]] = strings.Trim(kv[1], "\"")
				continue
			}
		}

		// Caller detection
		if strings.Contains(part, ".go:") {
			entry.Caller = part
			continue
		}

		// Everything else is part of the message
		if i > 0 && entry.Level != "" {
			// Join remaining parts as message
			entry.Message = strings.Join(parts[i:], " ")
			break
		}
	}

	// Clean up message
	entry.Message = strings.TrimSpace(entry.Message)

	return entry
}

// isLogLevel checks if a string is a log level
func isLogLevel(s string) bool {
	levels := []string{
		"TRC", "DBG", "INF", "WRN", "ERR", "FTL", "PNC",
		"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "PANIC",
	}

	upper := strings.ToUpper(strings.TrimSpace(s))
	for _, level := range levels {
		if upper == level {
			return true
		}
	}
	return false
}

// GlobalLogCapture is a global instance for capturing logs
var GlobalLogCapture *LogCaptureHook

// InitializeLogCapture sets up global log capture
func InitializeLogCapture(capacity int) *LogCaptureHook {
	if GlobalLogCapture == nil {
		GlobalLogCapture = NewLogCaptureHook(capacity)
	}
	return GlobalLogCapture
}

// GetGlobalLogBuffer returns the global log buffer
func GetGlobalLogBuffer() *RingBuffer {
	if GlobalLogCapture != nil {
		return GlobalLogCapture.GetBuffer()
	}
	return nil
}

// CreateCaptureLogger creates a logger that captures to a buffer
func CreateCaptureLogger(buffer *RingBuffer, originalWriter io.Writer) zerolog.Logger {
	captureWriter := NewLogCaptureWriter(buffer, originalWriter)
	return zerolog.New(captureWriter).With().Timestamp().Logger()
}

// LoggerWithCapture wraps an existing logger to capture logs
func LoggerWithCapture(logger zerolog.Logger, buffer *RingBuffer) zerolog.Logger {
	// This is a simplified approach - in production you'd want to properly
	// hook into the logger's output
	return logger.Output(NewLogCaptureWriter(buffer, logger))
}

// FormatLogEntry formats a log entry for display
func FormatLogEntry(entry LogEntry) string {
	// Format: [TIMESTAMP] LEVEL MESSAGE fields...
	var parts []string

	parts = append(parts, fmt.Sprintf("[%s]", entry.Timestamp.Format("2006-01-02 15:04:05.000")))
	parts = append(parts, strings.ToUpper(entry.Level))

	if entry.Message != "" {
		parts = append(parts, entry.Message)
	}

	// Add fields
	for k, v := range entry.Fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	if entry.Caller != "" {
		parts = append(parts, fmt.Sprintf("caller=%s", entry.Caller))
	}

	return strings.Join(parts, " ")
}
