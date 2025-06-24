package profiling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolProfiler_BasicProfiling(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)

	toolName := "test_tool"
	sessionID := "test_session"

	// Start execution
	session := profiler.StartExecution(toolName, sessionID)
	require.NotNil(t, session)
	assert.Equal(t, toolName, session.ToolName)
	assert.Equal(t, sessionID, session.SessionID)
	assert.False(t, session.StartTime.IsZero())

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Record dispatch complete
	profiler.RecordDispatchComplete(toolName, sessionID)

	// Simulate more work
	time.Sleep(10 * time.Millisecond)

	// End execution
	finalSession := profiler.EndExecution(toolName, sessionID, true, "")
	require.NotNil(t, finalSession)

	assert.True(t, finalSession.Success)
	assert.Empty(t, finalSession.ErrorType)
	assert.True(t, finalSession.TotalTime > finalSession.DispatchTime)
	assert.True(t, finalSession.ExecutionTime > 0)
	assert.False(t, finalSession.EndTime.IsZero())
}

func TestToolProfiler_DisabledProfiling(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, false)

	toolName := "test_tool"
	sessionID := "test_session"

	// When profiling is disabled, should return nil
	session := profiler.StartExecution(toolName, sessionID)
	assert.Nil(t, session)

	finalSession := profiler.EndExecution(toolName, sessionID, true, "")
	assert.Nil(t, finalSession)
}

func TestToolProfiler_Metadata(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)

	toolName := "test_tool"
	sessionID := "test_session"

	// Start execution and add metadata
	profiler.StartExecution(toolName, sessionID)
	profiler.SetMetadata(toolName, sessionID, "test_key", "test_value")
	profiler.SetStage(toolName, sessionID, "processing")

	// End execution
	finalSession := profiler.EndExecution(toolName, sessionID, true, "")
	require.NotNil(t, finalSession)

	assert.Equal(t, "test_value", finalSession.Metadata["test_key"])
	assert.Equal(t, "processing", finalSession.Stage)
}

func TestToolProfiler_ProfiledExecution(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)

	toolName := "test_tool"
	sessionID := "test_session"

	// Test successful execution
	result := profiler.ProfileToolExecution(
		context.Background(),
		toolName,
		sessionID,
		func(ctx context.Context) (interface{}, error) {
			time.Sleep(5 * time.Millisecond)
			return "success", nil
		},
	)

	require.NotNil(t, result)
	assert.Equal(t, "success", result.Result)
	assert.NoError(t, result.Error)
	assert.NotNil(t, result.Session)
	assert.True(t, result.Session.Success)

	// Test failed execution
	result = profiler.ProfileToolExecution(
		context.Background(),
		toolName,
		sessionID+"_fail",
		func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("test error")
		},
	)

	require.NotNil(t, result)
	assert.Nil(t, result.Result)
	assert.Error(t, result.Error)
	assert.NotNil(t, result.Session)
	assert.False(t, result.Session.Success)
	assert.Equal(t, "execution_error", result.Session.ErrorType)
}

func TestMetricsCollector_RecordExecution(t *testing.T) {
	collector := NewMetricsCollector()

	// Create test execution session
	session := &ExecutionSession{
		ToolName:      "test_tool",
		SessionID:     "test_session",
		StartTime:     time.Now(),
		EndTime:       time.Now().Add(100 * time.Millisecond),
		TotalTime:     100 * time.Millisecond,
		DispatchTime:  10 * time.Millisecond,
		ExecutionTime: 90 * time.Millisecond,
		Success:       true,
		MemoryDelta:   MemoryStats{HeapAlloc: 1024},
	}

	// Record execution
	collector.RecordExecution(session)

	// Check tool stats
	stats := collector.GetToolStats("test_tool")
	require.NotNil(t, stats)
	assert.Equal(t, "test_tool", stats.ToolName)
	assert.Equal(t, int64(1), stats.ExecutionCount)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, int64(0), stats.FailureCount)
	assert.Equal(t, 90*time.Millisecond, stats.TotalExecutionTime)
	assert.Equal(t, 90*time.Millisecond, stats.AvgExecutionTime)
}

func TestMetricsCollector_PerformanceReport(t *testing.T) {
	collector := NewMetricsCollector()

	// Record multiple executions
	sessions := []*ExecutionSession{
		{
			ToolName:      "tool_a",
			Success:       true,
			ExecutionTime: 50 * time.Millisecond,
			MemoryDelta:   MemoryStats{HeapAlloc: 512},
		},
		{
			ToolName:      "tool_a",
			Success:       false,
			ExecutionTime: 75 * time.Millisecond,
			MemoryDelta:   MemoryStats{HeapAlloc: 1024},
		},
		{
			ToolName:      "tool_b",
			Success:       true,
			ExecutionTime: 25 * time.Millisecond,
			MemoryDelta:   MemoryStats{HeapAlloc: 256},
		},
	}

	for _, session := range sessions {
		collector.RecordExecution(session)
	}

	// Generate report
	report := collector.GeneratePerformanceReport()
	require.NotNil(t, report)

	assert.Equal(t, int64(3), report.TotalExecutions)
	assert.Equal(t, int64(2), report.TotalSuccessful)
	assert.Equal(t, int64(1), report.TotalFailed)
	assert.InDelta(t, 66.67, report.OverallSuccessRate, 0.1)

	// Check tool-specific stats
	toolAStats := report.ToolStats["tool_a"]
	require.NotNil(t, toolAStats)
	assert.Equal(t, int64(2), toolAStats.ExecutionCount)
	assert.Equal(t, int64(1), toolAStats.SuccessCount)
	assert.Equal(t, int64(1), toolAStats.FailureCount)

	toolBStats := report.ToolStats["tool_b"]
	require.NotNil(t, toolBStats)
	assert.Equal(t, int64(1), toolBStats.ExecutionCount)
	assert.Equal(t, int64(1), toolBStats.SuccessCount)
	assert.Equal(t, int64(0), toolBStats.FailureCount)
}

func TestBenchmarkSuite_BasicBenchmark(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)
	suite := NewBenchmarkSuite(logger, profiler)

	config := BenchmarkConfig{
		Iterations:    10,
		Concurrency:   1,
		WarmupRounds:  2,
		ToolName:      "test_tool",
		SessionID:     "test_session",
		MonitorMemory: true,
		GCBetweenRuns: false,
	}

	// Test execution function
	executionCount := 0
	toolExecution := func(ctx context.Context) (interface{}, error) {
		executionCount++
		time.Sleep(1 * time.Millisecond) // Simulate work
		return "result", nil
	}

	// Run benchmark
	result := suite.RunBenchmark(config, toolExecution)

	require.NotNil(t, result)
	assert.Equal(t, config, result.Config)
	assert.Equal(t, int64(10), result.TotalOperations)
	assert.Equal(t, int64(10), result.SuccessfulOps)
	assert.Equal(t, int64(0), result.FailedOps)
	assert.True(t, result.AvgLatency > 0)
	assert.True(t, result.MinLatency <= result.AvgLatency)
	assert.True(t, result.AvgLatency <= result.MaxLatency)
	assert.True(t, result.OperationsPerSec > 0)
	assert.Equal(t, 0.0, result.ErrorRate)

	// Verify warmup + actual executions
	assert.Equal(t, 12, executionCount) // 2 warmup + 10 actual
}

func TestBenchmarkSuite_ConcurrentBenchmark(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)
	suite := NewBenchmarkSuite(logger, profiler)

	config := BenchmarkConfig{
		Iterations:    5,
		Concurrency:   3,
		WarmupRounds:  0,
		ToolName:      "test_tool",
		SessionID:     "test_session",
		MonitorMemory: true,
	}

	toolExecution := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "result", nil
	}

	// Run concurrent benchmark
	result := suite.RunConcurrentBenchmark(config, toolExecution)

	require.NotNil(t, result)
	assert.Equal(t, int64(15), result.TotalOperations) // 3 workers * 5 iterations
	assert.Equal(t, int64(15), result.SuccessfulOps)
	assert.Equal(t, int64(0), result.FailedOps)
	assert.True(t, result.ConcurrentAvgLatency > 0)
	assert.True(t, result.ThroughputPerCore > 0)
}

func TestBenchmarkSuite_ErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)
	suite := NewBenchmarkSuite(logger, profiler)

	config := BenchmarkConfig{
		Iterations:  5,
		Concurrency: 1,
		ToolName:    "test_tool",
		SessionID:   "test_session",
	}

	// Tool execution that fails half the time
	executionCount := 0
	toolExecution := func(ctx context.Context) (interface{}, error) {
		executionCount++
		if executionCount%2 == 0 {
			return nil, errors.New("test error")
		}
		return "result", nil
	}

	result := suite.RunBenchmark(config, toolExecution)

	require.NotNil(t, result)
	assert.Equal(t, int64(5), result.TotalOperations)
	assert.Equal(t, int64(3), result.SuccessfulOps) // iterations 1, 3, 5 succeed
	assert.Equal(t, int64(2), result.FailedOps)     // iterations 2, 4 fail
	assert.Equal(t, 40.0, result.ErrorRate)
	assert.Contains(t, result.ErrorTypes, "*errors.errorString")
}

func TestBenchmarkSuite_ComparisonAnalysis(t *testing.T) {
	logger := zerolog.Nop()
	profiler := NewToolProfiler(logger, true)
	suite := NewBenchmarkSuite(logger, profiler)

	// Create baseline result
	baseline := &BenchmarkResult{
		AvgLatency:       100 * time.Millisecond,
		OperationsPerSec: 10.0,
		MemoryGrowth:     1024,
	}

	// Create optimized result (2x better latency, 1.5x better throughput)
	optimized := &BenchmarkResult{
		AvgLatency:       50 * time.Millisecond,
		OperationsPerSec: 15.0,
		MemoryGrowth:     512,
	}

	comparison := suite.CompareBenchmarks(baseline, optimized)

	require.NotNil(t, comparison)
	assert.Equal(t, baseline, comparison.Baseline)
	assert.Equal(t, optimized, comparison.Optimized)

	// Latency improvement: 50ms / 100ms = 0.5 (improvement)
	assert.InDelta(t, 0.5, comparison.LatencyImprovement, 0.01)

	// Throughput improvement: 15.0 / 10.0 = 1.5 (improvement)
	assert.InDelta(t, 1.5, comparison.ThroughputImprovement, 0.01)

	// Memory improvement: 512 / 1024 = 0.5 (improvement)
	assert.InDelta(t, 0.5, comparison.MemoryImprovement, 0.01)

	assert.Contains(t, comparison.SignificantChanges, "Latency improved by 50.0%")
	assert.Contains(t, comparison.SignificantChanges, "Throughput improved by 50.0%")
	assert.Contains(t, comparison.SignificantChanges, "Memory usage improved by 50.0%")
	assert.Equal(t, "Significant Performance Improvement", comparison.OverallImprovement)
}

// Mock orchestrator for testing ProfiledOrchestrator
type mockOrchestrator struct {
	executeFunc    func(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
	validateFunc   func(toolName string, args interface{}) error
	executionDelay time.Duration
	shouldFail     bool
}

func (m *mockOrchestrator) ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, toolName, args, session)
	}

	if m.executionDelay > 0 {
		time.Sleep(m.executionDelay)
	}

	if m.shouldFail {
		return nil, errors.New("mock execution failed")
	}

	return "mock result", nil
}

func (m *mockOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	if m.validateFunc != nil {
		return m.validateFunc(toolName, args)
	}
	return nil
}

func TestProfiledOrchestrator_BasicExecution(t *testing.T) {
	logger := zerolog.Nop()
	mockOrch := &mockOrchestrator{
		executionDelay: 5 * time.Millisecond,
	}

	profiled := NewProfiledOrchestrator(mockOrch, logger)
	assert.True(t, profiled.IsProfilingEnabled())

	// Execute tool
	result, err := profiled.ExecuteTool(
		context.Background(),
		"test_tool",
		map[string]interface{}{"key": "value"},
		nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, "mock result", result)

	// Check metrics were recorded
	metrics := profiled.GetMetrics()
	stats := metrics.GetToolStats("test_tool")
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.ExecutionCount)
	assert.Equal(t, int64(1), stats.SuccessCount)
}

func TestProfiledOrchestrator_BenchmarkExecution(t *testing.T) {
	logger := zerolog.Nop()
	mockOrch := &mockOrchestrator{
		executionDelay: 1 * time.Millisecond,
	}

	profiled := NewProfiledOrchestrator(mockOrch, logger)

	config := BenchmarkConfig{
		Iterations:  3,
		Concurrency: 1,
		ToolName:    "test_tool",
		SessionID:   "test_session",
	}

	result := profiled.ExecuteToolWithBenchmark(
		context.Background(),
		"test_tool",
		map[string]interface{}{"key": "value"},
		nil,
		config,
	)

	require.NotNil(t, result)
	assert.Equal(t, "mock result", result.Result)
	assert.NoError(t, result.Error)
	require.NotNil(t, result.Benchmark)
	assert.Equal(t, int64(3), result.Benchmark.TotalOperations)
	assert.Equal(t, int64(3), result.Benchmark.SuccessfulOps)
}

func TestProfilingMiddleware(t *testing.T) {
	logger := zerolog.Nop()
	middleware := NewProfilingMiddleware(logger)

	executionCount := 0
	result, err := middleware.WrapExecution(
		"test_tool",
		"test_session",
		func() (interface{}, error) {
			executionCount++
			time.Sleep(2 * time.Millisecond)
			return "wrapped result", nil
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "wrapped result", result)
	assert.Equal(t, 1, executionCount)

	// Check profiler recorded the execution
	profiler := middleware.GetProfiler()
	metrics := profiler.GetMetrics()
	stats := metrics.GetToolStats("test_tool")
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.ExecutionCount)
}
