package transport

import (
	"fmt"

	"github.com/rs/zerolog"
)

// TransportPair holds related stdio transports
type TransportPair struct {
	MainTransport *StdioTransport
	LLMTransport  *StdioLLMTransport
}

// NewTransportPair creates both main and LLM stdio transports with shared configuration
func NewTransportPair(config Config) (*TransportPair, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create main transport with shared config
	mainTransport, err := NewStdioTransportWithConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create main stdio transport: %w", err)
	}

	// Create LLM transport that wraps the main transport
	llmConfig := config
	llmConfig.Component = "stdio_llm_transport"
	llmTransport, err := NewLLMTransportWithConfig(llmConfig, mainTransport)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM stdio transport: %w", err)
	}

	return &TransportPair{
		MainTransport: mainTransport,
		LLMTransport:  llmTransport,
	}, nil
}

// NewStdioTransportWithConfig creates a main stdio transport using shared configuration
func NewStdioTransportWithConfig(config Config) (*StdioTransport, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create logger with stdio context
	logger := config.CreateLogger()

	// Use existing constructor but with our standardized logger
	return NewStdioTransportWithLogger(logger), nil
}

// NewLLMTransportWithConfig creates an LLM stdio transport using shared configuration
func NewLLMTransportWithConfig(config Config, baseTransport *StdioTransport) (*StdioLLMTransport, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if baseTransport == nil {
		return nil, fmt.Errorf("base transport cannot be nil")
	}

	// Create logger with LLM transport context
	logger := config.CreateLogger()

	// Use existing constructor but with our standardized logger
	return NewStdioLLMTransport(baseTransport, logger), nil
}

// NewDefaultStdioTransport creates a stdio transport with default configuration
func NewDefaultStdioTransport(baseLogger zerolog.Logger) *StdioTransport {
	config := NewDefaultConfig(baseLogger)
	stdioTransport, err := NewStdioTransportWithConfig(config)
	if err != nil {
		// Fallback to original constructor if config fails
		return NewStdioTransportWithLogger(baseLogger)
	}
	return stdioTransport
}

// NewDefaultLLMTransport creates an LLM transport with default configuration
func NewDefaultLLMTransport(baseTransport *StdioTransport, baseLogger zerolog.Logger) *StdioLLMTransport {
	config := NewConfigWithComponent(baseLogger, "stdio_llm_transport")
	llmTransport, err := NewLLMTransportWithConfig(config, baseTransport)
	if err != nil {
		// Fallback to original constructor if config fails
		return NewStdioLLMTransport(baseTransport, baseLogger)
	}
	return llmTransport
}

// CreateStandardLoggerPair creates consistently configured loggers for both transports
func CreateStandardLoggerPair(baseLogger zerolog.Logger) (main, llm zerolog.Logger) {
	mainConfig := NewConfigWithComponent(baseLogger, "stdio_transport")
	llmConfig := NewConfigWithComponent(baseLogger, "stdio_llm_transport")

	return mainConfig.CreateLogger(), llmConfig.CreateLogger()
}
