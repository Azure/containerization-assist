package transport

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/shared"
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
		return nil, errors.NewError().Message("invalid configuration").Cause(err).WithLocation(

		// Create main transport with shared config
		).Build()
	}

	mainTransport, err := NewStdioTransportWithConfig(config)
	if err != nil {
		return nil, errors.NewError().Message("failed to create main stdio transport").Cause(err).WithLocation(

		// Create LLM transport that wraps the main transport
		).Build()
	}

	llmConfig := config
	llmConfig.Component = "stdio_llm_transport"
	llmTransport, err := NewLLMTransportWithConfig(llmConfig, mainTransport)
	if err != nil {
		return nil, errors.NewError().Message("failed to create LLM stdio transport").Cause(err).Build()
	}

	return &TransportPair{
		MainTransport: mainTransport,
		LLMTransport:  llmTransport,
	}, nil
}

// NewStdioTransportWithConfig creates a main stdio transport using shared configuration
func NewStdioTransportWithConfig(config Config) (*StdioTransport, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.NewError().Message("invalid configuration").Cause(err).WithLocation(

		// Create logger with stdio context
		).Build()
	}

	logger := config.CreateLogger()

	// Use existing constructor but with our standardized logger
	return NewStdioTransportWithLogger(logger), nil
}

// NewLLMTransportWithConfig creates an LLM stdio transport using shared configuration
func NewLLMTransportWithConfig(config Config, baseTransport *StdioTransport) (*StdioLLMTransport, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.NewError().Message("invalid configuration").Cause(err).Build()
	}

	if baseTransport == nil {
		return nil, errors.NewError().Messagef("base transport cannot be nil").WithLocation(

		// Create logger with LLM transport context
		).Build()
	}

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

// NewDefaultCoreStdioTransport creates a stdio transport that implements Transport
func NewDefaultCoreStdioTransport(baseLogger zerolog.Logger) shared.Transport {
	return NewDefaultStdioTransport(baseLogger)
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
