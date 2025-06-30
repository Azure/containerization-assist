package build

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

// PerformanceMonitor tracks and analyzes build performance metrics
type PerformanceMonitor struct {
	logger    zerolog.Logger
	metrics   *BuildMetrics
	analyzer  *PerformanceAnalyzer
	collector *MetricsCollector
	mu        sync.RWMutex
}

// BuildMetrics contains Prometheus metrics for build operations
type BuildMetrics struct {
	// Histograms for timing
	BuildDuration *prometheus.HistogramVec
	StageDuration *prometheus.HistogramVec

	// Counters for operations
	BuildsTotal     *prometheus.CounterVec
	BuildsSucceeded *prometheus.CounterVec
	BuildsFailed    *prometheus.CounterVec
	CacheHits       *prometheus.CounterVec
	CacheMisses     *prometheus.CounterVec

	// Gauges for current state
	ActiveBuilds   prometheus.Gauge
	BuildQueueSize prometheus.Gauge
	CacheSize      prometheus.Gauge
	LayerCount     *prometheus.GaugeVec
	ImageSize      *prometheus.GaugeVec
	ContextSize    *prometheus.GaugeVec

	// Summary for percentiles
	BuildTimeSummary *prometheus.SummaryVec
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger zerolog.Logger) *PerformanceMonitor {
	metrics := &BuildMetrics{
		BuildDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "container_kit_build_duration_seconds",
				Help:    "Duration of Docker build operations",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
			},
			[]string{"tool", "status", "strategy"},
		),

		StageDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "container_kit_build_stage_duration_seconds",
				Help:    "Duration of individual build stages",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
			},
			[]string{"tool", "stage", "status"},
		),

		BuildsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "container_kit_builds_total",
				Help: "Total number of build operations",
			},
			[]string{"tool", "type"},
		),

		BuildsSucceeded: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "container_kit_builds_succeeded_total",
				Help: "Total number of successful build operations",
			},
			[]string{"tool", "type"},
		),

		BuildsFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "container_kit_builds_failed_total",
				Help: "Total number of failed build operations",
			},
			[]string{"tool", "type", "error_type"},
		),

		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "container_kit_cache_hits_total",
				Help: "Total number of Docker cache hits",
			},
			[]string{"tool", "layer"},
		),

		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "container_kit_cache_misses_total",
				Help: "Total number of Docker cache misses",
			},
			[]string{"tool", "layer"},
		),

		ActiveBuilds: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "container_kit_active_builds",
				Help: "Number of currently active build operations",
			},
		),

		BuildQueueSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "container_kit_build_queue_size",
				Help: "Number of builds waiting in queue",
			},
		),

		CacheSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "container_kit_cache_size_bytes",
				Help: "Total size of Docker build cache",
			},
		),

		LayerCount: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "container_kit_image_layers",
				Help: "Number of layers in built images",
			},
			[]string{"image", "tag"},
		),

		ImageSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "container_kit_image_size_bytes",
				Help: "Size of built Docker images",
			},
			[]string{"image", "tag"},
		),

		ContextSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "container_kit_build_context_size_bytes",
				Help: "Size of build context",
			},
			[]string{"tool", "session"},
		),

		BuildTimeSummary: promauto.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "container_kit_build_time_summary_seconds",
				Help:       "Summary of build times with percentiles",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			},
			[]string{"tool", "type"},
		),
	}

	return &PerformanceMonitor{
		logger:    logger.With().Str("component", "performance_monitor").Logger(),
		metrics:   metrics,
		analyzer:  NewPerformanceAnalyzer(logger),
		collector: NewMetricsCollector(logger, metrics),
	}
}

// StartBuildMonitoring starts monitoring a build operation
func (m *PerformanceMonitor) StartBuildMonitoring(ctx context.Context, operation *BuildOperation) *BuildMonitor {
	m.metrics.BuildsTotal.WithLabelValues(operation.Tool, operation.Type).Inc()
	m.metrics.ActiveBuilds.Inc()

	monitor := &BuildMonitor{
		operation: operation,
		metrics:   m.metrics,
		logger:    m.logger,
		startTime: time.Now(),
		stages:    make(map[string]*StageMetrics),
		mu:        &sync.Mutex{},
	}

	// Track context size if available
	if operation.ContextSize > 0 {
		m.metrics.ContextSize.WithLabelValues(operation.Tool, operation.SessionID).Set(float64(operation.ContextSize))
	}

	return monitor
}

// AnalyzePerformance analyzes build performance and provides insights
func (m *PerformanceMonitor) AnalyzePerformance(ctx context.Context, sessionID string) (*BuildPerformanceAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.analyzer.Analyze(ctx, sessionID, m.metrics)
}

// GetMetricsSummary returns a summary of current metrics
func (m *PerformanceMonitor) GetMetricsSummary() *MetricsSummary {
	return m.collector.CollectSummary()
}

// BuildMonitor monitors an individual build operation
type BuildMonitor struct {
	operation *BuildOperation
	metrics   *BuildMetrics
	logger    zerolog.Logger
	startTime time.Time
	endTime   time.Time
	stages    map[string]*StageMetrics
	mu        *sync.Mutex
	success   bool
	errorType string
}

// StartStage marks the beginning of a build stage
func (b *BuildMonitor) StartStage(stageName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stages[stageName] = &StageMetrics{
		Name:      stageName,
		StartTime: time.Now(),
	}

	b.logger.Debug().
		Str("stage", stageName).
		Str("operation", b.operation.Name).
		Msg("Build stage started")
}

// EndStage marks the end of a build stage
func (b *BuildMonitor) EndStage(stageName string, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	stage, exists := b.stages[stageName]
	if !exists {
		b.logger.Warn().Str("stage", stageName).Msg("Ending unknown stage")
		return
	}

	stage.EndTime = time.Now()
	stage.Duration = stage.EndTime.Sub(stage.StartTime)
	stage.Success = success

	// Record stage metrics
	status := "success"
	if !success {
		status = "failure"
	}

	b.metrics.StageDuration.WithLabelValues(b.operation.Tool, stageName, status).Observe(stage.Duration.Seconds())

	b.logger.Debug().
		Str("stage", stageName).
		Dur("duration", stage.Duration).
		Bool("success", success).
		Msg("Build stage completed")
}

// RecordCacheHit records a Docker cache hit
func (b *BuildMonitor) RecordCacheHit(layer string) {
	b.metrics.CacheHits.WithLabelValues(b.operation.Tool, layer).Inc()
}

// RecordCacheMiss records a Docker cache miss
func (b *BuildMonitor) RecordCacheMiss(layer string) {
	b.metrics.CacheMisses.WithLabelValues(b.operation.Tool, layer).Inc()
}

// Complete marks the build operation as complete
func (b *BuildMonitor) Complete(success bool, errorType string, imageInfo *BuildImageInfo) {
	b.endTime = time.Now()
	b.success = success
	b.errorType = errorType

	duration := b.endTime.Sub(b.startTime)

	// Record completion metrics
	status := "success"
	if !success {
		status = "failure"
		b.metrics.BuildsFailed.WithLabelValues(b.operation.Tool, b.operation.Type, errorType).Inc()
	} else {
		b.metrics.BuildsSucceeded.WithLabelValues(b.operation.Tool, b.operation.Type).Inc()
	}

	b.metrics.BuildDuration.WithLabelValues(b.operation.Tool, status, b.operation.Strategy).Observe(duration.Seconds())
	b.metrics.BuildTimeSummary.WithLabelValues(b.operation.Tool, b.operation.Type).Observe(duration.Seconds())
	b.metrics.ActiveBuilds.Dec()

	// Record image metrics if available
	if imageInfo != nil && success {
		b.metrics.LayerCount.WithLabelValues(imageInfo.Name, imageInfo.Tag).Set(float64(imageInfo.LayerCount))
		b.metrics.ImageSize.WithLabelValues(imageInfo.Name, imageInfo.Tag).Set(float64(imageInfo.Size))
	}

	b.logger.Info().
		Str("operation", b.operation.Name).
		Dur("total_duration", duration).
		Bool("success", success).
		Str("error_type", errorType).
		Msg("Build operation completed")
}

// GetReport generates a performance report for this build
func (b *BuildMonitor) GetReport() *BuildPerformanceReport {
	b.mu.Lock()
	defer b.mu.Unlock()

	report := &BuildPerformanceReport{
		Operation:     b.operation,
		StartTime:     b.startTime,
		EndTime:       b.endTime,
		TotalDuration: b.endTime.Sub(b.startTime),
		Success:       b.success,
		ErrorType:     b.errorType,
		Stages:        make([]StageReport, 0, len(b.stages)),
	}

	// Add stage reports
	for _, stage := range b.stages {
		report.Stages = append(report.Stages, StageReport{
			Name:       stage.Name,
			Duration:   stage.Duration,
			Success:    stage.Success,
			Percentage: float64(stage.Duration) / float64(report.TotalDuration) * 100,
		})
	}

	return report
}

// PerformanceAnalyzer analyzes build performance patterns
type PerformanceAnalyzer struct {
	logger zerolog.Logger
}

func NewPerformanceAnalyzer(logger zerolog.Logger) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		logger: logger.With().Str("component", "performance_analyzer").Logger(),
	}
}

// Analyze performs performance analysis
func (a *PerformanceAnalyzer) Analyze(ctx context.Context, sessionID string, metrics *BuildMetrics) (*BuildPerformanceAnalysis, error) {
	analysis := &BuildPerformanceAnalysis{
		SessionID:       sessionID,
		AnalysisTime:    time.Now(),
		Insights:        []PerformanceInsight{},
		Recommendations: []string{},
		Trends:          PerformanceTrends{},
	}

	// Analyze cache efficiency
	// This would query metrics to calculate cache hit rate

	// Analyze build time trends
	// This would look at historical data to identify patterns

	// Generate insights
	analysis.Insights = a.generateInsights(metrics)

	// Generate recommendations
	analysis.Recommendations = a.generateRecommendations(analysis.Insights)

	return analysis, nil
}

// generateInsights generates performance insights
func (a *PerformanceAnalyzer) generateInsights(metrics *BuildMetrics) []PerformanceInsight {
	insights := []PerformanceInsight{}

	// Example insights - in real implementation would analyze actual metrics
	insights = append(insights, PerformanceInsight{
		Type:        "cache_efficiency",
		Severity:    "info",
		Title:       "Cache Performance",
		Description: "Docker cache is performing well with high hit rate",
		Metrics: map[string]interface{}{
			"cache_hit_rate": 0.85,
		},
	})

	return insights
}

// generateRecommendations generates performance recommendations
func (a *PerformanceAnalyzer) generateRecommendations(insights []PerformanceInsight) []string {
	recommendations := []string{}

	for _, insight := range insights {
		switch insight.Type {
		case "slow_builds":
			recommendations = append(recommendations, "Consider using BuildKit for improved build performance")
		case "large_context":
			recommendations = append(recommendations, "Reduce build context size with .dockerignore")
		case "cache_misses":
			recommendations = append(recommendations, "Optimize Dockerfile layer ordering for better caching")
		}
	}

	return recommendations
}

// MetricsCollector collects and summarizes metrics
type MetricsCollector struct {
	logger  zerolog.Logger
	metrics *BuildMetrics
}

func NewMetricsCollector(logger zerolog.Logger, metrics *BuildMetrics) *MetricsCollector {
	return &MetricsCollector{
		logger:  logger.With().Str("component", "metrics_collector").Logger(),
		metrics: metrics,
	}
}

// CollectSummary collects a summary of current metrics
func (c *MetricsCollector) CollectSummary() *MetricsSummary {
	// This would collect actual metric values
	// For now, return example summary
	return &MetricsSummary{
		Timestamp:    time.Now(),
		ActiveBuilds: 0,
		TotalBuilds:  100,
		SuccessRate:  0.95,
		AvgBuildTime: 5 * time.Minute,
		CacheHitRate: 0.85,
		QueueSize:    0,
	}
}

// Types for performance monitoring

// BuildOperation represents a build operation being monitored
type BuildOperation struct {
	Name        string `json:"name"`
	Tool        string `json:"tool"`
	Type        string `json:"type"`
	Strategy    string `json:"strategy"`
	SessionID   string `json:"session_id"`
	ContextSize int64  `json:"context_size"`
}

// StageMetrics contains metrics for a build stage
type StageMetrics struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Success   bool
}

// BuildImageInfo contains information about a built image
type BuildImageInfo struct {
	Name       string
	Tag        string
	Size       int64
	LayerCount int
}

// BuildPerformanceReport contains performance report for a build
type BuildPerformanceReport struct {
	Operation     *BuildOperation `json:"operation"`
	StartTime     time.Time       `json:"start_time"`
	EndTime       time.Time       `json:"end_time"`
	TotalDuration time.Duration   `json:"total_duration"`
	Success       bool            `json:"success"`
	ErrorType     string          `json:"error_type,omitempty"`
	Stages        []StageReport   `json:"stages"`
}

// StageReport contains report for a build stage
type StageReport struct {
	Name       string        `json:"name"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	Percentage float64       `json:"percentage"`
}

// BuildPerformanceAnalysis contains performance analysis results
type BuildPerformanceAnalysis struct {
	SessionID       string               `json:"session_id"`
	AnalysisTime    time.Time            `json:"analysis_time"`
	Insights        []PerformanceInsight `json:"insights"`
	Recommendations []string             `json:"recommendations"`
	Trends          PerformanceTrends    `json:"trends"`
}

// PerformanceInsight represents a performance insight
type PerformanceInsight struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Metrics     map[string]interface{} `json:"metrics"`
}

// PerformanceTrends contains performance trend data
type PerformanceTrends struct {
	BuildTimeChange      float64 `json:"build_time_change_percent"`
	CacheEfficiencyTrend string  `json:"cache_efficiency_trend"`
	ErrorRateTrend       string  `json:"error_rate_trend"`
}

// MetricsSummary contains a summary of current metrics
type MetricsSummary struct {
	Timestamp    time.Time     `json:"timestamp"`
	ActiveBuilds int           `json:"active_builds"`
	TotalBuilds  int           `json:"total_builds"`
	SuccessRate  float64       `json:"success_rate"`
	AvgBuildTime time.Duration `json:"avg_build_time"`
	CacheHitRate float64       `json:"cache_hit_rate"`
	QueueSize    int           `json:"queue_size"`
}
