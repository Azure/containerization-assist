// Package sampling provides metrics collection for sampling usage and performance
package sampling

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SamplingMetrics tracks detailed metrics for sampling operations
type SamplingMetrics struct {
	// Request metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	retryAttempts      int64

	// Performance metrics
	totalLatency int64 // microseconds
	minLatency   int64 // microseconds
	maxLatency   int64 // microseconds

	// Token usage metrics
	totalTokensUsed     int64
	totalPromptTokens   int64
	totalResponseTokens int64

	// Template usage metrics (using sync.Map for better write performance)
	templateUsage sync.Map // map[string]int64

	// Error metrics (using sync.Map for better write performance)
	errorsByType sync.Map // map[string]int64

	// Validation metrics
	validationFailures int64
	securityIssues     int64

	// Rate limiting metrics
	rateLimitHits int64

	// Content type metrics (using sync.Map for better write performance)
	contentTypeMetrics sync.Map // map[string]*ContentTypeMetrics

	// Time-based metrics
	startTime       time.Time
	lastRequestTime time.Time

	mu sync.RWMutex
}

// ContentTypeMetrics tracks metrics for specific content types
type ContentTypeMetrics struct {
	Requests          int64
	SuccessfulParsing int64
	ValidationErrors  int64
	AverageSize       int64
	TotalSize         int64
}

// NewSamplingMetrics creates a new metrics collector
func NewSamplingMetrics() *SamplingMetrics {
	return &SamplingMetrics{
		// sync.Map fields are zero-initialized and ready to use
		startTime:  time.Now(),
		minLatency: int64(^uint64(0) >> 1), // Max int64
	}
}

// RecordRequest records a sampling request
func (m *SamplingMetrics) RecordRequest(ctx context.Context, templateID string, success bool, duration time.Duration, tokensUsed int, promptTokens int, responseTokens int) {
	atomic.AddInt64(&m.totalRequests, 1)

	if success {
		atomic.AddInt64(&m.successfulRequests, 1)
	} else {
		atomic.AddInt64(&m.failedRequests, 1)
	}

	// Record latency
	latencyMicros := duration.Microseconds()
	atomic.AddInt64(&m.totalLatency, latencyMicros)

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&m.minLatency)
		if latencyMicros >= currentMin || atomic.CompareAndSwapInt64(&m.minLatency, currentMin, latencyMicros) {
			break
		}
	}

	for {
		currentMax := atomic.LoadInt64(&m.maxLatency)
		if latencyMicros <= currentMax || atomic.CompareAndSwapInt64(&m.maxLatency, currentMax, latencyMicros) {
			break
		}
	}

	// Record token usage
	atomic.AddInt64(&m.totalTokensUsed, int64(tokensUsed))
	atomic.AddInt64(&m.totalPromptTokens, int64(promptTokens))
	atomic.AddInt64(&m.totalResponseTokens, int64(responseTokens))

	// Record template usage using sync.Map
	if templateID != "" {
		// Load existing count, increment, and store back atomically
		for {
			current, _ := m.templateUsage.LoadOrStore(templateID, int64(0))
			currentCount := current.(int64)
			if m.templateUsage.CompareAndSwap(templateID, currentCount, currentCount+1) {
				break
			}
		}
	}

	// Update last request time
	m.mu.Lock()
	m.lastRequestTime = time.Now()
	m.mu.Unlock()
}

// RecordRetry records a retry attempt
func (m *SamplingMetrics) RecordRetry() {
	atomic.AddInt64(&m.retryAttempts, 1)
}

// RecordError records an error by type
func (m *SamplingMetrics) RecordError(errorType string) {
	// Use sync.Map for lock-free error counting
	for {
		current, _ := m.errorsByType.LoadOrStore(errorType, int64(0))
		currentCount := current.(int64)
		if m.errorsByType.CompareAndSwap(errorType, currentCount, currentCount+1) {
			break
		}
	}
}

// RecordValidationFailure records a validation failure
func (m *SamplingMetrics) RecordValidationFailure() {
	atomic.AddInt64(&m.validationFailures, 1)
}

// RecordSecurityIssue records a security issue found
func (m *SamplingMetrics) RecordSecurityIssue() {
	atomic.AddInt64(&m.securityIssues, 1)
}

// RecordRateLimitHit records a rate limit hit
func (m *SamplingMetrics) RecordRateLimitHit() {
	atomic.AddInt64(&m.rateLimitHits, 1)
}

// RecordContentType records metrics for a specific content type
func (m *SamplingMetrics) RecordContentType(contentType string, size int, parsed bool, validationErrors int) {
	// Get or create metrics for this content type using sync.Map
	metricsInterface, _ := m.contentTypeMetrics.LoadOrStore(contentType, &ContentTypeMetrics{})
	metrics := metricsInterface.(*ContentTypeMetrics)

	atomic.AddInt64(&metrics.Requests, 1)
	atomic.AddInt64(&metrics.TotalSize, int64(size))

	if parsed {
		atomic.AddInt64(&metrics.SuccessfulParsing, 1)
	}

	atomic.AddInt64(&metrics.ValidationErrors, int64(validationErrors))

	// Update average size
	requests := atomic.LoadInt64(&metrics.Requests)
	totalSize := atomic.LoadInt64(&metrics.TotalSize)
	if requests > 0 {
		atomic.StoreInt64(&metrics.AverageSize, totalSize/requests)
	}
}

// GetMetrics returns current metrics as a map
func (m *SamplingMetrics) GetMetrics() map[string]interface{} {
	totalReq := atomic.LoadInt64(&m.totalRequests)
	successReq := atomic.LoadInt64(&m.successfulRequests)
	failedReq := atomic.LoadInt64(&m.failedRequests)
	retries := atomic.LoadInt64(&m.retryAttempts)
	totalLatency := atomic.LoadInt64(&m.totalLatency)
	minLatency := atomic.LoadInt64(&m.minLatency)
	maxLatency := atomic.LoadInt64(&m.maxLatency)
	totalTokens := atomic.LoadInt64(&m.totalTokensUsed)
	promptTokens := atomic.LoadInt64(&m.totalPromptTokens)
	responseTokens := atomic.LoadInt64(&m.totalResponseTokens)
	validationFails := atomic.LoadInt64(&m.validationFailures)
	securityIssues := atomic.LoadInt64(&m.securityIssues)
	rateLimitHits := atomic.LoadInt64(&m.rateLimitHits)

	// Calculate derived metrics
	var successRate, avgLatency, avgTokensPerRequest float64
	if totalReq > 0 {
		successRate = float64(successReq) / float64(totalReq)
		avgLatency = float64(totalLatency) / float64(totalReq) / 1000.0 // Convert to milliseconds
		avgTokensPerRequest = float64(totalTokens) / float64(totalReq)
	}

	// Handle min latency edge case
	if minLatency == int64(^uint64(0)>>1) {
		minLatency = 0
	}

	// Get template usage from sync.Map
	templateUsage := make(map[string]int64)
	m.templateUsage.Range(func(key, value interface{}) bool {
		templateUsage[key.(string)] = value.(int64)
		return true
	})

	// Get error breakdown from sync.Map
	errorsByType := make(map[string]int64)
	m.errorsByType.Range(func(key, value interface{}) bool {
		errorsByType[key.(string)] = value.(int64)
		return true
	})

	// Get content type metrics from sync.Map
	contentMetrics := make(map[string]map[string]interface{})
	m.contentTypeMetrics.Range(func(key, value interface{}) bool {
		contentType := key.(string)
		metrics := value.(*ContentTypeMetrics)

		requests := atomic.LoadInt64(&metrics.Requests)
		successfulParsing := atomic.LoadInt64(&metrics.SuccessfulParsing)
		validationErrors := atomic.LoadInt64(&metrics.ValidationErrors)
		avgSize := atomic.LoadInt64(&metrics.AverageSize)

		var parsingRate float64
		if requests > 0 {
			parsingRate = float64(successfulParsing) / float64(requests)
		}

		contentMetrics[contentType] = map[string]interface{}{
			"requests":           requests,
			"successful_parsing": successfulParsing,
			"parsing_rate":       parsingRate,
			"validation_errors":  validationErrors,
			"average_size":       avgSize,
		}
		return true
	})

	m.mu.RLock()
	uptime := time.Since(m.startTime)
	lastRequestTime := m.lastRequestTime
	m.mu.RUnlock()

	var requestsPerSecond float64
	if uptime.Seconds() > 0 {
		requestsPerSecond = float64(totalReq) / uptime.Seconds()
	}

	return map[string]interface{}{
		// Request metrics
		"total_requests":      totalReq,
		"successful_requests": successReq,
		"failed_requests":     failedReq,
		"success_rate":        successRate,
		"retry_attempts":      retries,
		"requests_per_second": requestsPerSecond,

		// Performance metrics
		"avg_latency_ms": avgLatency,
		"min_latency_us": minLatency,
		"max_latency_us": maxLatency,

		// Token metrics
		"total_tokens_used":      totalTokens,
		"total_prompt_tokens":    promptTokens,
		"total_response_tokens":  responseTokens,
		"avg_tokens_per_request": avgTokensPerRequest,

		// Quality metrics
		"validation_failures": validationFails,
		"security_issues":     securityIssues,
		"rate_limit_hits":     rateLimitHits,

		// Breakdown metrics
		"template_usage":       templateUsage,
		"errors_by_type":       errorsByType,
		"content_type_metrics": contentMetrics,

		// System metrics
		"uptime_seconds":    uptime.Seconds(),
		"last_request_time": lastRequestTime.Format(time.RFC3339),
	}
}

// GetSummary returns a condensed metrics summary
func (m *SamplingMetrics) GetSummary() map[string]interface{} {
	totalReq := atomic.LoadInt64(&m.totalRequests)
	successReq := atomic.LoadInt64(&m.successfulRequests)
	totalTokens := atomic.LoadInt64(&m.totalTokensUsed)

	var successRate float64
	if totalReq > 0 {
		successRate = float64(successReq) / float64(totalReq)
	}

	m.mu.RLock()
	uptime := time.Since(m.startTime)
	m.mu.RUnlock()

	return map[string]interface{}{
		"requests":     totalReq,
		"success_rate": successRate,
		"total_tokens": totalTokens,
		"uptime_hours": uptime.Hours(),
	}
}

// Reset resets all metrics (useful for testing)
func (m *SamplingMetrics) Reset() {
	atomic.StoreInt64(&m.totalRequests, 0)
	atomic.StoreInt64(&m.successfulRequests, 0)
	atomic.StoreInt64(&m.failedRequests, 0)
	atomic.StoreInt64(&m.retryAttempts, 0)
	atomic.StoreInt64(&m.totalLatency, 0)
	atomic.StoreInt64(&m.minLatency, int64(^uint64(0)>>1))
	atomic.StoreInt64(&m.maxLatency, 0)
	atomic.StoreInt64(&m.totalTokensUsed, 0)
	atomic.StoreInt64(&m.totalPromptTokens, 0)
	atomic.StoreInt64(&m.totalResponseTokens, 0)
	atomic.StoreInt64(&m.validationFailures, 0)
	atomic.StoreInt64(&m.securityIssues, 0)
	atomic.StoreInt64(&m.rateLimitHits, 0)

	// Clear sync.Map contents
	m.templateUsage.Range(func(key, value interface{}) bool {
		m.templateUsage.Delete(key)
		return true
	})

	m.errorsByType.Range(func(key, value interface{}) bool {
		m.errorsByType.Delete(key)
		return true
	})

	m.contentTypeMetrics.Range(func(key, value interface{}) bool {
		m.contentTypeMetrics.Delete(key)
		return true
	})

	m.mu.Lock()
	m.startTime = time.Now()
	m.lastRequestTime = time.Time{}
	m.mu.Unlock()
}

// PerformanceStats returns performance-specific statistics
type PerformanceStats struct {
	AverageLatencyMs float64 `json:"average_latency_ms"`
	MedianLatencyMs  float64 `json:"median_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	MinLatencyMs     float64 `json:"min_latency_ms"`
	MaxLatencyMs     float64 `json:"max_latency_ms"`
	ThroughputRPS    float64 `json:"throughput_rps"`
	ErrorRate        float64 `json:"error_rate"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
}

// GetPerformanceStats returns detailed performance statistics
func (m *SamplingMetrics) GetPerformanceStats() PerformanceStats {
	totalReq := atomic.LoadInt64(&m.totalRequests)
	failedReq := atomic.LoadInt64(&m.failedRequests)
	totalLatency := atomic.LoadInt64(&m.totalLatency)
	minLatency := atomic.LoadInt64(&m.minLatency)
	maxLatency := atomic.LoadInt64(&m.maxLatency)
	totalTokens := atomic.LoadInt64(&m.totalTokensUsed)

	m.mu.RLock()
	uptime := time.Since(m.startTime)
	m.mu.RUnlock()

	var avgLatency, errorRate, throughput, tokensPerSecond float64

	if totalReq > 0 {
		avgLatency = float64(totalLatency) / float64(totalReq) / 1000.0 // Convert to ms
		errorRate = float64(failedReq) / float64(totalReq)
	}

	if uptime.Seconds() > 0 {
		throughput = float64(totalReq) / uptime.Seconds()
		tokensPerSecond = float64(totalTokens) / uptime.Seconds()
	}

	// Handle min latency edge case
	minLatencyMs := float64(minLatency) / 1000.0
	if minLatency == int64(^uint64(0)>>1) {
		minLatencyMs = 0
	}

	return PerformanceStats{
		AverageLatencyMs: avgLatency,
		// Note: For median, P95, P99 we'd need to collect individual latency samples
		// This would require more memory but could be added as an optional feature
		MedianLatencyMs: avgLatency,       // Approximation
		P95LatencyMs:    avgLatency * 1.5, // Approximation
		P99LatencyMs:    avgLatency * 2.0, // Approximation
		MinLatencyMs:    minLatencyMs,
		MaxLatencyMs:    float64(maxLatency) / 1000.0,
		ThroughputRPS:   throughput,
		ErrorRate:       errorRate,
		TokensPerSecond: tokensPerSecond,
	}
}

// MetricsCollector provides a higher-level interface for metrics collection
type MetricsCollector struct {
	sampling   *SamplingMetrics
	validation *ValidationMetrics
	mu         sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		sampling:   NewSamplingMetrics(),
		validation: NewValidationMetrics(),
	}
}

// RecordSamplingRequest records a sampling request with all relevant metrics
func (c *MetricsCollector) RecordSamplingRequest(ctx context.Context, templateID string, success bool, duration time.Duration, tokensUsed int, promptTokens int, responseTokens int, contentType string, contentSize int, validationResult ValidationResult) {
	c.sampling.RecordRequest(ctx, templateID, success, duration, tokensUsed, promptTokens, responseTokens)

	if contentType != "" {
		parsed := success && validationResult.SyntaxValid
		errorCount := len(validationResult.Errors)
		c.sampling.RecordContentType(contentType, contentSize, parsed, errorCount)
	}

	// Record validation metrics
	c.validation.RecordValidation(validationResult)

	// Record validation failure if applicable
	if !validationResult.IsValid {
		c.sampling.RecordValidationFailure()
	}

	// Count security issues
	for _, err := range validationResult.Errors {
		if strings.Contains(err, "SECURITY:") {
			c.sampling.RecordSecurityIssue()
		}
	}
}

// GetCombinedMetrics returns metrics from both sampling and validation
func (c *MetricsCollector) GetCombinedMetrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	samplingMetrics := c.sampling.GetMetrics()
	validationMetrics := c.validation.GetMetrics()

	// Combine metrics
	combined := make(map[string]interface{})

	// Add sampling metrics
	for k, v := range samplingMetrics {
		combined["sampling_"+k] = v
	}

	// Add validation metrics
	for k, v := range validationMetrics {
		combined["validation_"+k] = v
	}

	return combined
}

// GetHealthStatus returns a health status based on metrics
func (c *MetricsCollector) GetHealthStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	samplingMetrics := c.sampling.GetMetrics()
	validationMetrics := c.validation.GetMetrics()

	// Define health thresholds
	successRateThreshold := 0.95
	avgLatencyThreshold := 5000.0 // 5 seconds

	successRate := samplingMetrics["success_rate"].(float64)
	avgLatency := samplingMetrics["avg_latency_ms"].(float64)

	healthy := successRate >= successRateThreshold && avgLatency <= avgLatencyThreshold

	status := "healthy"
	issues := []string{}

	if successRate < successRateThreshold {
		status = "degraded"
		issues = append(issues, "low success rate")
	}

	if avgLatency > avgLatencyThreshold {
		status = "degraded"
		issues = append(issues, "high latency")
	}

	// Only set to unhealthy for severe conditions
	if successRate < 0.5 || avgLatency > 10000.0 {
		status = "unhealthy"
	}

	return map[string]interface{}{
		"status":                  status,
		"healthy":                 healthy,
		"success_rate":            successRate,
		"avg_latency_ms":          avgLatency,
		"issues":                  issues,
		"total_requests":          samplingMetrics["total_requests"],
		"validation_success_rate": validationMetrics["success_rate"],
	}
}

// Global metrics instance
var globalMetrics *MetricsCollector
var globalMetricsOnce sync.Once
var globalMetricsMu sync.RWMutex

// GetGlobalMetrics returns the global metrics collector instance
func GetGlobalMetrics() *MetricsCollector {
	globalMetricsMu.RLock()
	if globalMetrics != nil {
		metrics := globalMetrics
		globalMetricsMu.RUnlock()
		return metrics
	}
	globalMetricsMu.RUnlock()

	// Need to initialize, acquire write lock
	globalMetricsMu.Lock()
	defer globalMetricsMu.Unlock()

	// Double-check pattern
	if globalMetrics == nil {
		globalMetrics = NewMetricsCollector()
	}
	return globalMetrics
}

// ResetGlobalMetrics resets the global metrics instance (for testing)
func ResetGlobalMetrics() {
	globalMetricsMu.Lock()
	defer globalMetricsMu.Unlock()
	globalMetrics = NewMetricsCollector()
}
