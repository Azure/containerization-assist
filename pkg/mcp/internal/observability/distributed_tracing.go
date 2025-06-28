package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// DistributedTracingConfig holds configuration for distributed tracing
type DistributedTracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SampleRate     float64
	MaxTraceSize   int
	BatchTimeout   time.Duration
	ExportTimeout  time.Duration
	MaxExportBatch int
	MaxQueueSize   int
	Headers        map[string]string
}

// TracingManager manages distributed tracing
type TracingManager struct {
	tracer         trace.Tracer
	provider       *sdktrace.TracerProvider
	propagator     propagation.TextMapPropagator
	config         *DistributedTracingConfig
	spanProcessors []SpanProcessor
}

// SpanProcessor interface for custom span processing
type SpanProcessor interface {
	ProcessSpan(span sdktrace.ReadOnlySpan)
}

// TraceContext carries trace information across service boundaries
type TraceContext struct {
	TraceID      string                 `json:"trace_id"`
	SpanID       string                 `json:"span_id"`
	ParentSpanID string                 `json:"parent_span_id,omitempty"`
	Flags        byte                   `json:"flags"`
	Baggage      map[string]string      `json:"baggage,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// SpanEnricher adds contextual information to spans
type SpanEnricher struct {
	userID      func(context.Context) string
	sessionID   func(context.Context) string
	requestID   func(context.Context) string
	environment string
}

// NewDistributedTracingManager creates a new distributed tracing manager
func NewDistributedTracingManager(config *DistributedTracingConfig) (*TracingManager, error) {
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
			attribute.String("service.namespace", "mcp"),
			attribute.String("service.instance.id", generateInstanceID()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		otlptracehttp.WithTimeout(config.ExportTimeout),
	}

	// Add headers if provided
	if len(config.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(config.Headers))
	}

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(opts...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create sampler
	sampler := sdktrace.TraceIDRatioBased(config.SampleRate)

	// Create span processors
	batchProcessor := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithBatchTimeout(config.BatchTimeout),
		sdktrace.WithMaxExportBatchSize(config.MaxExportBatch),
		sdktrace.WithMaxQueueSize(config.MaxQueueSize),
	)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithSpanProcessor(&customSpanProcessor{}),
	)

	// Set as global provider
	otel.SetTracerProvider(tp)

	// Create propagator
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	// Create tracer
	tracer := tp.Tracer(
		config.ServiceName,
		trace.WithInstrumentationVersion(config.ServiceVersion),
		trace.WithSchemaURL(semconv.SchemaURL),
	)

	return &TracingManager{
		tracer:     tracer,
		provider:   tp,
		propagator: propagator,
		config:     config,
	}, nil
}

// StartSpan starts a new span with automatic enrichment
func (tm *TracingManager) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Add default attributes
	defaultOpts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("span.type", "internal"),
			attribute.String("service.name", tm.config.ServiceName),
			attribute.String("environment", tm.config.Environment),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	}

	// Combine with provided options
	allOpts := append(defaultOpts, opts...)

	// Start span
	ctx, span := tm.tracer.Start(ctx, name, allOpts...)

	// Enrich span with context
	tm.enrichSpan(ctx, span)

	return ctx, span
}

// StartToolSpan starts a span for tool execution
func (tm *TracingManager) StartToolSpan(ctx context.Context, toolName string, operation string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("tool.%s.%s", toolName, operation)

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
			attribute.String("tool.operation", operation),
			attribute.String("span.type", "tool"),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	}

	return tm.StartSpan(ctx, spanName, opts...)
}

// StartHTTPSpan starts a span for HTTP operations
func (tm *TracingManager) StartHTTPSpan(ctx context.Context, method, path string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("%s %s", method, path)

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("span.type", "http"),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	}

	return tm.StartSpan(ctx, spanName, opts...)
}

// StartDatabaseSpan starts a span for database operations
func (tm *TracingManager) StartDatabaseSpan(ctx context.Context, dbType, operation, table string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("db.%s.%s", dbType, operation)

	opts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("db.type", dbType),
			attribute.String("db.operation", operation),
			attribute.String("db.table", table),
			attribute.String("span.type", "database"),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	}

	return tm.StartSpan(ctx, spanName, opts...)
}

// RecordError records an error in the current span
func (tm *TracingManager) RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err, opts...)
		span.SetStatus(codes.Error, err.Error())

		// Add error attributes
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
			attribute.String("error.message", err.Error()),
		)
	}
}

// AddEvent adds an event to the current span
func (tm *TracingManager) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetAttributes sets attributes on the current span
func (tm *TracingManager) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// InjectContext injects trace context into a carrier
func (tm *TracingManager) InjectContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	tm.propagator.Inject(ctx, carrier)
}

// ExtractContext extracts trace context from a carrier
func (tm *TracingManager) ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return tm.propagator.Extract(ctx, carrier)
}

// GetTraceContext returns the current trace context
func (tm *TracingManager) GetTraceContext(ctx context.Context) *TraceContext {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}

	spanCtx := span.SpanContext()
	tc := &TraceContext{
		TraceID: spanCtx.TraceID().String(),
		SpanID:  spanCtx.SpanID().String(),
		Flags:   byte(spanCtx.TraceFlags()),
		Baggage: make(map[string]string),
	}

	// Extract baggage
	bag := baggage.FromContext(ctx)
	for _, member := range bag.Members() {
		tc.Baggage[member.Key()] = member.Value()
	}

	return tc
}

// CreateChildContext creates a child context with trace information
func (tm *TracingManager) CreateChildContext(parent *TraceContext) context.Context {
	// Parse trace ID
	traceID, err := trace.TraceIDFromHex(parent.TraceID)
	if err != nil {
		return context.Background()
	}

	// Parse span ID as parent
	spanID, err := trace.SpanIDFromHex(parent.SpanID)
	if err != nil {
		return context.Background()
	}

	// Create span context
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.TraceFlags(parent.Flags),
		Remote:     true,
	})

	// Create context with span
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanCtx)

	// Add baggage
	if len(parent.Baggage) > 0 {
		var members []baggage.Member
		for k, v := range parent.Baggage {
			member, _ := baggage.NewMember(k, v)
			members = append(members, member)
		}
		bag, _ := baggage.New(members...)
		ctx = baggage.ContextWithBaggage(ctx, bag)
	}

	return ctx
}

// Shutdown gracefully shuts down the tracing system
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	return tm.provider.Shutdown(ctx)
}

// enrichSpan adds contextual information to spans
func (tm *TracingManager) enrichSpan(ctx context.Context, span trace.Span) {
	// Add baggage as attributes
	bag := baggage.FromContext(ctx)
	for _, member := range bag.Members() {
		span.SetAttributes(attribute.String(
			fmt.Sprintf("baggage.%s", member.Key()),
			member.Value(),
		))
	}

	// Add custom enrichments
	if enricher := getSpanEnricher(ctx); enricher != nil {
		if userID := enricher.userID(ctx); userID != "" {
			span.SetAttributes(attribute.String("user.id", userID))
		}
		if sessionID := enricher.sessionID(ctx); sessionID != "" {
			span.SetAttributes(attribute.String("session.id", sessionID))
		}
		if requestID := enricher.requestID(ctx); requestID != "" {
			span.SetAttributes(attribute.String("request.id", requestID))
		}
	}
}

// customSpanProcessor processes spans for custom logic
type customSpanProcessor struct{}

func (p *customSpanProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	// Add start timestamp
	s.SetAttributes(attribute.Int64("span.start_time_unix", time.Now().Unix()))
}

func (p *customSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// Log slow spans for debugging
	duration := s.EndTime().Sub(s.StartTime())
	if duration > 1*time.Second {
		// Slow span detected - consider using proper logging instead of fmt.Printf
	}

	// Collect span metrics
	updateSpanMetrics(s)
}

func (p *customSpanProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *customSpanProcessor) ForceFlush(ctx context.Context) error {
	return nil
}

// Helper functions

func generateInstanceID() string {
	return fmt.Sprintf("%s-%d", getHostname(), time.Now().Unix())
}

func getHostname() string {
	// Simplified - in production use os.Hostname()
	return "mcp-instance"
}

func getSpanEnricher(ctx context.Context) *SpanEnricher {
	// Would retrieve from context
	return nil
}

func updateSpanMetrics(span sdktrace.ReadOnlySpan) {
	// Update metrics based on span data
	// This would integrate with the telemetry manager
}

// TracingMiddleware creates middleware for automatic tracing
func TracingMiddleware(tm *TracingManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			ctx := tm.ExtractContext(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start HTTP span
			ctx, span := tm.StartHTTPSpan(ctx, r.Method, r.URL.Path)
			defer span.End()

			// Add request attributes
			span.SetAttributes(
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
				attribute.String("http.host", r.Host),
			)

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			// Call next handler
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Set response attributes
			span.SetAttributes(
				attribute.Int("http.status_code", wrapped.statusCode),
			)

			// Set span status based on HTTP status
			if wrapped.statusCode >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", wrapped.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
