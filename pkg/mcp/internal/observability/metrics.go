package observability

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// MetricsCollector aggregates and analyzes tool execution metrics
type MetricsCollector struct {
	mu                sync.RWMutex
	executions        []*ExecutionSession
	toolStats         map[string]*ToolStats
	maxHistorySize    int
	aggregationWindow time.Duration
}

// ToolStats provides aggregated statistics for a specific tool
type ToolStats struct {
	ToolName           string
	ExecutionCount     int64
	SuccessCount       int64
	FailureCount       int64
	TotalExecutionTime time.Duration
	TotalDispatchTime  time.Duration

	// Timing statistics
	MinExecutionTime time.Duration
	MaxExecutionTime time.Duration
	AvgExecutionTime time.Duration
	P50ExecutionTime time.Duration
	P95ExecutionTime time.Duration
	P99ExecutionTime time.Duration

	// Memory statistics
	AvgMemoryUsage    uint64
	MaxMemoryUsage    uint64
	TotalMemoryAllocs uint64

	// Recent performance trend
	RecentExecutions []time.Duration
	LastUpdated      time.Time
}

// PerformanceReport provides a comprehensive performance analysis
type PerformanceReport struct {
	GeneratedAt        time.Time
	TotalExecutions    int64
	TotalSuccessful    int64
	TotalFailed        int64
	OverallSuccessRate float64

	// Aggregate metrics
	TotalExecutionTime time.Duration
	AvgExecutionTime   time.Duration
	TotalMemoryUsage   uint64

	// Tool-specific statistics
	ToolStats map[string]*ToolStats

	// Performance insights
	SlowestTools     []string
	FastestTools     []string
	MemoryHeavyTools []string
	MostFailedTools  []string

	// Recommendations
	Recommendations []string
}

// BenchmarkComparison compares performance before and after optimizations
type BenchmarkComparison struct {
	Baseline           *PerformanceReport
	Optimized          *PerformanceReport
	ImprovementFactors map[string]float64
	Summary            string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		executions:        make([]*ExecutionSession, 0),
		toolStats:         make(map[string]*ToolStats),
		maxHistorySize:    10000, // Keep last 10k executions
		aggregationWindow: 5 * time.Minute,
	}
}

// RecordExecution records a completed tool execution
func (mc *MetricsCollector) RecordExecution(session *ExecutionSession) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Add to execution history
	mc.executions = append(mc.executions, session)

	// Maintain history size limit
	if len(mc.executions) > mc.maxHistorySize {
		mc.executions = mc.executions[len(mc.executions)-mc.maxHistorySize:]
	}

	// Update tool-specific statistics
	mc.updateToolStats(session)
}

// GetToolStats returns statistics for a specific tool
func (mc *MetricsCollector) GetToolStats(toolName string) *ToolStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats, exists := mc.toolStats[toolName]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	statsCopy := *stats
	return &statsCopy
}

// GetAllToolStats returns statistics for all tools
func (mc *MetricsCollector) GetAllToolStats() map[string]*ToolStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*ToolStats)
	for toolName, stats := range mc.toolStats {
		statsCopy := *stats
		result[toolName] = &statsCopy
	}

	return result
}

// GeneratePerformanceReport creates a comprehensive performance report
func (mc *MetricsCollector) GeneratePerformanceReport() *PerformanceReport {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	report := &PerformanceReport{
		GeneratedAt: time.Now(),
		ToolStats:   make(map[string]*ToolStats),
	}

	// Calculate aggregate metrics
	var totalExecutions, totalSuccessful, totalFailed int64
	var totalExecutionTime time.Duration
	var totalMemoryUsage uint64

	for _, stats := range mc.toolStats {
		totalExecutions += stats.ExecutionCount
		totalSuccessful += stats.SuccessCount
		totalFailed += stats.FailureCount
		totalExecutionTime += stats.TotalExecutionTime
		totalMemoryUsage += stats.TotalMemoryAllocs

		// Copy tool stats
		statsCopy := *stats
		report.ToolStats[stats.ToolName] = &statsCopy
	}

	report.TotalExecutions = totalExecutions
	report.TotalSuccessful = totalSuccessful
	report.TotalFailed = totalFailed
	report.TotalExecutionTime = totalExecutionTime
	report.TotalMemoryUsage = totalMemoryUsage

	if totalExecutions > 0 {
		report.OverallSuccessRate = float64(totalSuccessful) / float64(totalExecutions) * 100
		report.AvgExecutionTime = totalExecutionTime / time.Duration(totalExecutions)
	}

	// Generate insights
	report.generateInsights()

	return report
}

// CompareWithBaseline compares current performance with a baseline
func (mc *MetricsCollector) CompareWithBaseline(baseline *PerformanceReport) *BenchmarkComparison {
	current := mc.GeneratePerformanceReport()

	comparison := &BenchmarkComparison{
		Baseline:           baseline,
		Optimized:          current,
		ImprovementFactors: make(map[string]float64),
	}

	// Calculate improvement factors
	if baseline.AvgExecutionTime > 0 && current.AvgExecutionTime > 0 {
		comparison.ImprovementFactors["avg_execution_time"] =
			float64(baseline.AvgExecutionTime) / float64(current.AvgExecutionTime)
	}

	if baseline.TotalMemoryUsage > 0 && current.TotalMemoryUsage > 0 {
		comparison.ImprovementFactors["memory_usage"] =
			float64(baseline.TotalMemoryUsage) / float64(current.TotalMemoryUsage)
	}

	if baseline.OverallSuccessRate > 0 {
		comparison.ImprovementFactors["success_rate"] =
			current.OverallSuccessRate / baseline.OverallSuccessRate
	}

	// Generate summary
	comparison.generateSummary()

	return comparison
}

// GetRecentExecutions returns executions within the specified time window
func (mc *MetricsCollector) GetRecentExecutions(since time.Time) []*ExecutionSession {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var recent []*ExecutionSession
	for _, execution := range mc.executions {
		if execution.StartTime.After(since) {
			recent = append(recent, execution)
		}
	}

	return recent
}

// updateToolStats updates statistics for a specific tool
func (mc *MetricsCollector) updateToolStats(session *ExecutionSession) {
	stats, exists := mc.toolStats[session.ToolName]
	if !exists {
		stats = &ToolStats{
			ToolName:         session.ToolName,
			MinExecutionTime: session.ExecutionTime,
			MaxExecutionTime: session.ExecutionTime,
			RecentExecutions: make([]time.Duration, 0, 100),
		}
		mc.toolStats[session.ToolName] = stats
	}

	// Update counts
	stats.ExecutionCount++
	if session.Success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}

	// Update timing statistics
	stats.TotalExecutionTime += session.ExecutionTime
	stats.TotalDispatchTime += session.DispatchTime
	stats.AvgExecutionTime = stats.TotalExecutionTime / time.Duration(stats.ExecutionCount)

	if session.ExecutionTime < stats.MinExecutionTime {
		stats.MinExecutionTime = session.ExecutionTime
	}
	if session.ExecutionTime > stats.MaxExecutionTime {
		stats.MaxExecutionTime = session.ExecutionTime
	}

	// Update memory statistics
	memoryUsed := session.MemoryDelta.HeapAlloc
	stats.TotalMemoryAllocs += memoryUsed
	stats.AvgMemoryUsage = stats.TotalMemoryAllocs / uint64(stats.ExecutionCount)
	if memoryUsed > stats.MaxMemoryUsage {
		stats.MaxMemoryUsage = memoryUsed
	}

	// Update recent executions for percentile calculations
	stats.RecentExecutions = append(stats.RecentExecutions, session.ExecutionTime)
	if len(stats.RecentExecutions) > 100 {
		stats.RecentExecutions = stats.RecentExecutions[len(stats.RecentExecutions)-100:]
	}

	// Calculate percentiles
	mc.calculatePercentiles(stats)

	stats.LastUpdated = time.Now()
}

// calculatePercentiles computes execution time percentiles
func (mc *MetricsCollector) calculatePercentiles(stats *ToolStats) {
	if len(stats.RecentExecutions) == 0 {
		return
	}

	// Sort execution times
	times := make([]time.Duration, len(stats.RecentExecutions))
	copy(times, stats.RecentExecutions)
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	// Calculate percentiles
	n := len(times)
	stats.P50ExecutionTime = times[n*50/100]
	stats.P95ExecutionTime = times[n*95/100]
	stats.P99ExecutionTime = times[n*99/100]
}

// generateInsights creates performance insights and recommendations
func (report *PerformanceReport) generateInsights() {
	// Find slowest tools
	type toolPerf struct {
		name    string
		avgTime time.Duration
	}

	var tools []toolPerf
	for name, stats := range report.ToolStats {
		tools = append(tools, toolPerf{name: name, avgTime: stats.AvgExecutionTime})
	}

	// Sort by average execution time (descending)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].avgTime > tools[j].avgTime
	})

	// Extract top 5 slowest and fastest
	for i, tool := range tools {
		if i < 5 {
			report.SlowestTools = append(report.SlowestTools, tool.name)
		}
		if i >= len(tools)-5 {
			report.FastestTools = append(report.FastestTools, tool.name)
		}
	}

	// Find memory-heavy tools
	sort.Slice(tools, func(i, j int) bool {
		return report.ToolStats[tools[i].name].AvgMemoryUsage >
			report.ToolStats[tools[j].name].AvgMemoryUsage
	})

	for i, tool := range tools {
		if i < 3 {
			report.MemoryHeavyTools = append(report.MemoryHeavyTools, tool.name)
		}
	}

	// Find tools with highest failure rates
	sort.Slice(tools, func(i, j int) bool {
		statsI := report.ToolStats[tools[i].name]
		statsJ := report.ToolStats[tools[j].name]
		failureRateI := float64(statsI.FailureCount) / float64(statsI.ExecutionCount)
		failureRateJ := float64(statsJ.FailureCount) / float64(statsJ.ExecutionCount)
		return failureRateI > failureRateJ
	})

	for i, tool := range tools {
		if i < 3 {
			stats := report.ToolStats[tool.name]
			if stats.FailureCount > 0 {
				report.MostFailedTools = append(report.MostFailedTools, tool.name)
			}
		}
	}

	// Generate recommendations
	report.generateRecommendations()
}

// generateRecommendations creates actionable performance recommendations
func (report *PerformanceReport) generateRecommendations() {
	if report.OverallSuccessRate < 95.0 {
		report.Recommendations = append(report.Recommendations,
			"Overall success rate is below 95%. Investigate error patterns in failing tools.")
	}

	if len(report.SlowestTools) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Focus optimization efforts on slowest tools: "+report.SlowestTools[0])
	}

	if len(report.MemoryHeavyTools) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Review memory usage in tools: "+report.MemoryHeavyTools[0])
	}

	if len(report.MostFailedTools) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Investigate failure patterns in: "+report.MostFailedTools[0])
	}

	if report.TotalExecutions > 1000 && report.AvgExecutionTime > 5*time.Second {
		report.Recommendations = append(report.Recommendations,
			"Consider implementing caching or optimizing slow operations.")
	}
}

// generateSummary creates a human-readable summary of performance improvements
func (comparison *BenchmarkComparison) generateSummary() {
	if factor, exists := comparison.ImprovementFactors["avg_execution_time"]; exists {
		if factor > 1.0 {
			comparison.Summary = "Performance improved by %.1fx in average execution time. "
			comparison.Summary = fmt.Sprintf(comparison.Summary, factor)
		} else {
			comparison.Summary = "Performance degraded by %.1fx in average execution time. "
			comparison.Summary = fmt.Sprintf(comparison.Summary, 1.0/factor)
		}
	}

	if factor, exists := comparison.ImprovementFactors["memory_usage"]; exists {
		if factor > 1.0 {
			comparison.Summary += fmt.Sprintf("Memory usage improved by %.1fx. ", factor)
		}
	}

	if comparison.Summary == "" {
		comparison.Summary = "No significant performance changes detected."
	}
}
