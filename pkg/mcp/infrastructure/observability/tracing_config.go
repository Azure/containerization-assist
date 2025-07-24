// Package observability provides unified monitoring, tracing, and health infrastructure
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry configuration
type Config struct {
	// Enabled controls whether tracing is active
	Enabled bool

	// Endpoint is the OTLP trace endpoint (e.g., http://localhost:4318/v1/traces)
	Endpoint string

	// Headers are additional headers to send with traces
	Headers map[string]string

	// ServiceName identifies the service in traces
	ServiceName string

	// ServiceVersion identifies the service version
	ServiceVersion string

	// Environment identifies the deployment environment
	Environment string

	// SampleRate controls the sampling rate (0.0-1.0)
	SampleRate float64

	// Timeout for trace export operations
	ExportTimeout time.Duration
}

// DefaultConfig returns a default tracing configuration
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		Endpoint:       "http://localhost:4318/v1/traces",
		Headers:        make(map[string]string),
		ServiceName:    "container-kit-mcp",
		ServiceVersion: "dev",
		Environment:    "development",
		SampleRate:     1.0,
		ExportTimeout:  30 * time.Second,
	}
}

// TracerProvider manages the global tracer provider
var globalTracerProvider *sdktrace.TracerProvider

// InitializeTracing sets up OpenTelemetry tracing with the given configuration
func InitializeTracing(ctx context.Context, config Config) error {
	if !config.Enabled {
		// Set a no-op tracer provider
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithHeaders(config.Headers),
		otlptracehttp.WithTimeout(config.ExportTimeout),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create tracer provider with sampling
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)
	globalTracerProvider = tracerProvider

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// Shutdown gracefully shuts down the tracing infrastructure
func Shutdown(ctx context.Context) error {
	if globalTracerProvider != nil {
		return globalTracerProvider.Shutdown(ctx)
	}
	return nil
}

// GetTracer returns a tracer for the given component name
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// SpanFromContext returns the current span from context, if any
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// StartSpan starts a new span with the given name and options
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := GetTracer("container-kit")
	return tracer.Start(ctx, name, opts...)
}
