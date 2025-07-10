package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// MetricsManager manages OpenTelemetry metrics
type MetricsManager struct {
	config   *Config
	provider *metric.MeterProvider
	meter    metric.Meter

	// Core metrics
	ToolExecutionDuration metric.Float64Histogram
	ToolExecutionCounter  metric.Int64Counter
	ToolErrorCounter      metric.Int64Counter

	PipelineExecutionDuration metric.Float64Histogram
	PipelineStageCounter      metric.Int64Counter
	PipelineErrorCounter      metric.Int64Counter

	SessionCounter  metric.Int64Counter
	SessionDuration metric.Float64Histogram

	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestCounter  metric.Int64Counter

	SystemMetrics *SystemMetrics
}

// SystemMetrics holds system-level metrics
type SystemMetrics struct {
	MemoryUsage    metric.Int64ObservableGauge
	CPUUsage       metric.Float64ObservableGauge
	GoroutineCount metric.Int64ObservableGauge
	GCDuration     metric.Float64Histogram
}

// NewMetricsManager creates a new metrics manager
func NewMetricsManager(config *Config) *MetricsManager {
	return &MetricsManager{
		config: config,
	}
}

// Initialize initializes the metrics system
func (mm *MetricsManager) Initialize(ctx context.Context, resource *resource.Resource) error {
	if !mm.config.MetricsEnabled {
		return nil
	}

	// Create exporter
	exporter, err := mm.createExporter()
	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	// Create meter provider
	provider := metric.NewMeterProvider(
		metric.WithResource(resource),
		metric.WithReader(exporter),
	)

	// Set global meter provider
	otel.SetMeterProvider(provider)

	mm.provider = provider
	mm.meter = provider.Meter(mm.config.ServiceName)

	// Initialize metrics
	if err := mm.initializeMetrics(); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the metrics system
func (mm *MetricsManager) Shutdown(ctx context.Context) error {
	if mm.provider != nil {
		return mm.provider.Shutdown(ctx)
	}
	return nil
}

// initializeMetrics creates all the metric instruments
func (mm *MetricsManager) initializeMetrics() error {
	var err error

	// Tool metrics
	mm.ToolExecutionDuration, err = mm.meter.Float64Histogram(
		"tool_execution_duration_seconds",
		metric.WithDescription("Duration of tool execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mm.ToolExecutionCounter, err = mm.meter.Int64Counter(
		"tool_execution_total",
		metric.WithDescription("Total number of tool executions"),
	)
	if err != nil {
		return err
	}

	mm.ToolErrorCounter, err = mm.meter.Int64Counter(
		"tool_errors_total",
		metric.WithDescription("Total number of tool execution errors"),
	)
	if err != nil {
		return err
	}

	// Pipeline metrics
	mm.PipelineExecutionDuration, err = mm.meter.Float64Histogram(
		"pipeline_execution_duration_seconds",
		metric.WithDescription("Duration of pipeline execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mm.PipelineStageCounter, err = mm.meter.Int64Counter(
		"pipeline_stage_execution_total",
		metric.WithDescription("Total number of pipeline stage executions"),
	)
	if err != nil {
		return err
	}

	mm.PipelineErrorCounter, err = mm.meter.Int64Counter(
		"pipeline_errors_total",
		metric.WithDescription("Total number of pipeline execution errors"),
	)
	if err != nil {
		return err
	}

	// Session metrics
	mm.SessionCounter, err = mm.meter.Int64Counter(
		"session_total",
		metric.WithDescription("Total number of sessions created"),
	)
	if err != nil {
		return err
	}

	mm.SessionDuration, err = mm.meter.Float64Histogram(
		"session_duration_seconds",
		metric.WithDescription("Duration of sessions in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// HTTP metrics
	mm.HTTPRequestDuration, err = mm.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mm.HTTPRequestCounter, err = mm.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return err
	}

	// Initialize system metrics
	return mm.initializeSystemMetrics()
}

// initializeSystemMetrics creates system-level metrics
func (mm *MetricsManager) initializeSystemMetrics() error {
	var err error

	mm.SystemMetrics = &SystemMetrics{}

	// Memory usage gauge
	mm.SystemMetrics.MemoryUsage, err = mm.meter.Int64ObservableGauge(
		"system_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	// CPU usage gauge
	mm.SystemMetrics.CPUUsage, err = mm.meter.Float64ObservableGauge(
		"system_cpu_usage_percent",
		metric.WithDescription("Current CPU usage percentage"),
		metric.WithUnit("%"),
	)
	if err != nil {
		return err
	}

	// Goroutine count gauge
	mm.SystemMetrics.GoroutineCount, err = mm.meter.Int64ObservableGauge(
		"system_goroutines_count",
		metric.WithDescription("Current number of goroutines"),
	)
	if err != nil {
		return err
	}

	// GC duration histogram
	mm.SystemMetrics.GCDuration, err = mm.meter.Float64Histogram(
		"system_gc_duration_seconds",
		metric.WithDescription("Garbage collection duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	return nil
}

// createExporter creates a metrics exporter
func (mm *MetricsManager) createExporter() (metric.Reader, error) {
	// For development, use stdout exporter
	if mm.config.Environment == "development" {
		return stdoutmetric.New()
	}

	// For production, use Prometheus exporter
	return prometheus.New()
}

// Metric recording methods

// RecordToolExecution records tool execution metrics
func (mm *MetricsManager) RecordToolExecution(ctx context.Context, toolName string, duration time.Duration, err error) {
	if !mm.config.MetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}

	// Record duration
	mm.ToolExecutionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	// Record execution count
	if err != nil {
		attrs = append(attrs, attribute.String("status", "error"))
		mm.ToolErrorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		attrs = append(attrs, attribute.String("status", "success"))
	}

	mm.ToolExecutionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordPipelineExecution records pipeline execution metrics
func (mm *MetricsManager) RecordPipelineExecution(ctx context.Context, pipelineType string, duration time.Duration, stageCount int, err error) {
	if !mm.config.MetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("pipeline.type", pipelineType),
		attribute.Int("pipeline.stages", stageCount),
	}

	// Record duration
	mm.PipelineExecutionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	// Record stage count
	mm.PipelineStageCounter.Add(ctx, int64(stageCount), metric.WithAttributes(attrs...))

	// Record errors
	if err != nil {
		mm.PipelineErrorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordSessionCreation records session creation metrics
func (mm *MetricsManager) RecordSessionCreation(ctx context.Context, sessionType string) {
	if !mm.config.MetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("session.type", sessionType),
	}

	mm.SessionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordSessionDuration records session duration metrics
func (mm *MetricsManager) RecordSessionDuration(ctx context.Context, sessionType string, duration time.Duration) {
	if !mm.config.MetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("session.type", sessionType),
	}

	mm.SessionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordHTTPRequest records HTTP request metrics
func (mm *MetricsManager) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	if !mm.config.MetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.Int("http.status_code", statusCode),
	}

	mm.HTTPRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	mm.HTTPRequestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordGCDuration records garbage collection duration
func (mm *MetricsManager) RecordGCDuration(ctx context.Context, duration time.Duration) {
	if !mm.config.MetricsEnabled {
		return
	}

	mm.SystemMetrics.GCDuration.Record(ctx, duration.Seconds())
}

// High-level instrumentation methods

// InstrumentToolExecution wraps tool execution with metrics
func (mm *MetricsManager) InstrumentToolExecution(ctx context.Context, toolName string, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	mm.RecordToolExecution(ctx, toolName, duration, err)
	return err
}

// InstrumentPipelineExecution wraps pipeline execution with metrics
func (mm *MetricsManager) InstrumentPipelineExecution(ctx context.Context, pipelineType string, stageCount int, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	mm.RecordPipelineExecution(ctx, pipelineType, duration, stageCount, err)
	return err
}

// InstrumentHTTPHandler wraps HTTP handler with metrics
func (mm *MetricsManager) InstrumentHTTPHandler(method, path string, fn func(context.Context) (int, error)) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		start := time.Now()
		statusCode, err := fn(ctx)
		duration := time.Since(start)

		mm.RecordHTTPRequest(ctx, method, path, statusCode, duration)
		return statusCode, err
	}
}
