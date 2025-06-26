package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

// Logger is the unified logging interface for the MCP codebase
type Logger interface {
	// Info logs an informational message
	Info(msg string, fields ...Field)
	// Warn logs a warning message
	Warn(msg string, fields ...Field)
	// Error logs an error message with optional error
	Error(msg string, err error, fields ...Field)
	// Debug logs a debug message
	Debug(msg string, fields ...Field)
	// Fatal logs a fatal message and exits the program
	Fatal(msg string, err error, fields ...Field)

	// With returns a new logger with the given fields
	With(fields ...Field) Logger
	// WithContext returns a new logger with context fields
	WithContext(ctx context.Context) Logger
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// Str creates a string field
func Str(key string, val string) Field {
	return Field{Key: key, Value: val}
}

// Int creates an integer field
func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

// Int64 creates an int64 field
func Int64(key string, val int64) Field {
	return Field{Key: key, Value: val}
}

// Bool creates a boolean field
func Bool(key string, val bool) Field {
	return Field{Key: key, Value: val}
}

// ErrorField creates an error field
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value
func Any(key string, val interface{}) Field {
	return Field{Key: key, Value: val}
}

// zerologLogger wraps zerolog.Logger to implement our unified interface
type zerologLogger struct {
	logger zerolog.Logger
}

// NewLogger creates a new logger instance
func NewLogger(name string) Logger {
	return &zerologLogger{
		logger: zerolog.New(os.Stdout).With().
			Timestamp().
			Str("component", name).
			Logger(),
	}
}

// NewLoggerWithWriter creates a new logger with a custom writer
func NewLoggerWithWriter(name string, w io.Writer) Logger {
	return &zerologLogger{
		logger: zerolog.New(w).With().
			Timestamp().
			Str("component", name).
			Logger(),
	}
}

// NewLoggerFromZerolog creates a logger from an existing zerolog instance
func NewLoggerFromZerolog(zl zerolog.Logger) Logger {
	return &zerologLogger{logger: zl}
}

func (l *zerologLogger) Info(msg string, fields ...Field) {
	event := l.logger.Info()
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *zerologLogger) Warn(msg string, fields ...Field) {
	event := l.logger.Warn()
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *zerologLogger) Error(msg string, err error, fields ...Field) {
	event := l.logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *zerologLogger) Debug(msg string, fields ...Field) {
	event := l.logger.Debug()
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *zerologLogger) Fatal(msg string, err error, fields ...Field) {
	event := l.logger.Fatal()
	if err != nil {
		event = event.Err(err)
	}
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *zerologLogger) With(fields ...Field) Logger {
	ctx := l.logger.With()
	for _, f := range fields {
		ctx = l.addFieldToContext(ctx, f)
	}
	return &zerologLogger{logger: ctx.Logger()}
}

func (l *zerologLogger) WithContext(ctx context.Context) Logger {
	return &zerologLogger{logger: l.logger.With().Ctx(ctx).Logger()}
}

func (l *zerologLogger) addFields(event *zerolog.Event, fields []Field) {
	for _, f := range fields {
		switch v := f.Value.(type) {
		case string:
			event.Str(f.Key, v)
		case int:
			event.Int(f.Key, v)
		case int64:
			event.Int64(f.Key, v)
		case bool:
			event.Bool(f.Key, v)
		case error:
			event.Err(v)
		case fmt.Stringer:
			event.Str(f.Key, v.String())
		default:
			event.Interface(f.Key, v)
		}
	}
}

func (l *zerologLogger) addFieldToContext(ctx zerolog.Context, field Field) zerolog.Context {
	switch v := field.Value.(type) {
	case string:
		return ctx.Str(field.Key, v)
	case int:
		return ctx.Int(field.Key, v)
	case int64:
		return ctx.Int64(field.Key, v)
	case bool:
		return ctx.Bool(field.Key, v)
	case error:
		return ctx.Err(v)
	case fmt.Stringer:
		return ctx.Str(field.Key, v.String())
	default:
		return ctx.Interface(field.Key, v)
	}
}

// Global logger configuration
var (
	globalLogger     Logger
	globalLoggerOnce sync.Once
)

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	globalLoggerOnce.Do(func() {
		globalLogger = logger
	})
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() Logger {
	globalLoggerOnce.Do(func() {
		globalLogger = NewLogger("global")
	})
	return globalLogger
}

// Convenience functions using the global logger

// Info logs an informational message using the global logger
func Info(msg string, fields ...Field) {
	GetGlobalLogger().Info(msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	GetGlobalLogger().Warn(msg, fields...)
}

// Error logs an error message using the global logger
func Error(msg string, err error, fields ...Field) {
	GetGlobalLogger().Error(msg, err, fields...)
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	GetGlobalLogger().Debug(msg, fields...)
}

// Fatal logs a fatal message using the global logger and exits
func Fatal(msg string, err error, fields ...Field) {
	GetGlobalLogger().Fatal(msg, err, fields...)
}
