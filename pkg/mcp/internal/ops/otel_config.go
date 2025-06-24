package ops

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// OTELConfig holds OpenTelemetry configuration
type OTELConfig struct {
	// Service identification
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Environment    string `json:"environment"`

	// OTLP exporter configuration
	EnableOTLP   bool              `json:"enable_otlp"`
	OTLPEndpoint string            `json:"otlp_endpoint"`
	OTLPHeaders  map[string]string `json:"otlp_headers"`
	OTLPInsecure bool              `json:"otlp_insecure"`
	OTLPTimeout  time.Duration     `json:"otlp_timeout"`

	// Sampling configuration
	TraceSampleRate  float64 `json:"trace_sample_rate"`
	EnableDebugTrace bool    `json:"enable_debug_trace"`

	// Resource attributes
	CustomAttributes map[string]string `json:"custom_attributes"`

	Logger zerolog.Logger `json:"-"`
}

// NewDefaultOTELConfig creates a default OpenTelemetry configuration
func NewDefaultOTELConfig(logger zerolog.Logger) *OTELConfig {
	return &OTELConfig{
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		EnableOTLP:      false,
		OTLPEndpoint:    "http://localhost:4318/v1/traces",
		OTLPHeaders:     make(map[string]string),
		OTLPInsecure:    true,
		OTLPTimeout:     10 * time.Second,
		TraceSampleRate: 1.0,
		CustomAttributes: map[string]string{
			"service.component": "mcp-server",
		},
		Logger: logger,
	}
}

// OTELProvider manages OpenTelemetry providers and lifecycle
type OTELProvider struct {
	config        *OTELConfig
	traceProvider *trace.TracerProvider
	shutdownFuncs []func(context.Context) error
	logger        zerolog.Logger
	initialized   bool
}

// NewOTELProvider creates a new OpenTelemetry provider
func NewOTELProvider(config *OTELConfig) *OTELProvider {
	return &OTELProvider{
		config:        config,
		logger:        config.Logger,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}
}

// Initialize sets up OpenTelemetry providers and exporters
func (p *OTELProvider) Initialize(ctx context.Context) error {
	if p.initialized {
		return nil
	}

	p.logger.Info().
		Str("service_name", p.config.ServiceName).
		Str("service_version", p.config.ServiceVersion).
		Bool("enable_otlp", p.config.EnableOTLP).
		Msg("Initializing OpenTelemetry")

	// Create resource with service information
	res, err := p.createResource()
	if err != nil {
		return fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Initialize trace provider
	if err := p.initializeTracing(ctx, res); err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Set global text map propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p.initialized = true
	p.logger.Info().Msg("OpenTelemetry initialized successfully")

	return nil
}

// createResource creates an OTEL resource with service identification
func (p *OTELProvider) createResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(p.config.ServiceName),
		semconv.ServiceVersion(p.config.ServiceVersion),
		semconv.DeploymentEnvironment(p.config.Environment),
	}

	// Add custom attributes
	for key, value := range p.config.CustomAttributes {
		attrs = append(attrs, attribute.String(key, value))
	}

	// Create resource without schema URL to avoid conflicts
	return resource.NewWithAttributes(
		"", // Empty schema URL to avoid conflicts
		attrs...,
	), nil
}

// initializeTracing sets up the trace provider with appropriate exporters
func (p *OTELProvider) initializeTracing(ctx context.Context, res *resource.Resource) error {
	var exporters []trace.SpanExporter

	// Add OTLP exporter if enabled
	if p.config.EnableOTLP {
		otlpExporter, err := p.createOTLPExporter(ctx)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		exporters = append(exporters, otlpExporter)

		p.logger.Info().
			Str("endpoint", p.config.OTLPEndpoint).
			Msg("OTLP trace exporter configured")
	}

	// If no exporters configured, use a no-op setup
	if len(exporters) == 0 {
		p.logger.Info().Msg("No trace exporters configured, using no-op provider")
		p.traceProvider = trace.NewTracerProvider(
			trace.WithResource(res),
		)
	} else {
		// Create batch span processors for all exporters
		var spanProcessors []trace.SpanProcessor
		for _, exporter := range exporters {
			processor := trace.NewBatchSpanProcessor(exporter)
			spanProcessors = append(spanProcessors, processor)
		}

		// Create tracer provider with sampling
		sampler := trace.AlwaysSample()
		if p.config.TraceSampleRate < 1.0 {
			sampler = trace.TraceIDRatioBased(p.config.TraceSampleRate)
		}

		var opts []trace.TracerProviderOption
		opts = append(opts, trace.WithResource(res))
		opts = append(opts, trace.WithSampler(sampler))
		for _, processor := range spanProcessors {
			opts = append(opts, trace.WithSpanProcessor(processor))
		}

		p.traceProvider = trace.NewTracerProvider(opts...)

		// Register shutdown functions
		for _, processor := range spanProcessors {
			processor := processor // capture for closure
			p.shutdownFuncs = append(p.shutdownFuncs, processor.Shutdown)
		}
	}

	// Set global trace provider
	otel.SetTracerProvider(p.traceProvider)

	return nil
}

// createOTLPExporter creates an OTLP HTTP trace exporter
func (p *OTELProvider) createOTLPExporter(ctx context.Context) (trace.SpanExporter, error) {
	// Validate endpoint URL
	if _, err := url.Parse(p.config.OTLPEndpoint); err != nil {
		return nil, fmt.Errorf("invalid OTLP endpoint URL: %w", err)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(p.config.OTLPEndpoint),
		otlptracehttp.WithTimeout(p.config.OTLPTimeout),
	}

	if p.config.OTLPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	if len(p.config.OTLPHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(p.config.OTLPHeaders))
	}

	return otlptracehttp.New(ctx, opts...)
}

// Shutdown gracefully shuts down all OpenTelemetry providers
func (p *OTELProvider) Shutdown(ctx context.Context) error {
	if !p.initialized {
		return nil
	}

	p.logger.Info().Msg("Shutting down OpenTelemetry providers")

	var errors []error
	for _, shutdown := range p.shutdownFuncs {
		if err := shutdown(ctx); err != nil {
			errors = append(errors, err)
			p.logger.Error().Err(err).Msg("Error during OTEL shutdown")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	p.initialized = false
	p.logger.Info().Msg("OpenTelemetry shutdown complete")
	return nil
}

// GetTracerProvider returns the configured tracer provider
func (p *OTELProvider) GetTracerProvider() *trace.TracerProvider {
	return p.traceProvider
}

// IsInitialized returns whether the provider has been initialized
func (p *OTELProvider) IsInitialized() bool {
	return p.initialized
}

// UpdateConfig updates the OTEL configuration from environment variables or other sources
func (p *OTELProvider) UpdateConfig(updates map[string]interface{}) {
	if endpoint, ok := updates["otlp_endpoint"].(string); ok && endpoint != "" {
		p.config.OTLPEndpoint = endpoint
		p.config.EnableOTLP = true
	}

	if headers, ok := updates["otlp_headers"].(map[string]string); ok {
		for k, v := range headers {
			p.config.OTLPHeaders[k] = v
		}
	}

	if sampleRate, ok := updates["trace_sample_rate"].(float64); ok {
		p.config.TraceSampleRate = sampleRate
	}

	if env, ok := updates["environment"].(string); ok && env != "" {
		p.config.Environment = env
	}

	p.logger.Info().Msg("OTEL configuration updated")
}

// EnableConsoleExporter enables console output for debugging (development only)
func (p *OTELProvider) EnableConsoleExporter() {
	if p.config.EnableDebugTrace {
		// Note: In a real implementation, you might want to add a console/stdout exporter
		// This is mainly for development/debugging purposes
		p.logger.Info().Msg("Console trace export enabled for debugging")
	}
}

// GetConfig returns the current OTEL configuration
func (p *OTELProvider) GetConfig() *OTELConfig {
	return p.config
}

// ValidateConfig validates the OTEL configuration
func (config *OTELConfig) Validate() error {
	if config.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}

	if config.EnableOTLP {
		if config.OTLPEndpoint == "" {
			return fmt.Errorf("otlp_endpoint is required when OTLP is enabled")
		}

		if _, err := url.Parse(config.OTLPEndpoint); err != nil {
			return fmt.Errorf("invalid otlp_endpoint URL: %w", err)
		}
	}

	if config.TraceSampleRate < 0.0 || config.TraceSampleRate > 1.0 {
		return fmt.Errorf("trace_sample_rate must be between 0.0 and 1.0")
	}

	return nil
}

// LogConfig logs the current configuration (without sensitive data)
func (config *OTELConfig) LogConfig(logger zerolog.Logger) {
	logger.Info().
		Str("service_name", config.ServiceName).
		Str("service_version", config.ServiceVersion).
		Str("environment", config.Environment).
		Bool("enable_otlp", config.EnableOTLP).
		Str("otlp_endpoint", config.OTLPEndpoint).
		Float64("trace_sample_rate", config.TraceSampleRate).
		Bool("otlp_insecure", config.OTLPInsecure).
		Dur("otlp_timeout", config.OTLPTimeout).
		Msg("OpenTelemetry configuration")
}
