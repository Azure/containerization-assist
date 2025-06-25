package observability

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ProfiledOrchestrator wraps an orchestrator with profiling capabilities
type ProfiledOrchestrator struct {
	orchestrator interface{}
	profiler     *ToolProfiler
	logger       zerolog.Logger
}

// NOTE: Using unified interface{} interface instead of local ToolOrchestrator

// ProfiledExecutionResult wraps the execution result with profiling data
type ProfiledExecutionResult struct {
	Result    interface{}
	Error     error
	Session   *ExecutionSession
	Benchmark *BenchmarkResult
}

// NewProfiledOrchestrator creates a new profiled orchestrator wrapper
func NewProfiledOrchestrator(orchestrator interface{}, logger zerolog.Logger) *ProfiledOrchestrator {
	// Check if profiling is enabled via environment variable
	enabled := true
	if envVal := os.Getenv("MCP_PROFILING_ENABLED"); envVal != "" {
		if val, err := strconv.ParseBool(envVal); err == nil {
			enabled = val
		}
	}

	profiler := NewToolProfiler(logger, enabled)

	return &ProfiledOrchestrator{
		orchestrator: orchestrator,
		profiler:     profiler,
		logger:       logger.With().Str("component", "profiled_orchestrator").Logger(),
	}
}

// ExecuteTool executes a tool with comprehensive profiling
func (po *ProfiledOrchestrator) ExecuteTool(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
) (interface{}, error) {
	// Extract session ID for profiling
	sessionID := po.extractSessionID(session)

	// Start profiling
	execSession := po.profiler.StartExecution(toolName, sessionID)

	// Add context metadata
	if execSession != nil {
		po.profiler.SetMetadata(toolName, sessionID, "args_type", getTypeName(args))
		po.profiler.SetStage(toolName, sessionID, "validation")
	}

	// Note: Validation is now handled internally by the orchestrator and individual tools

	// Record dispatch complete
	po.profiler.RecordDispatchComplete(toolName, sessionID)

	// Set execution stage
	if execSession != nil {
		po.profiler.SetStage(toolName, sessionID, "execution")
	}

	// Execute the tool (validation happens internally)
	result, err := po.orchestrator.ExecuteTool(ctx, toolName, args)

	// Record execution completion
	success := err == nil
	errorType := ""
	if err != nil {
		errorType = getTypeName(err)
	}

	finalSession := po.profiler.EndExecution(toolName, sessionID, success, errorType)

	// Log performance metrics if session available
	if finalSession != nil {
		po.logger.Info().
			Str("tool", toolName).
			Str("session_id", sessionID).
			Dur("total_time", finalSession.TotalTime).
			Dur("dispatch_time", finalSession.DispatchTime).
			Dur("execution_time", finalSession.ExecutionTime).
			Uint64("memory_used", finalSession.MemoryDelta.HeapAlloc).
			Bool("success", success).
			Msg("Tool execution profiled")
	}

	return result, err
}

// ExecuteToolWithBenchmark executes a tool with benchmarking
func (po *ProfiledOrchestrator) ExecuteToolWithBenchmark(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
	benchmarkConfig BenchmarkConfig,
) *ProfiledExecutionResult {
	sessionID := po.extractSessionID(session)

	// Create benchmark suite
	benchmarkSuite := NewBenchmarkSuite(po.logger, po.profiler)

	// Define the tool execution function
	toolExecution := func(ctx context.Context) (interface{}, error) {
		return po.ExecuteTool(ctx, toolName, args, session)
	}

	// Run benchmark
	benchmarkConfig.ToolName = toolName
	benchmarkConfig.SessionID = sessionID

	var benchmarkResult *BenchmarkResult
	if benchmarkConfig.Concurrency > 1 {
		benchmarkResult = benchmarkSuite.RunConcurrentBenchmark(benchmarkConfig, toolExecution)
	} else {
		benchmarkResult = benchmarkSuite.RunBenchmark(benchmarkConfig, toolExecution)
	}

	// Execute once more to get the actual result
	result, err := po.ExecuteTool(ctx, toolName, args, session)

	return &ProfiledExecutionResult{
		Result:    result,
		Error:     err,
		Session:   nil, // Already captured in benchmark
		Benchmark: benchmarkResult,
	}
}

// Note: ValidateToolArgs method removed as validation is handled internally by ExecuteTool

// GetProfiler returns the underlying profiler for direct access
func (po *ProfiledOrchestrator) GetProfiler() *ToolProfiler {
	return po.profiler
}

// GetMetrics returns current performance metrics
func (po *ProfiledOrchestrator) GetMetrics() *MetricsCollector {
	return po.profiler.GetMetrics()
}

// GeneratePerformanceReport creates a comprehensive performance report
func (po *ProfiledOrchestrator) GeneratePerformanceReport() *PerformanceReport {
	return po.profiler.GetMetrics().GeneratePerformanceReport()
}

// CompareWithBaseline compares current performance with a baseline
func (po *ProfiledOrchestrator) CompareWithBaseline(baseline *PerformanceReport) *BenchmarkComparison {
	return po.profiler.GetMetrics().CompareWithBaseline(baseline)
}

// BenchmarkToolPerformance runs a comprehensive benchmark for a specific tool
func (po *ProfiledOrchestrator) BenchmarkToolPerformance(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
	iterations int,
) *BenchmarkResult {
	config := BenchmarkConfig{
		Iterations:    iterations,
		Concurrency:   1,
		WarmupRounds:  5,
		CooldownDelay: 10 * time.Millisecond,
		ToolName:      toolName,
		MonitorMemory: true,
		MonitorCPU:    true,
		GCBetweenRuns: true,
	}

	result := po.ExecuteToolWithBenchmark(ctx, toolName, args, session, config)
	return result.Benchmark
}

// BenchmarkConcurrentPerformance runs a concurrent benchmark for a specific tool
func (po *ProfiledOrchestrator) BenchmarkConcurrentPerformance(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
	concurrency, iterations int,
) *BenchmarkResult {
	config := BenchmarkConfig{
		Iterations:    iterations,
		Concurrency:   concurrency,
		WarmupRounds:  5,
		CooldownDelay: 5 * time.Millisecond,
		ToolName:      toolName,
		MonitorMemory: true,
		MonitorCPU:    true,
		GCBetweenRuns: false, // Don't GC between runs in concurrent test
	}

	result := po.ExecuteToolWithBenchmark(ctx, toolName, args, session, config)
	return result.Benchmark
}

// EnableProfiling enables or disables profiling
func (po *ProfiledOrchestrator) EnableProfiling(enabled bool) {
	po.profiler.Enable(enabled)
	po.logger.Info().Bool("enabled", enabled).Msg("Profiling state changed")
}

// IsProfilingEnabled returns whether profiling is currently enabled
func (po *ProfiledOrchestrator) IsProfilingEnabled() bool {
	return po.profiler.IsEnabled()
}

// extractSessionID extracts session ID from session object
func (po *ProfiledOrchestrator) extractSessionID(session interface{}) string {
	if session == nil {
		return "unknown"
	}

	// Try to extract session ID via type assertion
	// This is a simplified approach - in practice, you'd have proper interfaces
	if sessionWithID, ok := session.(interface{ GetSessionID() string }); ok {
		return sessionWithID.GetSessionID()
	}

	// Fallback to string representation
	return "session"
}

// getTypeName returns the type name of an interface{}
func getTypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

// ProfilingMiddleware provides middleware for adding profiling to existing orchestrators
type ProfilingMiddleware struct {
	profiler *ToolProfiler
	logger   zerolog.Logger
}

// NewProfilingMiddleware creates a new profiling middleware
func NewProfilingMiddleware(logger zerolog.Logger) *ProfilingMiddleware {
	enabled := true
	if envVal := os.Getenv("MCP_PROFILING_ENABLED"); envVal != "" {
		if val, err := strconv.ParseBool(envVal); err == nil {
			enabled = val
		}
	}

	return &ProfilingMiddleware{
		profiler: NewToolProfiler(logger, enabled),
		logger:   logger.With().Str("component", "profiling_middleware").Logger(),
	}
}

// WrapExecution wraps a tool execution with profiling
func (pm *ProfilingMiddleware) WrapExecution(
	toolName, sessionID string,
	execution func() (interface{}, error),
) (interface{}, error) {
	// Start profiling
	pm.profiler.StartExecution(toolName, sessionID)

	// Record dispatch complete immediately (for middleware usage)
	pm.profiler.RecordDispatchComplete(toolName, sessionID)

	// Execute
	result, err := execution()

	// End profiling
	success := err == nil
	errorType := ""
	if err != nil {
		errorType = getTypeName(err)
	}

	pm.profiler.EndExecution(toolName, sessionID, success, errorType)

	return result, err
}

// GetProfiler returns the middleware's profiler
func (pm *ProfilingMiddleware) GetProfiler() *ToolProfiler {
	return pm.profiler
}
