package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// PerformanceMonitor tracks performance metrics across all teams
type PerformanceMonitor struct {
	logger       zerolog.Logger
	mutex        sync.RWMutex
	metrics      map[string]*PerformanceMetrics
	benchmarks   map[string]*BenchmarkResults
	thresholds   PerformanceThresholds
}

// PerformanceThresholds defines performance targets
type PerformanceThresholds struct {
	MaxP95Latency    time.Duration `json:"max_p95_latency"`    // Target: <300μs P95
	MaxP99Latency    time.Duration `json:"max_p99_latency"`    // Target: <1ms P99
	MinThroughput    float64       `json:"min_throughput"`     // Requests per second
	MaxMemoryUsage   int64         `json:"max_memory_usage"`   // Bytes
	MaxCPUUsage      float64       `json:"max_cpu_usage"`      // Percentage
}

// PerformanceMetrics tracks performance for a specific component
type PerformanceMetrics struct {
	ComponentName   string             `json:"component_name"`
	TeamName        string             `json:"team_name"`
	Measurements    []Measurement      `json:"measurements"`
	Statistics      Statistics         `json:"statistics"`
	LastUpdated     time.Time          `json:"last_updated"`
	AlertStatus     string             `json:"alert_status"` // "GREEN", "YELLOW", "RED"
}

// Measurement represents a single performance measurement
type Measurement struct {
	Timestamp    time.Time     `json:"timestamp"`
	Latency      time.Duration `json:"latency"`
	MemoryUsage  int64         `json:"memory_usage"`
	CPUUsage     float64       `json:"cpu_usage"`
	Success      bool          `json:"success"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// Statistics provides aggregated performance statistics
type Statistics struct {
	Count        int           `json:"count"`
	SuccessRate  float64       `json:"success_rate"`
	AvgLatency   time.Duration `json:"avg_latency"`
	P50Latency   time.Duration `json:"p50_latency"`
	P95Latency   time.Duration `json:"p95_latency"`
	P99Latency   time.Duration `json:"p99_latency"`
	MaxLatency   time.Duration `json:"max_latency"`
	MinLatency   time.Duration `json:"min_latency"`
	AvgMemory    int64         `json:"avg_memory"`
	MaxMemory    int64         `json:"max_memory"`
	AvgCPU       float64       `json:"avg_cpu"`
	MaxCPU       float64       `json:"max_cpu"`
	Throughput   float64       `json:"throughput"` // requests per second
}

// BenchmarkResults tracks benchmark performance over time
type BenchmarkResults struct {
	BenchmarkName string        `json:"benchmark_name"`
	TeamName      string        `json:"team_name"`
	Runs          []BenchmarkRun `json:"runs"`
	Baseline      *BenchmarkRun  `json:"baseline,omitempty"`
	Trend         string         `json:"trend"` // "IMPROVING", "STABLE", "DEGRADING"
}

// BenchmarkRun represents a single benchmark execution
type BenchmarkRun struct {
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	Operations    int64         `json:"operations"`
	OpsPerSecond  float64       `json:"ops_per_second"`
	P95Latency    time.Duration `json:"p95_latency"`
	MemoryUsage   int64         `json:"memory_usage"`
	Success       bool          `json:"success"`
	Version       string        `json:"version,omitempty"`
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger zerolog.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger:     logger.With().Str("component", "performance_monitor").Logger(),
		metrics:    make(map[string]*PerformanceMetrics),
		benchmarks: make(map[string]*BenchmarkResults),
		thresholds: PerformanceThresholds{
			MaxP95Latency:  300 * time.Microsecond, // From CLAUDE.md requirements
			MaxP99Latency:  1 * time.Millisecond,
			MinThroughput:  100.0, // 100 RPS minimum
			MaxMemoryUsage: 512 * 1024 * 1024, // 512MB
			MaxCPUUsage:    80.0, // 80% CPU
		},
	}
}

// RecordMeasurement records a performance measurement
func (pm *PerformanceMonitor) RecordMeasurement(teamName, componentName string, measurement Measurement) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", teamName, componentName)
	
	if pm.metrics[key] == nil {
		pm.metrics[key] = &PerformanceMetrics{
			ComponentName: componentName,
			TeamName:      teamName,
			Measurements:  make([]Measurement, 0),
		}
	}

	metrics := pm.metrics[key]
	metrics.Measurements = append(metrics.Measurements, measurement)
	
	// Keep only last 1000 measurements to prevent memory growth
	if len(metrics.Measurements) > 1000 {
		metrics.Measurements = metrics.Measurements[len(metrics.Measurements)-1000:]
	}

	// Update statistics
	pm.updateStatistics(metrics)
	metrics.LastUpdated = time.Now()
	metrics.AlertStatus = pm.calculateAlertStatus(metrics.Statistics)

	pm.logger.Debug().
		Str("team", teamName).
		Str("component", componentName).
		Dur("latency", measurement.Latency).
		Bool("success", measurement.Success).
		Msg("Performance measurement recorded")
}

// RecordBenchmark records a benchmark result
func (pm *PerformanceMonitor) RecordBenchmark(teamName, benchmarkName string, run BenchmarkRun) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", teamName, benchmarkName)
	
	if pm.benchmarks[key] == nil {
		pm.benchmarks[key] = &BenchmarkResults{
			BenchmarkName: benchmarkName,
			TeamName:      teamName,
			Runs:          make([]BenchmarkRun, 0),
		}
	}

	benchmark := pm.benchmarks[key]
	benchmark.Runs = append(benchmark.Runs, run)
	
	// Keep only last 100 runs
	if len(benchmark.Runs) > 100 {
		benchmark.Runs = benchmark.Runs[len(benchmark.Runs)-100:]
	}

	// Calculate trend
	benchmark.Trend = pm.calculateTrend(benchmark.Runs)

	pm.logger.Info().
		Str("team", teamName).
		Str("benchmark", benchmarkName).
		Dur("duration", run.Duration).
		Float64("ops_per_second", run.OpsPerSecond).
		Dur("p95_latency", run.P95Latency).
		Str("trend", benchmark.Trend).
		Msg("Benchmark recorded")
}

// GetPerformanceReport generates a comprehensive performance report
func (pm *PerformanceMonitor) GetPerformanceReport() *TeamPerformanceReport {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	report := &TeamPerformanceReport{
		Timestamp:      time.Now(),
		OverallHealth:  "GREEN",
		TeamMetrics:    make(map[string]TeamPerformance),
		SystemSummary:  pm.calculateSystemSummary(),
		AlertsSummary:  pm.calculateAlertsSummary(),
	}

	// Group metrics by team
	for _, metrics := range pm.metrics {
		teamName := metrics.TeamName
		teamPerf, exists := report.TeamMetrics[teamName]
		if !exists {
			teamPerf = TeamPerformance{
				TeamName:   teamName,
				Components: make(map[string]PerformanceMetrics),
			}
		}
		
		teamPerf.Components[metrics.ComponentName] = *metrics
		report.TeamMetrics[teamName] = teamPerf
	}

	// Calculate overall health
	report.OverallHealth = pm.calculateOverallHealth(report.TeamMetrics)

	return report
}

// GeneratePerformanceSummary creates a human-readable performance summary
func (pm *PerformanceMonitor) GeneratePerformanceSummary() string {
	report := pm.GetPerformanceReport()
	
	summary := fmt.Sprintf(`PERFORMANCE MONITORING SUMMARY
==============================
Overall Performance Health: %s
Report Generated: %s

SYSTEM-WIDE METRICS:
├─ Average P95 Latency: %v (target: <%v)
├─ Average Throughput: %.1f RPS (target: >%.1f)
├─ Average Memory Usage: %d MB (target: <%d MB)
├─ Average CPU Usage: %.1f%% (target: <%.1f%%)
└─ Overall Success Rate: %.2f%%

TEAM PERFORMANCE:
`, 
		report.OverallHealth,
		report.Timestamp.Format("2006-01-02 15:04:05"),
		report.SystemSummary.AvgP95Latency,
		pm.thresholds.MaxP95Latency,
		report.SystemSummary.AvgThroughput,
		pm.thresholds.MinThroughput,
		report.SystemSummary.AvgMemoryUsage/(1024*1024),
		pm.thresholds.MaxMemoryUsage/(1024*1024),
		report.SystemSummary.AvgCPUUsage,
		pm.thresholds.MaxCPUUsage,
		report.SystemSummary.OverallSuccessRate,
	)

	for teamName, teamPerf := range report.TeamMetrics {
		summary += fmt.Sprintf("├─ %s: %s\n", teamName, pm.getTeamHealthStatus(teamPerf))
		for componentName, metrics := range teamPerf.Components {
			summary += fmt.Sprintf("│  └─ %s: P95=%v, Success=%.1f%%\n", 
				componentName, 
				metrics.Statistics.P95Latency,
				metrics.Statistics.SuccessRate,
			)
		}
	}

	if len(report.AlertsSummary.ActiveAlerts) > 0 {
		summary += "\nACTIVE PERFORMANCE ALERTS:\n"
		for _, alert := range report.AlertsSummary.ActiveAlerts {
			summary += fmt.Sprintf("⚠️  %s: %s\n", alert.Component, alert.Message)
		}
	} else {
		summary += "\n✅ No active performance alerts\n"
	}

	return summary
}

// SavePerformanceReport saves the performance report to disk
func (pm *PerformanceMonitor) SavePerformanceReport(ctx context.Context, filename string) error {
	report := pm.GetPerformanceReport()
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal performance report: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write performance report: %v", err)
	}

	pm.logger.Info().Str("filename", filename).Msg("Performance report saved")
	return nil
}

// Helper types for reporting

type TeamPerformanceReport struct {
	Timestamp      time.Time                   `json:"timestamp"`
	OverallHealth  string                      `json:"overall_health"`
	TeamMetrics    map[string]TeamPerformance  `json:"team_metrics"`
	SystemSummary  SystemPerformanceSummary    `json:"system_summary"`
	AlertsSummary  AlertsSummary               `json:"alerts_summary"`
}

type TeamPerformance struct {
	TeamName   string                          `json:"team_name"`
	Components map[string]PerformanceMetrics   `json:"components"`
}

type SystemPerformanceSummary struct {
	AvgP95Latency       time.Duration `json:"avg_p95_latency"`
	AvgThroughput       float64       `json:"avg_throughput"`
	AvgMemoryUsage      int64         `json:"avg_memory_usage"`
	AvgCPUUsage         float64       `json:"avg_cpu_usage"`
	OverallSuccessRate  float64       `json:"overall_success_rate"`
	TotalMeasurements   int           `json:"total_measurements"`
}

type AlertsSummary struct {
	ActiveAlerts []TeamPerformanceAlert `json:"active_alerts"`
	TotalAlerts  int                    `json:"total_alerts"`
}

type TeamPerformanceAlert struct {
	Component  string    `json:"component"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// Helper methods

func (pm *PerformanceMonitor) updateStatistics(metrics *PerformanceMetrics) {
	if len(metrics.Measurements) == 0 {
		return
	}

	measurements := metrics.Measurements
	stats := &metrics.Statistics

	// Basic counts
	stats.Count = len(measurements)
	successCount := 0
	var totalLatency time.Duration
	var totalMemory int64
	var totalCPU float64
	
	latencies := make([]time.Duration, 0, len(measurements))
	
	for _, m := range measurements {
		if m.Success {
			successCount++
		}
		totalLatency += m.Latency
		totalMemory += m.MemoryUsage
		totalCPU += m.CPUUsage
		latencies = append(latencies, m.Latency)
	}

	// Success rate
	stats.SuccessRate = float64(successCount) / float64(len(measurements)) * 100

	// Averages
	stats.AvgLatency = totalLatency / time.Duration(len(measurements))
	stats.AvgMemory = totalMemory / int64(len(measurements))
	stats.AvgCPU = totalCPU / float64(len(measurements))

	// Sort latencies for percentile calculations
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	// Percentiles
	if len(latencies) > 0 {
		stats.MinLatency = latencies[0]
		stats.MaxLatency = latencies[len(latencies)-1]
		stats.P50Latency = latencies[len(latencies)*50/100]
		stats.P95Latency = latencies[len(latencies)*95/100]
		stats.P99Latency = latencies[len(latencies)*99/100]
	}

	// Memory statistics
	if len(measurements) > 0 {
		stats.MaxMemory = measurements[0].MemoryUsage
		stats.MaxCPU = measurements[0].CPUUsage
		for _, m := range measurements {
			if m.MemoryUsage > stats.MaxMemory {
				stats.MaxMemory = m.MemoryUsage
			}
			if m.CPUUsage > stats.MaxCPU {
				stats.MaxCPU = m.CPUUsage
			}
		}
	}

	// Throughput (requests per second) - estimate based on recent measurements
	if len(measurements) > 1 {
		timeSpan := measurements[len(measurements)-1].Timestamp.Sub(measurements[0].Timestamp)
		if timeSpan > 0 {
			stats.Throughput = float64(len(measurements)) / timeSpan.Seconds()
		}
	}
}

func (pm *PerformanceMonitor) calculateAlertStatus(stats Statistics) string {
	if stats.P95Latency > pm.thresholds.MaxP95Latency ||
		stats.SuccessRate < 95.0 ||
		stats.MaxMemory > pm.thresholds.MaxMemoryUsage ||
		stats.MaxCPU > pm.thresholds.MaxCPUUsage {
		return "RED"
	}
	
	if stats.P95Latency > pm.thresholds.MaxP95Latency*80/100 ||
		stats.SuccessRate < 98.0 ||
		stats.MaxMemory > pm.thresholds.MaxMemoryUsage*80/100 ||
		stats.MaxCPU > pm.thresholds.MaxCPUUsage*80/100 {
		return "YELLOW"
	}
	
	return "GREEN"
}

func (pm *PerformanceMonitor) calculateTrend(runs []BenchmarkRun) string {
	if len(runs) < 3 {
		return "STABLE"
	}

	// Compare last 3 runs with previous 3 runs
	recent := runs[len(runs)-3:]
	previous := runs[len(runs)-6 : len(runs)-3]

	recentAvg := 0.0
	previousAvg := 0.0

	for _, run := range recent {
		recentAvg += run.OpsPerSecond
	}
	recentAvg /= float64(len(recent))

	for _, run := range previous {
		previousAvg += run.OpsPerSecond
	}
	previousAvg /= float64(len(previous))

	change := (recentAvg - previousAvg) / previousAvg * 100

	if change > 5.0 {
		return "IMPROVING"
	} else if change < -5.0 {
		return "DEGRADING"
	}
	return "STABLE"
}

func (pm *PerformanceMonitor) calculateSystemSummary() SystemPerformanceSummary {
	var totalP95 time.Duration
	var totalThroughput float64
	var totalMemory int64
	var totalCPU float64
	var totalSuccess float64
	var totalMeasurements int
	count := 0

	for _, metrics := range pm.metrics {
		if metrics.Statistics.Count > 0 {
			totalP95 += metrics.Statistics.P95Latency
			totalThroughput += metrics.Statistics.Throughput
			totalMemory += metrics.Statistics.AvgMemory
			totalCPU += metrics.Statistics.AvgCPU
			totalSuccess += metrics.Statistics.SuccessRate
			totalMeasurements += metrics.Statistics.Count
			count++
		}
	}

	if count == 0 {
		return SystemPerformanceSummary{}
	}

	return SystemPerformanceSummary{
		AvgP95Latency:      totalP95 / time.Duration(count),
		AvgThroughput:      totalThroughput / float64(count),
		AvgMemoryUsage:     totalMemory / int64(count),
		AvgCPUUsage:        totalCPU / float64(count),
		OverallSuccessRate: totalSuccess / float64(count),
		TotalMeasurements:  totalMeasurements,
	}
}

func (pm *PerformanceMonitor) calculateAlertsSummary() AlertsSummary {
	alerts := []TeamPerformanceAlert{}

	for key, metrics := range pm.metrics {
		if metrics.AlertStatus == "RED" {
			alert := TeamPerformanceAlert{
				Component: key,
				Severity:  "HIGH",
				Message:   pm.generateAlertMessage(metrics),
				Timestamp: metrics.LastUpdated,
			}
			alerts = append(alerts, alert)
		} else if metrics.AlertStatus == "YELLOW" {
			alert := TeamPerformanceAlert{
				Component: key,
				Severity:  "MEDIUM",
				Message:   pm.generateAlertMessage(metrics),
				Timestamp: metrics.LastUpdated,
			}
			alerts = append(alerts, alert)
		}
	}

	return AlertsSummary{
		ActiveAlerts: alerts,
		TotalAlerts:  len(alerts),
	}
}

func (pm *PerformanceMonitor) generateAlertMessage(metrics *PerformanceMetrics) string {
	issues := []string{}

	if metrics.Statistics.P95Latency > pm.thresholds.MaxP95Latency {
		issues = append(issues, fmt.Sprintf("P95 latency %v exceeds threshold %v", 
			metrics.Statistics.P95Latency, pm.thresholds.MaxP95Latency))
	}

	if metrics.Statistics.SuccessRate < 95.0 {
		issues = append(issues, fmt.Sprintf("Success rate %.1f%% below threshold 95%%", 
			metrics.Statistics.SuccessRate))
	}

	if metrics.Statistics.MaxMemory > pm.thresholds.MaxMemoryUsage {
		issues = append(issues, fmt.Sprintf("Memory usage %d MB exceeds threshold %d MB", 
			metrics.Statistics.MaxMemory/(1024*1024), pm.thresholds.MaxMemoryUsage/(1024*1024)))
	}

	if len(issues) == 0 {
		return "Performance degradation detected"
	}

	return strings.Join(issues, "; ")
}

func (pm *PerformanceMonitor) calculateOverallHealth(teamMetrics map[string]TeamPerformance) string {
	redCount := 0
	yellowCount := 0
	greenCount := 0

	for _, team := range teamMetrics {
		for _, metrics := range team.Components {
			switch metrics.AlertStatus {
			case "RED":
				redCount++
			case "YELLOW":
				yellowCount++
			case "GREEN":
				greenCount++
			}
		}
	}

	if redCount > 0 {
		return "RED"
	} else if yellowCount > 0 {
		return "YELLOW"
	} else if greenCount > 0 {
		return "GREEN"
	}
	return "UNKNOWN"
}

func (pm *PerformanceMonitor) getTeamHealthStatus(teamPerf TeamPerformance) string {
	redCount := 0
	yellowCount := 0
	greenCount := 0

	for _, metrics := range teamPerf.Components {
		switch metrics.AlertStatus {
		case "RED":
			redCount++
		case "YELLOW":
			yellowCount++
		case "GREEN":
			greenCount++
		}
	}

	if redCount > 0 {
		return "RED"
	} else if yellowCount > 0 {
		return "YELLOW"
	} else if greenCount > 0 {
		return "GREEN"
	}
	return "UNKNOWN"
}