package transport

import (
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Config holds common configuration for stdio transports
type Config struct {
	// Logger is the base logger - transport-specific context will be added
	Logger *slog.Logger

	// EnableErrorHandler enables enhanced error handling for the main transport
	EnableErrorHandler bool

	// LogLevel can override the logger level for stdio-specific logging
	LogLevel string

	// BufferSize for stdio communication (optional, uses defaults if 0)
	BufferSize int

	// Component name for logging context (will be added to logger)
	Component string
}

// NewDefaultConfig creates a default configuration with reasonable defaults
func NewDefaultConfig(baseLogger *slog.Logger) Config {
	return Config{
		Logger:             baseLogger,
		EnableErrorHandler: true,
		LogLevel:           "info",
		BufferSize:         0, // Use system defaults
		Component:          "stdio_transport",
	}
}

// NewConfigWithComponent creates a default config with a specific component name
func NewConfigWithComponent(baseLogger *slog.Logger, component string) Config {
	config := NewDefaultConfig(baseLogger)
	config.Component = component
	return config
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Note: We can't easily validate if zerolog.Logger is initialized since it can't be compared
	// We'll rely on runtime behavior and panics if the logger is invalid

	if c.Component == "" {
		return errors.NewError().Messagef("component name is required").Build()
	}

	if c.BufferSize < 0 {
		return errors.NewError().Messagef("buffer size cannot be negative").Build()
	}

	return nil
}

// CreateLogger creates a properly configured logger for stdio transport
func (c Config) CreateLogger() *slog.Logger {
	return c.Logger.With(
		"transport", "stdio",
		"component", c.Component,
	)
}

// CreateDefaultLogger creates a fallback logger when none is provided
func CreateDefaultLogger(component string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil)).With(
		"transport", "stdio",
		"component", component,
	)
}
