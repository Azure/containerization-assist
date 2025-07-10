package logging

import (
	"log/slog"
	"os"
)

// Logger interface defines the logging contract for the domain layer
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithComponent(component string) Logger
}

// Standards is the standard logging interface used throughout the domain
type Standards interface {
	Logger
	// Additional methods can be added here as needed
}

// Config represents logging configuration
type Config struct {
	Level                   Level
	Output                  *os.File
	EnableStructuredLogging bool
	EnableRingBuffer        bool
	BufferSize              int
}

// Level represents the logging level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// SlogAdapter adapts slog.Logger to our Logger interface
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new slog adapter
func NewSlogAdapter(logger *slog.Logger) Standards {
	return &SlogAdapter{logger: logger}
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config Config) Standards {
	// Convert our Level to slog.Level
	var slogLevel slog.Level
	switch config.Level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	output := config.Output
	if output == nil {
		output = os.Stdout
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: slogLevel,
	})

	logger := slog.New(handler)
	return NewSlogAdapter(logger)
}

// NewTestLogger creates a logger suitable for testing
func NewTestLogger() Standards {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	return NewSlogAdapter(logger)
}

func (s *SlogAdapter) Debug(msg string, args ...any) {
	s.logger.Debug(msg, args...)
}

func (s *SlogAdapter) Info(msg string, args ...any) {
	s.logger.Info(msg, args...)
}

func (s *SlogAdapter) Warn(msg string, args ...any) {
	s.logger.Warn(msg, args...)
}

func (s *SlogAdapter) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

func (s *SlogAdapter) With(args ...any) Logger {
	return &SlogAdapter{logger: s.logger.With(args...)}
}

func (s *SlogAdapter) WithComponent(component string) Logger {
	return &SlogAdapter{logger: s.logger.With("component", component)}
}
