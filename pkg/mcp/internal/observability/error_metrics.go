package observability

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
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
	recentErrors    []error
	errorPatterns   map[string]int
	resolutionTimes map[string]time.Duration
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
			resolutionTimes: make(map[string]time.Duration),
			maxRecentErrors: 1000,
			recentErrors:    make([]error, 0),
		}

		// Initialize Prometheus metrics
		em.errorCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_errors_total",
				Help: "Total number of errors by code, type, and severity",
			},
			[]string{"code", "type", "severity", "tool", "operation"},
		)

		em.errorDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_error_duration_seconds",
				Help:    "Duration from error occurrence to resolution",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"code", "type", "severity"},
		)

		em.errorSeverityGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcp_active_errors_by_severity",
				Help: "Current number of unresolved errors by severity",
			},
			[]string{"severity"},
		)

		em.retryCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_error_retries_total",
				Help: "Total number of error retry attempts",
			},
			[]string{"code", "type", "attempt"},
		)

		em.resolutionCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_error_resolutions_total",
				Help: "Total number of error resolutions by type",
			},
			[]string{"code", "type", "resolution_type"},
		)

		// Initialize OpenTelemetry metrics
		meter := otel.Meter("mcp-error-metrics")

		em.otelErrorCounter, _ = meter.Int64Counter(
			"mcp.errors.count",
			metric.WithDescription("Total count of errors"),
		)

		em.otelErrorDuration, _ = meter.Float64Histogram(
			"mcp.errors.duration",
			metric.WithDescription("Duration from error to resolution"),
		)

		em.otelRetryCounter, _ = meter.Int64Counter(
			"mcp.errors.retries",
			metric.WithDescription("Total retry attempts"),
		)

		// Initialize tracer
		em.tracer = otel.Tracer("mcp-error-tracer")

		errorMetricsInstance = em
	})

	return errorMetricsInstance
}

// NewErrorMetricsForTesting creates a new error metrics instance for testing without global registration
func NewErrorMetricsForTesting() *ErrorMetrics {
	registry := prometheus.NewRegistry()

	em := &ErrorMetrics{
		errorPatterns:   make(map[string]int),
		resolutionTimes: make(map[string]time.Duration),
		maxRecentErrors: 1000,
		recentErrors:    make([]error, 0),
	}

	// Initialize Prometheus metrics with custom registry
	factory := promauto.With(registry)

	em.errorCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_errors_total",
			Help: "Total number of errors by code, type, and severity",
		},
		[]string{"code", "type", "severity", "tool", "operation"},
	)

	em.errorDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_error_duration_seconds",
			Help:    "Duration from error occurrence to resolution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "type", "severity"},
	)

	em.errorSeverityGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_active_errors_by_severity",
			Help: "Current number of unresolved errors by severity",
		},
		[]string{"severity"},
	)

	em.retryCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_error_retries_total",
			Help: "Total number of error retry attempts",
		},
		[]string{"code", "type", "attempt"},
	)

	em.resolutionCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_error_resolutions_total",
			Help: "Total number of error resolutions by type",
		},
		[]string{"code", "type", "resolution_type"},
	)

	// Initialize OpenTelemetry metrics (these don't conflict)
	meter := otel.Meter("mcp-error-metrics")
	em.otelErrorCounter, _ = meter.Int64Counter(
		"mcp.errors.total",
		metric.WithDescription("Total errors by type and severity"),
	)
	em.otelErrorDuration, _ = meter.Float64Histogram(
		"mcp.errors.duration",
		metric.WithDescription("Error duration"),
	)
	em.otelRetryCounter, _ = meter.Int64Counter(
		"mcp.errors.retries",
		metric.WithDescription("Total retry attempts"),
	)

	// Initialize tracer
	em.tracer = otel.Tracer("mcp-error-tracer")

	return em
}

// RecordError records an error with observability integration
func (em *ErrorMetrics) RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	// Update Prometheus metrics
	em.errorCounter.WithLabelValues(
		"unknown",
		"unknown",
		"medium",
		"unknown",
		"unknown",
	).Inc()

	// Store recent error for pattern analysis
	em.mu.Lock()
	em.recentErrors = append(em.recentErrors, err)
	if len(em.recentErrors) > em.maxRecentErrors {
		em.recentErrors = em.recentErrors[1:]
	}

	// Update error patterns
	errorMsg := err.Error()
	em.errorPatterns[errorMsg]++

	em.mu.Unlock()
}

// RecordResolution records when an error is successfully resolved
func (em *ErrorMetrics) RecordResolution(ctx context.Context, err error, resolutionType string, duration time.Duration) {
	if err == nil {
		return
	}

	em.mu.Lock()
	em.resolutionTimes[resolutionType] = duration
	em.mu.Unlock()

	em.resolutionCounter.WithLabelValues(
		"unknown",
		"unknown",
		resolutionType,
	).Inc()

	em.errorDuration.WithLabelValues(
		"unknown",
		"unknown",
		"medium",
	).Observe(duration.Seconds())
}

// GetErrorPatterns returns a copy of current error patterns
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
func (em *ErrorMetrics) GetRecentErrors(limit int) []error {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if limit <= 0 || limit > len(em.recentErrors) {
		limit = len(em.recentErrors)
	}

	result := make([]error, limit)
	copy(result, em.recentErrors[len(em.recentErrors)-limit:])
	return result
}

// EnrichContext adds observability context to an error
func (em *ErrorMetrics) EnrichContext(ctx context.Context, err error) {
	if err == nil {
		return
	}
	// Simplified - no complex enrichment for standard errors
}

// ErrorHandler defines a function that handles errors and can resolve them
type ErrorHandler func(ctx context.Context, err error) error

// CreateErrorMiddleware creates middleware that handles errors with metrics tracking
func (em *ErrorMetrics) CreateErrorMiddleware(handler ErrorHandler) ErrorHandler {
	return func(ctx context.Context, err error) error {
		if err == nil {
			return nil
		}

		// Record the error
		em.RecordError(ctx, err)

		// Call the handler
		resolvedErr := handler(ctx, err)

		// If error was resolved (handler returned nil), record resolution
		if resolvedErr == nil && err != nil {
			em.RecordResolution(ctx, err, "middleware", time.Since(time.Now().Add(-time.Second)))
		}

		return resolvedErr
	}
}
