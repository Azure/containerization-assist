package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Manager coordinates tracing and metrics
type Manager struct {
	config  *Config
	tracing *TracingManager
	metrics *MetricsManager

	// System monitoring
	systemMonitor *SystemMonitor
}

// NewManager creates a new telemetry manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		config:  config,
		tracing: NewTracingManager(config),
		metrics: NewMetricsManager(config),
	}
}

// Initialize initializes the telemetry system
func (m *Manager) Initialize(ctx context.Context) error {
	// Load configuration from environment
	m.config.LoadFromEnv()

	// Validate configuration
	if err := m.config.Validate(); err != nil {
		return errors.NewError().Code(errors.CodeValidationFailed).Message("invalid telemetry configuration").Cause(err).Build()
	}

	// Initialize tracing
	if err := m.tracing.Initialize(ctx); err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to initialize tracing").Cause(err).Build()
	}

	// Create resource for metrics (reuse tracing resource creation logic)
	resource, err := m.createResource()
	if err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to create resource").Cause(err).Build()
	}

	// Initialize metrics
	if err := m.metrics.Initialize(ctx, resource); err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to initialize metrics").Cause(err).Build()
	}

	// Start system monitoring
	if m.config.MetricsEnabled {
		m.systemMonitor = NewSystemMonitor(m.metrics)
		m.systemMonitor.Start(ctx)
	}

	return nil
}

// Shutdown gracefully shuts down the telemetry system
func (m *Manager) Shutdown(ctx context.Context) error {
	// Stop system monitoring
	if m.systemMonitor != nil {
		m.systemMonitor.Stop()
	}

	// Shutdown metrics
	if err := m.metrics.Shutdown(ctx); err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to shutdown metrics").Cause(err).Build()
	}

	// Shutdown tracing
	if err := m.tracing.Shutdown(ctx); err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to shutdown tracing").Cause(err).Build()
	}

	return nil
}

// Tracing returns the tracing manager
func (m *Manager) Tracing() *TracingManager {
	return m.tracing
}

// Metrics returns the metrics manager
func (m *Manager) Metrics() *MetricsManager {
	return m.metrics
}

// Config returns the telemetry configuration
func (m *Manager) Config() *Config {
	return m.config
}

// createResource creates a shared OpenTelemetry resource
func (m *Manager) createResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", m.config.ServiceName),
		attribute.String("service.version", m.config.ServiceVersion),
		attribute.String("deployment.environment", m.config.Environment),
		attribute.String("telemetry.sdk.name", "opentelemetry"),
		attribute.String("telemetry.sdk.language", "go"),
	}

	// Add custom resource attributes
	for key, value := range m.config.ResourceAttributes {
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

// High-level instrumentation methods that combine tracing and metrics

// InstrumentOperation wraps an operation with both tracing and metrics
func (m *Manager) InstrumentOperation(ctx context.Context, operationName string, operationType string, fn func(context.Context) error) error {
	// Start tracing span
	ctx, span := m.tracing.StartSpan(ctx, operationName)
	defer span.End()

	// Add operation type attribute
	m.tracing.AddSpanAttributes(span, attribute.String("operation.type", operationType))

	// Record start time for metrics
	start := time.Now()

	// Execute operation
	err := fn(ctx)

	// Record duration
	duration := time.Since(start)

	// Record error in span if present
	if err != nil {
		m.tracing.RecordError(span, err)
	}

	// Record metrics based on operation type
	switch operationType {
	case "tool_execution":
		toolName := getToolNameFromOperation(operationName)
		m.metrics.RecordToolExecution(ctx, toolName, duration, err)
	case "pipeline_execution":
		pipelineType := getPipelineTypeFromOperation(operationName)
		m.metrics.RecordPipelineExecution(ctx, pipelineType, duration, 1, err)
	}

	return err
}

// InstrumentToolExecution wraps tool execution with full telemetry
func (m *Manager) InstrumentToolExecution(ctx context.Context, toolName string, fn func(context.Context) error) error {
	return m.InstrumentOperation(ctx, fmt.Sprintf("tool.%s", toolName), "tool_execution", func(ctx context.Context) error {
		span := m.tracing.SpanFromContext(ctx)
		m.tracing.AddSpanAttributes(span, attribute.String("tool.name", toolName))
		return fn(ctx)
	})
}

// InstrumentPipelineStage wraps pipeline stage execution with full telemetry
func (m *Manager) InstrumentPipelineStage(ctx context.Context, pipelineName, stageName string, fn func(context.Context) error) error {
	return m.InstrumentOperation(ctx, fmt.Sprintf("pipeline.%s.%s", pipelineName, stageName), "pipeline_stage", func(ctx context.Context) error {
		span := m.tracing.SpanFromContext(ctx)
		m.tracing.AddSpanAttributes(span,
			attribute.String("pipeline.name", pipelineName),
			attribute.String("pipeline.stage", stageName),
		)
		return fn(ctx)
	})
}

// InstrumentHTTPRequest wraps HTTP request handling with full telemetry
func (m *Manager) InstrumentHTTPRequest(ctx context.Context, method, path string, fn func(context.Context) (int, error)) (int, error) {
	// Start tracing span
	ctx, span := m.tracing.StartSpan(ctx, fmt.Sprintf("http.%s %s", method, path))
	defer span.End()

	// Add HTTP attributes
	m.tracing.AddSpanAttributes(span,
		attribute.String("http.method", method),
		attribute.String("http.route", path),
	)

	// Record start time
	start := time.Now()

	// Execute request
	statusCode, err := fn(ctx)

	// Record duration
	duration := time.Since(start)

	// Add response attributes
	m.tracing.AddSpanAttributes(span, attribute.Int("http.status_code", statusCode))

	// Record error if present
	if err != nil {
		m.tracing.RecordError(span, err)
	}

	// Record metrics
	m.metrics.RecordHTTPRequest(ctx, method, path, statusCode, duration)

	return statusCode, err
}

// AddContextualAttributes adds contextual information to the current span
func (m *Manager) AddContextualAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := m.tracing.SpanFromContext(ctx)
	m.tracing.AddSpanAttributes(span, attrs...)
}

// RecordEvent records an event in the current span
func (m *Manager) RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := m.tracing.SpanFromContext(ctx)
	m.tracing.AddSpanEvent(span, name, attrs...)
}

// GetTraceID returns the current trace ID as a string
func (m *Manager) GetTraceID(ctx context.Context) string {
	span := m.tracing.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the current span ID as a string
func (m *Manager) GetSpanID(ctx context.Context) string {
	span := m.tracing.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// IsTracingEnabled returns whether tracing is enabled
func (m *Manager) IsTracingEnabled() bool {
	return m.config.TracingEnabled
}

// IsMetricsEnabled returns whether metrics are enabled
func (m *Manager) IsMetricsEnabled() bool {
	return m.config.MetricsEnabled
}

// Helper functions

func getToolNameFromOperation(operationName string) string {
	// Extract tool name from operation like "tool.analyze"
	if len(operationName) > 5 && operationName[:5] == "tool." {
		return operationName[5:]
	}
	return "unknown"
}

func getPipelineTypeFromOperation(operationName string) string {
	// Extract pipeline type from operation like "pipeline.container-build.analyze"
	if len(operationName) > 9 && operationName[:9] == "pipeline." {
		// Find the first dot after "pipeline."
		if idx := func() int {
			for i := 9; i < len(operationName); i++ {
				if operationName[i] == '.' {
					return i
				}
			}
			return len(operationName)
		}(); idx > 9 {
			return operationName[9:idx]
		}
		return operationName[9:]
	}
	return "unknown"
}

// SystemMonitor monitors system-level metrics
type SystemMonitor struct {
	metrics *MetricsManager
	stopCh  chan struct{}
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(metrics *MetricsManager) *SystemMonitor {
	return &SystemMonitor{
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}
}

// Start starts the system monitor
func (sm *SystemMonitor) Start(ctx context.Context) {
	go sm.monitorLoop(ctx)
}

// Stop stops the system monitor
func (sm *SystemMonitor) Stop() {
	close(sm.stopCh)
}

// monitorLoop runs the monitoring loop
func (sm *SystemMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.collectSystemMetrics(ctx)
		case <-sm.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// collectSystemMetrics collects system-level metrics
func (sm *SystemMonitor) collectSystemMetrics(ctx context.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Note: These would be recorded via observable instruments in a real implementation
	// For now, we'll track them manually

	// Memory usage
	// sm.metrics.SystemMetrics.MemoryUsage would be updated via callback

	// Goroutine count
	goroutineCount := runtime.NumGoroutine()
	_ = goroutineCount // Would be recorded via observable gauge

	// GC stats would be collected via runtime hooks
}
