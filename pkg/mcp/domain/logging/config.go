package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Level represents log level severity.
type Level int

const (
	// LevelDebug represents debug level logging.
	LevelDebug Level = iota

	// LevelInfo represents info level logging.
	LevelInfo

	// LevelWarn represents warning level logging.
	LevelWarn

	// LevelError represents error level logging.
	LevelError

	// LevelFatal represents fatal level logging.
	LevelFatal
)

// Config holds configuration for creating a logger.
type Config struct {
	// Level is the minimum log level to output.
	Level Level

	// Output is the writer to output logs to.
	Output io.Writer

	// BufferSize is the size of the ring buffer for log capture.
	BufferSize int

	// Fields contains default fields to include in all log messages.
	Fields map[string]interface{}

	// EnableStructuredLogging enables structured logging output.
	EnableStructuredLogging bool

	// EnableRingBuffer enables the ring buffer for log capture.
	EnableRingBuffer bool

	// TimeFormat specifies the format for timestamps.
	TimeFormat string

	// EnableCaller adds caller information to log messages.
	EnableCaller bool

	// EnableStackTrace adds stack traces to error messages.
	EnableStackTrace bool
}

// DefaultConfig returns a default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:                   LevelInfo,
		Output:                  os.Stdout,
		BufferSize:              1000,
		Fields:                  make(map[string]interface{}),
		EnableStructuredLogging: true,
		EnableRingBuffer:        true,
		TimeFormat:              time.RFC3339,
		EnableCaller:            false,
		EnableStackTrace:        false,
	}
}

// toZerologLevel converts our Level to zerolog.Level.
func (l Level) toZerologLevel() zerolog.Level {
	switch l {
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	case LevelFatal:
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}
