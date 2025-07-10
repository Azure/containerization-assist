package telemetry

import (
	"context"
	"fmt"
	"runtime"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TracingManager manages OpenTelemetry tracing
type TracingManager struct {
	config   *Config
	provider *trace.TracerProvider
	tracer   oteltrace.Tracer
}

// NewTracingManager creates a new tracing manager
func NewTracingManager(config *Config) *TracingManager {
	return &TracingManager{
		config: config,
	}
}

// Initialize initializes the tracing system
func (tm *TracingManager) Initialize(ctx context.Context) error {
	if !tm.config.TracingEnabled {
		// Use no-op tracer
		tm.tracer = otel.Tracer(tm.config.ServiceName)
		return nil
	}

	// Create resource
	res, err := tm.createResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	exporter, err := tm.createExporter()
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(tm.config.TraceSampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tm.provider = tp
	tm.tracer = tp.Tracer(tm.config.ServiceName)

	return nil
}

// Shutdown gracefully shuts down the tracing system
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.provider != nil {
		return tm.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new tracing span
func (tm *TracingManager) StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	if tm.tracer == nil {
		return ctx, oteltrace.SpanFromContext(ctx)
	}
	return tm.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the span from context
func (tm *TracingManager) SpanFromContext(ctx context.Context) oteltrace.Span {
	return oteltrace.SpanFromContext(ctx)
}

// RecordError records an error in the span
func (tm *TracingManager) RecordError(span oteltrace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// AddSpanAttributes adds attributes to a span
func (tm *TracingManager) AddSpanAttributes(span oteltrace.Span, attrs ...attribute.KeyValue) {
	if span != nil {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to a span
func (tm *TracingManager) AddSpanEvent(span oteltrace.Span, name string, attrs ...attribute.KeyValue) {
	if span != nil {
		span.AddEvent(name, oteltrace.WithAttributes(attrs...))
	}
}

// createResource creates an OpenTelemetry resource
func (tm *TracingManager) createResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(tm.config.ServiceName),
		semconv.ServiceVersion(tm.config.ServiceVersion),
		semconv.DeploymentEnvironment(tm.config.Environment),
		attribute.String("telemetry.sdk.name", "opentelemetry"),
		attribute.String("telemetry.sdk.language", "go"),
		attribute.String("telemetry.sdk.version", otel.Version()),
	}

	// Add custom resource attributes
	for key, value := range tm.config.ResourceAttributes {
		attrs = append(attrs, attribute.String(key, value))
	}

	return resource.New(context.Background(),
		resource.WithAttributes(attrs...),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
}

// createExporter creates a trace exporter
func (tm *TracingManager) createExporter() (trace.SpanExporter, error) {
	// For development, we can use stdout exporter as fallback
	if tm.config.Environment == "development" && tm.config.TracingEndpoint == "" {
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}

	// Use Jaeger exporter for production
	return jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(tm.config.TracingEndpoint),
	))
}

// Instrumentation helpers

// InstrumentFunction wraps a function with tracing
func (tm *TracingManager) InstrumentFunction(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, span := tm.StartSpan(ctx, name)
	defer span.End()

	// Add caller information
	if pc, file, line, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			tm.AddSpanAttributes(span,
				attribute.String("code.function", fn.Name()),
				attribute.String("code.filepath", file),
				attribute.Int("code.lineno", line),
			)
		}
	}

	err := fn(ctx)
	if err != nil {
		tm.RecordError(span, err)
	}

	return err
}

// InstrumentHTTPHandler wraps an HTTP handler with tracing
func (tm *TracingManager) InstrumentHTTPHandler(name string, handler func(ctx context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return tm.InstrumentFunction(ctx, fmt.Sprintf("http.%s", name), handler)
	}
}

// InstrumentToolExecution wraps tool execution with tracing
func (tm *TracingManager) InstrumentToolExecution(ctx context.Context, toolName string, fn func(context.Context) error) error {
	return tm.InstrumentFunction(ctx, fmt.Sprintf("tool.%s", toolName), func(ctx context.Context) error {
		span := tm.SpanFromContext(ctx)
		tm.AddSpanAttributes(span,
			attribute.String("tool.name", toolName),
			attribute.String("operation.type", "tool_execution"),
		)
		return fn(ctx)
	})
}

// InstrumentPipelineStage wraps pipeline stage execution with tracing
func (tm *TracingManager) InstrumentPipelineStage(ctx context.Context, stageName string, fn func(context.Context) error) error {
	return tm.InstrumentFunction(ctx, fmt.Sprintf("pipeline.stage.%s", stageName), func(ctx context.Context) error {
		span := tm.SpanFromContext(ctx)
		tm.AddSpanAttributes(span,
			attribute.String("pipeline.stage", stageName),
			attribute.String("operation.type", "pipeline_stage"),
		)
		return fn(ctx)
	})
}
