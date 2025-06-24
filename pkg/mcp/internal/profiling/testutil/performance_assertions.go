package testutil

import (
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/profiling"
)

// PerformanceAssertion provides utilities for performance-specific assertions
type PerformanceAssertion struct {
	t *testing.T
}

// NewPerformanceAssertion creates a new performance assertion helper
func NewPerformanceAssertion(t *testing.T) *PerformanceAssertion {
	return &PerformanceAssertion{t: t}
}

// Profiling Assertions

// AssertExecutionTiming verifies that an execution session has expected timing
func (pa *PerformanceAssertion) AssertExecutionTiming(
	session *profiling.ExecutionSession,
	minDuration, maxDuration time.Duration,
) {
	pa.t.Helper()

	if session == nil {
		pa.t.Error("Expected execution session, got nil")
		return
	}

	if session.TotalTime < minDuration {
		pa.t.Errorf("Execution time %v is less than minimum %v", session.TotalTime, minDuration)
	}

	if session.TotalTime > maxDuration {
		pa.t.Errorf("Execution time %v exceeds maximum %v", session.TotalTime, maxDuration)
	}
}

// AssertDispatchOverhead verifies that dispatch time is within reasonable bounds
func (pa *PerformanceAssertion) AssertDispatchOverhead(
	session *profiling.ExecutionSession,
	maxOverhead time.Duration,
) {
	pa.t.Helper()

	if session == nil {
		pa.t.Error("Expected execution session, got nil")
		return
	}

	if session.DispatchTime > maxOverhead {
		pa.t.Errorf("Dispatch time %v exceeds maximum overhead %v", session.DispatchTime, maxOverhead)
	}

	// Verify dispatch time is a reasonable portion of total time
	if session.TotalTime > 0 {
		overheadPercentage := float64(session.DispatchTime) / float64(session.TotalTime) * 100
		if overheadPercentage > 50 { // More than 50% overhead is suspicious
			pa.t.Errorf("Dispatch overhead is %f%% of total time, which seems excessive", overheadPercentage)
		}
	}
}

// AssertMemoryUsage verifies memory usage patterns
func (pa *PerformanceAssertion) AssertMemoryUsage(
	session *profiling.ExecutionSession,
	maxMemoryDelta uint64,
) {
	pa.t.Helper()

	if session == nil {
		pa.t.Error("Expected execution session, got nil")
		return
	}

	if session.MemoryDelta.HeapAlloc > maxMemoryDelta {
		pa.t.Errorf("Memory usage %d bytes exceeds maximum %d bytes",
			session.MemoryDelta.HeapAlloc, maxMemoryDelta)
	}
}

// AssertNoMemoryLeak verifies that memory delta is reasonable
func (pa *PerformanceAssertion) AssertNoMemoryLeak(
	session *profiling.ExecutionSession,
	maxLeakBytes uint64,
) {
	pa.t.Helper()

	if session == nil {
		pa.t.Error("Expected execution session, got nil")
		return
	}

	// Memory delta should be reasonable for the operation
	if session.MemoryDelta.HeapAlloc > maxLeakBytes {
		pa.t.Errorf("Potential memory leak detected: %d bytes allocated during execution, max allowed: %d",
			session.MemoryDelta.HeapAlloc, maxLeakBytes)
	}
}

// Metrics Assertions

// AssertToolStats verifies tool statistics meet expectations
func (pa *PerformanceAssertion) AssertToolStats(
	stats *profiling.ToolStats,
	expectations ToolStatsExpectations,
) {
	pa.t.Helper()

	if stats == nil {
		pa.t.Error("Expected tool stats, got nil")
		return
	}

	// Execution count
	if expectations.MinExecutions > 0 && stats.ExecutionCount < int64(expectations.MinExecutions) {
		pa.t.Errorf("Tool execution count %d is below minimum %d", stats.ExecutionCount, expectations.MinExecutions)
	}

	if expectations.MaxExecutions > 0 && stats.ExecutionCount > int64(expectations.MaxExecutions) {
		pa.t.Errorf("Tool execution count %d exceeds maximum %d", stats.ExecutionCount, expectations.MaxExecutions)
	}

	// Success rate
	if expectations.MinSuccessRate > 0 {
		actualSuccessRate := float64(stats.SuccessCount) / float64(stats.ExecutionCount) * 100
		if actualSuccessRate < expectations.MinSuccessRate {
			pa.t.Errorf("Success rate %f%% is below minimum %f%%", actualSuccessRate, expectations.MinSuccessRate)
		}
	}

	// Timing
	if expectations.MaxAvgExecutionTime > 0 && stats.AvgExecutionTime > expectations.MaxAvgExecutionTime {
		pa.t.Errorf("Average execution time %v exceeds maximum %v", stats.AvgExecutionTime, expectations.MaxAvgExecutionTime)
	}

	if expectations.MaxExecutionTime > 0 && stats.MaxExecutionTime > expectations.MaxExecutionTime {
		pa.t.Errorf("Maximum execution time %v exceeds limit %v", stats.MaxExecutionTime, expectations.MaxExecutionTime)
	}

	// Memory
	if expectations.MaxMemoryUsage > 0 && stats.MaxMemoryUsage > expectations.MaxMemoryUsage {
		pa.t.Errorf("Maximum memory usage %d bytes exceeds limit %d bytes", stats.MaxMemoryUsage, expectations.MaxMemoryUsage)
	}
}

// AssertPerformanceReport verifies overall performance report
func (pa *PerformanceAssertion) AssertPerformanceReport(
	report *profiling.PerformanceReport,
	expectations ReportExpectations,
) {
	pa.t.Helper()

	if report == nil {
		pa.t.Error("Expected performance report, got nil")
		return
	}

	// Overall success rate
	if expectations.MinSuccessRate > 0 && report.OverallSuccessRate < expectations.MinSuccessRate {
		pa.t.Errorf("Overall success rate %f%% is below minimum %f%%", report.OverallSuccessRate, expectations.MinSuccessRate)
	}

	// Total executions
	if expectations.MinTotalExecutions > 0 && report.TotalExecutions < int64(expectations.MinTotalExecutions) {
		pa.t.Errorf("Total executions %d is below minimum %d", report.TotalExecutions, expectations.MinTotalExecutions)
	}

	// Total execution time
	if expectations.MaxTotalExecutionTime > 0 && report.TotalExecutionTime > expectations.MaxTotalExecutionTime {
		pa.t.Errorf("Total execution time %v exceeds maximum %v", report.TotalExecutionTime, expectations.MaxTotalExecutionTime)
	}

	// Average execution time
	if expectations.MaxAvgExecutionTime > 0 && report.AvgExecutionTime > expectations.MaxAvgExecutionTime {
		pa.t.Errorf("Average execution time %v exceeds maximum %v", report.AvgExecutionTime, expectations.MaxAvgExecutionTime)
	}
}

// Benchmark Assertions

// AssertBenchmarkResult verifies benchmark results meet expectations
func (pa *PerformanceAssertion) AssertBenchmarkResult(
	result *profiling.BenchmarkResult,
	expectations BenchmarkExpectations,
) {
	pa.t.Helper()

	if result == nil {
		pa.t.Error("Expected benchmark result, got nil")
		return
	}

	// Operations
	if expectations.MinOperationsPerSec > 0 && result.OperationsPerSec < expectations.MinOperationsPerSec {
		pa.t.Errorf("Operations per second %f is below minimum %f", result.OperationsPerSec, expectations.MinOperationsPerSec)
	}

	if expectations.MaxOperationsPerSec > 0 && result.OperationsPerSec > expectations.MaxOperationsPerSec {
		pa.t.Errorf("Operations per second %f exceeds maximum %f", result.OperationsPerSec, expectations.MaxOperationsPerSec)
	}

	// Latency
	if expectations.MaxAvgLatency > 0 && result.AvgLatency > expectations.MaxAvgLatency {
		pa.t.Errorf("Average latency %v exceeds maximum %v", result.AvgLatency, expectations.MaxAvgLatency)
	}

	if expectations.MaxLatency > 0 && result.MaxLatency > expectations.MaxLatency {
		pa.t.Errorf("Maximum latency %v exceeds limit %v", result.MaxLatency, expectations.MaxLatency)
	}

	// Error rate
	if expectations.MaxErrorRate > 0 && result.ErrorRate > expectations.MaxErrorRate {
		pa.t.Errorf("Error rate %f%% exceeds maximum %f%%", result.ErrorRate, expectations.MaxErrorRate)
	}

	// Success rate (derived from error rate)
	successRate := 100.0 - result.ErrorRate
	if expectations.MinSuccessRate > 0 && successRate < expectations.MinSuccessRate {
		pa.t.Errorf("Success rate %f%% is below minimum %f%%", successRate, expectations.MinSuccessRate)
	}

	// Total operations
	if expectations.ExpectedOperations > 0 && result.TotalOperations != int64(expectations.ExpectedOperations) {
		pa.t.Errorf("Total operations %d does not match expected %d", result.TotalOperations, expectations.ExpectedOperations)
	}

	// Memory growth
	if expectations.MaxMemoryGrowth > 0 && uint64(result.MemoryGrowth) > expectations.MaxMemoryGrowth {
		pa.t.Errorf("Memory growth %d bytes exceeds maximum %d bytes", result.MemoryGrowth, expectations.MaxMemoryGrowth)
	}
}

// AssertBenchmarkComparison verifies benchmark comparison results
func (pa *PerformanceAssertion) AssertBenchmarkComparison(
	comparison *profiling.BenchmarkComparison,
	expectations ComparisonExpectations,
) {
	pa.t.Helper()

	if comparison == nil {
		pa.t.Error("Expected benchmark comparison, got nil")
		return
	}

	// Check improvement factors
	if expectations.MinLatencyImprovement > 0 {
		if latencyFactor, exists := comparison.ImprovementFactors["latency"]; exists {
			if latencyFactor > expectations.MinLatencyImprovement {
				pa.t.Errorf("Latency improvement factor %f is worse than expected minimum %f",
					latencyFactor, expectations.MinLatencyImprovement)
			}
		}
	}

	if expectations.MinThroughputImprovement > 0 {
		if throughputFactor, exists := comparison.ImprovementFactors["throughput"]; exists {
			if throughputFactor < expectations.MinThroughputImprovement {
				pa.t.Errorf("Throughput improvement factor %f is below minimum %f",
					throughputFactor, expectations.MinThroughputImprovement)
			}
		}
	}

	if expectations.MinMemoryImprovement > 0 {
		if memoryFactor, exists := comparison.ImprovementFactors["memory"]; exists {
			if memoryFactor > expectations.MinMemoryImprovement {
				pa.t.Errorf("Memory improvement factor %f is worse than expected minimum %f",
					memoryFactor, expectations.MinMemoryImprovement)
			}
		}
	}

	// Overall improvement classification
	if expectations.ExpectedImprovement != "" {
		if comparison.Summary != expectations.ExpectedImprovement {
			pa.t.Errorf("Overall improvement summary '%s' does not match expected '%s'",
				comparison.Summary, expectations.ExpectedImprovement)
		}
	}
}

// Performance Regression Detection

// AssertNoPerformanceRegression verifies that performance hasn't degraded
func (pa *PerformanceAssertion) AssertNoPerformanceRegression(
	baseline, current *profiling.BenchmarkResult,
	tolerancePercent float64,
) {
	pa.t.Helper()

	if baseline == nil || current == nil {
		pa.t.Error("Expected both baseline and current benchmark results")
		return
	}

	// Check latency regression (higher is worse)
	latencyChange := (current.AvgLatency.Seconds() - baseline.AvgLatency.Seconds()) / baseline.AvgLatency.Seconds() * 100
	if latencyChange > tolerancePercent {
		pa.t.Errorf("Latency regression detected: %f%% increase (tolerance: %f%%)", latencyChange, tolerancePercent)
	}

	// Check throughput regression (lower is worse)
	throughputChange := (baseline.OperationsPerSec - current.OperationsPerSec) / baseline.OperationsPerSec * 100
	if throughputChange > tolerancePercent {
		pa.t.Errorf("Throughput regression detected: %f%% decrease (tolerance: %f%%)", throughputChange, tolerancePercent)
	}

	// Check error rate regression (higher is worse)
	errorRateChange := current.ErrorRate - baseline.ErrorRate
	if errorRateChange > tolerancePercent {
		pa.t.Errorf("Error rate regression detected: %f%% increase (tolerance: %f%%)", errorRateChange, tolerancePercent)
	}
}

// AssertPerformanceImprovement verifies that performance has improved
func (pa *PerformanceAssertion) AssertPerformanceImprovement(
	baseline, current *profiling.BenchmarkResult,
	minimumImprovementPercent float64,
) {
	pa.t.Helper()

	if baseline == nil || current == nil {
		pa.t.Error("Expected both baseline and current benchmark results")
		return
	}

	// Check latency improvement (lower is better)
	latencyImprovement := (baseline.AvgLatency.Seconds() - current.AvgLatency.Seconds()) / baseline.AvgLatency.Seconds() * 100
	if latencyImprovement < minimumImprovementPercent {
		pa.t.Errorf("Insufficient latency improvement: %f%% (minimum: %f%%)", latencyImprovement, minimumImprovementPercent)
	}

	// Check throughput improvement (higher is better)
	throughputImprovement := (current.OperationsPerSec - baseline.OperationsPerSec) / baseline.OperationsPerSec * 100
	if throughputImprovement < minimumImprovementPercent {
		pa.t.Errorf("Insufficient throughput improvement: %f%% (minimum: %f%%)", throughputImprovement, minimumImprovementPercent)
	}
}

// Expectation Types

// ToolStatsExpectations defines expectations for tool statistics
type ToolStatsExpectations struct {
	MinExecutions       int
	MaxExecutions       int
	MinSuccessRate      float64
	MaxAvgExecutionTime time.Duration
	MaxExecutionTime    time.Duration
	MaxMemoryUsage      uint64
}

// ReportExpectations defines expectations for performance reports
type ReportExpectations struct {
	MinSuccessRate        float64
	MinTotalExecutions    int
	MaxTotalExecutionTime time.Duration
	MaxAvgExecutionTime   time.Duration
}

// BenchmarkExpectations defines expectations for benchmark results
type BenchmarkExpectations struct {
	MinOperationsPerSec float64
	MaxOperationsPerSec float64
	MaxAvgLatency       time.Duration
	MaxLatency          time.Duration
	MaxErrorRate        float64
	MinSuccessRate      float64
	ExpectedOperations  int
	MaxMemoryGrowth     uint64
}

// ComparisonExpectations defines expectations for benchmark comparisons
type ComparisonExpectations struct {
	MinLatencyImprovement    float64 // Lower is better (closer to 0)
	MinThroughputImprovement float64 // Higher is better
	MinMemoryImprovement     float64 // Lower is better (closer to 0)
	ExpectedImprovement      string  // Expected overall classification
}
