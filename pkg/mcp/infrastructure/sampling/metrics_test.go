package sampling

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSamplingMetrics_RecordRequest(t *testing.T) {
	metrics := NewSamplingMetrics()

	ctx := context.Background()
	templateID := "test-template"
	duration := 150 * time.Millisecond

	// Record a successful request
	metrics.RecordRequest(ctx, templateID, true, duration, 100, 50, 50)

	// Record a failed request
	metrics.RecordRequest(ctx, templateID, false, 200*time.Millisecond, 75, 40, 35)

	metricsData := metrics.GetMetrics()

	assert.Equal(t, int64(2), metricsData["total_requests"])
	assert.Equal(t, int64(1), metricsData["successful_requests"])
	assert.Equal(t, int64(1), metricsData["failed_requests"])
	assert.Equal(t, 0.5, metricsData["success_rate"])
	assert.Equal(t, int64(175), metricsData["total_tokens_used"])
	assert.Equal(t, int64(90), metricsData["total_prompt_tokens"])
	assert.Equal(t, int64(85), metricsData["total_response_tokens"])
	assert.Equal(t, 87.5, metricsData["avg_tokens_per_request"])

	// Check template usage
	templateUsage := metricsData["template_usage"].(map[string]int64)
	assert.Equal(t, int64(2), templateUsage[templateID])

	// Check latency metrics
	assert.Greater(t, metricsData["avg_latency_ms"].(float64), 0.0)
	assert.Greater(t, metricsData["min_latency_us"].(int64), int64(0))
	assert.Greater(t, metricsData["max_latency_us"].(int64), int64(0))
}

func TestSamplingMetrics_RecordRetry(t *testing.T) {
	metrics := NewSamplingMetrics()

	metrics.RecordRetry()
	metrics.RecordRetry()

	metricsData := metrics.GetMetrics()
	assert.Equal(t, int64(2), metricsData["retry_attempts"])
}

func TestSamplingMetrics_RecordError(t *testing.T) {
	metrics := NewSamplingMetrics()

	metrics.RecordError("timeout")
	metrics.RecordError("rate_limit")
	metrics.RecordError("timeout")

	metricsData := metrics.GetMetrics()
	errorsByType := metricsData["errors_by_type"].(map[string]int64)

	assert.Equal(t, int64(2), errorsByType["timeout"])
	assert.Equal(t, int64(1), errorsByType["rate_limit"])
}

func TestSamplingMetrics_RecordValidationFailure(t *testing.T) {
	metrics := NewSamplingMetrics()

	metrics.RecordValidationFailure()
	metrics.RecordValidationFailure()

	metricsData := metrics.GetMetrics()
	assert.Equal(t, int64(2), metricsData["validation_failures"])
}

func TestSamplingMetrics_RecordSecurityIssue(t *testing.T) {
	metrics := NewSamplingMetrics()

	metrics.RecordSecurityIssue()
	metrics.RecordSecurityIssue()
	metrics.RecordSecurityIssue()

	metricsData := metrics.GetMetrics()
	assert.Equal(t, int64(3), metricsData["security_issues"])
}

func TestSamplingMetrics_RecordRateLimitHit(t *testing.T) {
	metrics := NewSamplingMetrics()

	metrics.RecordRateLimitHit()

	metricsData := metrics.GetMetrics()
	assert.Equal(t, int64(1), metricsData["rate_limit_hits"])
}

func TestSamplingMetrics_RecordContentType(t *testing.T) {
	metrics := NewSamplingMetrics()

	// Record some Dockerfile metrics
	metrics.RecordContentType("dockerfile", 500, true, 0)
	metrics.RecordContentType("dockerfile", 600, true, 1)
	metrics.RecordContentType("dockerfile", 400, false, 2)

	// Record some manifest metrics
	metrics.RecordContentType("manifest", 1000, true, 0)
	metrics.RecordContentType("manifest", 1200, true, 0)

	metricsData := metrics.GetMetrics()
	contentMetrics := metricsData["content_type_metrics"].(map[string]map[string]interface{})

	// Check Dockerfile metrics
	dockerfileMetrics := contentMetrics["dockerfile"]
	assert.Equal(t, int64(3), dockerfileMetrics["requests"])
	assert.Equal(t, int64(2), dockerfileMetrics["successful_parsing"])
	assert.InDelta(t, 0.667, dockerfileMetrics["parsing_rate"], 0.01)
	assert.Equal(t, int64(3), dockerfileMetrics["validation_errors"])
	assert.Equal(t, int64(500), dockerfileMetrics["average_size"])

	// Check manifest metrics
	manifestMetrics := contentMetrics["manifest"]
	assert.Equal(t, int64(2), manifestMetrics["requests"])
	assert.Equal(t, int64(2), manifestMetrics["successful_parsing"])
	assert.Equal(t, 1.0, manifestMetrics["parsing_rate"])
	assert.Equal(t, int64(0), manifestMetrics["validation_errors"])
	assert.Equal(t, int64(1100), manifestMetrics["average_size"])
}

func TestSamplingMetrics_GetSummary(t *testing.T) {
	metrics := NewSamplingMetrics()

	// Record some requests
	ctx := context.Background()
	metrics.RecordRequest(ctx, "template1", true, 100*time.Millisecond, 50, 25, 25)
	metrics.RecordRequest(ctx, "template2", true, 150*time.Millisecond, 75, 35, 40)
	metrics.RecordRequest(ctx, "template1", false, 200*time.Millisecond, 60, 30, 30)

	summary := metrics.GetSummary()

	assert.Equal(t, int64(3), summary["requests"])
	assert.InDelta(t, 0.667, summary["success_rate"], 0.01)
	assert.Equal(t, int64(185), summary["total_tokens"])
	assert.Greater(t, summary["uptime_hours"].(float64), 0.0)
}

func TestSamplingMetrics_GetPerformanceStats(t *testing.T) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	// Record requests with known latencies
	metrics.RecordRequest(ctx, "template1", true, 100*time.Millisecond, 50, 25, 25)
	metrics.RecordRequest(ctx, "template1", true, 200*time.Millisecond, 60, 30, 30)
	metrics.RecordRequest(ctx, "template1", false, 300*time.Millisecond, 70, 35, 35)

	stats := metrics.GetPerformanceStats()

	assert.InDelta(t, 200.0, stats.AverageLatencyMs, 1.0) // Average of 100, 200, 300 ms
	assert.Equal(t, 100.0, stats.MinLatencyMs)
	assert.Equal(t, 300.0, stats.MaxLatencyMs)
	assert.InDelta(t, 0.333, stats.ErrorRate, 0.01) // 1 failed out of 3
	assert.Greater(t, stats.ThroughputRPS, 0.0)
	assert.Greater(t, stats.TokensPerSecond, 0.0)
}

func TestSamplingMetrics_Reset(t *testing.T) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	// Record some data
	metrics.RecordRequest(ctx, "template1", true, 100*time.Millisecond, 50, 25, 25)
	metrics.RecordError("timeout")
	metrics.RecordValidationFailure()
	metrics.RecordSecurityIssue()
	metrics.RecordContentType("dockerfile", 500, true, 0)

	// Verify data exists
	metricsData := metrics.GetMetrics()
	assert.Greater(t, metricsData["total_requests"].(int64), int64(0))

	// Reset and verify everything is cleared
	metrics.Reset()

	metricsData = metrics.GetMetrics()
	assert.Equal(t, int64(0), metricsData["total_requests"])
	assert.Equal(t, int64(0), metricsData["successful_requests"])
	assert.Equal(t, int64(0), metricsData["failed_requests"])
	assert.Equal(t, int64(0), metricsData["total_tokens_used"])
	assert.Equal(t, int64(0), metricsData["validation_failures"])
	assert.Equal(t, int64(0), metricsData["security_issues"])

	templateUsage := metricsData["template_usage"].(map[string]int64)
	assert.Empty(t, templateUsage)

	errorsByType := metricsData["errors_by_type"].(map[string]int64)
	assert.Empty(t, errorsByType)

	contentMetrics := metricsData["content_type_metrics"].(map[string]map[string]interface{})
	assert.Empty(t, contentMetrics)
}

func TestMetricsCollector_RecordSamplingRequest(t *testing.T) {
	collector := NewMetricsCollector()
	ctx := context.Background()

	// Create a validation result with errors and warnings
	validationResult := ValidationResult{
		IsValid:       false,
		SyntaxValid:   true,
		BestPractices: false,
		Errors:        []string{"SECURITY: privileged container", "missing required field"},
		Warnings:      []string{"best practice warning"},
	}

	collector.RecordSamplingRequest(
		ctx,
		"dockerfile-fix",
		true,
		150*time.Millisecond,
		100,
		50,
		50,
		"dockerfile",
		500,
		validationResult,
	)

	combinedMetrics := collector.GetCombinedMetrics()

	// Check sampling metrics
	assert.Equal(t, int64(1), combinedMetrics["sampling_total_requests"])
	assert.Equal(t, int64(1), combinedMetrics["sampling_successful_requests"])
	assert.Equal(t, int64(1), combinedMetrics["sampling_validation_failures"])
	assert.Equal(t, int64(1), combinedMetrics["sampling_security_issues"])

	// Check validation metrics
	assert.Equal(t, int64(1), combinedMetrics["validation_total_validations"])
	assert.Equal(t, int64(0), combinedMetrics["validation_successful_validations"])
	assert.Equal(t, int64(1), combinedMetrics["validation_failed_validations"])
	assert.Equal(t, int64(1), combinedMetrics["validation_security_issues_found"])
	assert.Equal(t, int64(1), combinedMetrics["validation_best_practice_warnings"])
}

func TestMetricsCollector_GetHealthStatus(t *testing.T) {
	collector := NewMetricsCollector()
	ctx := context.Background()

	// Record mostly successful requests with good latency
	for i := 0; i < 10; i++ {
		validationResult := ValidationResult{
			IsValid:       true,
			SyntaxValid:   true,
			BestPractices: true,
		}

		collector.RecordSamplingRequest(
			ctx,
			"test-template",
			true,
			100*time.Millisecond,
			50,
			25,
			25,
			"dockerfile",
			500,
			validationResult,
		)
	}

	health := collector.GetHealthStatus()

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, true, health["healthy"])
	assert.Equal(t, 1.0, health["success_rate"])
	assert.Greater(t, health["avg_latency_ms"].(float64), 0.0)
	assert.Equal(t, int64(10), health["total_requests"])
	assert.Empty(t, health["issues"].([]string))
}

func TestMetricsCollector_GetHealthStatus_Degraded(t *testing.T) {
	collector := NewMetricsCollector()
	ctx := context.Background()

	// Record some failures to bring success rate below threshold
	for i := 0; i < 7; i++ {
		validationResult := ValidationResult{
			IsValid:       true,
			SyntaxValid:   true,
			BestPractices: true,
		}

		collector.RecordSamplingRequest(
			ctx,
			"test-template",
			true,
			100*time.Millisecond,
			50,
			25,
			25,
			"dockerfile",
			500,
			validationResult,
		)
	}

	// Add failures
	for i := 0; i < 3; i++ {
		validationResult := ValidationResult{
			IsValid:       false,
			SyntaxValid:   false,
			BestPractices: false,
		}

		collector.RecordSamplingRequest(
			ctx,
			"test-template",
			false,
			100*time.Millisecond,
			50,
			25,
			25,
			"dockerfile",
			500,
			validationResult,
		)
	}

	health := collector.GetHealthStatus()

	assert.Equal(t, "degraded", health["status"])
	assert.Equal(t, false, health["healthy"])
	assert.Equal(t, 0.7, health["success_rate"]) // 7/10 = 0.7, below 0.95 threshold
	assert.Contains(t, health["issues"].([]string), "low success rate")
}

func TestMetricsCollector_GetHealthStatus_HighLatency(t *testing.T) {
	collector := NewMetricsCollector()
	ctx := context.Background()

	// Record requests with high latency
	for i := 0; i < 5; i++ {
		validationResult := ValidationResult{
			IsValid:       true,
			SyntaxValid:   true,
			BestPractices: true,
		}

		collector.RecordSamplingRequest(
			ctx,
			"test-template",
			true,
			6*time.Second, // High latency
			50,
			25,
			25,
			"dockerfile",
			500,
			validationResult,
		)
	}

	health := collector.GetHealthStatus()

	assert.Equal(t, "degraded", health["status"])
	assert.Equal(t, false, health["healthy"])
	assert.Greater(t, health["avg_latency_ms"].(float64), 5000.0)
	assert.Contains(t, health["issues"].([]string), "high latency")
}

func TestGlobalMetrics(t *testing.T) {
	// Reset global metrics to ensure clean state
	ResetGlobalMetrics()

	// Get global metrics instance
	metrics1 := GetGlobalMetrics()
	metrics2 := GetGlobalMetrics()

	// Should return the same instance
	assert.Same(t, metrics1, metrics2)

	// Test that it's functional
	ctx := context.Background()
	validationResult := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
	}

	metrics1.RecordSamplingRequest(
		ctx,
		"test-template",
		true,
		100*time.Millisecond,
		50,
		25,
		25,
		"dockerfile",
		500,
		validationResult,
	)

	combinedMetrics := metrics2.GetCombinedMetrics()
	assert.Equal(t, int64(1), combinedMetrics["sampling_total_requests"])
}

func TestSamplingMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	// Test concurrent access
	done := make(chan bool)

	// Start multiple goroutines recording metrics
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				metrics.RecordRequest(ctx, "template", true, 100*time.Millisecond, 50, 25, 25)
				metrics.RecordError("timeout")
				metrics.RecordValidationFailure()
				metrics.RecordSecurityIssue()
				metrics.RecordContentType("dockerfile", 500, true, 0)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics are consistent
	metricsData := metrics.GetMetrics()
	assert.Equal(t, int64(1000), metricsData["total_requests"])
	assert.Equal(t, int64(1000), metricsData["successful_requests"])
	assert.Equal(t, int64(1000), metricsData["validation_failures"])
	assert.Equal(t, int64(1000), metricsData["security_issues"])

	errorsByType := metricsData["errors_by_type"].(map[string]int64)
	assert.Equal(t, int64(1000), errorsByType["timeout"])
}

func TestSamplingMetrics_LatencyCalculation(t *testing.T) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	// Record requests with specific latencies to test min/max tracking
	latencies := []time.Duration{
		50 * time.Millisecond, // min
		150 * time.Millisecond,
		250 * time.Millisecond, // max
		100 * time.Millisecond,
		200 * time.Millisecond,
	}

	for _, latency := range latencies {
		metrics.RecordRequest(ctx, "template", true, latency, 50, 25, 25)
	}

	metricsData := metrics.GetMetrics()

	// Check average latency (should be 150ms = (50+150+250+100+200)/5)
	expectedAvgMs := 150.0
	assert.InDelta(t, expectedAvgMs, metricsData["avg_latency_ms"], 1.0)

	// Check min latency (50ms = 50000 microseconds)
	assert.Equal(t, int64(50000), metricsData["min_latency_us"])

	// Check max latency (250ms = 250000 microseconds)
	assert.Equal(t, int64(250000), metricsData["max_latency_us"])
}

func TestContentTypeMetrics_AverageSize(t *testing.T) {
	metrics := NewSamplingMetrics()

	// Record content with different sizes
	sizes := []int{100, 200, 300, 400, 500}

	for _, size := range sizes {
		metrics.RecordContentType("dockerfile", size, true, 0)
	}

	metricsData := metrics.GetMetrics()
	contentMetrics := metricsData["content_type_metrics"].(map[string]map[string]interface{})
	dockerfileMetrics := contentMetrics["dockerfile"]

	// Average should be (100+200+300+400+500)/5 = 300
	assert.Equal(t, int64(300), dockerfileMetrics["average_size"])
	assert.Equal(t, int64(5), dockerfileMetrics["requests"])
}

func BenchmarkSamplingMetrics_RecordRequest(b *testing.B) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordRequest(ctx, "template", true, 100*time.Millisecond, 50, 25, 25)
	}
}

func BenchmarkSamplingMetrics_GetMetrics(b *testing.B) {
	metrics := NewSamplingMetrics()
	ctx := context.Background()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		metrics.RecordRequest(ctx, "template", true, 100*time.Millisecond, 50, 25, 25)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GetMetrics()
	}
}

func BenchmarkMetricsCollector_RecordSamplingRequest(b *testing.B) {
	collector := NewMetricsCollector()
	ctx := context.Background()
	validationResult := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordSamplingRequest(
			ctx,
			"template",
			true,
			100*time.Millisecond,
			50,
			25,
			25,
			"dockerfile",
			500,
			validationResult,
		)
	}
}
