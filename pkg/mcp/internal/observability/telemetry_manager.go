package observability

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
)

// TelemetryManager manages metrics collection and export
type TelemetryManager struct {
	registry   *prometheus.Registry
	httpServer *http.Server
	logger     zerolog.Logger

	// Metrics
	toolDuration     *prometheus.HistogramVec
	toolExecutions   *prometheus.CounterVec
	toolErrors       *prometheus.CounterVec
	tokenUsage       *prometheus.CounterVec
	promptTokens     *prometheus.CounterVec
	completionTokens *prometheus.CounterVec
	sessionDuration  *prometheus.HistogramVec
	stageTransitions *prometheus.CounterVec
	activeSessions   prometheus.Gauge
	preflightResults *prometheus.CounterVec

	// Infrastructure operation metrics
	manifestGeneration     *prometheus.HistogramVec
	registryAuthentication *prometheus.CounterVec
	registryValidation     *prometheus.HistogramVec
	kubernetesOperations   *prometheus.CounterVec

	// Performance tracking
	p95Target         time.Duration
	performanceAlerts chan PerformanceAlert
	mutex             sync.RWMutex

	// OpenTelemetry integration
	otelProvider *OTELProvider
}

// TelemetryConfig holds configuration for telemetry
type TelemetryConfig struct {
	MetricsPort      int
	P95Target        time.Duration
	Logger           zerolog.Logger
	EnableAutoExport bool

	// OpenTelemetry configuration
	OTELConfig *OTELConfig `json:"otel_config,omitempty"`
}

// PerformanceAlert represents a performance budget violation
type PerformanceAlert struct {
	Tool      string
	Duration  time.Duration
	Threshold time.Duration
	Timestamp time.Time
}

// NewTelemetryManager creates a new telemetry manager
func NewTelemetryManager(config TelemetryConfig) *TelemetryManager {
	if config.P95Target == 0 {
		config.P95Target = 2 * time.Second
	}

	tm := &TelemetryManager{
		registry:          prometheus.NewRegistry(),
		logger:            config.Logger,
		p95Target:         config.P95Target,
		performanceAlerts: make(chan PerformanceAlert, 100),
	}

	// Initialize OpenTelemetry if configured
	if config.OTELConfig != nil {
		tm.otelProvider = NewOTELProvider(config.OTELConfig)
		if err := tm.otelProvider.Initialize(context.Background()); err != nil {
			config.Logger.Error().Err(err).Msg("Failed to initialize OpenTelemetry")
		} else {
			config.Logger.Info().Msg("OpenTelemetry initialized successfully")
		}
	}

	// Initialize metrics
	tm.initializeMetrics()

	// Register metrics
	tm.registry.MustRegister(
		tm.toolDuration,
		tm.toolExecutions,
		tm.toolErrors,
		tm.tokenUsage,
		tm.promptTokens,
		tm.completionTokens,
		tm.sessionDuration,
		tm.stageTransitions,
		tm.activeSessions,
		tm.preflightResults,
		tm.manifestGeneration,
		tm.registryAuthentication,
		tm.registryValidation,
		tm.kubernetesOperations,
	)

	// Start HTTP server if auto-export enabled
	if config.EnableAutoExport && config.MetricsPort > 0 {
		tm.startMetricsServer(config.MetricsPort)
	}

	// Start performance monitoring
	go tm.monitorPerformance()

	return tm
}

// initializeMetrics creates all Prometheus metrics
func (tm *TelemetryManager) initializeMetrics() {
	// Tool execution duration histogram
	tm.toolDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_tool_duration_seconds",
			Help:    "Tool execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~51.2s
		},
		[]string{"tool", "status", "dry_run"},
	)

	// Tool execution counter
	tm.toolExecutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tool_executions_total",
			Help: "Total number of tool executions",
		},
		[]string{"tool", "status", "dry_run"},
	)

	// Tool error counter
	tm.toolErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tool_errors_total",
			Help: "Total number of tool execution errors",
		},
		[]string{"tool", "error_type"},
	)

	// Token usage counter (legacy - kept for backward compatibility)
	tm.tokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tokens_used_total",
			Help: "Total tokens used by tool",
		},
		[]string{"tool"},
	)

	// LLM prompt tokens counter
	tm.promptTokens = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_prompt_tokens_total",
			Help: "Total prompt tokens sent to LLM",
		},
		[]string{"tool", "model"},
	)

	// LLM completion tokens counter
	tm.completionTokens = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_completion_tokens_total",
			Help: "Total completion tokens received from LLM",
		},
		[]string{"tool", "model"},
	)

	// Session duration histogram
	tm.sessionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_session_duration_seconds",
			Help:    "Session duration from start to completion",
			Buckets: prometheus.ExponentialBuckets(60, 2, 10), // 1min to ~17hrs
		},
		[]string{"completed"},
	)

	// Stage transition counter
	tm.stageTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_stage_transitions_total",
			Help: "Total number of stage transitions",
		},
		[]string{"from_stage", "to_stage"},
	)

	// Active sessions gauge
	tm.activeSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_active_sessions",
			Help: "Number of currently active sessions",
		},
	)

	// Pre-flight check results
	tm.preflightResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_preflight_checks_total",
			Help: "Pre-flight check results",
		},
		[]string{"check", "status"},
	)

	// Infrastructure operation metrics
	tm.manifestGeneration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_manifest_generation_duration_seconds",
			Help:    "Kubernetes manifest generation duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 8), // 0.01s to ~1.28s
		},
		[]string{"manifest_type", "status"},
	)

	tm.registryAuthentication = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_registry_authentication_total",
			Help: "Total number of registry authentication attempts",
		},
		[]string{"registry", "auth_type", "status"},
	)

	tm.registryValidation = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_registry_validation_duration_seconds",
			Help:    "Registry validation duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 8), // 0.1s to ~12.8s
		},
		[]string{"registry", "validation_type", "status"},
	)

	tm.kubernetesOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_kubernetes_operations_total",
			Help: "Total number of Kubernetes operations",
		},
		[]string{"operation", "resource_type", "status"},
	)
}

// RecordToolExecution records metrics for a tool execution
func (tm *TelemetryManager) RecordToolExecution(metrics types.ToolMetrics) {
	status := "success"
	if !metrics.Success {
		status = "failure"
	}

	dryRun := "false"
	if metrics.DryRun {
		dryRun = "true"
	}

	// Record duration
	tm.toolDuration.WithLabelValues(metrics.Tool, status, dryRun).
		Observe(metrics.Duration.Seconds())

	// Increment execution counter
	tm.toolExecutions.WithLabelValues(metrics.Tool, status, dryRun).Inc()

	// Record token usage if applicable
	if metrics.TokensUsed > 0 {
		tm.tokenUsage.WithLabelValues(metrics.Tool).
			Add(float64(metrics.TokensUsed))
	}

	// Check performance budget
	if metrics.Duration > tm.p95Target && !metrics.DryRun {
		alert := PerformanceAlert{
			Tool:      metrics.Tool,
			Duration:  metrics.Duration,
			Threshold: tm.p95Target,
			Timestamp: time.Now(),
		}

		// Non-blocking send
		select {
		case tm.performanceAlerts <- alert:
		default:
			tm.logger.Warn().
				Str("tool", metrics.Tool).
				Dur("duration", metrics.Duration).
				Msg("Performance alert channel full")
		}
	}
}

// RecordToolError records a tool execution error
func (tm *TelemetryManager) RecordToolError(tool, errorType string) {
	tm.toolErrors.WithLabelValues(tool, errorType).Inc()
}

// RecordLLMTokenUsage records LLM token usage metrics
func (tm *TelemetryManager) RecordLLMTokenUsage(tool, model string, promptTokens, completionTokens int) {
	if promptTokens > 0 {
		tm.promptTokens.WithLabelValues(tool, model).
			Add(float64(promptTokens))
	}

	if completionTokens > 0 {
		tm.completionTokens.WithLabelValues(tool, model).
			Add(float64(completionTokens))
	}

	// Also update the legacy total token counter for backward compatibility
	totalTokens := promptTokens + completionTokens
	if totalTokens > 0 {
		tm.tokenUsage.WithLabelValues(tool).
			Add(float64(totalTokens))
	}
}

// RecordSessionStart records the start of a session
func (tm *TelemetryManager) RecordSessionStart() {
	tm.activeSessions.Inc()
}

// RecordSessionEnd records the end of a session
func (tm *TelemetryManager) RecordSessionEnd(duration time.Duration, completed bool) {
	tm.activeSessions.Dec()

	completedStr := "false"
	if completed {
		completedStr = "true"
	}

	tm.sessionDuration.WithLabelValues(completedStr).
		Observe(duration.Seconds())
}

// RecordStageTransition records a conversation stage transition
func (tm *TelemetryManager) RecordStageTransition(fromStage, toStage string) {
	tm.stageTransitions.WithLabelValues(fromStage, toStage).Inc()
}

// RecordPreflightCheck records pre-flight check results
func (tm *TelemetryManager) RecordPreflightCheck(checkName string, status CheckStatus) {
	tm.preflightResults.WithLabelValues(checkName, string(status)).Inc()
}

// GetMetrics returns current metrics as a map
func (tm *TelemetryManager) GetMetrics() (map[string]interface{}, error) {
	metricFamilies, err := tm.registry.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather metrics: %w", err)
	}

	metrics := make(map[string]interface{})

	for _, mf := range metricFamilies {
		name := mf.GetName()
		metrics[name] = mf.GetMetric()
	}

	return metrics, nil
}

// GetSLOPerformanceReport generates a performance report
func (tm *TelemetryManager) GetSLOPerformanceReport() SLOPerformanceReport {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// Gather tool performance stats
	// This is simplified - in production, you'd query Prometheus
	report := SLOPerformanceReport{
		Timestamp:      time.Now(),
		P95Target:      tm.p95Target,
		ToolStats:      make(map[string]ToolPerformanceStats),
		ViolationCount: 0,
	}

	// Count recent violations
	timeout := time.After(10 * time.Millisecond)
	for {
		select {
		case alert := <-tm.performanceAlerts:
			report.ViolationCount++
			if stats, ok := report.ToolStats[alert.Tool]; ok {
				stats.Violations++
				if alert.Duration > stats.MaxDuration {
					stats.MaxDuration = alert.Duration
				}
				report.ToolStats[alert.Tool] = stats
			} else {
				report.ToolStats[alert.Tool] = ToolPerformanceStats{
					Tool:        alert.Tool,
					Violations:  1,
					MaxDuration: alert.Duration,
				}
			}
		case <-timeout:
			return report
		}
	}
}

// startMetricsServer starts the Prometheus metrics HTTP server
func (tm *TelemetryManager) startMetricsServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(tm.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			// Log error but response is already committed
			tm.logger.Debug().Err(err).Msg("Failed to write health check response")
		}
	})

	tm.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		tm.logger.Info().Int("port", port).Msg("Starting metrics server")
		if err := tm.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			tm.logger.Error().Err(err).Msg("Metrics server error")
		}
	}()
}

// monitorPerformance monitors for performance issues
func (tm *TelemetryManager) monitorPerformance() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		report := tm.GetSLOPerformanceReport()
		if report.ViolationCount > 0 {
			tm.logger.Warn().
				Int("violations", report.ViolationCount).
				Dur("p95_target", tm.p95Target).
				Msg("Performance budget violations detected")

			// Log details for each tool with violations
			for tool, stats := range report.ToolStats {
				if stats.Violations > 0 {
					tm.logger.Warn().
						Str("tool", tool).
						Int("violations", stats.Violations).
						Dur("max_duration", stats.MaxDuration).
						Msg("Tool performance violation details")
				}
			}
		}
	}
}

// Shutdown gracefully shuts down the telemetry manager
func (tm *TelemetryManager) Shutdown(ctx context.Context) error {
	var shutdownErrors []error

	// Shutdown OpenTelemetry first
	if tm.otelProvider != nil {
		if err := tm.otelProvider.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, err)
			tm.logger.Error().Err(err).Msg("Error shutting down OpenTelemetry")
		}
	}

	// Shutdown HTTP metrics server
	if tm.httpServer != nil {
		if err := tm.httpServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, err)
			tm.logger.Error().Err(err).Msg("Error shutting down metrics server")
		}
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("telemetry shutdown errors: %v", shutdownErrors)
	}

	return nil
}

// GetOTELProvider returns the OpenTelemetry provider
func (tm *TelemetryManager) GetOTELProvider() *OTELProvider {
	return tm.otelProvider
}

// IsOTELEnabled returns whether OpenTelemetry is enabled and initialized
func (tm *TelemetryManager) IsOTELEnabled() bool {
	return tm.otelProvider != nil && tm.otelProvider.IsInitialized()
}

// UpdateOTELConfig updates the OpenTelemetry configuration
func (tm *TelemetryManager) UpdateOTELConfig(updates map[string]interface{}) {
	if tm.otelProvider != nil {
		tm.otelProvider.UpdateConfig(updates)
	}
}

// SLOPerformanceReport represents a performance analysis report
type SLOPerformanceReport struct {
	Timestamp      time.Time                       `json:"timestamp"`
	P95Target      time.Duration                   `json:"p95_target"`
	ToolStats      map[string]ToolPerformanceStats `json:"tool_stats"`
	ViolationCount int                             `json:"violation_count"`
}

// ToolPerformanceStats represents performance statistics for a tool
type ToolPerformanceStats struct {
	Tool        string        `json:"tool"`
	Violations  int           `json:"violations"`
	MaxDuration time.Duration `json:"max_duration"`
	AvgDuration time.Duration `json:"avg_duration,omitempty"`
	P95Duration time.Duration `json:"p95_duration,omitempty"`
}

// ExportMetrics exports metrics in Prometheus format
func (tm *TelemetryManager) ExportMetrics() (string, error) {
	metricFamilies, err := tm.registry.Gather()
	if err != nil {
		return "", fmt.Errorf("failed to gather metrics: %w", err)
	}

	// Use proper Prometheus text format encoder
	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.FmtText)

	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			return "", fmt.Errorf("failed to encode metric family: %w", err)
		}
	}

	return buf.String(), nil
}

// RecordManifestGeneration records metrics for manifest generation operations
func (tm *TelemetryManager) RecordManifestGeneration(manifestType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	tm.manifestGeneration.WithLabelValues(manifestType, status).Observe(duration.Seconds())
}

// RecordRegistryAuthentication records metrics for registry authentication attempts
func (tm *TelemetryManager) RecordRegistryAuthentication(registry, authType string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	tm.registryAuthentication.WithLabelValues(registry, authType, status).Inc()
}

// RecordRegistryValidation records metrics for registry validation operations
func (tm *TelemetryManager) RecordRegistryValidation(registry, validationType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	tm.registryValidation.WithLabelValues(registry, validationType, status).Observe(duration.Seconds())
}

// RecordKubernetesOperation records metrics for Kubernetes operations
func (tm *TelemetryManager) RecordKubernetesOperation(operation, resourceType string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	tm.kubernetesOperations.WithLabelValues(operation, resourceType, status).Inc()
}
