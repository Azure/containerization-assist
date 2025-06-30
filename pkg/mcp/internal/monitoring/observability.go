package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityManager manages distributed tracing and metric
type ObservabilityManager struct {
	logger         zerolog.Logger
	config         ObservabilityConfig
	tracer         trace.Tracer
	meter          metric.Meter
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider

	// Instruments
	requestCounter    metric.Int64Counter
	requestDuration   metric.Float64Histogram
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
	errorCounter      metric.Int64Counter
	cacheHitCounter   metric.Int64Counter
	retryCounter      metric.Int64Counter
}

// ObservabilityConfig configures the observability system
type ObservabilityConfig struct {
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Environment    string `json:"environment"`

	// Tracing configuration
	EnableTracing     bool    `json:"enable_tracing"`
	JaegerEndpoint    string  `json:"jaeger_endpoint"`
	TracingSampleRate float64 `json:"tracing_sample_rate"`

	// Metrics configuration
	EnableMetrics      bool   `json:"enable_metrics"`
	PrometheusEndpoint string `json:"prometheus_endpoint"`
	MetricsPort        int    `json:"metrics_port"`

	// Additional attributes
	ResourceAttributes map[string]string `json:"resource_attributes"`
}

// SpanContext provides context for distributed tracing
type SpanContext struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Attributes map[string]interface{}
}

// NewObservabilityManager creates a new observability manager
func NewObservabilityManager(config ObservabilityConfig, logger zerolog.Logger) (*ObservabilityManager, error) {
	if config.ServiceName == "" {
		config.ServiceName = "container-kit"
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = "1.0.0"
	}
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.TracingSampleRate == 0 {
		config.TracingSampleRate = 1.0
	}

	om := &ObservabilityManager{
		logger: logger.With().Str("component", "observability").Logger(),
		config: config,
	}

	// Initialize OpenTelemetry
	if err := om.initializeOpenTelemetry(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	return om, nil
}

// initializeOpenTelemetry sets up OpenTelemetry tracing and metrics
func (om *ObservabilityManager) initializeOpenTelemetry() error {
	// Create resource with service information
	res, err := om.createResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracing if enabled
	if om.config.EnableTracing {
		if err := om.initializeTracing(res); err != nil {
			return fmt.Errorf("failed to initialize tracing: %w", err)
		}
	}

	// Initialize metrics if enabled
	if om.config.EnableMetrics {
		if err := om.initializeMetrics(res); err != nil {
			return fmt.Errorf("failed to initialize metrics: %w", err)
		}
	}

	// Initialize instruments
	if err := om.initializeInstruments(); err != nil {
		return fmt.Errorf("failed to initialize instruments: %w", err)
	}

	om.logger.Info().
		Bool("tracing_enabled", om.config.EnableTracing).
		Bool("metrics_enabled", om.config.EnableMetrics).
		Str("service_name", om.config.ServiceName).
		Msg("OpenTelemetry initialized")

	return nil
}

// createResource creates an OpenTelemetry resource with service information
func (om *ObservabilityManager) createResource() (*resource.Resource, error) {
	attributes := []attribute.KeyValue{
		semconv.ServiceNameKey.String(om.config.ServiceName),
		semconv.ServiceVersionKey.String(om.config.ServiceVersion),
		semconv.DeploymentEnvironmentKey.String(om.config.Environment),
	}

	// Add custom resource attributes
	for key, value := range om.config.ResourceAttributes {
		attributes = append(attributes, attribute.String(key, value))
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attributes...,
	), nil
}

// initializeTracing sets up distributed tracing
func (om *ObservabilityManager) initializeTracing(res *resource.Resource) error {
	// Create Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(om.config.JaegerEndpoint)))
	if err != nil {
		return fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(om.config.TracingSampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	om.tracerProvider = tp
	om.tracer = tp.Tracer("container-kit",)

	return nil
}

// initializeMetrics sets up metrics collection
func (om *ObservabilityManager) initializeMetrics(res *resource.Resource) error {
	// Create Prometheus exporter
	exp, err := prometheus.New()
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(res),
	)

	// Set global meter provider
	otel.SetMeterProvider(mp)

	om.meterProvider = mp
	om.meter = mp.Meter("container-kit",)

	return nil
}

// initializeInstruments creates metric instruments
func (om *ObservabilityManager) initializeInstruments() error {
	var err error

	if om.meter == nil {
		return nil // Metrics not enabled
	}

	// Request counter
	om.requestCounter, err = om.meter.Int64Counter(
		"containerkit_requests_total",
		metric.WithDescription("Total number of requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request counter: %w", err)
	}

	// Request duration histogram
	om.requestDuration, err = om.meter.Float64Histogram(
		"containerkit_request_duration_seconds",
		metric.WithDescription("Request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	// Operation counter
	om.operationCounter, err = om.meter.Int64Counter(
		"containerkit_operations_total",
		metric.WithDescription("Total number of operations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create operation counter: %w", err)
	}

	// Operation duration histogram
	om.operationDuration, err = om.meter.Float64Histogram(
		"containerkit_operation_duration_seconds",
		metric.WithDescription("Operation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	// Error counter
	om.errorCounter, err = om.meter.Int64Counter(
		"containerkit_errors_total",
		metric.WithDescription("Total number of errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error counter: %w", err)
	}

	// Cache hit counter
	om.cacheHitCounter, err = om.meter.Int64Counter(
		"containerkit_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create cache hit counter: %w", err)
	}

	// Retry counter
	om.retryCounter, err = om.meter.Int64Counter(
		"containerkit_retries_total",
		metric.WithDescription("Total number of retries"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create retry counter: %w", err)
	}

	return nil
}

// StartSpan starts a new trace span
func (om *ObservabilityManager) StartSpan(ctx context.Context, operationName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if om.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	return om.tracer.Start(ctx, operationName, trace.WithAttributes(attrs...))
}

// StartSpanWithParent starts a new span with explicit parent
func (om *ObservabilityManager) StartSpanWithParent(ctx context.Context, parentSpan trace.Span, operationName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if om.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	parentCtx := trace.ContextWithSpan(ctx, parentSpan)
	return om.tracer.Start(parentCtx, operationName, trace.WithAttributes(attrs...))
}

// AddSpanEvent adds an event to the current span
func (om *ObservabilityManager) AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetSpanError sets error information on the current span
func (om *ObservabilityManager) SetSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanAttributes sets attributes on the current span
func (om *ObservabilityManager) SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// RecordRequest records a request metric
func (om *ObservabilityManager) RecordRequest(ctx context.Context, method, endpoint, status string, duration time.Duration) {
	if om.requestCounter != nil {
		om.requestCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", endpoint),
			attribute.String("status", status),
		))
	}

	if om.requestDuration != nil {
		om.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", endpoint),
			attribute.String("status", status),
		))
	}
}

// RecordOperation records an operation metric
func (om *ObservabilityManager) RecordOperation(ctx context.Context, operation, status string, duration time.Duration) {
	if om.operationCounter != nil {
		om.operationCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("status", status),
		))
	}

	if om.operationDuration != nil {
		om.operationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("status", status),
		))
	}
}

// RecordError records an error metric
func (om *ObservabilityManager) RecordError(ctx context.Context, errorType, component string) {
	if om.errorCounter != nil {
		om.errorCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error_type", errorType),
			attribute.String("component", component),
		))
	}
}

// RecordCacheHit records a cache hit metric
func (om *ObservabilityManager) RecordCacheHit(ctx context.Context, cacheType string, hit bool) {
	if om.cacheHitCounter != nil {
		status := "miss"
		if hit {
			status = "hit"
		}
		om.cacheHitCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("cache_type", cacheType),
			attribute.String("status", status),
		))
	}
}

// RecordRetry records a retry metric
func (om *ObservabilityManager) RecordRetry(ctx context.Context, operation, policy string, attempt int) {
	if om.retryCounter != nil {
		om.retryCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("policy", policy),
			attribute.Int("attempt", attempt),
		))
	}
}

// GetSpanContext extracts span context information
func (om *ObservabilityManager) GetSpanContext(ctx context.Context) *SpanContext {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	spanCtx := span.SpanContext()
	return &SpanContext{
		TraceID: spanCtx.TraceID().String(),
		SpanID:  spanCtx.SpanID().String(),
	}
}

// InjectTraceContext injects trace context into a carrier
func (om *ObservabilityManager) InjectTraceContext(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

// ExtractTraceContext extracts trace context from a carrier
func (om *ObservabilityManager) ExtractTraceContext(ctx context.Context, carrier map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}

// TraceHTTPRequest creates a middleware for tracing HTTP requests
func (om *ObservabilityManager) TraceHTTPRequest(next func(ctx context.Context, req interface{}) (interface{}, error)) func(ctx context.Context, req interface{}) (interface{}, error) {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		ctx, span := om.StartSpan(ctx, "http_request",
			attribute.String("component", "http"),
		)
		defer span.End()

		start := time.Now()
		result, err := next(ctx, req)
		duration := time.Since(start)

		if err != nil {
			om.SetSpanError(ctx, err)
			om.RecordError(ctx, "http_error", "handler")
		}

		om.RecordRequest(ctx, "POST", "/mcp", getStatusFromError(err), duration)
		return result, err
	}
}

// TraceDockerOperation creates a middleware for tracing Docker operations
func (om *ObservabilityManager) TraceDockerOperation(operation string, next func(ctx context.Context) (interface{}, error)) func(ctx context.Context) (interface{}, error) {
	return func(ctx context.Context) (interface{}, error) {
		ctx, span := om.StartSpan(ctx, fmt.Sprintf("docker_%s", operation),
			attribute.String("component", "docker"),
			attribute.String("operation", operation),
		)
		defer span.End()

		start := time.Now()
		result, err := next(ctx)
		duration := time.Since(start)

		status := "success"
		if err != nil {
			status = "error"
			om.SetSpanError(ctx, err)
			om.RecordError(ctx, "docker_error", "operation")
		}

		om.RecordOperation(ctx, operation, status, duration)
		return result, err
	}
}

// TraceSessionOperation creates a middleware for tracing session operations
func (om *ObservabilityManager) TraceSessionOperation(operation string, sessionID string, next func(ctx context.Context) (interface{}, error)) func(ctx context.Context) (interface{}, error) {
	return func(ctx context.Context) (interface{}, error) {
		ctx, span := om.StartSpan(ctx, fmt.Sprintf("session_%s", operation),
			attribute.String("component", "session"),
			attribute.String("operation", operation),
			attribute.String("session_id", sessionID),
		)
		defer span.End()

		start := time.Now()
		result, err := next(ctx)
		duration := time.Since(start)

		status := "success"
		if err != nil {
			status = "error"
			om.SetSpanError(ctx, err)
			om.RecordError(ctx, "session_error", "operation")
		}

		om.RecordOperation(ctx, fmt.Sprintf("session_%s", operation), status, duration)
		return result, err
	}
}

// Shutdown gracefully shuts down the observability system
func (om *ObservabilityManager) Shutdown(ctx context.Context) error {
	var err error

	if om.tracerProvider != nil {
		if shutdownErr := om.tracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown tracer provider: %w", shutdownErr)
		}
	}

	if om.meterProvider != nil {
		if shutdownErr := om.meterProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; failed to shutdown meter provider: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("failed to shutdown meter provider: %w", shutdownErr)
			}
		}
	}

	om.logger.Info().Msg("Observability system shutdown")
	return err
}

// getStatusFromError converts an error to a status string
func getStatusFromError(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

// TraceableFunc is a function that can be traced
type TraceableFunc func(ctx context.Context) (interface{}, error)

// WithTracing wraps a function with tracing
func (om *ObservabilityManager) WithTracing(operationName string, fn TraceableFunc) TraceableFunc {
	return func(ctx context.Context) (interface{}, error) {
		ctx, span := om.StartSpan(ctx, operationName)
		defer span.End()

		result, err := fn(ctx)
		if err != nil {
			om.SetSpanError(ctx, err)
		}

		return result, err
	}
}

// BatchTraceExporter allows batching trace exports for better performance
type BatchTraceExporter struct {
	exporter   sdktrace.SpanExporter
	batchSize  int
	timeout    time.Duration
	spans      []sdktrace.ReadOnlySpan
	lastExport time.Time
	mutex      sync.Mutex
}

// NewBatchTraceExporter creates a new batch trace exporter
func NewBatchTraceExporter(exporter sdktrace.SpanExporter, batchSize int, timeout time.Duration) *BatchTraceExporter {
	return &BatchTraceExporter{
		exporter:  exporter,
		batchSize: batchSize,
		timeout:   timeout,
		spans:     make([]sdktrace.ReadOnlySpan, 0, batchSize),
	}
}

// Export implements the SpanExporter interface
func (bte *BatchTraceExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	bte.mutex.Lock()
	defer bte.mutex.Unlock()

	bte.spans = append(bte.spans, spans...)

	// Export if batch is full or timeout reached
	if len(bte.spans) >= bte.batchSize || time.Since(bte.lastExport) > bte.timeout {
		return bte.flush(ctx)
	}

	return nil
}

// flush exports all pending spans
func (bte *BatchTraceExporter) flush(ctx context.Context) error {
	if len(bte.spans) == 0 {
		return nil
	}

	err := bte.exporter.ExportSpans(ctx, bte.spans)
	bte.spans = bte.spans[:0] // Clear the slice but keep capacity
	bte.lastExport = time.Now()

	return err
}

// Shutdown implements the SpanExporter interface
func (bte *BatchTraceExporter) Shutdown(ctx context.Context) error {
	bte.mutex.Lock()
	defer bte.mutex.Unlock()

	if err := bte.flush(ctx); err != nil {
		return err
	}

	return bte.exporter.Shutdown(ctx)
}
