package build

import (
	"context"
	"sync"
	"time"

	"log/slog"
)

var (
	metricsOnce   sync.Once
	globalMetrics *BuildMetrics
)

// PerformanceMonitor tracks and analyzes build performance metrics
type PerformanceMonitor struct {
	logger    *slog.Logger
	metrics   *BuildMetrics
	analyzer  *PerformanceAnalyzer
	collector *MetricsCollector
	mu        sync.RWMutex
}

// BuildMetrics contains basic metrics for build operations
type BuildMetrics struct {
	// Counters for operations
	BuildsTotal     map[string]int64
	BuildsSucceeded map[string]int64
	BuildsFailed    map[string]int64
	CacheHits       map[string]int64
	CacheMisses     map[string]int64

	// Gauges for current state
	ActiveBuilds   int64
	BuildQueueSize int64
	CacheSize      int64

	// Durations tracking
	BuildDurations []time.Duration
	StageDurations map[string][]time.Duration

	mu sync.RWMutex
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger *slog.Logger) *PerformanceMonitor {
	// Initialize metrics only once
	metricsOnce.Do(func() {
		globalMetrics = &BuildMetrics{
			BuildsTotal:     make(map[string]int64),
			BuildsSucceeded: make(map[string]int64),
			BuildsFailed:    make(map[string]int64),
			CacheHits:       make(map[string]int64),
			CacheMisses:     make(map[string]int64),
			BuildDurations:  make([]time.Duration, 0),
			StageDurations:  make(map[string][]time.Duration),
		}
	})

	return &PerformanceMonitor{
		logger:    logger.With("component", "performance_monitor"),
		metrics:   globalMetrics,
		analyzer:  NewPerformanceAnalyzer(logger),
		collector: NewMetricsCollector(logger, globalMetrics),
	}
}

// StartBuildMonitoring starts monitoring a build operation
func (m *PerformanceMonitor) StartBuildMonitoring(ctx context.Context, operation *BuildOperation) *BuildMonitor {
	m.metrics.mu.Lock()
	key := operation.Tool + "_" + operation.Type
	m.metrics.BuildsTotal[key]++
	m.metrics.ActiveBuilds++
	m.metrics.mu.Unlock()

	monitor := &BuildMonitor{
		operation: operation,
		metrics:   m.metrics,
		logger:    m.logger,
		startTime: time.Now(),
		stages:    make(map[string]*StageMetrics),
		mu:        &sync.Mutex{},
	}

	m.logger.Info("Build monitoring started",
		"operation", operation.Name,
		"tool", operation.Tool,
		"type", operation.Type,
		"context_size", operation.ContextSize)

	// Log context size if available
	if operation.ContextSize > 0 {
		m.logger.Debug("Build context size recorded",
			"tool", operation.Tool,
			"session_id", operation.SessionID,
			"context_size", operation.ContextSize)
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
	logger    *slog.Logger
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

	b.logger.Debug("Build stage started",
		"stage", stageName,
		"operation", b.operation.Name)
}

// EndStage marks the end of a build stage
func (b *BuildMonitor) EndStage(stageName string, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	stage, exists := b.stages[stageName]
	if !exists {
		b.logger.Warn("Ending unknown stage", "stage", stageName)
		return
	}

	stage.EndTime = time.Now()
	stage.Duration = stage.EndTime.Sub(stage.StartTime)
	stage.Success = success

	// Record stage duration
	b.metrics.mu.Lock()
	if b.metrics.StageDurations[stageName] == nil {
		b.metrics.StageDurations[stageName] = make([]time.Duration, 0)
	}
	b.metrics.StageDurations[stageName] = append(b.metrics.StageDurations[stageName], stage.Duration)
	b.metrics.mu.Unlock()

	b.logger.Debug("Build stage completed",
		"stage", stageName,
		"duration", stage.Duration,
		"success", success)
}

// RecordCacheHit records a Docker cache hit
func (b *BuildMonitor) RecordCacheHit(layer string) {
	b.metrics.mu.Lock()
	key := b.operation.Tool + "_" + layer
	b.metrics.CacheHits[key]++
	b.metrics.mu.Unlock()

	b.logger.Debug("Cache hit recorded",
		"layer", layer,
		"tool", b.operation.Tool)
}

// RecordCacheMiss records a Docker cache miss
func (b *BuildMonitor) RecordCacheMiss(layer string) {
	b.metrics.mu.Lock()
	key := b.operation.Tool + "_" + layer
	b.metrics.CacheMisses[key]++
	b.metrics.mu.Unlock()

	b.logger.Debug("Cache miss recorded",
		"layer", layer,
		"tool", b.operation.Tool)
}

// Complete marks the build operation as complete
func (b *BuildMonitor) Complete(success bool, errorType string, imageInfo *BuiltImageInfo) {
	b.endTime = time.Now()
	b.success = success
	b.errorType = errorType

	duration := b.endTime.Sub(b.startTime)

	// Record completion metrics
	b.metrics.mu.Lock()
	key := b.operation.Tool + "_" + b.operation.Type
	if !success {
		b.metrics.BuildsFailed[key+"_"+errorType]++
	} else {
		b.metrics.BuildsSucceeded[key]++
	}
	b.metrics.BuildDurations = append(b.metrics.BuildDurations, duration)
	b.metrics.ActiveBuilds--
	b.metrics.mu.Unlock()

	// Log build completion with context
	logArgs := []interface{}{
		"operation", b.operation.Name,
		"total_duration", duration,
		"success", success,
		"error_type", errorType,
	}

	// Add image metrics if available
	if imageInfo != nil && success {
		logArgs = append(logArgs,
			"image_name", imageInfo.Name,
			"image_tag", imageInfo.Tag,
			"layer_count", imageInfo.LayerCount,
			"image_size", imageInfo.Size,
		)
	}

	b.logger.Info("Build operation completed", logArgs...)
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
	logger *slog.Logger
}

func NewPerformanceAnalyzer(logger *slog.Logger) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		logger: logger.With("component", "performance_analyzer"),
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
	logger  *slog.Logger
	metrics *BuildMetrics
}

func NewMetricsCollector(logger *slog.Logger, metrics *BuildMetrics) *MetricsCollector {
	return &MetricsCollector{
		logger:  logger.With("component", "metrics_collector"),
		metrics: metrics,
	}
}

// CollectSummary collects a summary of current metrics
func (c *MetricsCollector) CollectSummary() *MetricsSummary {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	// Calculate totals
	totalBuilds := int64(0)
	successfulBuilds := int64(0)
	totalCacheHits := int64(0)
	totalCacheMisses := int64(0)

	for _, count := range c.metrics.BuildsTotal {
		totalBuilds += count
	}
	for _, count := range c.metrics.BuildsSucceeded {
		successfulBuilds += count
	}
	for _, count := range c.metrics.CacheHits {
		totalCacheHits += count
	}
	for _, count := range c.metrics.CacheMisses {
		totalCacheMisses += count
	}

	// Calculate success rate
	successRate := 0.0
	if totalBuilds > 0 {
		successRate = float64(successfulBuilds) / float64(totalBuilds)
	}

	// Calculate cache hit rate
	cacheHitRate := 0.0
	totalCacheOps := totalCacheHits + totalCacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(totalCacheHits) / float64(totalCacheOps)
	}

	// Calculate average build time
	avgBuildTime := time.Duration(0)
	if len(c.metrics.BuildDurations) > 0 {
		totalDuration := time.Duration(0)
		for _, duration := range c.metrics.BuildDurations {
			totalDuration += duration
		}
		avgBuildTime = totalDuration / time.Duration(len(c.metrics.BuildDurations))
	}

	return &MetricsSummary{
		Timestamp:    time.Now(),
		ActiveBuilds: int(c.metrics.ActiveBuilds),
		TotalBuilds:  int(totalBuilds),
		SuccessRate:  successRate,
		AvgBuildTime: avgBuildTime,
		CacheHitRate: cacheHitRate,
		QueueSize:    int(c.metrics.BuildQueueSize),
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

// BuiltImageInfo contains information about a built image
type BuiltImageInfo struct {
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
