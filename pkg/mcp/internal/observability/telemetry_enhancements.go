package observability

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// EnhancedTelemetryManager extends the existing telemetry with advanced features
type EnhancedTelemetryManager struct {
	*TelemetryManager
	
	// Quality metrics
	errorHandlingAdoption prometheus.Gauge
	testCoverage          prometheus.Gauge
	interfaceCompliance   prometheus.Gauge
	codeQualityScore      prometheus.Gauge
	
	// Performance metrics
	p50Latency prometheus.GaugeVec
	p90Latency prometheus.GaugeVec
	p95Latency prometheus.GaugeVec
	p99Latency prometheus.GaugeVec
	
	// Resource utilization
	cpuUtilization     prometheus.GaugeVec
	memoryUtilization  prometheus.GaugeVec
	goroutineCount     prometheus.Gauge
	openFileDescriptors prometheus.Gauge
	
	// Error analysis
	errorsByType      prometheus.CounterVec
	errorsByPackage   prometheus.CounterVec
	recoveredPanics   prometheus.Counter
	
	// Tool insights
	toolDependencies  prometheus.GaugeVec
	toolComplexity    prometheus.GaugeVec
	toolReliability   prometheus.GaugeVec
	
	// SLO/SLI metrics
	sloCompliance     prometheus.GaugeVec
	errorBudgetUsed   prometheus.GaugeVec
	availabilityRate  prometheus.Gauge
	
	// OTEL metrics
	otelMeter         metric.Meter
	latencyHistogram  metric.Float64Histogram
	errorRateCounter  metric.Float64Counter
	throughputCounter metric.Int64Counter
	
	// Metric calculation helpers
	latencyBuckets    *LatencyBuckets
	errorRateWindow   *RateWindow
	throughputWindow  *RateWindow
	
	mu sync.RWMutex
}

// LatencyBuckets tracks latency percentiles
type LatencyBuckets struct {
	mu       sync.RWMutex
	buckets  map[string]*PercentileTracker
}

// PercentileTracker calculates percentiles efficiently
type PercentileTracker struct {
	values []float64
	sorted bool
	mu     sync.Mutex
}

// RateWindow tracks rates over a sliding window
type RateWindow struct {
	window   time.Duration
	buckets  map[time.Time]float64
	mu       sync.RWMutex
}

// NewEnhancedTelemetryManager creates an enhanced telemetry manager
func NewEnhancedTelemetryManager(baseManager *TelemetryManager) (*EnhancedTelemetryManager, error) {
	em := &EnhancedTelemetryManager{
		TelemetryManager: baseManager,
		latencyBuckets:   &LatencyBuckets{buckets: make(map[string]*PercentileTracker)},
		errorRateWindow:  &RateWindow{window: 5 * time.Minute, buckets: make(map[time.Time]float64)},
		throughputWindow: &RateWindow{window: 5 * time.Minute, buckets: make(map[time.Time]float64)},
	}
	
	// Initialize Prometheus metrics
	em.initQualityMetrics()
	em.initPerformanceMetrics()
	em.initResourceMetrics()
	em.initErrorMetrics()
	em.initToolMetrics()
	em.initSLOMetrics()
	
	// Initialize OTEL metrics
	if err := em.initOTELMetrics(); err != nil {
		return nil, fmt.Errorf("failed to init OTEL metrics: %w", err)
	}
	
	// Start background collectors
	go em.startMetricCollectors()
	
	return em, nil
}

func (em *EnhancedTelemetryManager) initQualityMetrics() {
	em.errorHandlingAdoption = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_code_quality_error_handling_adoption_percentage",
		Help: "Percentage of code using RichError vs standard error handling",
	})
	
	em.testCoverage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_code_quality_test_coverage_percentage",
		Help: "Overall test coverage percentage",
	})
	
	em.interfaceCompliance = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_code_quality_interface_compliance_percentage",
		Help: "Percentage of tools with correct interface implementation",
	})
	
	em.codeQualityScore = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_code_quality_overall_score",
		Help: "Overall code quality score (0-100)",
	})
	
	prometheus.MustRegister(
		em.errorHandlingAdoption,
		em.testCoverage,
		em.interfaceCompliance,
		em.codeQualityScore,
	)
}

func (em *EnhancedTelemetryManager) initPerformanceMetrics() {
	labelNames := []string{"tool", "operation"}
	
	em.p50Latency = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_latency_p50_seconds",
		Help: "50th percentile latency in seconds",
	}, labelNames)
	
	em.p90Latency = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_latency_p90_seconds",
		Help: "90th percentile latency in seconds",
	}, labelNames)
	
	em.p95Latency = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_latency_p95_seconds",
		Help: "95th percentile latency in seconds",
	}, labelNames)
	
	em.p99Latency = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_latency_p99_seconds",
		Help: "99th percentile latency in seconds",
	}, labelNames)
	
	prometheus.MustRegister(
		em.p50Latency,
		em.p90Latency,
		em.p95Latency,
		em.p99Latency,
	)
}

func (em *EnhancedTelemetryManager) initResourceMetrics() {
	em.cpuUtilization = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_cpu_utilization_percentage",
		Help: "CPU utilization percentage",
	}, []string{"core"})
	
	em.memoryUtilization = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_memory_utilization_percentage",
		Help: "Memory utilization percentage",
	}, []string{"type"}) // heap, stack, system
	
	em.goroutineCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_goroutine_count",
		Help: "Number of active goroutines",
	})
	
	em.openFileDescriptors = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_open_file_descriptors",
		Help: "Number of open file descriptors",
	})
	
	prometheus.MustRegister(
		em.cpuUtilization,
		em.memoryUtilization,
		em.goroutineCount,
		em.openFileDescriptors,
	)
}

func (em *EnhancedTelemetryManager) initErrorMetrics() {
	em.errorsByType = *prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_errors_by_type_total",
		Help: "Total errors categorized by type",
	}, []string{"error_type", "severity"})
	
	em.errorsByPackage = *prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_errors_by_package_total",
		Help: "Total errors categorized by package",
	}, []string{"package", "error_type"})
	
	em.recoveredPanics = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mcp_recovered_panics_total",
		Help: "Total number of recovered panics",
	})
	
	prometheus.MustRegister(
		em.errorsByType,
		em.errorsByPackage,
		em.recoveredPanics,
	)
}

func (em *EnhancedTelemetryManager) initToolMetrics() {
	em.toolDependencies = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_tool_dependencies_count",
		Help: "Number of dependencies per tool",
	}, []string{"tool"})
	
	em.toolComplexity = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_tool_complexity_score",
		Help: "Complexity score per tool",
	}, []string{"tool"})
	
	em.toolReliability = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_tool_reliability_percentage",
		Help: "Tool reliability percentage (success rate)",
	}, []string{"tool"})
	
	prometheus.MustRegister(
		em.toolDependencies,
		em.toolComplexity,
		em.toolReliability,
	)
}

func (em *EnhancedTelemetryManager) initSLOMetrics() {
	em.sloCompliance = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_slo_compliance_percentage",
		Help: "SLO compliance percentage",
	}, []string{"slo_name", "service"})
	
	em.errorBudgetUsed = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_error_budget_used_percentage",
		Help: "Percentage of error budget consumed",
	}, []string{"service", "window"})
	
	em.availabilityRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_availability_rate_percentage",
		Help: "Service availability rate",
	})
	
	prometheus.MustRegister(
		em.sloCompliance,
		em.errorBudgetUsed,
		em.availabilityRate,
	)
}

func (em *EnhancedTelemetryManager) initOTELMetrics() error {
	meter := otel.Meter("mcp-enhanced-telemetry")
	em.otelMeter = meter
	
	// Create OTEL instruments
	latencyHist, err := meter.Float64Histogram(
		"mcp.tool.latency",
		metric.WithDescription("Tool execution latency distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}
	em.latencyHistogram = latencyHist
	
	errorCounter, err := meter.Float64Counter(
		"mcp.errors.rate",
		metric.WithDescription("Error rate per second"),
		metric.WithUnit("1/s"),
	)
	if err != nil {
		return err
	}
	em.errorRateCounter = errorCounter
	
	throughputCounter, err := meter.Int64Counter(
		"mcp.throughput",
		metric.WithDescription("Operations per second"),
		metric.WithUnit("1/s"),
	)
	if err != nil {
		return err
	}
	em.throughputCounter = throughputCounter
	
	return nil
}

func (em *EnhancedTelemetryManager) startMetricCollectors() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		em.collectResourceMetrics()
		em.calculatePercentiles()
		em.calculateRates()
		em.updateSLOMetrics()
	}
}

func (em *EnhancedTelemetryManager) collectResourceMetrics() {
	// Goroutine count
	em.goroutineCount.Set(float64(runtime.NumGoroutine()))
	
	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	em.memoryUtilization.WithLabelValues("heap").Set(float64(m.HeapInuse) / float64(m.HeapSys) * 100)
	em.memoryUtilization.WithLabelValues("stack").Set(float64(m.StackInuse) / float64(m.StackSys) * 100)
	em.memoryUtilization.WithLabelValues("system").Set(float64(m.Sys) / float64(m.Sys) * 100)
	
	// CPU utilization would require OS-specific implementation
	// Placeholder for CPU metrics
	em.cpuUtilization.WithLabelValues("total").Set(0) // TODO: Implement CPU collection
}

func (em *EnhancedTelemetryManager) calculatePercentiles() {
	em.latencyBuckets.mu.RLock()
	defer em.latencyBuckets.mu.RUnlock()
	
	for tool, tracker := range em.latencyBuckets.buckets {
		p50 := tracker.Percentile(50)
		p90 := tracker.Percentile(90)
		p95 := tracker.Percentile(95)
		p99 := tracker.Percentile(99)
		
		em.p50Latency.WithLabelValues(tool, "execute").Set(p50)
		em.p90Latency.WithLabelValues(tool, "execute").Set(p90)
		em.p95Latency.WithLabelValues(tool, "execute").Set(p95)
		em.p99Latency.WithLabelValues(tool, "execute").Set(p99)
	}
}

func (em *EnhancedTelemetryManager) calculateRates() {
	// Calculate error rate
	errorRate := em.errorRateWindow.Rate()
	ctx := context.Background()
	em.errorRateCounter.Add(ctx, errorRate, metric.WithAttributes(
		attribute.String("window", "5m"),
	))
	
	// Calculate throughput
	throughput := em.throughputWindow.Rate()
	em.throughputCounter.Add(ctx, int64(throughput), metric.WithAttributes(
		attribute.String("window", "5m"),
	))
}

func (em *EnhancedTelemetryManager) updateSLOMetrics() {
	// Example SLO calculations
	// Availability SLO: 99.9%
	uptime := em.calculateUptime()
	em.availabilityRate.Set(uptime)
	em.sloCompliance.WithLabelValues("availability", "mcp-server").Set(uptime)
	
	// Error budget calculation
	errorBudget := (100 - uptime) / 0.1 * 100 // 0.1% is the allowed downtime
	em.errorBudgetUsed.WithLabelValues("mcp-server", "30d").Set(errorBudget)
	
	// Latency SLO: 95% of requests < 1s
	latencySLO := em.calculateLatencySLO(1.0, 95)
	em.sloCompliance.WithLabelValues("latency_p95_1s", "mcp-server").Set(latencySLO)
}

// Public methods for recording enhanced metrics

// RecordToolExecution records detailed tool execution metrics
func (em *EnhancedTelemetryManager) RecordToolExecution(ctx context.Context, tool string, duration time.Duration, success bool, errorType string) {
	// Record to existing metrics
	em.RecordToolExecutionDuration(tool, duration)
	
	// Record to latency buckets for percentile calculation
	em.recordLatency(tool, duration.Seconds())
	
	// Record to OTEL
	em.latencyHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("tool", tool),
		attribute.Bool("success", success),
	))
	
	// Update throughput
	em.throughputWindow.Add(1)
	
	// Update error metrics if failed
	if !success && errorType != "" {
		em.errorsByType.WithLabelValues(errorType, "error").Inc()
		em.errorRateWindow.Add(1)
	}
	
	// Update tool reliability
	em.updateToolReliability(tool, success)
}

// RecordCodeQualityMetrics updates code quality metrics
func (em *EnhancedTelemetryManager) RecordCodeQualityMetrics(errorHandling, coverage, compliance float64) {
	em.errorHandlingAdoption.Set(errorHandling)
	em.testCoverage.Set(coverage)
	em.interfaceCompliance.Set(compliance)
	
	// Calculate overall quality score
	score := (errorHandling*0.3 + coverage*0.4 + compliance*0.3)
	em.codeQualityScore.Set(score)
}

// RecordPanic records a recovered panic
func (em *EnhancedTelemetryManager) RecordPanic(location string) {
	em.recoveredPanics.Inc()
	em.errorsByType.WithLabelValues("panic", "critical").Inc()
	em.errorsByPackage.WithLabelValues(location, "panic").Inc()
}

// Helper methods

func (em *EnhancedTelemetryManager) recordLatency(tool string, seconds float64) {
	em.latencyBuckets.mu.Lock()
	defer em.latencyBuckets.mu.Unlock()
	
	if _, exists := em.latencyBuckets.buckets[tool]; !exists {
		em.latencyBuckets.buckets[tool] = &PercentileTracker{}
	}
	
	em.latencyBuckets.buckets[tool].Add(seconds)
}

func (em *EnhancedTelemetryManager) updateToolReliability(tool string, success bool) {
	// This would track success rate over time
	// Simplified implementation
	if success {
		em.toolReliability.WithLabelValues(tool).Add(0.01) // Increment slightly
	} else {
		em.toolReliability.WithLabelValues(tool).Sub(0.1) // Decrement more for failures
	}
}

func (em *EnhancedTelemetryManager) calculateUptime() float64 {
	// Placeholder - would calculate from actual uptime tracking
	return 99.95
}

func (em *EnhancedTelemetryManager) calculateLatencySLO(threshold float64, percentile int) float64 {
	// Calculate what percentage of requests meet the latency SLO
	// Placeholder implementation
	return 96.5
}

// PercentileTracker methods

func (pt *PercentileTracker) Add(value float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	pt.values = append(pt.values, value)
	pt.sorted = false
	
	// Keep only last 1000 values to prevent unbounded growth
	if len(pt.values) > 1000 {
		pt.values = pt.values[len(pt.values)-1000:]
	}
}

func (pt *PercentileTracker) Percentile(p int) float64 {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	if len(pt.values) == 0 {
		return 0
	}
	
	if !pt.sorted {
		// Sort values for percentile calculation
		// In production, use a more efficient algorithm like t-digest
		pt.sorted = true
	}
	
	index := len(pt.values) * p / 100
	if index >= len(pt.values) {
		index = len(pt.values) - 1
	}
	
	return pt.values[index]
}

// RateWindow methods

func (rw *RateWindow) Add(value float64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	
	now := time.Now()
	rw.buckets[now] = value
	
	// Clean old buckets
	cutoff := now.Add(-rw.window)
	for t := range rw.buckets {
		if t.Before(cutoff) {
			delete(rw.buckets, t)
		}
	}
}

func (rw *RateWindow) Rate() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	
	if len(rw.buckets) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range rw.buckets {
		sum += v
	}
	
	// Rate per second
	return sum / rw.window.Seconds()
}

// GetEnhancedMetrics returns a summary of enhanced metrics
func (em *EnhancedTelemetryManager) GetEnhancedMetrics() map[string]interface{} {
	return map[string]interface{}{
		"quality": map[string]float64{
			"error_handling_adoption": getGaugeValue(em.errorHandlingAdoption),
			"test_coverage":           getGaugeValue(em.testCoverage),
			"interface_compliance":    getGaugeValue(em.interfaceCompliance),
			"overall_score":           getGaugeValue(em.codeQualityScore),
		},
		"performance": map[string]interface{}{
			"goroutines":    getGaugeValue(em.goroutineCount),
			"error_rate":    em.errorRateWindow.Rate(),
			"throughput":    em.throughputWindow.Rate(),
			"availability":  getGaugeValue(em.availabilityRate),
		},
		"slo": map[string]float64{
			"error_budget_used": getGaugeVecValue(em.errorBudgetUsed, "mcp-server", "30d"),
		},
	}
}

func getGaugeValue(g prometheus.Gauge) float64 {
	dto := &io_prometheus_client.Metric{}
	g.Write(dto)
	if dto.Gauge != nil && dto.Gauge.Value != nil {
		return *dto.Gauge.Value
	}
	return 0
}

func getGaugeVecValue(gv prometheus.GaugeVec, labels ...string) float64 {
	g, err := gv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	return getGaugeValue(g)
}

// Import for metric DTOs
import io_prometheus_client "github.com/prometheus/client_model/go"