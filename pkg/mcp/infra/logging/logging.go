package logging

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/rs/zerolog"
)

// Logger is the public logging interface
type Logger = logging.UnifiedLogger

// Config for logger setup
type Config = logging.Config

// New creates a new logger instance
var New = logging.NewLogger

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

// Float64 creates a float64 field
func Float64(key string, val float64) Field {
	return Field{Key: key, Value: val}
}

// Bool creates a boolean field
func Bool(key string, val bool) Field {
	return Field{Key: key, Value: val}
}

// Err creates an error field
func Err(key string, val error) Field {
	return Field{Key: key, Value: val}
}

// Time creates a time field
func Time(key string, val time.Time) Field {
	return Field{Key: key, Value: val}
}

// Dur creates a duration field
func Dur(key string, val time.Duration) Field {
	return Field{Key: key, Value: val}
}

// Level constants
const (
	LevelDebug = zerolog.DebugLevel
	LevelInfo  = zerolog.InfoLevel
	LevelWarn  = zerolog.WarnLevel
	LevelError = zerolog.ErrorLevel
	LevelFatal = zerolog.FatalLevel
	LevelPanic = zerolog.PanicLevel
)
