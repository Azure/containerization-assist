package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

// MetricsCollector provides comprehensive metrics
type MetricsCollector struct {
	logger  zerolog.Logger
	mutex   sync.RWMutex
	enabled bool

	// Core metrics
	sessionMetrics     *SessionMetrics
	dockerMetrics      *DockerMetrics
	retryMetrics       *RetryMetrics
	performanceMetrics *PerformanceMetrics
	healthMetrics      *HealthMetrics

	// Custom metrics registry
	customMetrics map[string]prometheus.Collector
	registry      *prometheus.Registry
}

// SessionMetrics tracks session-related metrics
type SessionMetrics struct {
	TotalSessions    prometheus.Counter
	ActiveSessions   prometheus.Gauge
	SessionDuration  prometheus.Histogram
	SessionErrors    prometheus.Counter
	SessionsByStatus *prometheus.CounterVec
	SessionsByTeam   *prometheus.CounterVec
	ResourceUsage    *prometheus.GaugeVec
}

// DockerMetrics tracks Docker operation metrics
type DockerMetrics struct {
	OperationCount    *prometheus.CounterVec
	OperationDuration *prometheus.HistogramVec
	OperationErrors   *prometheus.CounterVec
	CacheHitRate      prometheus.Gauge
	CacheSize         prometheus.Gauge
	PullCount         prometheus.Counter
	PushCount         prometheus.Counter
	TagCount          prometheus.Counter
	ImageSizes        prometheus.Histogram
}

// RetryMetrics tracks retry mechanism metrics
type RetryMetrics struct {
	RetryAttempts       *prometheus.CounterVec
	RetrySuccessRate    *prometheus.GaugeVec
	CircuitBreakerState *prometheus.GaugeVec
	FallbackUsage       *prometheus.CounterVec
	AdaptiveDelays      *prometheus.HistogramVec
}

// PerformanceMetrics tracks system performance
type PerformanceMetrics struct {
	RequestDuration     *prometheus.HistogramVec
	RequestRate         *prometheus.CounterVec
	ErrorRate           *prometheus.CounterVec
	Throughput          *prometheus.GaugeVec
	LatencyPercentiles  *prometheus.SummaryVec
	ResourceUtilization *prometheus.GaugeVec
}

// HealthMetrics tracks system health indicators
type HealthMetrics struct {
	ComponentHealth  *prometheus.GaugeVec
	ServiceUptime    prometheus.Gauge
	DependencyHealth *prometheus.GaugeVec
	AlertsActive     prometheus.Gauge
	AlertsResolved   prometheus.Counter
	SystemLoad       *prometheus.GaugeVec
}

// MetricsConfig configures the metrics collector
type MetricsConfig struct {
	Enabled         bool                 `json:"enabled"`
	Namespace       string               `json:"namespace"`
	Subsystem       string               `json:"subsystem"`
	Labels          map[string]string    `json:"labels"`
	Registry        *prometheus.Registry `json:"-"`
	UpdateInterval  time.Duration        `json:"update_interval"`
	RetentionPeriod time.Duration        `json:"retention_period"`
	Buckets         []float64            `json:"buckets"`
	Objectives      map[float64]float64  `json:"objectives"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config MetricsConfig, logger zerolog.Logger) *MetricsCollector {
	if config.Namespace == "" {
		config.Namespace = "ContainerKit"
	}
	if config.Subsystem == "" {
		config.Subsystem = "mcp"
	}
	if config.UpdateInterval == 0 {
		config.UpdateInterval = 30 * time.Second
	}
	if config.Buckets == nil {
		config.Buckets = prometheus.DefBuckets
	}
	if config.Objectives == nil {
		config.Objectives = map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.95: 0.005,
			0.99: 0.001,
		}
	}

	registry := config.Registry
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	mc := &MetricsCollector{
		logger:        logger.With().Str("component", "metrics").Logger(),
		enabled:       config.Enabled,
		customMetrics: make(map[string]prometheus.Collector),
		registry:      registry,
	}

	if config.Enabled {
		mc.initializeMetrics(config)
	}

	return mc
}

// initializeMetrics sets up all metric collectors
func (mc *MetricsCollector) initializeMetrics(config MetricsConfig) {
	mc.sessionMetrics = mc.createSessionMetrics(config)
	mc.dockerMetrics = mc.createDockerMetrics(config)
	mc.retryMetrics = mc.createRetryMetrics(config)
	mc.performanceMetrics = mc.createPerformanceMetrics(config)
	mc.healthMetrics = mc.createHealthMetrics(config)

	mc.logger.Info().
		Str("namespace", config.Namespace).
		Str("subsystem", config.Subsystem).
		Msg("Metrics collector initialized")
}

// createSessionMetrics initializes session-related metrics
func (mc *MetricsCollector) createSessionMetrics(config MetricsConfig) *SessionMetrics {
	return &SessionMetrics{
		TotalSessions: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "sessions_total",
			Help:      "Total number of sessions created",
		}),

		ActiveSessions: promauto.With(mc.registry).NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "sessions_active",
			Help:      "Number of currently active sessions",
		}),

		SessionDuration: promauto.With(mc.registry).NewHistogram(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "session_duration_seconds",
			Help:      "Session duration in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		}),

		SessionErrors: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "session_errors_total",
			Help:      "Total number of session errors",
		}),

		SessionsByStatus: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "sessions_by_status_total",
			Help:      "Sessions count by status",
		}, []string{"status"}),

		SessionsByTeam: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "sessions_by_team_total",
			Help:      "Sessions count by team",
		}, []string{"team"}),

		ResourceUsage: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "session_resource_usage",
			Help:      "Session resource usage",
		}, []string{"resource_type", "session_id"}),
	}
}

// createDockerMetrics initializes Docker operation metrics
func (mc *MetricsCollector) createDockerMetrics(config MetricsConfig) *DockerMetrics {
	return &DockerMetrics{
		OperationCount: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_operations_total",
			Help:      "Total number of Docker operations",
		}, []string{"operation", "status"}),

		OperationDuration: promauto.With(mc.registry).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_operation_duration_seconds",
			Help:      "Docker operation duration in seconds",
			Buckets:   []float64{0.001, 0.01, 0.1, 1, 5, 10, 30, 60, 120},
		}, []string{"operation"}),

		OperationErrors: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_operation_errors_total",
			Help:      "Total number of Docker operation errors",
		}, []string{"operation", "error_type"}),

		CacheHitRate: promauto.With(mc.registry).NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_cache_hit_rate",
			Help:      "Docker operation cache hit rate percentage",
		}),

		CacheSize: promauto.With(mc.registry).NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_cache_size",
			Help:      "Number of entries in Docker operation cache",
		}),

		PullCount: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_pulls_total",
			Help:      "Total number of Docker image pulls",
		}),

		PushCount: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_pushes_total",
			Help:      "Total number of Docker image pushes",
		}),

		TagCount: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_tags_total",
			Help:      "Total number of Docker image tags",
		}),

		ImageSizes: promauto.With(mc.registry).NewHistogram(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "docker_image_size_bytes",
			Help:      "Docker image sizes in bytes",
			Buckets:   prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to ~1GB
		}),
	}
}

// createRetryMetrics initializes retry mechanism metrics
func (mc *MetricsCollector) createRetryMetrics(config MetricsConfig) *RetryMetrics {
	return &RetryMetrics{
		RetryAttempts: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "retry_attempts_total",
			Help:      "Total number of retry attempts",
		}, []string{"operation", "policy"}),

		RetrySuccessRate: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "retry_success_rate",
			Help:      "Retry success rate percentage",
		}, []string{"operation", "policy"}),

		CircuitBreakerState: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "circuit_breaker_state",
			Help:      "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		}, []string{"operation"}),

		FallbackUsage: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "fallback_usage_total",
			Help:      "Total number of fallback strategy executions",
		}, []string{"operation", "strategy"}),

		AdaptiveDelays: promauto.With(mc.registry).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "adaptive_delay_seconds",
			Help:      "Adaptive retry delay duration in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		}, []string{"operation"}),
	}
}

// createPerformanceMetrics initializes performance metrics
func (mc *MetricsCollector) createPerformanceMetrics(config MetricsConfig) *PerformanceMetrics {
	return &PerformanceMetrics{
		RequestDuration: promauto.With(mc.registry).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds",
			Buckets:   config.Buckets,
		}, []string{"method", "endpoint", "status"}),

		RequestRate: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "requests_total",
			Help:      "Total number of requests",
		}, []string{"method", "endpoint"}),

		ErrorRate: promauto.With(mc.registry).NewCounterVec(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total number of errors",
		}, []string{"type", "component"}),

		Throughput: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "throughput_operations_per_second",
			Help:      "Operations throughput per second",
		}, []string{"operation"}),

		LatencyPercentiles: promauto.With(mc.registry).NewSummaryVec(prometheus.SummaryOpts{
			Namespace:  config.Namespace,
			Subsystem:  config.Subsystem,
			Name:       "latency_percentiles_seconds",
			Help:       "Latency percentiles in seconds",
			Objectives: config.Objectives,
		}, []string{"operation"}),

		ResourceUtilization: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "resource_utilization_percent",
			Help:      "Resource utilization percentage",
		}, []string{"resource"}),
	}
}

// createHealthMetrics initializes health metrics
func (mc *MetricsCollector) createHealthMetrics(config MetricsConfig) *HealthMetrics {
	return &HealthMetrics{
		ComponentHealth: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "component_health",
			Help:      "Component health status (1=healthy, 0=unhealthy)",
		}, []string{"component"}),

		ServiceUptime: promauto.With(mc.registry).NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "service_uptime_seconds",
			Help:      "Service uptime in seconds",
		}),

		DependencyHealth: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "dependency_health",
			Help:      "Dependency health status (1=healthy, 0=unhealthy)",
		}, []string{"dependency"}),

		AlertsActive: promauto.With(mc.registry).NewGauge(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "alerts_active",
			Help:      "Number of active alerts",
		}),

		AlertsResolved: promauto.With(mc.registry).NewCounter(prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "alerts_resolved_total",
			Help:      "Total number of resolved alerts",
		}),

		SystemLoad: promauto.With(mc.registry).NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "system_load",
			Help:      "System load averages",
		}, []string{"period"}),
	}
}

// Session metric methods
func (mc *MetricsCollector) RecordSessionCreated(team string) {
	if !mc.enabled {
		return
	}
	mc.sessionMetrics.TotalSessions.Inc()
	mc.sessionMetrics.SessionsByTeam.WithLabelValues(team).Inc()
}

func (mc *MetricsCollector) RecordSessionCompleted(duration time.Duration, status string) {
	if !mc.enabled {
		return
	}
	mc.sessionMetrics.SessionDuration.Observe(duration.Seconds())
	mc.sessionMetrics.SessionsByStatus.WithLabelValues(status).Inc()
}

func (mc *MetricsCollector) UpdateActiveSessionCount(count int) {
	if !mc.enabled {
		return
	}
	mc.sessionMetrics.ActiveSessions.Set(float64(count))
}

func (mc *MetricsCollector) RecordSessionError() {
	if !mc.enabled {
		return
	}
	mc.sessionMetrics.SessionErrors.Inc()
}

// Docker metric methods
func (mc *MetricsCollector) RecordDockerOperation(operation string, duration time.Duration, success bool, errorType string) {
	if !mc.enabled {
		return
	}

	status := "success"
	if !success {
		status = "failure"
		mc.dockerMetrics.OperationErrors.WithLabelValues(operation, errorType).Inc()
	}

	mc.dockerMetrics.OperationCount.WithLabelValues(operation, status).Inc()
	mc.dockerMetrics.OperationDuration.WithLabelValues(operation).Observe(duration.Seconds())

	switch operation {
	case "pull":
		mc.dockerMetrics.PullCount.Inc()
	case "push":
		mc.dockerMetrics.PushCount.Inc()
	case "tag":
		mc.dockerMetrics.TagCount.Inc()
	}
}

func (mc *MetricsCollector) UpdateDockerCacheMetrics(hitRate float64, size int) {
	if !mc.enabled {
		return
	}
	mc.dockerMetrics.CacheHitRate.Set(hitRate)
	mc.dockerMetrics.CacheSize.Set(float64(size))
}

func (mc *MetricsCollector) RecordImageSize(sizeBytes int64) {
	if !mc.enabled {
		return
	}
	mc.dockerMetrics.ImageSizes.Observe(float64(sizeBytes))
}

// Retry metric methods
func (mc *MetricsCollector) RecordRetryAttempt(operation, policy string) {
	if !mc.enabled {
		return
	}
	mc.retryMetrics.RetryAttempts.WithLabelValues(operation, policy).Inc()
}

func (mc *MetricsCollector) UpdateRetrySuccessRate(operation, policy string, rate float64) {
	if !mc.enabled {
		return
	}
	mc.retryMetrics.RetrySuccessRate.WithLabelValues(operation, policy).Set(rate)
}

func (mc *MetricsCollector) UpdateCircuitBreakerState(operation string, state int) {
	if !mc.enabled {
		return
	}
	mc.retryMetrics.CircuitBreakerState.WithLabelValues(operation).Set(float64(state))
}

func (mc *MetricsCollector) RecordFallbackUsage(operation, strategy string) {
	if !mc.enabled {
		return
	}
	mc.retryMetrics.FallbackUsage.WithLabelValues(operation, strategy).Inc()
}

func (mc *MetricsCollector) RecordAdaptiveDelay(operation string, delay time.Duration) {
	if !mc.enabled {
		return
	}
	mc.retryMetrics.AdaptiveDelays.WithLabelValues(operation).Observe(delay.Seconds())
}

// Performance metric methods
func (mc *MetricsCollector) RecordRequest(method, endpoint string, duration time.Duration, status string) {
	if !mc.enabled {
		return
	}
	mc.performanceMetrics.RequestDuration.WithLabelValues(method, endpoint, status).Observe(duration.Seconds())
	mc.performanceMetrics.RequestRate.WithLabelValues(method, endpoint).Inc()
	mc.performanceMetrics.LatencyPercentiles.WithLabelValues(fmt.Sprintf("%s_%s", method, endpoint)).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordError(errorType, component string) {
	if !mc.enabled {
		return
	}
	mc.performanceMetrics.ErrorRate.WithLabelValues(errorType, component).Inc()
}

func (mc *MetricsCollector) UpdateThroughput(operation string, opsPerSecond float64) {
	if !mc.enabled {
		return
	}
	mc.performanceMetrics.Throughput.WithLabelValues(operation).Set(opsPerSecond)
}

func (mc *MetricsCollector) UpdateResourceUtilization(resource string, percent float64) {
	if !mc.enabled {
		return
	}
	mc.performanceMetrics.ResourceUtilization.WithLabelValues(resource).Set(percent)
}

// Health metric methods
func (mc *MetricsCollector) UpdateComponentHealth(component string, healthy bool) {
	if !mc.enabled {
		return
	}
	value := 0.0
	if healthy {
		value = 1.0
	}
	mc.healthMetrics.ComponentHealth.WithLabelValues(component).Set(value)
}

func (mc *MetricsCollector) UpdateServiceUptime(uptime time.Duration) {
	if !mc.enabled {
		return
	}
	mc.healthMetrics.ServiceUptime.Set(uptime.Seconds())
}

func (mc *MetricsCollector) UpdateDependencyHealth(dependency string, healthy bool) {
	if !mc.enabled {
		return
	}
	value := 0.0
	if healthy {
		value = 1.0
	}
	mc.healthMetrics.DependencyHealth.WithLabelValues(dependency).Set(value)
}

func (mc *MetricsCollector) UpdateActiveAlerts(count int) {
	if !mc.enabled {
		return
	}
	mc.healthMetrics.AlertsActive.Set(float64(count))
}

func (mc *MetricsCollector) RecordAlertResolved() {
	if !mc.enabled {
		return
	}
	mc.healthMetrics.AlertsResolved.Inc()
}

func (mc *MetricsCollector) UpdateSystemLoad(period string, load float64) {
	if !mc.enabled {
		return
	}
	mc.healthMetrics.SystemLoad.WithLabelValues(period).Set(load)
}

// Custom metrics
func (mc *MetricsCollector) RegisterCustomMetric(name string, metric prometheus.Collector) error {
	if !mc.enabled {
		return nil
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if _, exists := mc.customMetrics[name]; exists {
		return fmt.Errorf("metric %s already registered", name)
	}

	if err := mc.registry.Register(metric); err != nil {
		return fmt.Errorf("failed to register metric %s: %w", name, err)
	}

	mc.customMetrics[name] = metric

	mc.logger.Info().
		Str("metric_name", name).
		Msg("Custom metric registered")

	return nil
}

func (mc *MetricsCollector) UnregisterCustomMetric(name string) error {
	if !mc.enabled {
		return nil
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	metric, exists := mc.customMetrics[name]
	if !exists {
		return fmt.Errorf("metric %s not found", name)
	}

	if !mc.registry.Unregister(metric) {
		return fmt.Errorf("failed to unregister metric %s", name)
	}

	delete(mc.customMetrics, name)

	mc.logger.Info().
		Str("metric_name", name).
		Msg("Custom metric unregistered")

	return nil
}

// GetRegistry returns the Prometheus registry
func (mc *MetricsCollector) GetRegistry() *prometheus.Registry {
	return mc.registry
}

// IsEnabled returns whether metrics collection is enabled
func (mc *MetricsCollector) IsEnabled() bool {
	return mc.enabled
}

// StartPeriodicCollection starts periodic metric collection
func (mc *MetricsCollector) StartPeriodicCollection(ctx context.Context, interval time.Duration) {
	if !mc.enabled {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mc.collectSystemMetrics()
			}
		}
	}()

	mc.logger.Info().
		Dur("interval", interval).
		Msg("Started periodic metrics collection")
}

// collectSystemMetrics collects system-level metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	// This would collect system metrics like CPU, memory, disk usage
	// Implementation would depend on the target platform

	mc.logger.Debug().Msg("Collecting system metrics")
}
