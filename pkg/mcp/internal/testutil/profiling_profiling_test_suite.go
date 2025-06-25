package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
)

// ProfiledTestSuite provides a test suite with built-in profiling capabilities
type ProfiledTestSuite struct {
	t             *testing.T
	profiler      *observability.ToolProfiler
	mockProfiler  *MockProfiler
	logger        zerolog.Logger
	testStartTime time.Time
	enabled       bool
}

// NewProfiledTestSuite creates a new profiled test suite
func NewProfiledTestSuite(t *testing.T, logger zerolog.Logger) *ProfiledTestSuite {
	// Use a test-specific logger
	testLogger := logger.With().
		Str("test", t.Name()).
		Str("component", "profiled_test_suite").
		Logger()

	return &ProfiledTestSuite{
		t:             t,
		profiler:      observability.NewToolProfiler(testLogger, true),
		mockProfiler:  NewMockProfiler(),
		logger:        testLogger,
		testStartTime: time.Now(),
		enabled:       true,
	}
}

// WithMockProfiler configures the suite to use a mock profiler for controlled testing
func (pts *ProfiledTestSuite) WithMockProfiler() *ProfiledTestSuite {
	pts.enabled = false // Disable real profiler when using mock
	return pts
}

// GetProfiler returns the real profiler instance
func (pts *ProfiledTestSuite) GetProfiler() *observability.ToolProfiler {
	return pts.profiler
}

// GetMockProfiler returns the mock profiler instance
func (pts *ProfiledTestSuite) GetMockProfiler() *MockProfiler {
	return pts.mockProfiler
}

// ProfileExecution profiles a test tool execution
func (pts *ProfiledTestSuite) ProfileExecution(
	toolName, sessionID string,
	execution func(context.Context) (interface{}, error),
) (interface{}, error) {
	if !pts.enabled {
		return pts.mockProfiler.ProfileExecution(toolName, sessionID, execution)
	}

	ctx := context.Background()
	return pts.profiler.ProfileToolExecution(ctx, toolName, sessionID, execution).Result,
		pts.profiler.ProfileToolExecution(ctx, toolName, sessionID, execution).Error
}

// StartBenchmark starts a benchmark for the test
func (pts *ProfiledTestSuite) StartBenchmark(toolName string, config observability.BenchmarkConfig) *observability.BenchmarkSuite {
	if !pts.enabled {
		// Return a mock benchmark suite
		return NewMockBenchmarkSuite(pts.logger, pts.mockProfiler)
	}

	return observability.NewBenchmarkSuite(pts.logger, pts.profiler)
}

// AssertPerformance performs performance assertions on the test execution
func (pts *ProfiledTestSuite) AssertPerformance(expectations PerformanceExpectations) {
	pts.t.Helper()

	if !pts.enabled {
		pts.assertMockPerformance(expectations)
		return
	}

	metrics := pts.profiler.GetMetrics()
	report := metrics.GeneratePerformanceReport()

	// Assert total execution time
	if expectations.MaxTotalExecutionTime != nil {
		totalTime := time.Since(pts.testStartTime)
		if totalTime > *expectations.MaxTotalExecutionTime {
			pts.t.Errorf("Test execution time %v exceeds maximum %v", totalTime, *expectations.MaxTotalExecutionTime)
		}
	}

	// Assert tool-specific expectations
	for toolName, toolExpectations := range expectations.ToolExpectations {
		stats := metrics.GetToolStats(toolName)
		if stats == nil {
			if !toolExpectations.Optional {
				pts.t.Errorf("Expected tool %s to be executed, but no statistics found", toolName)
			}
			continue
		}

		pts.assertToolPerformance(toolName, stats, toolExpectations)
	}

	// Assert overall success rate
	if expectations.MinSuccessRate != nil {
		if report.OverallSuccessRate < *expectations.MinSuccessRate {
			pts.t.Errorf("Overall success rate %f%% is below minimum %f%%",
				report.OverallSuccessRate, *expectations.MinSuccessRate)
		}
	}
}

func (pts *ProfiledTestSuite) assertToolPerformance(toolName string, stats *observability.ToolStats, expectations ToolPerformanceExpectations) {
	pts.t.Helper()

	// Assert execution count
	if expectations.MinExecutions != nil && stats.ExecutionCount < int64(*expectations.MinExecutions) {
		pts.t.Errorf("Tool %s executed %d times, below minimum %d", toolName, stats.ExecutionCount, *expectations.MinExecutions)
	}

	if expectations.MaxExecutions != nil && stats.ExecutionCount > int64(*expectations.MaxExecutions) {
		pts.t.Errorf("Tool %s executed %d times, above maximum %d", toolName, stats.ExecutionCount, *expectations.MaxExecutions)
	}

	// Assert execution time
	if expectations.MaxAvgExecutionTime != nil && stats.AvgExecutionTime > *expectations.MaxAvgExecutionTime {
		pts.t.Errorf("Tool %s average execution time %v exceeds maximum %v",
			toolName, stats.AvgExecutionTime, *expectations.MaxAvgExecutionTime)
	}

	if expectations.MaxExecutionTime != nil && stats.MaxExecutionTime > *expectations.MaxExecutionTime {
		pts.t.Errorf("Tool %s maximum execution time %v exceeds limit %v",
			toolName, stats.MaxExecutionTime, *expectations.MaxExecutionTime)
	}

	// Assert success rate
	if expectations.MinSuccessRate != nil {
		successRate := float64(stats.SuccessCount) / float64(stats.ExecutionCount) * 100
		if successRate < *expectations.MinSuccessRate {
			pts.t.Errorf("Tool %s success rate %f%% is below minimum %f%%",
				toolName, successRate, *expectations.MinSuccessRate)
		}
	}

	// Assert memory usage
	if expectations.MaxMemoryUsage != nil && stats.MaxMemoryUsage > *expectations.MaxMemoryUsage {
		pts.t.Errorf("Tool %s maximum memory usage %d bytes exceeds limit %d bytes",
			toolName, stats.MaxMemoryUsage, *expectations.MaxMemoryUsage)
	}
}

func (pts *ProfiledTestSuite) assertMockPerformance(expectations PerformanceExpectations) {
	pts.t.Helper()

	// Verify mock profiler received expected calls
	for toolName, toolExpectations := range expectations.ToolExpectations {
		executions := pts.mockProfiler.GetExecutionsForTool(toolName)

		if len(executions) == 0 && !toolExpectations.Optional {
			pts.t.Errorf("Expected tool %s to be profiled, but no executions found", toolName)
			continue
		}

		if toolExpectations.MinExecutions != nil && len(executions) < *toolExpectations.MinExecutions {
			pts.t.Errorf("Tool %s profiled %d times, below minimum %d", toolName, len(executions), *toolExpectations.MinExecutions)
		}

		if toolExpectations.MaxExecutions != nil && len(executions) > *toolExpectations.MaxExecutions {
			pts.t.Errorf("Tool %s profiled %d times, above maximum %d", toolName, len(executions), *toolExpectations.MaxExecutions)
		}
	}
}

// PerformanceExpectations defines performance expectations for a test
type PerformanceExpectations struct {
	MaxTotalExecutionTime *time.Duration
	MinSuccessRate        *float64
	ToolExpectations      map[string]ToolPerformanceExpectations
}

// ToolPerformanceExpectations defines performance expectations for a specific tool
type ToolPerformanceExpectations struct {
	MinExecutions       *int
	MaxExecutions       *int
	MaxAvgExecutionTime *time.Duration
	MaxExecutionTime    *time.Duration
	MinSuccessRate      *float64
	MaxMemoryUsage      *uint64
	Optional            bool
}

// MockProfiler provides a controllable mock for profiling testing
type MockProfiler struct {
	mu              sync.RWMutex
	executions      []MockExecution
	benchmarks      []MockBenchmark
	enabled         bool
	shouldFail      bool
	configuredDelay time.Duration
}

// MockExecution represents a mock execution record
type MockExecution struct {
	ToolName    string
	SessionID   string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Success     bool
	Error       error
	Result      interface{}
	MemoryUsage uint64
}

// MockBenchmark represents a mock benchmark record
type MockBenchmark struct {
	ToolName         string
	Iterations       int
	Concurrency      int
	TotalDuration    time.Duration
	AvgLatency       time.Duration
	OperationsPerSec float64
	SuccessRate      float64
}

// NewMockProfiler creates a new mock profiler
func NewMockProfiler() *MockProfiler {
	return &MockProfiler{
		executions: make([]MockExecution, 0),
		benchmarks: make([]MockBenchmark, 0),
		enabled:    true,
	}
}

// ProfileExecution mocks tool execution profiling
func (mp *MockProfiler) ProfileExecution(
	toolName, sessionID string,
	execution func(context.Context) (interface{}, error),
) (interface{}, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if !mp.enabled {
		return execution(context.Background())
	}

	startTime := time.Now()

	// Apply configured delay
	if mp.configuredDelay > 0 {
		time.Sleep(mp.configuredDelay)
	}

	var result interface{}
	var err error

	if mp.shouldFail {
		err = &MockProfilingError{Message: "mock profiling failure"}
	} else {
		result, err = execution(context.Background())
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	mockExecution := MockExecution{
		ToolName:    toolName,
		SessionID:   sessionID,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    duration,
		Success:     err == nil,
		Error:       err,
		Result:      result,
		MemoryUsage: 1024, // Mock memory usage
	}

	mp.executions = append(mp.executions, mockExecution)

	return result, err
}

// RunBenchmark mocks benchmark execution
func (mp *MockProfiler) RunBenchmark(
	toolName string,
	iterations, concurrency int,
	execution func(context.Context) (interface{}, error),
) MockBenchmark {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	startTime := time.Now()

	// Simulate benchmark execution
	successfulOps := iterations
	if mp.shouldFail {
		successfulOps = iterations / 2 // Simulate 50% failure rate
	}

	// Apply configured delay per operation
	if mp.configuredDelay > 0 {
		time.Sleep(time.Duration(iterations) * mp.configuredDelay)
	}

	totalDuration := time.Since(startTime)
	avgLatency := totalDuration / time.Duration(iterations)
	operationsPerSec := float64(iterations) / totalDuration.Seconds()
	successRate := float64(successfulOps) / float64(iterations) * 100

	benchmark := MockBenchmark{
		ToolName:         toolName,
		Iterations:       iterations,
		Concurrency:      concurrency,
		TotalDuration:    totalDuration,
		AvgLatency:       avgLatency,
		OperationsPerSec: operationsPerSec,
		SuccessRate:      successRate,
	}

	mp.benchmarks = append(mp.benchmarks, benchmark)

	return benchmark
}

// GetExecutionsForTool returns mock executions for a specific tool
func (mp *MockProfiler) GetExecutionsForTool(toolName string) []MockExecution {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var toolExecutions []MockExecution
	for _, execution := range mp.executions {
		if execution.ToolName == toolName {
			toolExecutions = append(toolExecutions, execution)
		}
	}
	return toolExecutions
}

// GetBenchmarksForTool returns mock benchmarks for a specific tool
func (mp *MockProfiler) GetBenchmarksForTool(toolName string) []MockBenchmark {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var toolBenchmarks []MockBenchmark
	for _, benchmark := range mp.benchmarks {
		if benchmark.ToolName == toolName {
			toolBenchmarks = append(toolBenchmarks, benchmark)
		}
	}
	return toolBenchmarks
}

// SetShouldFail configures the mock profiler to simulate failures
func (mp *MockProfiler) SetShouldFail(shouldFail bool) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.shouldFail = shouldFail
}

// SetConfiguredDelay configures artificial delay for testing timing
func (mp *MockProfiler) SetConfiguredDelay(delay time.Duration) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.configuredDelay = delay
}

// Clear resets the mock profiler state
func (mp *MockProfiler) Clear() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.executions = make([]MockExecution, 0)
	mp.benchmarks = make([]MockBenchmark, 0)
}

// MockProfilingError represents a mock profiling error
type MockProfilingError struct {
	Message string
}

func (e *MockProfilingError) Error() string {
	return e.Message
}

// Mock benchmark suite for testing
func NewMockBenchmarkSuite(logger zerolog.Logger, mockProfiler *MockProfiler) *observability.BenchmarkSuite {
	// For now, return the real benchmark suite
	// In a full implementation, we'd create a mock benchmark suite
	return observability.NewBenchmarkSuite(logger, observability.NewToolProfiler(logger, false))
}
