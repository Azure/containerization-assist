// Package observability provides unified monitoring, tracing, and health infrastructure
// for the MCP components. It consolidates telemetry, distributed tracing, health checks,
// and logging enrichment into a single coherent package.
package observability

import (
	"context"
	"log/slog"
	"time"
)

// Observer provides a unified interface for all observability operations
type Observer interface {
	// Event tracking
	TrackEvent(ctx context.Context, event *Event)
	TrackError(ctx context.Context, err error)

	// Performance monitoring
	StartOperation(ctx context.Context, operation string) *OperationContext
	StartSpan(ctx context.Context, name string) *SpanContext

	// Health monitoring
	RecordHealthCheck(component string, status HealthStatus, latency time.Duration)
	RecordMetric(name string, value float64, tags map[string]string)

	// Counter and gauges
	IncrementCounter(name string, tags map[string]string)
	SetGauge(name string, value float64, tags map[string]string)
	RecordHistogram(name string, value float64, tags map[string]string)

	// Resource monitoring
	RecordResourceUsage(ctx context.Context, resource *ResourceUsage)

	// Structured logging
	Logger() *slog.Logger

	// Reports and analysis
	GetObservabilityReport() *ObservabilityReport

	// Configuration
	SetSamplingRate(rate float64)
	SetLogLevel(level slog.Level)
}

// Event represents a trackable event in the system
type Event struct {
	Name       string                 `json:"name"`
	Type       EventType              `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration,omitempty"`
	WorkflowID string                 `json:"workflow_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Component  string                 `json:"component"`
	Operation  string                 `json:"operation"`
	Success    bool                   `json:"success"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Metrics    map[string]float64     `json:"metrics,omitempty"`
	Tags       map[string]string      `json:"tags,omitempty"`
}

// EventType categorizes different types of events
type EventType string

const (
	EventTypeOperation   EventType = "operation"
	EventTypeWorkflow    EventType = "workflow"
	EventTypeError       EventType = "error"
	EventTypeHealth      EventType = "health"
	EventTypePerformance EventType = "performance"
	EventTypeResource    EventType = "resource"
	EventTypeUser        EventType = "user"
	EventTypeSystem      EventType = "system"
)

// OperationContext provides context for tracking operation performance
type OperationContext struct {
	Name       string
	StartTime  time.Time
	Context    context.Context
	observer   Observer
	properties map[string]interface{}
	metrics    map[string]float64
	tags       map[string]string
}

// SpanContext provides context for distributed tracing
type SpanContext struct {
	TraceID   string
	SpanID    string
	Name      string
	StartTime time.Time
	Context   context.Context
	observer  Observer
	tags      map[string]string
}

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ResourceUsage represents system resource utilization
type ResourceUsage struct {
	Component string             `json:"component"`
	Timestamp time.Time          `json:"timestamp"`
	CPU       *ResourceMetric    `json:"cpu,omitempty"`
	Memory    *ResourceMetric    `json:"memory,omitempty"`
	Disk      *ResourceMetric    `json:"disk,omitempty"`
	Network   *NetworkMetric     `json:"network,omitempty"`
	Custom    map[string]float64 `json:"custom,omitempty"`
}

// ResourceMetric represents a resource utilization metric
type ResourceMetric struct {
	Used      float64 `json:"used"`
	Available float64 `json:"available"`
	Percent   float64 `json:"percent"`
	Unit      string  `json:"unit"`
}

// NetworkMetric represents network utilization metrics
type NetworkMetric struct {
	BytesIn    int64 `json:"bytes_in"`
	BytesOut   int64 `json:"bytes_out"`
	PacketsIn  int64 `json:"packets_in"`
	PacketsOut int64 `json:"packets_out"`
	Errors     int64 `json:"errors"`
}

// ObservabilityReport provides a comprehensive view of system observability
type ObservabilityReport struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Period      TimePeriod `json:"period"`

	// Event statistics
	EventSummary EventSummary `json:"event_summary"`

	// Error analysis
	ErrorAnalysis ErrorAnalysis `json:"error_analysis"`

	// Performance metrics
	Performance PerformanceMetrics `json:"performance"`

	// Health status
	HealthStatus map[string]ComponentHealth `json:"health_status"`

	// Resource utilization
	ResourceUsage ResourceSummary `json:"resource_usage"`

	// Trends and patterns
	Trends TrendAnalysis `json:"trends"`

	// Recommendations
	Recommendations []Recommendation `json:"recommendations"`
}

// TimePeriod represents a time range
type TimePeriod struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// EventSummary provides summary statistics for events
type EventSummary struct {
	TotalEvents       int64               `json:"total_events"`
	EventsByType      map[EventType]int64 `json:"events_by_type"`
	EventsByComponent map[string]int64    `json:"events_by_component"`
	SuccessRate       float64             `json:"success_rate"`
	AvgDuration       time.Duration       `json:"avg_duration"`
}

// ErrorAnalysis provides detailed error analysis
type ErrorAnalysis struct {
	TotalErrors       int64    `json:"total_errors"`
	RecoverableErrors int64    `json:"recoverable_errors"`
	CriticalErrors    int64    `json:"critical_errors"`
	ErrorRate         float64  `json:"error_rate"`
	TopErrors         []string `json:"top_errors"`
}

// PerformanceMetrics provides performance analysis
type PerformanceMetrics struct {
	OperationMetrics     map[string]OperationMetrics `json:"operation_metrics"`
	AvgResponseTime      time.Duration               `json:"avg_response_time"`
	P50ResponseTime      time.Duration               `json:"p50_response_time"`
	P95ResponseTime      time.Duration               `json:"p95_response_time"`
	P99ResponseTime      time.Duration               `json:"p99_response_time"`
	ThroughputRPS        float64                     `json:"throughput_rps"`
	ConcurrentOperations int64                       `json:"concurrent_operations"`
}

// OperationMetrics provides metrics for a specific operation
type OperationMetrics struct {
	Count        int64         `json:"count"`
	SuccessRate  float64       `json:"success_rate"`
	AvgDuration  time.Duration `json:"avg_duration"`
	MinDuration  time.Duration `json:"min_duration"`
	MaxDuration  time.Duration `json:"max_duration"`
	ErrorRate    float64       `json:"error_rate"`
	LastExecuted time.Time     `json:"last_executed"`
}

// ComponentHealth represents the health status of a component
type ComponentHealth struct {
	Status       HealthStatus  `json:"status"`
	LastCheck    time.Time     `json:"last_check"`
	ResponseTime time.Duration `json:"response_time"`
	Uptime       time.Duration `json:"uptime"`
	ErrorCount   int64         `json:"error_count"`
	Message      string        `json:"message,omitempty"`
}

// ResourceSummary provides resource utilization summary
type ResourceSummary struct {
	OverallUtilization float64                  `json:"overall_utilization"`
	ComponentUsage     map[string]ResourceUsage `json:"component_usage"`
	Trends             map[string]ResourceTrend `json:"trends"`
	Alerts             []ResourceAlert          `json:"alerts"`
}

// ResourceTrend represents resource usage trends
type ResourceTrend struct {
	Direction  string  `json:"direction"`  // "increasing", "decreasing", "stable"
	Rate       float64 `json:"rate"`       // Rate of change per hour
	Confidence float64 `json:"confidence"` // Confidence in trend (0-1)
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Component    string    `json:"component"`
	Resource     string    `json:"resource"`
	Severity     string    `json:"severity"`
	Threshold    float64   `json:"threshold"`
	CurrentValue float64   `json:"current_value"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
}

// TrendAnalysis provides trend analysis across all metrics
type TrendAnalysis struct {
	ErrorTrends       map[string]string          `json:"error_trends"`
	PerformanceTrends map[string]string          `json:"performance_trends"`
	UsageTrends       map[string]string          `json:"usage_trends"`
	Predictions       map[string]TrendPrediction `json:"predictions"`
}

// TrendPrediction provides predictive analysis
type TrendPrediction struct {
	Metric      string        `json:"metric"`
	Prediction  float64       `json:"prediction"`
	Confidence  float64       `json:"confidence"`
	TimeHorizon time.Duration `json:"time_horizon"`
	Reasoning   string        `json:"reasoning"`
}

// Recommendation provides actionable recommendations
type Recommendation struct {
	Type        RecommendationType `json:"type"`
	Priority    Priority           `json:"priority"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Actions     []Action           `json:"actions"`
	Impact      string             `json:"impact"`
	Effort      string             `json:"effort"`
}

// RecommendationType categorizes recommendations
type RecommendationType string

const (
	RecommendationTypePerformance RecommendationType = "performance"
	RecommendationTypeReliability RecommendationType = "reliability"
	RecommendationTypeSecurity    RecommendationType = "security"
	RecommendationTypeCost        RecommendationType = "cost"
	RecommendationTypeOperational RecommendationType = "operational"
)

// Priority represents recommendation priority
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

// Action represents a specific action to take
type Action struct {
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Automated   bool              `json:"automated"`
}

// OperationContext methods

// AddProperty adds a property to the operation context
func (oc *OperationContext) AddProperty(key string, value interface{}) *OperationContext {
	if oc.properties == nil {
		oc.properties = make(map[string]interface{})
	}
	oc.properties[key] = value
	return oc
}

// AddMetric adds a metric to the operation context
func (oc *OperationContext) AddMetric(key string, value float64) *OperationContext {
	if oc.metrics == nil {
		oc.metrics = make(map[string]float64)
	}
	oc.metrics[key] = value
	return oc
}

// AddTag adds a tag to the operation context
func (oc *OperationContext) AddTag(key, value string) *OperationContext {
	if oc.tags == nil {
		oc.tags = make(map[string]string)
	}
	oc.tags[key] = value
	return oc
}

// Finish completes the operation and records the event
func (oc *OperationContext) Finish(success bool) {
	duration := time.Since(oc.StartTime)

	event := &Event{
		Name:       oc.Name,
		Type:       EventTypeOperation,
		Timestamp:  oc.StartTime,
		Duration:   duration,
		Success:    success,
		Component:  "operation",
		Operation:  oc.Name,
		Properties: oc.properties,
		Metrics:    oc.metrics,
		Tags:       oc.tags,
	}

	oc.observer.TrackEvent(oc.Context, event)
}

// FinishWithError completes the operation with an error
func (oc *OperationContext) FinishWithError(err error) {
	oc.Finish(false)
	oc.observer.TrackError(oc.Context, err)
}

// SpanContext methods

// AddTag adds a tag to the span context
func (sc *SpanContext) AddTag(key, value string) *SpanContext {
	if sc.tags == nil {
		sc.tags = make(map[string]string)
	}
	sc.tags[key] = value
	return sc
}

// Finish completes the span
func (sc *SpanContext) Finish(success bool) {
	duration := time.Since(sc.StartTime)

	event := &Event{
		Name:      sc.Name,
		Type:      EventTypeOperation,
		Timestamp: sc.StartTime,
		Duration:  duration,
		Success:   success,
		Component: "tracing",
		Operation: sc.Name,
		Tags:      sc.tags,
		Properties: map[string]interface{}{
			"trace_id": sc.TraceID,
			"span_id":  sc.SpanID,
		},
	}

	sc.observer.TrackEvent(sc.Context, event)
}
