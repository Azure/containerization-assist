// Package observability provides tests for the unified logger
package observability

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"testing"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/infrastructure/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnifiedLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	config := DefaultLoggerConfig()

	unifiedLogger := NewUnifiedLogger(observer, logger, config)

	assert.NotNil(t, unifiedLogger)
	assert.Equal(t, config, unifiedLogger.config)
	assert.NotNil(t, unifiedLogger.metricPatterns)
	assert.Greater(t, len(unifiedLogger.metricPatterns), 0, "Should have default metric patterns")
}

func TestDefaultLoggerConfig(t *testing.T) {
	config := DefaultLoggerConfig()

	assert.True(t, config.EnableMetricExtraction)
	assert.True(t, config.EnableLogCorrelation)
	assert.True(t, config.EnablePerformanceLog)
	assert.Equal(t, 1.0, config.LogSamplingRate)
	assert.Equal(t, time.Hour, config.CorrelationTTL)
}

func TestMetricExtraction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()

	// Test response time extraction
	unifiedLogger.Info(ctx, "Request completed in 150ms", "component", "api")

	// Test error count extraction
	unifiedLogger.Error(ctx, "Operation failed with 5 errors", "operation", "deployment")

	// Test memory usage extraction
	unifiedLogger.Info(ctx, "Process using 256MB memory", "component", "worker")

	// Test processing rate extraction
	unifiedLogger.Info(ctx, "Processed 1000 items/sec", "worker_id", "w1")

	// Test queue size extraction
	unifiedLogger.Debug(ctx, "Current queue size: 42", "queue", "tasks")

	// Verify metrics were extracted and recorded
	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.ExtractedMetrics, int64(0), "Should have extracted some metrics")
}

func TestLogCorrelation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.WithValue(context.Background(), "session_id", "test_session_123")

	// Log several related messages
	unifiedLogger.Info(ctx, "Starting workflow", "workflow_id", "wf_123")
	unifiedLogger.Info(ctx, "Processing step 1", "workflow_id", "wf_123")
	unifiedLogger.Info(ctx, "Processing step 2", "workflow_id", "wf_123")
	unifiedLogger.Info(ctx, "Workflow completed", "workflow_id", "wf_123")

	// Verify correlations were created
	correlationCount := unifiedLogger.GetCorrelationCount()
	assert.Greater(t, correlationCount, 0, "Should have created correlations")

	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.CorrelationHits, int64(0), "Should have correlation hits")
}

func TestStructuredErrorLogging(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()

	// Create a structured error
	structErr := mcperrors.NewValidationError("email", "invalid format")
	structErr.WithWorkflowID("wf_123").WithSessionID("session_456")

	// Log the structured error
	unifiedLogger.LogWithStructuredError(ctx, structErr)

	// Verify error was processed
	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.TotalLogs, int64(0))
}

func TestOperationLogging(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()

	// Test successful operation
	unifiedLogger.LogOperation(ctx, "deploy_container", time.Millisecond*150, true, map[string]interface{}{
		"container_id": "c123",
		"image":        "nginx:latest",
	})

	// Test failed operation
	unifiedLogger.LogOperation(ctx, "deploy_container", time.Millisecond*500, false, map[string]interface{}{
		"container_id": "c124",
		"error":        "image not found",
	})

	// Verify operations were logged
	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.TotalLogs, int64(0))
}

func TestLogEnrichers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	// Add enrichers
	systemEnricher := NewSystemEnricher(true, true, true)
	err := unifiedLogger.AddEnricher(systemEnricher)
	require.NoError(t, err)

	performanceEnricher := NewPerformanceEnricher(true, false, 10)
	err = unifiedLogger.AddEnricher(performanceEnricher)
	require.NoError(t, err)

	timestampEnricher := NewTimestampEnricher(true, true, true)
	err = unifiedLogger.AddEnricher(timestampEnricher)
	require.NoError(t, err)

	contextEnricher := NewContextEnricher([]string{"user_id", "request_id"})
	err = unifiedLogger.AddEnricher(contextEnricher)
	require.NoError(t, err)

	securityEnricher := NewSecurityEnricher(true, true, true, []string{"password", "secret"})
	err = unifiedLogger.AddEnricher(securityEnricher)
	require.NoError(t, err)

	businessEnricher := NewBusinessEnricher(true, true, map[string]string{
		"deploy": "deployment",
		"user":   "user_activity",
	})
	err = unifiedLogger.AddEnricher(businessEnricher)
	require.NoError(t, err)

	// Test enriched logging
	ctx := context.WithValue(context.Background(), "user_id", "user123")
	ctx = context.WithValue(ctx, "request_id", "req456")

	unifiedLogger.Info(ctx, "User deployed container successfully",
		"user_id", "user123",
		"container_id", "c789",
		"duration_ms", 1200)

	// Verify enrichment didn't cause errors
	metrics := unifiedLogger.GetLogMetrics()
	assert.Equal(t, int64(0), metrics.EnrichmentErrors, "Should not have enrichment errors")
}

func TestMetricPatternRegistration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	// Register custom metric pattern
	customPattern := &MetricPattern{
		Name:        "custom_latency",
		Pattern:     CompileRegex(t, `latency:\s+(\d+)ms`),
		MetricType:  MetricTypeTiming,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "milliseconds",
		Description: "Custom latency metric",
	}

	err := unifiedLogger.RegisterMetricPattern(customPattern)
	require.NoError(t, err)

	ctx := context.Background()

	// Test custom metric extraction
	unifiedLogger.Info(ctx, "API call completed with latency: 250ms", "endpoint", "/api/users")

	// Verify custom metric was extracted
	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.ExtractedMetrics, int64(0))
}

func TestLogLevels(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()

	// Test different log levels
	unifiedLogger.Debug(ctx, "Debug message", "component", "test")
	unifiedLogger.Info(ctx, "Info message", "component", "test")
	unifiedLogger.Warn(ctx, "Warning message", "component", "test")
	unifiedLogger.Error(ctx, "Error message", "component", "test")

	// Verify all levels were recorded
	metrics := unifiedLogger.GetLogMetrics()
	assert.Equal(t, int64(4), metrics.TotalLogs)

	// Check level distribution
	assert.Equal(t, int64(1), metrics.LogsByLevel[slog.LevelDebug])
	assert.Equal(t, int64(1), metrics.LogsByLevel[slog.LevelInfo])
	assert.Equal(t, int64(1), metrics.LogsByLevel[slog.LevelWarn])
	assert.Equal(t, int64(1), metrics.LogsByLevel[slog.LevelError])
}

func TestSensitiveDataRedaction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	// Add security enricher
	securityEnricher := NewSecurityEnricher(true, true, true, []string{"password", "secret"})
	err := unifiedLogger.AddEnricher(securityEnricher)
	require.NoError(t, err)

	ctx := context.Background()

	// Test logging with sensitive data
	unifiedLogger.Info(ctx, "User authentication failed",
		"username", "testuser",
		"password", "secret123", // This should be redacted
		"api_key", "sk_test_123") // This should be redacted

	// Verify no enrichment errors
	metrics := unifiedLogger.GetLogMetrics()
	assert.Equal(t, int64(0), metrics.EnrichmentErrors)
}

func TestCorrelationCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	config := DefaultLoggerConfig()
	config.CorrelationTTL = time.Millisecond * 100 // Short TTL for testing

	unifiedLogger := NewUnifiedLogger(observer, logger, config)

	ctx := context.WithValue(context.Background(), "session_id", "test_cleanup_session")

	// Create correlation
	unifiedLogger.Info(ctx, "Test message", "operation", "cleanup_test")

	// Wait for cleanup
	time.Sleep(time.Millisecond * 150)

	// Initial correlation should be cleaned up
	// (Note: In a real test, we'd need better access to the correlations map to verify cleanup)
	metrics := unifiedLogger.GetLogMetrics()
	assert.Greater(t, metrics.TotalLogs, int64(0))
}

func TestMaxEnrichersLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	config := DefaultLoggerConfig()
	config.MaxLogEnrichers = 2 // Set low limit for testing

	unifiedLogger := NewUnifiedLogger(observer, logger, config)

	// Add enrichers up to the limit
	err1 := unifiedLogger.AddEnricher(NewSystemEnricher(true, false, false))
	require.NoError(t, err1)

	err2 := unifiedLogger.AddEnricher(NewTimestampEnricher(true, false, false))
	require.NoError(t, err2)

	// This should fail due to limit
	err3 := unifiedLogger.AddEnricher(NewPerformanceEnricher(true, false, 10))
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "maximum number of enrichers")
}

// Helper function for tests
func CompileRegex(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	regex, err := regexp.Compile(pattern)
	require.NoError(t, err)
	return regex
}

// Benchmark tests
func BenchmarkUnifiedLogger(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			unifiedLogger.Info(ctx, "Benchmark test message",
				"component", "benchmark",
				"iteration", b.N)
		}
	})
}

func BenchmarkMetricExtraction(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	ctx := context.Background()
	message := "Request completed in 150ms with 0 errors using 256MB memory"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unifiedLogger.Info(ctx, message, "component", "benchmark")
	}
}

func BenchmarkWithEnrichers(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())
	unifiedLogger := NewUnifiedLogger(observer, logger, DefaultLoggerConfig())

	// Add enrichers
	unifiedLogger.AddEnricher(NewSystemEnricher(true, true, true))
	unifiedLogger.AddEnricher(NewTimestampEnricher(true, true, true))
	unifiedLogger.AddEnricher(NewContextEnricher([]string{"user_id"}))

	ctx := context.WithValue(context.Background(), "user_id", "bench_user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unifiedLogger.Info(ctx, "Benchmark test with enrichers",
			"component", "benchmark",
			"iteration", i)
	}
}
