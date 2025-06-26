package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ErrorMetrics provides structured error tracking for observability
type ErrorMetrics struct {
	// Prometheus metrics
	errorCounter       *prometheus.CounterVec
	errorDuration      *prometheus.HistogramVec
	errorSeverityGauge *prometheus.GaugeVec
	retryCounter       *prometheus.CounterVec
	resolutionCounter  *prometheus.CounterVec

	// OpenTelemetry metrics
	otelErrorCounter  metric.Int64Counter
	otelErrorDuration metric.Float64Histogram
	otelRetryCounter  metric.Int64Counter

	// OpenTelemetry tracer
	tracer trace.Tracer

	// Internal state
	mu              sync.RWMutex
	recentErrors    []*types.RichError
	errorPatterns   map[string]int
	maxRecentErrors int
}

var (
	// Singleton instances for metrics to avoid duplicate registration
	errorMetricsOnce     sync.Once
	errorMetricsInstance *ErrorMetrics
)

// NewErrorMetrics creates a new error metrics collector (singleton)
func NewErrorMetrics() *ErrorMetrics {
	errorMetricsOnce.Do(func() {
		em := &ErrorMetrics{
			errorPatterns:   make(map[string]int),
			maxRecentErrors: 1000,
			recentErrors:    make([]*types.RichError, 0, 1000),
		}

		// Initialize Prometheus metrics
		em.errorCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_errors_total",
				Help: "Total number of errors by code, type, and severity",
			},
			[]string{"code", "type", "severity", "component", "operation"},
		)

		em.errorDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_error_duration_seconds",
				Help:    "Duration from error occurrence to resolution",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
			},
			[]string{"code", "type", "severity"},
		)

		em.errorSeverityGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcp_error_severity_current",
				Help: "Current count of errors by severity",
			},
			[]string{"severity"},
		)

		em.retryCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_error_retries_total",
				Help: "Total number of retry attempts by error code",
			},
			[]string{"code", "type", "attempt_number"},
		)

		em.resolutionCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_error_resolutions_total",
				Help: "Total number of successful error resolutions",
			},
			[]string{"code", "type", "resolution_type"},
		)

		// Initialize OpenTelemetry metrics
		meter := otel.Meter("github.com/Azure/container-kit/mcp")

		em.otelErrorCounter, _ = meter.Int64Counter(
			"mcp.errors",
			metric.WithDescription("Total number of errors"),
			metric.WithUnit("1"),
		)

		em.otelErrorDuration, _ = meter.Float64Histogram(
			"mcp.error.duration",
			metric.WithDescription("Error duration from occurrence to resolution"),
			metric.WithUnit("s"),
		)

		em.otelRetryCounter, _ = meter.Int64Counter(
			"mcp.error.retries",
			metric.WithDescription("Total number of retry attempts"),
			metric.WithUnit("1"),
		)

		// Initialize tracer
		em.tracer = otel.Tracer("github.com/Azure/container-kit/mcp/errors")

		errorMetricsInstance = em
	})

	return errorMetricsInstance
}

// RecordError records a RichError with full observability integration
func (em *ErrorMetrics) RecordError(ctx context.Context, err *types.RichError) {
	if err == nil {
		return
	}

	// Start span for error recording
	ctx, span := em.tracer.Start(ctx, "error.record",
		trace.WithAttributes(
			attribute.String("error.code", err.Code),
			attribute.String("error.type", err.Type),
			attribute.String("error.severity", err.Severity),
			attribute.String("error.message", err.Message),
		),
	)
	defer span.End()

	// Update Prometheus metrics
	em.errorCounter.WithLabelValues(
		err.Code,
		err.Type,
		err.Severity,
		err.Context.Component,
		err.Context.Operation,
	).Inc()

	// Update severity gauge
	em.updateSeverityGauge(err.Severity, 1)

	// Record retry information
	if err.AttemptNumber > 0 {
		em.retryCounter.WithLabelValues(
			err.Code,
			err.Type,
			fmt.Sprintf("%d", err.AttemptNumber),
		).Inc()
	}

	// Update OpenTelemetry metrics
	em.otelErrorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("error.code", err.Code),
			attribute.String("error.type", err.Type),
			attribute.String("error.severity", err.Severity),
		),
	)

	// Store recent error for pattern analysis
	em.mu.Lock()
	em.recentErrors = append(em.recentErrors, err)
	if len(em.recentErrors) > em.maxRecentErrors {
		em.recentErrors = em.recentErrors[1:]
	}

	// Track error patterns
	patternKey := fmt.Sprintf("%s:%s", err.Code, err.Type)
	em.errorPatterns[patternKey]++
	em.mu.Unlock()

	// Add error event to span
	span.AddEvent("error.occurred",
		trace.WithAttributes(
			attribute.String("root_cause", err.Diagnostics.RootCause),
			attribute.String("error_pattern", err.Diagnostics.ErrorPattern),
			attribute.StringSlice("symptoms", err.Diagnostics.Symptoms),
		),
	)
}

// RecordResolution records when an error is successfully resolved
func (em *ErrorMetrics) RecordResolution(ctx context.Context, err *types.RichError, resolutionType string, duration time.Duration) {
	if err == nil {
		return
	}

	// Start span for resolution recording
	ctx, span := em.tracer.Start(ctx, "error.resolution",
		trace.WithAttributes(
			attribute.String("error.code", err.Code),
			attribute.String("resolution.type", resolutionType),
			attribute.Float64("resolution.duration_seconds", duration.Seconds()),
		),
	)
	defer span.End()

	// Update Prometheus metrics
	em.resolutionCounter.WithLabelValues(
		err.Code,
		err.Type,
		resolutionType,
	).Inc()

	em.errorDuration.WithLabelValues(
		err.Code,
		err.Type,
		err.Severity,
	).Observe(duration.Seconds())

	// Update severity gauge (decrement)
	em.updateSeverityGauge(err.Severity, -1)

	// Update OpenTelemetry metrics
	em.otelErrorDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("error.code", err.Code),
			attribute.String("error.type", err.Type),
			attribute.String("resolution.type", resolutionType),
		),
	)
}

// GetErrorPatterns returns the most common error patterns
func (em *ErrorMetrics) GetErrorPatterns() map[string]int {
	em.mu.RLock()
	defer em.mu.RUnlock()

	patterns := make(map[string]int)
	for k, v := range em.errorPatterns {
		patterns[k] = v
	}
	return patterns
}

// GetRecentErrors returns recent errors for analysis
func (em *ErrorMetrics) GetRecentErrors(limit int) []*types.RichError {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if limit <= 0 || limit > len(em.recentErrors) {
		limit = len(em.recentErrors)
	}

	result := make([]*types.RichError, limit)
	copy(result, em.recentErrors[len(em.recentErrors)-limit:])
	return result
}

// EnrichContext adds observability context to a RichError
func (em *ErrorMetrics) EnrichContext(ctx context.Context, err *types.RichError) {
	if err == nil {
		return
	}

	// Extract trace and span IDs if available
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		err.Context.Metadata.AddCustom("trace_id", spanCtx.TraceID().String())
		err.Context.Metadata.AddCustom("span_id", spanCtx.SpanID().String())
	}

	// Add correlation ID if available
	if corrID := ctx.Value("correlation_id"); corrID != nil {
		err.Context.Metadata.AddCustom("correlation_id", corrID)
	}
}

// updateSeverityGauge updates the severity gauge metric
func (em *ErrorMetrics) updateSeverityGauge(severity string, delta float64) {
	em.errorSeverityGauge.WithLabelValues(severity).Add(delta)
}

// ErrorMetricsMiddleware provides middleware for automatic error tracking
func ErrorMetricsMiddleware(em *ErrorMetrics) func(next func(context.Context, *types.RichError) error) func(context.Context, *types.RichError) error {
	return func(next func(context.Context, *types.RichError) error) func(context.Context, *types.RichError) error {
		return func(ctx context.Context, err *types.RichError) error {
			start := time.Now()

			// Record the error
			em.RecordError(ctx, err)

			// Call the next handler
			result := next(ctx, err)

			// If error was resolved (result is nil), record resolution
			if result == nil && err != nil {
				em.RecordResolution(ctx, err, "handled", time.Since(start))
			}

			return result
		}
	}
}
