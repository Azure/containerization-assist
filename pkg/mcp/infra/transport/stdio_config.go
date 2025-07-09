package transport

import (
	"os"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// Config holds common configuration for stdio transports
type Config struct {
	// Logger is the base logger - transport-specific context will be added
	Logger zerolog.Logger

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
func NewDefaultConfig(baseLogger zerolog.Logger) Config {
	return Config{
		Logger:             baseLogger,
		EnableErrorHandler: true,
		LogLevel:           "info",
		BufferSize:         0, // Use system defaults
		Component:          "stdio_transport",
	}
}

// NewConfigWithComponent creates a default config with a specific component name
func NewConfigWithComponent(baseLogger zerolog.Logger, component string) Config {
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
		return errors.NewError().Messagef("buffer size cannot be negative").Build(

		// CreateLogger creates a properly configured logger for stdio transport
		)
	}

	return nil
}

func (c Config) CreateLogger() zerolog.Logger {
	logger := c.Logger.With().
		Str("transport", "stdio").
		Str("component", c.Component).
		Logger()

	// Apply log level if specified
	if c.LogLevel != "" {
		if level, err := zerolog.ParseLevel(c.LogLevel); err == nil {
			logger = logger.Level(level)
		}
	}

	return logger
}

// CreateDefaultLogger creates a fallback logger when none is provided
func CreateDefaultLogger(component string) zerolog.Logger {
	return zerolog.New(os.Stderr).With().
		Timestamp().
		Str("transport", "stdio").
		Str("component", component).
		Logger()
}
