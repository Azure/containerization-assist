package observability

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// EnhancedMetricsCollector provides enhanced metrics collection for MCP operations
type EnhancedMetricsCollector struct {
	logger zerolog.Logger
	config *types.ObservabilityConfig
	meter  metric.Meter
	mu     sync.RWMutex

	// Core metrics
	toolExecutions  metric.Int64Counter
	toolDuration    metric.Float64Histogram
	toolErrors      metric.Int64Counter
	sessionDuration metric.Float64Histogram
	sessionCount    metric.Int64Counter
	resourceUsage   metric.Float64Gauge

	// Performance metrics
	concurrentTools metric.Int64UpDownCounter
	memoryUsage     metric.Int64Gauge
	cpuUsage        metric.Float64Gauge
	diskUsage       metric.Int64Gauge

	// Business metrics
	successRate metric.Float64Gauge
	errorRate   metric.Float64Gauge
	throughput  metric.Float64Gauge
	latencyP95  metric.Float64Gauge
	latencyP99  metric.Float64Gauge

	// Custom metrics registry
	customMetrics map[string]interface{}
}

// NewEnhancedMetricsCollector creates a new enhanced metrics collector
func NewEnhancedMetricsCollector(logger zerolog.Logger, config *types.ObservabilityConfig) (*EnhancedMetricsCollector, error) {
	meter := otel.Meter("container-copilot-mcp")

	mc := &EnhancedMetricsCollector{
		logger:        logger.With().Str("component", "metrics").Logger(),
		config:        config,
		meter:         meter,
		customMetrics: make(map[string]interface{}),
	}

	if err := mc.initializeMetrics(); err != nil {
		return nil, err
	}

	return mc, nil
}

// initializeMetrics creates all metric instruments
func (mc *EnhancedMetricsCollector) initializeMetrics() error {
	var err error

	// Tool execution metrics
	mc.toolExecutions, err = mc.meter.Int64Counter(
		"mcp_tool_executions_total",
		metric.WithDescription("Total number of tool executions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Get histogram buckets from config
	buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0}
	if toolConfig, exists := mc.config.OpenTelemetry.Metrics.CustomMetrics["tool_executions"]; exists {
		if len(toolConfig.HistogramBuckets) > 0 {
			buckets = toolConfig.HistogramBuckets
		}
	}

	mc.toolDuration, err = mc.meter.Float64Histogram(
		"mcp_tool_execution_duration_seconds",
		metric.WithDescription("Tool execution duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
	if err != nil {
		return err
	}

	mc.toolErrors, err = mc.meter.Int64Counter(
		"mcp_tool_errors_total",
		metric.WithDescription("Total number of tool execution errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Session metrics
	sessionBuckets := []float64{1, 5, 10, 30, 60, 300, 600}
	if sessionConfig, exists := mc.config.OpenTelemetry.Metrics.CustomMetrics["session_metrics"]; exists {
		if len(sessionConfig.HistogramBuckets) > 0 {
			sessionBuckets = sessionConfig.HistogramBuckets
		}
	}

	mc.sessionDuration, err = mc.meter.Float64Histogram(
		"mcp_session_duration_seconds",
		metric.WithDescription("Session duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(sessionBuckets...),
	)
	if err != nil {
		return err
	}

	mc.sessionCount, err = mc.meter.Int64Counter(
		"mcp_sessions_total",
		metric.WithDescription("Total number of sessions"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Resource usage metrics
	mc.resourceUsage, err = mc.meter.Float64Gauge(
		"mcp_resource_usage_ratio",
		metric.WithDescription("Resource usage ratio (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Performance metrics
	mc.concurrentTools, err = mc.meter.Int64UpDownCounter(
		"mcp_concurrent_tools",
		metric.WithDescription("Number of concurrently executing tools"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mc.memoryUsage, err = mc.meter.Int64Gauge(
		"mcp_memory_usage_bytes",
		metric.WithDescription("Memory usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	mc.cpuUsage, err = mc.meter.Float64Gauge(
		"mcp_cpu_usage_ratio",
		metric.WithDescription("CPU usage ratio (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mc.diskUsage, err = mc.meter.Int64Gauge(
		"mcp_disk_usage_bytes",
		metric.WithDescription("Disk usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	// Business metrics
	mc.successRate, err = mc.meter.Float64Gauge(
		"mcp_success_rate",
		metric.WithDescription("Tool execution success rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mc.errorRate, err = mc.meter.Float64Gauge(
		"mcp_error_rate",
		metric.WithDescription("Tool execution error rate"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	mc.throughput, err = mc.meter.Float64Gauge(
		"mcp_throughput_ops_per_second",
		metric.WithDescription("Operations per second"),
		metric.WithUnit("1/s"),
	)
	if err != nil {
		return err
	}

	mc.latencyP95, err = mc.meter.Float64Gauge(
		"mcp_latency_p95_seconds",
		metric.WithDescription("95th percentile latency"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	mc.latencyP99, err = mc.meter.Float64Gauge(
		"mcp_latency_p99_seconds",
		metric.WithDescription("99th percentile latency"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordToolExecution records a tool execution event
func (mc *EnhancedMetricsCollector) RecordToolExecution(ctx context.Context, toolName string, duration time.Duration, success bool, errorCode string) {
	labels := []attribute.KeyValue{
		attribute.String("tool_name", toolName),
		attribute.Bool("success", success),
	}

	if errorCode != "" {
		labels = append(labels, attribute.String("error_code", errorCode))
	}

	// Record execution count
	mc.toolExecutions.Add(ctx, 1, metric.WithAttributes(labels...))

	// Record duration
	mc.toolDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(labels...))

	// Record error if applicable
	if !success {
		errorLabels := []attribute.KeyValue{
			attribute.String("tool_name", toolName),
			attribute.String("error_code", errorCode),
		}
		mc.toolErrors.Add(ctx, 1, metric.WithAttributes(errorLabels...))
	}

	mc.logger.Debug().
		Str("tool", toolName).
		Dur("duration", duration).
		Bool("success", success).
		Str("error_code", errorCode).
		Msg("Recorded tool execution metrics")
}

// RecordSessionStart records the start of a session
func (mc *EnhancedMetricsCollector) RecordSessionStart(ctx context.Context, sessionID string) {
	labels := []attribute.KeyValue{
		attribute.String("session_id", sessionID),
	}

	mc.sessionCount.Add(ctx, 1, metric.WithAttributes(labels...))
	mc.concurrentTools.Add(ctx, 1, metric.WithAttributes(labels...))
}

// RecordSessionEnd records the end of a session
func (mc *EnhancedMetricsCollector) RecordSessionEnd(ctx context.Context, sessionID string, duration time.Duration) {
	labels := []attribute.KeyValue{
		attribute.String("session_id", sessionID),
	}

	mc.sessionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(labels...))
	mc.concurrentTools.Add(ctx, -1, metric.WithAttributes(labels...))
}

// UpdateResourceUsage updates resource usage metrics
func (mc *EnhancedMetricsCollector) UpdateResourceUsage(ctx context.Context, resourceType string, usage float64) {
	labels := []attribute.KeyValue{
		attribute.String("resource_type", resourceType),
	}

	mc.resourceUsage.Record(ctx, usage, metric.WithAttributes(labels...))
}

// UpdateSystemMetrics updates system-level performance metrics
func (mc *EnhancedMetricsCollector) UpdateSystemMetrics(ctx context.Context, memoryBytes int64, cpuRatio float64, diskBytes int64) {
	mc.memoryUsage.Record(ctx, memoryBytes)
	mc.cpuUsage.Record(ctx, cpuRatio)
	mc.diskUsage.Record(ctx, diskBytes)
}

// UpdateBusinessMetrics updates business-level metrics
func (mc *EnhancedMetricsCollector) UpdateBusinessMetrics(ctx context.Context, successRate, errorRate, throughput, p95Latency, p99Latency float64) {
	mc.successRate.Record(ctx, successRate)
	mc.errorRate.Record(ctx, errorRate)
	mc.throughput.Record(ctx, throughput)
	mc.latencyP95.Record(ctx, p95Latency)
	mc.latencyP99.Record(ctx, p99Latency)
}

// CreateCustomCounter creates a custom counter metric
func (mc *EnhancedMetricsCollector) CreateCustomCounter(name, description, unit string) (metric.Int64Counter, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if instrument, exists := mc.customMetrics[name]; exists {
		if counter, ok := instrument.(metric.Int64Counter); ok {
			return counter, nil
		}
	}

	counter, err := mc.meter.Int64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return nil, err
	}

	mc.customMetrics[name] = counter
	return counter, nil
}

// CreateCustomGauge creates a custom gauge metric
func (mc *EnhancedMetricsCollector) CreateCustomGauge(name, description, unit string) (metric.Float64Gauge, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if instrument, exists := mc.customMetrics[name]; exists {
		if gauge, ok := instrument.(metric.Float64Gauge); ok {
			return gauge, nil
		}
	}

	gauge, err := mc.meter.Float64Gauge(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return nil, err
	}

	mc.customMetrics[name] = gauge
	return gauge, nil
}

// CreateCustomHistogram creates a custom histogram metric
func (mc *EnhancedMetricsCollector) CreateCustomHistogram(name, description, unit string, buckets []float64) (metric.Float64Histogram, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if instrument, exists := mc.customMetrics[name]; exists {
		if histogram, ok := instrument.(metric.Float64Histogram); ok {
			return histogram, nil
		}
	}

	histogram, err := mc.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
	if err != nil {
		return nil, err
	}

	mc.customMetrics[name] = histogram
	return histogram, nil
}

// GetMeter returns the OpenTelemetry meter for advanced usage
func (mc *EnhancedMetricsCollector) GetMeter() metric.Meter {
	return mc.meter
}

// Close performs cleanup when shutting down
func (mc *EnhancedMetricsCollector) Close() error {
	mc.logger.Info().Msg("Metrics collector shutting down")
	return nil
}
