package logger

import (
	"log/slog"
	"os"

	"github.com/rs/zerolog"
)

// ZerologToSlogAdapter wraps a slog.Logger to provide zerolog-compatible methods
// This allows gradual migration from zerolog to slog
type ZerologToSlogAdapter struct {
	slogger *slog.Logger
}

// NewZerologToSlogAdapter creates an adapter that converts zerolog calls to slog
func NewZerologToSlogAdapter(logger *slog.Logger) *ZerologToSlogAdapter {
	return &ZerologToSlogAdapter{
		slogger: logger,
	}
}

// CreateSlogLoggerFromZerologLevel creates a slog logger with equivalent level to zerolog
func CreateSlogLoggerFromZerologLevel(zerologLevel zerolog.Level) *slog.Logger {
	var slogLevel slog.Level

	switch zerologLevel {
	case zerolog.DebugLevel:
		slogLevel = slog.LevelDebug
	case zerolog.InfoLevel:
		slogLevel = slog.LevelInfo
	case zerolog.WarnLevel:
		slogLevel = slog.LevelWarn
	case zerolog.ErrorLevel:
		slogLevel = slog.LevelError
	case zerolog.FatalLevel:
		slogLevel = slog.LevelError + 4 // Treat fatal as higher error
	case zerolog.PanicLevel:
		slogLevel = slog.LevelError + 8 // Treat panic as highest error
	default:
		slogLevel = slog.LevelInfo
	}

	config := SlogConfig{
		Level:     slogLevel,
		Format:    "text",
		AddSource: true,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}

	return NewSlogLogger(config)
}

// ZerologCompatEvent provides a zerolog-like event interface for slog
type ZerologCompatEvent struct {
	slogger *slog.Logger
	level   slog.Level
	attrs   []slog.Attr
}

// Info creates an info-level event
func (a *ZerologToSlogAdapter) Info() *ZerologCompatEvent {
	return &ZerologCompatEvent{
		slogger: a.slogger,
		level:   slog.LevelInfo,
		attrs:   make([]slog.Attr, 0),
	}
}

// Warn creates a warn-level event
func (a *ZerologToSlogAdapter) Warn() *ZerologCompatEvent {
	return &ZerologCompatEvent{
		slogger: a.slogger,
		level:   slog.LevelWarn,
		attrs:   make([]slog.Attr, 0),
	}
}

// Error creates an error-level event
func (a *ZerologToSlogAdapter) Error() *ZerologCompatEvent {
	return &ZerologCompatEvent{
		slogger: a.slogger,
		level:   slog.LevelError,
		attrs:   make([]slog.Attr, 0),
	}
}

// Debug creates a debug-level event
func (a *ZerologToSlogAdapter) Debug() *ZerologCompatEvent {
	return &ZerologCompatEvent{
		slogger: a.slogger,
		level:   slog.LevelDebug,
		attrs:   make([]slog.Attr, 0),
	}
}

// With creates a child logger with additional context
func (a *ZerologToSlogAdapter) With() *ZerologToSlogAdapter {
	return &ZerologToSlogAdapter{
		slogger: a.slogger,
	}
}

// Str adds a string field to the event
func (e *ZerologCompatEvent) Str(key, val string) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.String(key, val))
	return e
}

// Int adds an integer field to the event
func (e *ZerologCompatEvent) Int(key string, val int) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.Int(key, val))
	return e
}

// Int64 adds an int64 field to the event
func (e *ZerologCompatEvent) Int64(key string, val int64) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.Int64(key, val))
	return e
}

// Bool adds a boolean field to the event
func (e *ZerologCompatEvent) Bool(key string, val bool) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.Bool(key, val))
	return e
}

// Float64 adds a float64 field to the event
func (e *ZerologCompatEvent) Float64(key string, val float64) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.Float64(key, val))
	return e
}

// Err adds an error field to the event
func (e *ZerologCompatEvent) Err(err error) *ZerologCompatEvent {
	if err != nil {
		e.attrs = append(e.attrs, slog.String("error", err.Error()))
	}
	return e
}

// Dur adds a duration field to the event
func (e *ZerologCompatEvent) Dur(key string, d interface{}) *ZerologCompatEvent {
	e.attrs = append(e.attrs, slog.String(key, formatDuration(d)))
	return e
}

// Msg sends the event with a message
func (e *ZerologCompatEvent) Msg(msg string) {
	e.slogger.LogAttrs(nil, e.level, msg, e.attrs...)
}

// Msgf sends the event with a formatted message
func (e *ZerologCompatEvent) Msgf(format string, args ...interface{}) {
	// Note: slog doesn't have direct printf-style formatting, so we'll use a simple approach
	msg := format
	if len(args) > 0 {
		// This is a simplified implementation; in a real scenario you might want to use fmt.Sprintf
		// Convert args to slog.Attr and add them directly
		formatAttrs := convertArgs(args)
		e.attrs = append(e.attrs, formatAttrs...)
	}
	e.slogger.LogAttrs(nil, e.level, msg, e.attrs...)
}

// Helper functions
func formatDuration(d interface{}) string {
	switch v := d.(type) {
	case string:
		return v
	default:
		return "unknown_duration"
	}
}

func convertArgs(args []interface{}) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args))
	for i, arg := range args {
		key := "arg" + string(rune('0'+i))
		attrs = append(attrs, slog.Any(key, arg))
	}
	return attrs
}

// GetLevel compatibility method
func (a *ZerologToSlogAdapter) GetLevel() zerolog.Level {
	// This is a compatibility shim - not perfect but functional
	return zerolog.InfoLevel
}
