// Package metrics provides Prometheus-based metrics collection for workflows
package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// WorkflowMetricsCollector implements comprehensive metrics collection for containerization workflows
type WorkflowMetricsCollector struct {
	// Prometheus metrics
	workflowTotal       *prometheus.CounterVec
	workflowDuration    *prometheus.HistogramVec
	stepDuration        *prometheus.HistogramVec
	stepTotal           *prometheus.CounterVec
	retryTotal          *prometheus.CounterVec
	errorTotal          *prometheus.CounterVec
	dockerBuildDuration prometheus.Histogram
	imagePushDuration   *prometheus.HistogramVec
	deployDuration      *prometheus.HistogramVec
	imageSize           prometheus.Histogram
	vulnerabilities     *prometheus.GaugeVec
	cacheHits           *prometheus.CounterVec
	cacheMisses         *prometheus.CounterVec
	llmTokensUsed       *prometheus.CounterVec
	patternAccuracy     *prometheus.CounterVec
	enhancementImpact   prometheus.Histogram
	adaptationApplied   *prometheus.CounterVec

	// In-memory metrics for real-time access
	activeWorkflows atomic.Int64
	recentErrors    []workflow.ErrorEvent
	errorsMutex     sync.RWMutex
	stepExecutions  sync.Map // map[string]*atomic.Int64
	thresholds      sync.Map // map[string]float64
	stepDurations   sync.Map // map[string]time.Duration - temporary storage for durations

	// Metrics cache for snapshots
	metricsCache *metricsCache
}

// metricsCache provides thread-safe caching of metrics data
type metricsCache struct {
	mu                sync.RWMutex
	workflowCount     int64
	successCount      int64
	failureCount      int64
	totalDuration     time.Duration
	stepMetrics       map[string]*workflow.StepMetrics
	errorsByCategory  map[string]int64
	totalRetries      int64
	dockerBuildTime   time.Duration
	dockerBuildCount  int64
	totalImageSize    int64
	imageCount        int64
	deployTime        time.Duration
	deployCount       int64
	cacheHits         int64
	cacheMisses       int64
	totalTokens       int64
	patternCorrect    int64
	patternTotal      int64
	enhancementTotal  float64
	enhancementCount  int64
	deploymentSuccess int64
	deploymentTotal   int64
	failureReasons    map[string]int64
	securityMetrics   *workflow.SecurityMetrics
}

// NewWorkflowMetricsCollector creates a new Prometheus-based metrics collector
func NewWorkflowMetricsCollector(namespace string) *WorkflowMetricsCollector {
	collector := &WorkflowMetricsCollector{
		workflowTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "workflow_total",
			Help:      "Total number of workflows executed",
		}, []string{"status"}),

		workflowDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "workflow_duration_seconds",
			Help:      "Workflow execution duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(10, 2, 10), // 10s to ~2.8h
		}, []string{"status"}),

		stepDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "step_duration_seconds",
			Help:      "Step execution duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
		}, []string{"step", "status"}),

		stepTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "step_total",
			Help:      "Total number of step executions",
		}, []string{"step", "status"}),

		retryTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retry_total",
			Help:      "Total number of retries",
		}, []string{"step", "attempt"}),

		errorTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "error_total",
			Help:      "Total number of errors by category",
		}, []string{"category", "step"}),

		dockerBuildDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "docker_build_duration_seconds",
			Help:      "Docker build duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(30, 2, 8), // 30s to ~2h
		}),

		imagePushDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "image_push_duration_seconds",
			Help:      "Image push duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(10, 2, 8), // 10s to ~43min
		}, []string{"registry"}),

		deployDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "deploy_duration_seconds",
			Help:      "Deployment duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(30, 2, 8), // 30s to ~2h
		}, []string{"namespace"}),

		imageSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "image_size_bytes",
			Help:      "Docker image size in bytes",
			Buckets:   prometheus.ExponentialBuckets(1e6, 2, 10), // 1MB to ~1GB
		}),

		vulnerabilities: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vulnerabilities_total",
			Help:      "Total vulnerabilities found by severity",
		}, []string{"severity"}),

		cacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total cache hits",
		}, []string{"cache_type"}),

		cacheMisses: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_misses_total",
			Help:      "Total cache misses",
		}, []string{"cache_type"}),

		llmTokensUsed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "llm_tokens_total",
			Help:      "Total LLM tokens used",
		}, []string{"type"}), // prompt, completion

		patternAccuracy: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pattern_recognition_total",
			Help:      "Pattern recognition accuracy",
		}, []string{"result"}), // correct, incorrect

		enhancementImpact: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "enhancement_impact_percent",
			Help:      "Step enhancement improvement percentage",
			Buckets:   prometheus.LinearBuckets(0, 10, 11), // 0% to 100%
		}),

		adaptationApplied: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "adaptation_applied_total",
			Help:      "Total adaptations applied",
		}, []string{"type", "status"}),

		recentErrors: make([]workflow.ErrorEvent, 0, 100),
		metricsCache: &metricsCache{
			stepMetrics:      make(map[string]*workflow.StepMetrics),
			errorsByCategory: make(map[string]int64),
			failureReasons:   make(map[string]int64),
			securityMetrics:  &workflow.SecurityMetrics{},
		},
	}

	return collector
}

// RecordStepDuration implements MetricsCollector
func (m *WorkflowMetricsCollector) RecordStepDuration(stepName string, duration time.Duration) {
	// Record duration metric only - step counting happens in RecordStepSuccess/Failure
	m.stepDuration.WithLabelValues(stepName, "completed").Observe(duration.Seconds())
	// Store duration temporarily for use in RecordStepSuccess
	m.stepDurations.Store(stepName, duration)
}

// RecordStepSuccess implements MetricsCollector
func (m *WorkflowMetricsCollector) RecordStepSuccess(stepName string) {
	m.stepTotal.WithLabelValues(stepName, "success").Inc()
	// Get stored duration if available
	duration := time.Duration(0)
	if d, ok := m.stepDurations.LoadAndDelete(stepName); ok {
		duration = d.(time.Duration)
	}
	m.updateStepMetrics(stepName, duration, true)
}

// RecordStepFailure implements MetricsCollector
func (m *WorkflowMetricsCollector) RecordStepFailure(stepName string) {
	m.stepTotal.WithLabelValues(stepName, "failure").Inc()
	m.updateStepMetrics(stepName, 0, false)
}

// RecordWorkflowStart implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordWorkflowStart(workflowID string) {
	m.activeWorkflows.Add(1)
}

// RecordWorkflowEnd implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordWorkflowEnd(workflowID string, success bool, duration time.Duration) {
	m.activeWorkflows.Add(-1)

	status := "success"
	if !success {
		status = "failure"
	}

	m.workflowTotal.WithLabelValues(status).Inc()
	m.workflowDuration.WithLabelValues(status).Observe(duration.Seconds())

	// Update cache
	m.metricsCache.mu.Lock()
	m.metricsCache.workflowCount++
	if success {
		m.metricsCache.successCount++
	} else {
		m.metricsCache.failureCount++
	}
	m.metricsCache.totalDuration += duration
	m.metricsCache.mu.Unlock()
}

// RecordWorkflowRetry implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordWorkflowRetry(workflowID string, stepName string, attempt int) {
	m.retryTotal.WithLabelValues(stepName, fmt.Sprintf("%d", attempt)).Inc()

	m.metricsCache.mu.Lock()
	m.metricsCache.totalRetries++
	m.metricsCache.mu.Unlock()
}

// RecordDockerBuildTime implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordDockerBuildTime(duration time.Duration, imageSize int64) {
	m.dockerBuildDuration.Observe(duration.Seconds())
	m.imageSize.Observe(float64(imageSize))

	m.metricsCache.mu.Lock()
	m.metricsCache.dockerBuildTime += duration
	m.metricsCache.dockerBuildCount++
	m.metricsCache.totalImageSize += imageSize
	m.metricsCache.imageCount++
	m.metricsCache.mu.Unlock()
}

// RecordImagePushTime implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordImagePushTime(duration time.Duration, registryURL string) {
	m.imagePushDuration.WithLabelValues(registryURL).Observe(duration.Seconds())
}

// RecordKubernetesDeployTime implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordKubernetesDeployTime(duration time.Duration, namespace string) {
	m.deployDuration.WithLabelValues(namespace).Observe(duration.Seconds())

	m.metricsCache.mu.Lock()
	m.metricsCache.deployTime += duration
	m.metricsCache.deployCount++
	m.metricsCache.mu.Unlock()
}

// RecordErrorByCategory implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordErrorByCategory(category string, stepName string) {
	m.errorTotal.WithLabelValues(category, stepName).Inc()

	m.metricsCache.mu.Lock()
	m.metricsCache.errorsByCategory[category]++
	m.metricsCache.mu.Unlock()

	// Add to recent errors
	m.errorsMutex.Lock()
	m.recentErrors = append([]workflow.ErrorEvent{{
		Timestamp: time.Now(),
		Category:  category,
		StepName:  stepName,
	}}, m.recentErrors...)
	if len(m.recentErrors) > 100 {
		m.recentErrors = m.recentErrors[:100]
	}
	m.errorsMutex.Unlock()
}

// RecordAdaptationApplied implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordAdaptationApplied(adaptationType workflow.AdaptationType, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.adaptationApplied.WithLabelValues(string(adaptationType), status).Inc()
}

// RecordStepQueueTime implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordStepQueueTime(stepName string, duration time.Duration) {
	// Could be recorded as a separate metric if needed
}

// RecordStepProcessingTime implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordStepProcessingTime(stepName string, duration time.Duration) {
	// Processing time is recorded in RecordStepDuration
}

// RecordConcurrentSteps implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordConcurrentSteps(count int) {
	// Could be recorded as a gauge if needed
}

// RecordDeploymentSuccess implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordDeploymentSuccess(repoURL string, branch string) {
	m.metricsCache.mu.Lock()
	m.metricsCache.deploymentSuccess++
	m.metricsCache.deploymentTotal++
	m.metricsCache.mu.Unlock()
}

// RecordDeploymentFailure implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordDeploymentFailure(repoURL string, branch string, reason string) {
	m.metricsCache.mu.Lock()
	m.metricsCache.deploymentTotal++
	m.metricsCache.failureReasons[reason]++
	m.metricsCache.mu.Unlock()
}

// RecordScanVulnerabilities implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordScanVulnerabilities(critical, high, medium, low int) {
	m.vulnerabilities.WithLabelValues("critical").Set(float64(critical))
	m.vulnerabilities.WithLabelValues("high").Set(float64(high))
	m.vulnerabilities.WithLabelValues("medium").Set(float64(medium))
	m.vulnerabilities.WithLabelValues("low").Set(float64(low))

	m.metricsCache.mu.Lock()
	m.metricsCache.securityMetrics.TotalScans++
	m.metricsCache.securityMetrics.CriticalVulns += int64(critical)
	m.metricsCache.securityMetrics.HighVulns += int64(high)
	m.metricsCache.securityMetrics.MediumVulns += int64(medium)
	m.metricsCache.securityMetrics.LowVulns += int64(low)
	m.metricsCache.mu.Unlock()
}

// RecordCacheHit implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordCacheHit(cacheType string) {
	m.cacheHits.WithLabelValues(cacheType).Inc()

	m.metricsCache.mu.Lock()
	m.metricsCache.cacheHits++
	m.metricsCache.mu.Unlock()
}

// RecordCacheMiss implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordCacheMiss(cacheType string) {
	m.cacheMisses.WithLabelValues(cacheType).Inc()

	m.metricsCache.mu.Lock()
	m.metricsCache.cacheMisses++
	m.metricsCache.mu.Unlock()
}

// RecordLLMTokenUsage implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordLLMTokenUsage(promptTokens, completionTokens int) {
	m.llmTokensUsed.WithLabelValues("prompt").Add(float64(promptTokens))
	m.llmTokensUsed.WithLabelValues("completion").Add(float64(completionTokens))

	m.metricsCache.mu.Lock()
	m.metricsCache.totalTokens += int64(promptTokens + completionTokens)
	m.metricsCache.mu.Unlock()
}

// RecordPatternRecognitionAccuracy implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordPatternRecognitionAccuracy(predicted, actual string, correct bool) {
	result := "incorrect"
	if correct {
		result = "correct"
	}
	m.patternAccuracy.WithLabelValues(result).Inc()

	m.metricsCache.mu.Lock()
	m.metricsCache.patternTotal++
	if correct {
		m.metricsCache.patternCorrect++
	}
	m.metricsCache.mu.Unlock()
}

// RecordStepEnhancementImpact implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) RecordStepEnhancementImpact(stepName string, improvementPercent float64) {
	m.enhancementImpact.Observe(improvementPercent)

	m.metricsCache.mu.Lock()
	m.metricsCache.enhancementTotal += improvementPercent
	m.metricsCache.enhancementCount++
	m.metricsCache.mu.Unlock()
}

// GetMetricsSnapshot implements ExtendedMetricsCollector
func (m *WorkflowMetricsCollector) GetMetricsSnapshot() *workflow.MetricsSnapshot {
	m.metricsCache.mu.RLock()
	defer m.metricsCache.mu.RUnlock()

	snapshot := &workflow.MetricsSnapshot{
		TotalWorkflows:      m.metricsCache.workflowCount,
		SuccessfulWorkflows: m.metricsCache.successCount,
		FailedWorkflows:     m.metricsCache.failureCount,
		StepMetrics:         make(map[string]*workflow.StepMetrics),
		ErrorsByCategory:    make(map[string]int64),
		TotalRetries:        m.metricsCache.totalRetries,
		TopFailureReasons:   make(map[string]int64),
		SecurityMetrics:     m.metricsCache.securityMetrics,
		Timestamp:           time.Now(),
	}

	// Calculate averages
	if m.metricsCache.workflowCount > 0 {
		snapshot.AvgWorkflowDuration = m.metricsCache.totalDuration / time.Duration(m.metricsCache.workflowCount)
	}

	if m.metricsCache.dockerBuildCount > 0 {
		snapshot.AvgDockerBuildTime = m.metricsCache.dockerBuildTime / time.Duration(m.metricsCache.dockerBuildCount)
	}

	if m.metricsCache.imageCount > 0 {
		snapshot.AvgImageSize = m.metricsCache.totalImageSize / m.metricsCache.imageCount
	}

	if m.metricsCache.deployCount > 0 {
		snapshot.AvgDeployTime = m.metricsCache.deployTime / time.Duration(m.metricsCache.deployCount)
	}

	// Calculate rates
	totalCache := m.metricsCache.cacheHits + m.metricsCache.cacheMisses
	if totalCache > 0 {
		snapshot.CacheHitRate = float64(m.metricsCache.cacheHits) / float64(totalCache)
	}

	snapshot.TotalTokensUsed = m.metricsCache.totalTokens

	if m.metricsCache.patternTotal > 0 {
		snapshot.PatternAccuracy = float64(m.metricsCache.patternCorrect) / float64(m.metricsCache.patternTotal)
	}

	if m.metricsCache.enhancementCount > 0 {
		snapshot.AvgEnhancementImpact = m.metricsCache.enhancementTotal / float64(m.metricsCache.enhancementCount)
	}

	if m.metricsCache.deploymentTotal > 0 {
		snapshot.DeploymentSuccessRate = float64(m.metricsCache.deploymentSuccess) / float64(m.metricsCache.deploymentTotal)
	}

	// Copy maps
	for k, v := range m.metricsCache.stepMetrics {
		snapshot.StepMetrics[k] = v
	}

	for k, v := range m.metricsCache.errorsByCategory {
		snapshot.ErrorsByCategory[k] = v
	}

	for k, v := range m.metricsCache.failureReasons {
		snapshot.TopFailureReasons[k] = v
	}

	return snapshot
}

// GetHealthStatus implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) GetHealthStatus() workflow.HealthStatus {
	snapshot := m.GetMetricsSnapshot()

	// Calculate health based on metrics
	status := "healthy"
	workflowHealth := workflow.ComponentHealth{
		Status:       "healthy",
		ErrorRate:    0.0,
		Availability: 1.0,
	}

	if snapshot.TotalWorkflows > 0 {
		errorRate := float64(snapshot.FailedWorkflows) / float64(snapshot.TotalWorkflows)
		workflowHealth.ErrorRate = errorRate

		if errorRate > 0.1 {
			status = "degraded"
			workflowHealth.Status = "degraded"
		}
		if errorRate > 0.3 {
			status = "unhealthy"
			workflowHealth.Status = "unhealthy"
		}
	}

	return workflow.HealthStatus{
		Status:         status,
		WorkflowHealth: workflowHealth,
		LastChecked:    time.Now(),
		Details: map[string]interface{}{
			"active_workflows": m.activeWorkflows.Load(),
			"total_workflows":  snapshot.TotalWorkflows,
			"success_rate":     snapshot.DeploymentSuccessRate,
		},
	}
}

// RecordHealthCheck implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) RecordHealthCheck(status workflow.HealthStatus, details map[string]interface{}) {
	// Could record health check results
}

// GetCurrentWorkflowCount implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) GetCurrentWorkflowCount() int {
	return int(m.activeWorkflows.Load())
}

// GetCurrentStepExecutions implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) GetCurrentStepExecutions() map[string]int {
	result := make(map[string]int)
	m.stepExecutions.Range(func(key, value interface{}) bool {
		if counter, ok := value.(*atomic.Int64); ok {
			result[key.(string)] = int(counter.Load())
		}
		return true
	})
	return result
}

// GetRecentErrors implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) GetRecentErrors(limit int) []workflow.ErrorEvent {
	m.errorsMutex.RLock()
	defer m.errorsMutex.RUnlock()

	if limit > len(m.recentErrors) {
		limit = len(m.recentErrors)
	}

	result := make([]workflow.ErrorEvent, limit)
	copy(result, m.recentErrors[:limit])
	return result
}

// CheckThresholds implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) CheckThresholds() []workflow.Alert {
	var alerts []workflow.Alert
	snapshot := m.GetMetricsSnapshot()

	// Check error rate threshold
	if threshold, ok := m.getThreshold("error_rate"); ok {
		errorRate := 0.0
		if snapshot.TotalWorkflows > 0 {
			errorRate = float64(snapshot.FailedWorkflows) / float64(snapshot.TotalWorkflows)
		}

		if errorRate > threshold {
			alerts = append(alerts, workflow.Alert{
				Metric:    "error_rate",
				Threshold: threshold,
				Current:   errorRate,
				Severity:  "critical",
				Message:   fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", errorRate*100, threshold*100),
				Timestamp: time.Now(),
			})
		}
	}

	// Check average workflow duration threshold
	if threshold, ok := m.getThreshold("avg_duration"); ok {
		avgDuration := snapshot.AvgWorkflowDuration.Seconds()
		if avgDuration > threshold {
			alerts = append(alerts, workflow.Alert{
				Metric:    "avg_duration",
				Threshold: threshold,
				Current:   avgDuration,
				Severity:  "warning",
				Message:   fmt.Sprintf("Average workflow duration %.1fs exceeds threshold %.1fs", avgDuration, threshold),
				Timestamp: time.Now(),
			})
		}
	}

	return alerts
}

// SetThreshold implements WorkflowMetricsCollector
func (m *WorkflowMetricsCollector) SetThreshold(metric string, threshold float64) {
	m.thresholds.Store(metric, threshold)
}

// Helper methods

func (m *WorkflowMetricsCollector) updateStepMetrics(stepName string, duration time.Duration, success bool) {
	m.metricsCache.mu.Lock()
	defer m.metricsCache.mu.Unlock()

	metrics, exists := m.metricsCache.stepMetrics[stepName]
	if !exists {
		metrics = &workflow.StepMetrics{
			MinDuration: duration,
			MaxDuration: duration,
		}
		m.metricsCache.stepMetrics[stepName] = metrics
	}

	metrics.ExecutionCount++
	if success {
		metrics.SuccessCount++
		// Update durations only for successful executions
		if duration > 0 {
			metrics.AvgDuration = (metrics.AvgDuration*time.Duration(metrics.SuccessCount-1) + duration) / time.Duration(metrics.SuccessCount)
			if duration < metrics.MinDuration || metrics.MinDuration == 0 {
				metrics.MinDuration = duration
			}
			if duration > metrics.MaxDuration {
				metrics.MaxDuration = duration
			}
		}
	} else {
		metrics.FailureCount++
	}
	metrics.LastExecuted = time.Now()

	// Update step execution counter
	if counter, ok := m.stepExecutions.Load(stepName); ok {
		counter.(*atomic.Int64).Add(1)
	} else {
		newCounter := &atomic.Int64{}
		newCounter.Store(1)
		m.stepExecutions.Store(stepName, newCounter)
	}
}

func (m *WorkflowMetricsCollector) getThreshold(metric string) (float64, bool) {
	if value, ok := m.thresholds.Load(metric); ok {
		return value.(float64), true
	}
	return 0, false
}
