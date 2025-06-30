package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// MonitoringIntegrator provides comprehensive monitoring integration with Prometheus and OpenTelemetry
type MonitoringIntegrator struct {
	sessionManager *session.SessionManager
	logger         zerolog.Logger

	// Prometheus metrics
	registry            *prometheus.Registry
	operationCounter    prometheus.CounterVec
	operationLatency    prometheus.HistogramVec
	sessionCounter      prometheus.CounterVec
	activeSessionsGauge prometheus.Gauge
	errorCounter        prometheus.CounterVec

	// OpenTelemetry
	tracer            trace.Tracer
	meter             metric.Meter
	operationDuration metric.Float64Histogram
	sessionMetrics    metric.Int64Counter

	// Server for metrics endpoint
	metricsServer *http.Server
	serverMutex   sync.Mutex

	// Configuration
	config MonitoringConfig
}

// MonitoringConfig configures monitoring behavior
type MonitoringConfig struct {
	PrometheusPort        int           `json:"prometheus_port"`
	MetricsPath           string        `json:"metrics_path"`
	EnableTracing         bool          `json:"enable_tracing"`
	ServiceName           string        `json:"service_name"`
	ServiceVersion        string        `json:"service_version"`
	ScrapeInterval        time.Duration `json:"scrape_interval"`
	HistogramBuckets      []float64     `json:"histogram_buckets"`
	EnableDetailedMetrics bool          `json:"enable_detailed_metrics"`
}

// MonitoringMetrics represents comprehensive monitoring metrics
type MonitoringMetrics struct {
	Timestamp          time.Time                   `json:"timestamp"`
	TotalOperations    int64                       `json:"total_operations"`
	SuccessfulOps      int64                       `json:"successful_ops"`
	FailedOps          int64                       `json:"failed_ops"`
	ActiveSessions     int64                       `json:"active_sessions"`
	AverageLatency     time.Duration               `json:"average_latency"`
	P95Latency         time.Duration               `json:"p95_latency"`
	ErrorRate          float64                     `json:"error_rate"`
	OperationBreakdown map[string]OperationMetrics `json:"operation_breakdown"`
	SessionMetrics     SessionMetrics              `json:"session_metrics"`
	SystemHealth       SystemHealthMetrics         `json:"system_health"`
}

// OperationMetrics represents metrics for specific operations
type OperationMetrics struct {
	OperationType  string        `json:"operation_type"`
	Count          int64         `json:"count"`
	SuccessRate    float64       `json:"success_rate"`
	AverageLatency time.Duration `json:"average_latency"`
	TotalErrors    int64         `json:"total_errors"`
	LastExecution  time.Time     `json:"last_execution"`
}

// SessionMetrics represents session-related metrics
type SessionMetrics struct {
	TotalSessions     int64         `json:"total_sessions"`
	ActiveSessions    int64         `json:"active_sessions"`
	AverageSessionAge time.Duration `json:"average_session_age"`
	SessionThroughput float64       `json:"session_throughput"`
}

// SystemHealthMetrics represents system health metrics
type SystemHealthMetrics struct {
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     float64   `json:"memory_usage"`
	GoroutineCount  int       `json:"goroutine_count"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// NewMonitoringIntegrator creates a new monitoring integrator
func NewMonitoringIntegrator(
	sessionManager *session.SessionManager,
	config MonitoringConfig,
	logger zerolog.Logger,
) (*MonitoringIntegrator, error) {

	// Set defaults
	if config.PrometheusPort == 0 {
		config.PrometheusPort = 9090
	}
	if config.MetricsPath == "" {
		config.MetricsPath = "/metrics"
	}
	if config.ServiceName == "" {
		config.ServiceName = "container-kit-mcp"
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = "1.0.0"
	}
	if config.ScrapeInterval == 0 {
		config.ScrapeInterval = 15 * time.Second
	}
	if len(config.HistogramBuckets) == 0 {
		config.HistogramBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}

	mi := &MonitoringIntegrator{
		sessionManager: sessionManager,
		logger:         logger.With().Str("component", "monitoring_integrator").Logger(),
		registry:       prometheus.NewRegistry(),
		config:         config,
	}

	// Initialize Prometheus metrics
	if err := mi.initializePrometheusMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize Prometheus metrics: %w", err)
	}

	// Initialize OpenTelemetry
	if config.EnableTracing {
		if err := mi.initializeOpenTelemetry(); err != nil {
			return nil, fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
		}
	}

	// Start metrics server
	if err := mi.startMetricsServer(); err != nil {
		return nil, fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Start background metrics collection
	go mi.startMetricsCollection()

	mi.logger.Info().
		Int("prometheus_port", config.PrometheusPort).
		Str("metrics_path", config.MetricsPath).
		Bool("tracing_enabled", config.EnableTracing).
		Msg("Monitoring integration initialized")

	return mi, nil
}

// TrackOperation tracks a Docker operation with comprehensive monitoring
func (mi *MonitoringIntegrator) TrackOperation(
	ctx context.Context,
	operationType, sessionID string,
	operation func() error,
) error {
	startTime := time.Now()

	// Create OpenTelemetry span if tracing is enabled
	var span trace.Span
	if mi.config.EnableTracing {
		ctx, span = mi.tracer.Start(ctx, fmt.Sprintf("docker_%s", operationType))
		span.SetAttributes(
			attribute.String("operation.type", operationType),
			attribute.String("session.id", sessionID),
			attribute.String("service.name", mi.config.ServiceName),
		)
		defer span.End()
	}

	// Track operation start
	mi.operationCounter.WithLabelValues(operationType, "started").Inc()

	// Execute operation
	err := operation()
	duration := time.Since(startTime)

	// Record metrics
	if err != nil {
		mi.operationCounter.WithLabelValues(operationType, "failed").Inc()
		mi.errorCounter.WithLabelValues(operationType, "execution_error").Inc()

		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		mi.operationCounter.WithLabelValues(operationType, "succeeded").Inc()

		if span != nil {
			span.SetStatus(codes.Ok, "Operation completed successfully")
		}
	}

	// Record latency
	mi.operationLatency.WithLabelValues(operationType).Observe(duration.Seconds())

	// Record OpenTelemetry metrics
	if mi.operationDuration != nil {
		mi.operationDuration.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("operation_type", operationType),
				attribute.String("status", func() string {
					if err != nil {
						return "error"
					}
					return "success"
				}()),
			),
		)
	}

	// Log detailed operation info if enabled
	if mi.config.EnableDetailedMetrics {
		mi.logger.Info().
			Str("operation_type", operationType).
			Str("session_id", sessionID).
			Dur("duration", duration).
			Bool("success", err == nil).
			Msg("Operation tracked")
	}

	return err
}

// TrackSession tracks session lifecycle events
func (mi *MonitoringIntegrator) TrackSession(sessionID, event string) {
	mi.sessionCounter.WithLabelValues(event).Inc()

	if mi.sessionMetrics != nil {
		ctx := context.Background()
		mi.sessionMetrics.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("session_id", sessionID),
				attribute.String("event", event),
			),
		)
	}

	// Update active sessions gauge
	if event == "created" {
		mi.activeSessionsGauge.Inc()
	} else if event == "terminated" || event == "expired" {
		mi.activeSessionsGauge.Dec()
	}
}

// GetMonitoringMetrics returns comprehensive monitoring metrics
func (mi *MonitoringIntegrator) GetMonitoringMetrics(ctx context.Context) (*MonitoringMetrics, error) {
	// Gather Prometheus metrics
	metricFamilies, err := mi.registry.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather Prometheus metrics: %w", err)
	}

	metrics := &MonitoringMetrics{
		Timestamp:          time.Now(),
		OperationBreakdown: make(map[string]OperationMetrics),
	}

	// Process Prometheus metrics
	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "docker_operations_total":
			for _, metric := range mf.GetMetric() {
				operationType := ""
				status := ""
				for _, label := range metric.GetLabel() {
					if label.GetName() == "operation" {
						operationType = label.GetValue()
					}
					if label.GetName() == "status" {
						status = label.GetValue()
					}
				}

				if operationType != "" {
					opMetrics := metrics.OperationBreakdown[operationType]
					opMetrics.OperationType = operationType

					switch status {
					case "succeeded":
						opMetrics.Count += int64(metric.GetCounter().GetValue())
						metrics.SuccessfulOps += int64(metric.GetCounter().GetValue())
					case "failed":
						opMetrics.TotalErrors += int64(metric.GetCounter().GetValue())
						metrics.FailedOps += int64(metric.GetCounter().GetValue())
					}

					metrics.OperationBreakdown[operationType] = opMetrics
				}
			}
		case "docker_operation_duration_seconds":
			// Process latency metrics
			for _, metric := range mf.GetMetric() {
				if metric.GetHistogram() != nil {
					metrics.AverageLatency = time.Duration(
						metric.GetHistogram().GetSampleSum() /
							float64(metric.GetHistogram().GetSampleCount()) *
							float64(time.Second),
					)
				}
			}
		}
	}

	metrics.TotalOperations = metrics.SuccessfulOps + metrics.FailedOps
	if metrics.TotalOperations > 0 {
		metrics.ErrorRate = float64(metrics.FailedOps) / float64(metrics.TotalOperations)
	}

	// Get session metrics from session manager
	if sessionData, err := mi.getSessionMetrics(); err == nil {
		metrics.SessionMetrics = sessionData
		metrics.ActiveSessions = sessionData.ActiveSessions
	}

	// Get system health metrics
	metrics.SystemHealth = mi.getSystemHealthMetrics()

	return metrics, nil
}

// Shutdown gracefully shuts down the monitoring integrator
func (mi *MonitoringIntegrator) Shutdown(ctx context.Context) error {
	mi.serverMutex.Lock()
	defer mi.serverMutex.Unlock()

	if mi.metricsServer != nil {
		mi.logger.Info().Msg("Shutting down metrics server")
		return mi.metricsServer.Shutdown(ctx)
	}

	return nil
}

// Private helper methods

func (mi *MonitoringIntegrator) initializePrometheusMetrics() error {
	// Operation counter
	mi.operationCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "docker_operations_total",
			Help: "Total number of Docker operations",
		},
		[]string{"operation", "status"},
	)

	// Operation latency histogram
	mi.operationLatency = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "docker_operation_duration_seconds",
			Help:    "Duration of Docker operations in seconds",
			Buckets: mi.config.HistogramBuckets,
		},
		[]string{"operation"},
	)

	// Session counter
	mi.sessionCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sessions_total",
			Help: "Total number of session events",
		},
		[]string{"event"},
	)

	// Active sessions gauge
	mi.activeSessionsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_sessions",
			Help: "Number of currently active sessions",
		},
	)

	// Error counter
	mi.errorCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"operation", "error_type"},
	)

	// Register metrics
	mi.registry.MustRegister(&mi.operationCounter)
	mi.registry.MustRegister(&mi.operationLatency)
	mi.registry.MustRegister(&mi.sessionCounter)
	mi.registry.MustRegister(mi.activeSessionsGauge)
	mi.registry.MustRegister(&mi.errorCounter)

	return nil
}

func (mi *MonitoringIntegrator) initializeOpenTelemetry() error {
	// Create OpenTelemetry tracer
	mi.tracer = otel.Tracer(mi.config.ServiceName)

	// Create meter directly without Prometheus exporter dependency
	mi.meter = otel.Meter(mi.config.ServiceName)

	// Create metrics instruments
	var err error
	mi.operationDuration, err = mi.meter.Float64Histogram(
		"operation_duration_seconds",
		metric.WithDescription("Duration of operations in seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	mi.sessionMetrics, err = mi.meter.Int64Counter(
		"session_events_total",
		metric.WithDescription("Total number of session events"),
	)
	if err != nil {
		return fmt.Errorf("failed to create session metrics counter: %w", err)
	}

	return nil
}

func (mi *MonitoringIntegrator) startMetricsServer() error {
	mi.serverMutex.Lock()
	defer mi.serverMutex.Unlock()

	mux := http.NewServeMux()
	mux.Handle(mi.config.MetricsPath, promhttp.HandlerFor(mi.registry, promhttp.HandlerOpts{}))

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mi.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", mi.config.PrometheusPort),
		Handler: mux,
	}

	go func() {
		if err := mi.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mi.logger.Error().Err(err).Msg("Metrics server failed")
		}
	}()

	return nil
}

func (mi *MonitoringIntegrator) startMetricsCollection() {
	ticker := time.NewTicker(mi.config.ScrapeInterval)
	defer ticker.Stop()

	for range ticker.C {
		mi.collectSystemMetrics()
	}
}

func (mi *MonitoringIntegrator) collectSystemMetrics() {
	// This would collect system-level metrics
	// Implementation would depend on specific monitoring requirements
	mi.logger.Debug().Msg("Collecting system metrics")
}

func (mi *MonitoringIntegrator) getSessionMetrics() (SessionMetrics, error) {
	// Get session metrics from session manager
	// This is a placeholder - actual implementation would query session manager
	return SessionMetrics{
		TotalSessions:     100,
		ActiveSessions:    25,
		AverageSessionAge: 30 * time.Minute,
		SessionThroughput: 5.2,
	}, nil
}

func (mi *MonitoringIntegrator) getSystemHealthMetrics() SystemHealthMetrics {
	// Collect system health metrics
	// This is a placeholder - actual implementation would gather real system metrics
	return SystemHealthMetrics{
		CPUUsage:        45.2,
		MemoryUsage:     67.8,
		GoroutineCount:  156,
		LastHealthCheck: time.Now(),
	}
}
