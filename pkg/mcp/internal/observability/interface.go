package observability

import (
	"context"
	"time"
)

// ObservabilityMetricsCollector defines the interface for metrics collection
// This allows for different implementations (Prometheus, no-op, etc.)
type ObservabilityMetricsCollector interface {
	// Tool metrics
	IncrementToolExecution(toolName string)
	IncrementToolError(toolName string, errorType string)
	RecordToolDuration(toolName string, duration time.Duration)

	// Session metrics
	IncrementActiveSessions()
	DecrementActiveSessions()
	RecordSessionDuration(duration time.Duration)

	// System metrics
	RecordMemoryUsage(bytes int64)
	RecordDiskUsage(bytes int64)
	RecordCPUUsage(percent float64)

	// Custom metrics
	RecordCustomMetric(name string, value float64, labels map[string]string)
}

// TracingProvider defines the interface for distributed tracing
type TracingProvider interface {
	StartSpan(ctx context.Context, name string) (context.Context, SpanContext)
	EndSpan(spanCtx SpanContext)
	AddSpanAttribute(spanCtx SpanContext, key string, value interface{})
	SetSpanStatus(spanCtx SpanContext, code StatusCode, message string)
}

// SpanContext represents a tracing span
type SpanContext interface {
	TraceID() string
	SpanID() string
	SetAttribute(key string, value interface{})
	SetStatus(code StatusCode, message string)
	End()
}

// StatusCode represents span status
type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

// ObservabilityManager manages all observability features
type ObservabilityManager interface {
	Metrics() ObservabilityMetricsCollector
	Tracing() TracingProvider
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsEnabled() bool
}

// NoOpMetricsCollector provides a no-op implementation for minimal builds
type NoOpMetricsCollector struct{}

func (n *NoOpMetricsCollector) IncrementToolExecution(toolName string)                     {}
func (n *NoOpMetricsCollector) IncrementToolError(toolName string, errorType string)       {}
func (n *NoOpMetricsCollector) RecordToolDuration(toolName string, duration time.Duration) {}
func (n *NoOpMetricsCollector) IncrementActiveSessions()                                   {}
func (n *NoOpMetricsCollector) DecrementActiveSessions()                                   {}
func (n *NoOpMetricsCollector) RecordSessionDuration(duration time.Duration)               {}
func (n *NoOpMetricsCollector) RecordMemoryUsage(bytes int64)                              {}
func (n *NoOpMetricsCollector) RecordDiskUsage(bytes int64)                                {}
func (n *NoOpMetricsCollector) RecordCPUUsage(percent float64)                             {}
func (n *NoOpMetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) {
}

// NoOpTracingProvider provides a no-op implementation for minimal builds
type NoOpTracingProvider struct{}

func (n *NoOpTracingProvider) StartSpan(ctx context.Context, name string) (context.Context, SpanContext) {
	return ctx, &NoOpSpanContext{}
}
func (n *NoOpTracingProvider) EndSpan(spanCtx SpanContext)                                         {}
func (n *NoOpTracingProvider) AddSpanAttribute(spanCtx SpanContext, key string, value interface{}) {}
func (n *NoOpTracingProvider) SetSpanStatus(spanCtx SpanContext, code StatusCode, message string)  {}

// NoOpSpanContext provides a no-op span implementation
type NoOpSpanContext struct{}

func (n *NoOpSpanContext) TraceID() string                            { return "" }
func (n *NoOpSpanContext) SpanID() string                             { return "" }
func (n *NoOpSpanContext) SetAttribute(key string, value interface{}) {}
func (n *NoOpSpanContext) SetStatus(code StatusCode, message string)  {}
func (n *NoOpSpanContext) End()                                       {}

// NoOpObservabilityManager provides a no-op implementation for minimal builds
type NoOpObservabilityManager struct {
	metrics *NoOpMetricsCollector
	tracing *NoOpTracingProvider
}

func NewNoOpObservabilityManager() *NoOpObservabilityManager {
	return &NoOpObservabilityManager{
		metrics: &NoOpMetricsCollector{},
		tracing: &NoOpTracingProvider{},
	}
}

func (n *NoOpObservabilityManager) Metrics() ObservabilityMetricsCollector { return n.metrics }
func (n *NoOpObservabilityManager) Tracing() TracingProvider               { return n.tracing }
func (n *NoOpObservabilityManager) Start(ctx context.Context) error        { return nil }
func (n *NoOpObservabilityManager) Stop(ctx context.Context) error         { return nil }
func (n *NoOpObservabilityManager) IsEnabled() bool                        { return false }
