package profiling

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// BenchmarkSuite provides comprehensive benchmarking capabilities
type BenchmarkSuite struct {
	logger   zerolog.Logger
	profiler *ToolProfiler
}

// BenchmarkConfig configures benchmark execution parameters
type BenchmarkConfig struct {
	// Test parameters
	Iterations    int
	Concurrency   int
	WarmupRounds  int
	CooldownDelay time.Duration

	// Tool configuration
	ToolName  string
	SessionID string

	// Resource monitoring
	MonitorMemory bool
	MonitorCPU    bool
	GCBetweenRuns bool
}

// BenchmarkResult contains the results of a benchmark run
type BenchmarkResult struct {
	Config        BenchmarkConfig
	StartTime     time.Time
	EndTime       time.Time
	TotalDuration time.Duration

	// Execution metrics
	TotalOperations  int64
	SuccessfulOps    int64
	FailedOps        int64
	OperationsPerSec float64

	// Timing statistics
	MinLatency time.Duration
	MaxLatency time.Duration
	AvgLatency time.Duration
	P50Latency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration

	// Resource usage
	StartMemory  MemoryStats
	EndMemory    MemoryStats
	PeakMemory   uint64
	MemoryGrowth uint64

	// Concurrent execution metrics
	ConcurrentAvgLatency time.Duration
	ThroughputPerCore    float64

	// Error analysis
	ErrorTypes map[string]int64
	ErrorRate  float64
}

// PerformanceComparison compares two benchmark results
type PerformanceComparison struct {
	Baseline  *BenchmarkResult
	Optimized *BenchmarkResult

	// Performance ratios (optimized/baseline)
	LatencyImprovement    float64 // <1.0 means improvement
	ThroughputImprovement float64 // >1.0 means improvement
	MemoryImprovement     float64 // <1.0 means improvement

	// Summary
	OverallImprovement string
	SignificantChanges []string
	Recommendations    []string
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(logger zerolog.Logger, profiler *ToolProfiler) *BenchmarkSuite {
	return &BenchmarkSuite{
		logger:   logger.With().Str("component", "benchmark_suite").Logger(),
		profiler: profiler,
	}
}

// RunBenchmark executes a comprehensive benchmark with the given configuration
func (bs *BenchmarkSuite) RunBenchmark(
	config BenchmarkConfig,
	toolExecution func(context.Context) (interface{}, error),
) *BenchmarkResult {

	bs.logger.Info().
		Str("tool", config.ToolName).
		Int("iterations", config.Iterations).
		Int("concurrency", config.Concurrency).
		Msg("Starting benchmark")

	result := &BenchmarkResult{
		Config:     config,
		StartTime:  time.Now(),
		ErrorTypes: make(map[string]int64),
	}

	// Capture initial memory state
	if config.MonitorMemory {
		result.StartMemory = bs.captureMemoryStats()
	}

	// Warmup runs
	if config.WarmupRounds > 0 {
		bs.logger.Debug().Int("warmup_rounds", config.WarmupRounds).Msg("Running warmup")
		bs.runWarmup(config, toolExecution)
		if config.GCBetweenRuns {
			runtime.GC()
		}
	}

	// Main benchmark execution
	latencies := bs.runMainBenchmark(config, toolExecution, result)

	// Calculate statistics
	bs.calculateStatistics(result, latencies)

	// Capture final memory state
	if config.MonitorMemory {
		result.EndMemory = bs.captureMemoryStats()
		result.MemoryGrowth = result.EndMemory.HeapAlloc - result.StartMemory.HeapAlloc
	}

	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)

	if result.TotalOperations > 0 {
		result.OperationsPerSec = float64(result.TotalOperations) / result.TotalDuration.Seconds()
		result.ErrorRate = float64(result.FailedOps) / float64(result.TotalOperations) * 100
	}

	bs.logger.Info().
		Str("tool", config.ToolName).
		Int64("operations", result.TotalOperations).
		Float64("ops_per_sec", result.OperationsPerSec).
		Dur("avg_latency", result.AvgLatency).
		Dur("p95_latency", result.P95Latency).
		Float64("error_rate", result.ErrorRate).
		Msg("Benchmark completed")

	return result
}

// RunConcurrentBenchmark executes a benchmark with concurrent workers
func (bs *BenchmarkSuite) RunConcurrentBenchmark(
	config BenchmarkConfig,
	toolExecution func(context.Context) (interface{}, error),
) *BenchmarkResult {

	bs.logger.Info().
		Str("tool", config.ToolName).
		Int("concurrency", config.Concurrency).
		Int("iterations_per_worker", config.Iterations).
		Msg("Starting concurrent benchmark")

	result := &BenchmarkResult{
		Config:     config,
		StartTime:  time.Now(),
		ErrorTypes: make(map[string]int64),
	}

	// Capture initial memory
	if config.MonitorMemory {
		result.StartMemory = bs.captureMemoryStats()
	}

	// Channel to collect latencies from all workers
	latencyChan := make(chan time.Duration, config.Concurrency*config.Iterations)
	errorChan := make(chan string, config.Concurrency*config.Iterations)

	var wg sync.WaitGroup

	// Start concurrent workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			bs.runWorker(workerID, config, toolExecution, latencyChan, errorChan)
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(latencyChan)
	close(errorChan)

	// Collect results
	var latencies []time.Duration
	for latency := range latencyChan {
		latencies = append(latencies, latency)
		result.TotalOperations++
		result.SuccessfulOps++
	}

	// Collect errors
	for errorType := range errorChan {
		result.ErrorTypes[errorType]++
		result.TotalOperations++
		result.FailedOps++
	}

	// Calculate statistics
	bs.calculateStatistics(result, latencies)

	// Capture final memory
	if config.MonitorMemory {
		result.EndMemory = bs.captureMemoryStats()
		result.MemoryGrowth = result.EndMemory.HeapAlloc - result.StartMemory.HeapAlloc
	}

	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)

	if result.TotalOperations > 0 {
		result.OperationsPerSec = float64(result.TotalOperations) / result.TotalDuration.Seconds()
		result.ErrorRate = float64(result.FailedOps) / float64(result.TotalOperations) * 100
	}

	// Calculate concurrent-specific metrics
	if config.Concurrency > 0 {
		result.ConcurrentAvgLatency = result.AvgLatency
		result.ThroughputPerCore = result.OperationsPerSec / float64(runtime.NumCPU())
	}

	bs.logger.Info().
		Str("tool", config.ToolName).
		Int("workers", config.Concurrency).
		Int64("operations", result.TotalOperations).
		Float64("ops_per_sec", result.OperationsPerSec).
		Dur("avg_latency", result.AvgLatency).
		Float64("throughput_per_core", result.ThroughputPerCore).
		Msg("Concurrent benchmark completed")

	return result
}

// CompareBenchmarks compares two benchmark results and provides analysis
func (bs *BenchmarkSuite) CompareBenchmarks(baseline, optimized *BenchmarkResult) *PerformanceComparison {
	comparison := &PerformanceComparison{
		Baseline:  baseline,
		Optimized: optimized,
	}

	// Calculate improvement ratios
	if baseline.AvgLatency > 0 {
		comparison.LatencyImprovement = float64(optimized.AvgLatency) / float64(baseline.AvgLatency)
	}

	if baseline.OperationsPerSec > 0 {
		comparison.ThroughputImprovement = optimized.OperationsPerSec / baseline.OperationsPerSec
	}

	if baseline.MemoryGrowth > 0 {
		comparison.MemoryImprovement = float64(optimized.MemoryGrowth) / float64(baseline.MemoryGrowth)
	}

	// Generate analysis
	bs.generateComparisonAnalysis(comparison)

	return comparison
}

// runWarmup executes warmup iterations to stabilize performance
func (bs *BenchmarkSuite) runWarmup(
	config BenchmarkConfig,
	toolExecution func(context.Context) (interface{}, error),
) {
	for i := 0; i < config.WarmupRounds; i++ {
		ctx := context.Background()
		_, _ = toolExecution(ctx)
	}
}

// runMainBenchmark executes the main benchmark iterations
func (bs *BenchmarkSuite) runMainBenchmark(
	config BenchmarkConfig,
	toolExecution func(context.Context) (interface{}, error),
	result *BenchmarkResult,
) []time.Duration {

	latencies := make([]time.Duration, 0, config.Iterations)

	for i := 0; i < config.Iterations; i++ {
		start := time.Now()
		ctx := context.Background()

		_, err := toolExecution(ctx)

		latency := time.Since(start)
		latencies = append(latencies, latency)

		result.TotalOperations++
		if err != nil {
			result.FailedOps++
			errorType := "execution_error"
			if err != nil {
				errorType = fmt.Sprintf("%T", err)
			}
			result.ErrorTypes[errorType]++
		} else {
			result.SuccessfulOps++
		}

		// Optional cooldown between iterations
		if config.CooldownDelay > 0 {
			time.Sleep(config.CooldownDelay)
		}
	}

	return latencies
}

// runWorker executes benchmark iterations in a single worker goroutine
func (bs *BenchmarkSuite) runWorker(
	workerID int,
	config BenchmarkConfig,
	toolExecution func(context.Context) (interface{}, error),
	latencyChan chan<- time.Duration,
	errorChan chan<- string,
) {
	for i := 0; i < config.Iterations; i++ {
		start := time.Now()
		ctx := context.Background()

		_, err := toolExecution(ctx)

		latency := time.Since(start)

		if err != nil {
			errorType := "execution_error"
			if err != nil {
				errorType = fmt.Sprintf("%T", err)
			}
			errorChan <- errorType
		} else {
			latencyChan <- latency
		}

		// Optional cooldown between iterations
		if config.CooldownDelay > 0 {
			time.Sleep(config.CooldownDelay)
		}
	}
}

// calculateStatistics computes latency percentiles and averages
func (bs *BenchmarkSuite) calculateStatistics(result *BenchmarkResult, latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}

	// Calculate statistics
	result.MinLatency = sortedLatencies[0]
	result.MaxLatency = sortedLatencies[len(sortedLatencies)-1]

	// Calculate average
	var totalLatency time.Duration
	for _, latency := range latencies {
		totalLatency += latency
	}
	result.AvgLatency = totalLatency / time.Duration(len(latencies))

	// Calculate percentiles
	n := len(sortedLatencies)
	result.P50Latency = sortedLatencies[n*50/100]
	result.P95Latency = sortedLatencies[n*95/100]
	result.P99Latency = sortedLatencies[n*99/100]
}

// generateComparisonAnalysis creates analysis and recommendations
func (bs *BenchmarkSuite) generateComparisonAnalysis(comparison *PerformanceComparison) {
	// Latency analysis
	if comparison.LatencyImprovement < 0.9 {
		improvement := (1.0 - comparison.LatencyImprovement) * 100
		comparison.SignificantChanges = append(comparison.SignificantChanges,
			fmt.Sprintf("Latency improved by %.1f%%", improvement))
		comparison.OverallImprovement = "Significant Performance Improvement"
	} else if comparison.LatencyImprovement > 1.1 {
		degradation := (comparison.LatencyImprovement - 1.0) * 100
		comparison.SignificantChanges = append(comparison.SignificantChanges,
			fmt.Sprintf("Latency degraded by %.1f%%", degradation))
		comparison.OverallImprovement = "Performance Degradation Detected"
	}

	// Throughput analysis
	if comparison.ThroughputImprovement > 1.1 {
		improvement := (comparison.ThroughputImprovement - 1.0) * 100
		comparison.SignificantChanges = append(comparison.SignificantChanges,
			fmt.Sprintf("Throughput improved by %.1f%%", improvement))
	} else if comparison.ThroughputImprovement < 0.9 {
		degradation := (1.0 - comparison.ThroughputImprovement) * 100
		comparison.SignificantChanges = append(comparison.SignificantChanges,
			fmt.Sprintf("Throughput degraded by %.1f%%", degradation))
	}

	// Memory analysis
	if comparison.MemoryImprovement < 0.9 {
		improvement := (1.0 - comparison.MemoryImprovement) * 100
		comparison.SignificantChanges = append(comparison.SignificantChanges,
			fmt.Sprintf("Memory usage improved by %.1f%%", improvement))
	}

	// Generate recommendations
	if comparison.LatencyImprovement > 1.2 {
		comparison.Recommendations = append(comparison.Recommendations,
			"Investigate performance regression - latency significantly increased")
	}

	if comparison.ThroughputImprovement < 0.8 {
		comparison.Recommendations = append(comparison.Recommendations,
			"Review optimization strategy - throughput significantly decreased")
	}

	if len(comparison.SignificantChanges) == 0 {
		comparison.OverallImprovement = "No Significant Performance Changes"
	}
}

// captureMemoryStats captures current memory statistics
func (bs *BenchmarkSuite) captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		GCCPUFraction: m.GCCPUFraction,
	}
}
