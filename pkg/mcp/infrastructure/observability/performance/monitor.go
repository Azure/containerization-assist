// Package performance provides advanced performance monitoring capabilities
package performance

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability"
)

// PerformanceMonitor provides comprehensive performance monitoring capabilities
type PerformanceMonitor struct {
	observer observability.Observer
	config   *MonitorConfig

	// System metrics
	systemMetrics *SystemMetrics

	// Application metrics
	appMetrics *ApplicationMetrics

	// Operation tracking
	operations sync.Map // map[string]*OperationProfile

	// Resource tracking
	resourceTracker *ResourceTracker

	// Performance profiler
	profiler *Profiler

	// Lifecycle
	startTime time.Time
	ticker    *time.Ticker
	stopCh    chan struct{}
	mu        sync.RWMutex
}

// MonitorConfig provides configuration for performance monitoring
type MonitorConfig struct {
	// Collection intervals
	SystemMetricsInterval time.Duration
	AppMetricsInterval    time.Duration
	ProfileInterval       time.Duration

	// Retention settings
	OperationHistorySize int
	ProfileHistorySize   int

	// Thresholds for alerting
	CPUThreshold       float64
	MemoryThreshold    float64
	ResponseTimeP95    time.Duration
	ErrorRateThreshold float64

	// Feature flags
	EnableSystemMetrics       bool
	EnableProfiling           bool
	EnableGCMetrics           bool
	EnableMemoryLeakDetection bool

	// Sampling
	ProfileSamplingRate float64
	MetricsSamplingRate float64
}

// SystemMetrics tracks system-level performance metrics
type SystemMetrics struct {
	// CPU metrics
	CPUUsage    atomic.Value // float64
	CPUCores    int
	LoadAverage atomic.Value // [3]float64

	// Memory metrics
	MemoryUsed      atomic.Value // uint64
	MemoryTotal     atomic.Value // uint64
	MemoryAvailable atomic.Value // uint64
	MemoryPercent   atomic.Value // float64

	// GC metrics
	GCPauses     []time.Duration
	GCPauseTotal atomic.Value // time.Duration
	GCPauseCount atomic.Value // uint64
	GCFrequency  atomic.Value // float64 (per second)

	// Goroutine metrics
	GoroutineCount atomic.Value // int
	CGOCalls       atomic.Value // int64

	// File descriptor metrics
	OpenFDs atomic.Value // int
	MaxFDs  atomic.Value // int

	mu sync.RWMutex
}

// ApplicationMetrics tracks application-level performance metrics
type ApplicationMetrics struct {
	// Request metrics
	RequestCount    atomic.Value // int64
	RequestRate     atomic.Value // float64 (per second)
	ResponseTimeP50 atomic.Value // time.Duration
	ResponseTimeP95 atomic.Value // time.Duration
	ResponseTimeP99 atomic.Value // time.Duration

	// Error metrics
	ErrorCount atomic.Value // int64
	ErrorRate  atomic.Value // float64

	// Throughput metrics
	BytesProcessed atomic.Value // int64
	ItemsProcessed atomic.Value // int64
	ProcessingRate atomic.Value // float64

	// Cache metrics
	CacheHitRate   atomic.Value // float64
	CacheSize      atomic.Value // int64
	CacheEvictions atomic.Value // int64

	// Database metrics
	DBConnections atomic.Value // int
	DBQueryTime   atomic.Value // time.Duration
	DBQueryCount  atomic.Value // int64

	// Custom metrics
	CustomMetrics sync.Map // map[string]interface{}

	mu sync.RWMutex
}

// OperationProfile tracks performance profile for a specific operation
type OperationProfile struct {
	Name             string
	Count            int64
	TotalDuration    time.Duration
	MinDuration      time.Duration
	MaxDuration      time.Duration
	RecentDurations  []time.Duration
	ErrorCount       int64
	SuccessCount     int64
	LastExecuted     time.Time
	ConcurrentActive int64
	MaxConcurrent    int64
	ResourceUsage    *OperationResourceUsage
	mu               sync.RWMutex
}

// OperationResourceUsage tracks resource usage for operations
type OperationResourceUsage struct {
	CPUTime         time.Duration
	MemoryAllocated int64
	MemoryPeak      int64
	IOOperations    int64
	NetworkBytes    int64
}

// ResourceTracker monitors system resource usage trends
type ResourceTracker struct {
	history         []ResourceSnapshot
	alertThresholds map[string]float64
	leakDetection   *MemoryLeakDetector
	mu              sync.RWMutex
}

// ResourceSnapshot captures resource usage at a point in time
type ResourceSnapshot struct {
	Timestamp     time.Time
	CPU           float64
	Memory        uint64
	MemoryPercent float64
	Goroutines    int
	GCPauses      time.Duration
	OpenFDs       int
}

// MemoryLeakDetector detects potential memory leaks
type MemoryLeakDetector struct {
	baseline       uint64
	trendWindow    []uint64
	alertThreshold float64
	lastAlert      time.Time
	growthRate     float64
	mu             sync.RWMutex
}

// Profiler provides CPU and memory profiling capabilities
type Profiler struct {
	cpuProfiles    []CPUProfile
	memoryProfiles []MemoryProfile
	config         *ProfilerConfig
	mu             sync.RWMutex
}

// ProfilerConfig configures the profiler
type ProfilerConfig struct {
	CPUProfileDuration    time.Duration
	MemoryProfileInterval time.Duration
	MaxProfiles           int
	EnableCPUProfile      bool
	EnableMemProfile      bool
	EnableBlockProfile    bool
	EnableMutexProfile    bool
}

// CPUProfile represents a CPU profiling session
type CPUProfile struct {
	Timestamp    time.Time
	Duration     time.Duration
	SampleCount  int
	TopFunctions []FunctionProfile
}

// MemoryProfile represents a memory profiling session
type MemoryProfile struct {
	Timestamp     time.Time
	HeapSize      int64
	HeapObjects   int64
	StackSize     int64
	TopAllocators []AllocationProfile
}

// FunctionProfile represents CPU usage for a function
type FunctionProfile struct {
	Function   string
	File       string
	Line       int
	CPUTime    time.Duration
	Percentage float64
	Calls      int64
}

// AllocationProfile represents memory allocation for a function
type AllocationProfile struct {
	Function    string
	File        string
	Line        int
	Allocations int64
	Bytes       int64
	Percentage  float64
}

// PerformanceReport provides comprehensive performance analysis
type PerformanceReport struct {
	GeneratedAt   time.Time     `json:"generated_at"`
	MonitorUptime time.Duration `json:"monitor_uptime"`

	// System performance
	SystemHealth SystemHealthReport `json:"system_health"`

	// Application performance
	AppPerformance AppPerformanceReport `json:"app_performance"`

	// Operation analysis
	Operations []OperationReport `json:"operations"`

	// Resource analysis
	ResourceTrends ResourceTrendsReport `json:"resource_trends"`

	// Performance alerts
	Alerts []PerformanceAlert `json:"alerts"`

	// Recommendations
	Recommendations []PerformanceRecommendation `json:"recommendations"`

	// Profiling results
	ProfilingResults *ProfilingReport `json:"profiling_results,omitempty"`
}

// SystemHealthReport provides system health analysis
type SystemHealthReport struct {
	CPUHealth       HealthStatus `json:"cpu_health"`
	MemoryHealth    HealthStatus `json:"memory_health"`
	GCHealth        HealthStatus `json:"gc_health"`
	GoroutineHealth HealthStatus `json:"goroutine_health"`

	CPUUsage       float64       `json:"cpu_usage"`
	MemoryUsage    float64       `json:"memory_usage"`
	GoroutineCount int           `json:"goroutine_count"`
	GCPauseP95     time.Duration `json:"gc_pause_p95"`

	Score  float64 `json:"health_score"`
	Status string  `json:"status"`
}

// AppPerformanceReport provides application performance analysis
type AppPerformanceReport struct {
	RequestRate     float64       `json:"request_rate"`
	ResponseTimeP95 time.Duration `json:"response_time_p95"`
	ErrorRate       float64       `json:"error_rate"`
	Throughput      float64       `json:"throughput"`

	PerformanceScore float64 `json:"performance_score"`
	Status           string  `json:"status"`
}

// OperationReport provides detailed operation analysis
type OperationReport struct {
	Name          string        `json:"name"`
	Count         int64         `json:"count"`
	SuccessRate   float64       `json:"success_rate"`
	AvgDuration   time.Duration `json:"avg_duration"`
	P95Duration   time.Duration `json:"p95_duration"`
	MaxConcurrent int64         `json:"max_concurrent"`

	PerformanceIssues []string `json:"performance_issues"`
	Score             float64  `json:"performance_score"`
}

// ResourceTrendsReport provides resource usage trend analysis
type ResourceTrendsReport struct {
	CPUTrend       TrendDirection `json:"cpu_trend"`
	MemoryTrend    TrendDirection `json:"memory_trend"`
	GoroutineTrend TrendDirection `json:"goroutine_trend"`

	MemoryLeakRisk float64 `json:"memory_leak_risk"`
	ResourceScore  float64 `json:"resource_score"`
}

// TrendDirection represents the direction of a trend
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendStable     TrendDirection = "stable"
	TrendVolatile   TrendDirection = "volatile"
)

// HealthStatus represents health status
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"
	HealthWarning  HealthStatus = "warning"
	HealthCritical HealthStatus = "critical"
	HealthUnknown  HealthStatus = "unknown"
)

// PerformanceAlert represents a performance-related alert
type PerformanceAlert struct {
	Type      AlertType     `json:"type"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
	Component string        `json:"component"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
}

// AlertType categorizes performance alerts
type AlertType string

const (
	AlertCPU          AlertType = "cpu"
	AlertMemory       AlertType = "memory"
	AlertResponseTime AlertType = "response_time"
	AlertErrorRate    AlertType = "error_rate"
	AlertGoroutines   AlertType = "goroutines"
	AlertGC           AlertType = "gc"
	AlertMemoryLeak   AlertType = "memory_leak"
	AlertThroughput   AlertType = "throughput"
)

// AlertSeverity represents alert severity levels
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// PerformanceRecommendation provides actionable performance recommendations
type PerformanceRecommendation struct {
	Type        RecommendationType `json:"type"`
	Priority    Priority           `json:"priority"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Impact      string             `json:"impact"`
	Effort      string             `json:"effort"`
	Actions     []string           `json:"actions"`
	Metrics     map[string]float64 `json:"metrics"`
}

// RecommendationType categorizes recommendations
type RecommendationType string

const (
	RecommendationCPU          RecommendationType = "cpu_optimization"
	RecommendationMemory       RecommendationType = "memory_optimization"
	RecommendationGC           RecommendationType = "gc_tuning"
	RecommendationConcurrency  RecommendationType = "concurrency_optimization"
	RecommendationIO           RecommendationType = "io_optimization"
	RecommendationCaching      RecommendationType = "caching_optimization"
	RecommendationDatabase     RecommendationType = "database_optimization"
	RecommendationArchitecture RecommendationType = "architecture_optimization"
)

// Priority represents recommendation priority
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// ProfilingReport provides profiling analysis
type ProfilingReport struct {
	CPUProfile    *CPUProfileSummary    `json:"cpu_profile,omitempty"`
	MemoryProfile *MemoryProfileSummary `json:"memory_profile,omitempty"`
	HotSpots      []HotSpot             `json:"hot_spots"`
}

// CPUProfileSummary provides CPU profiling summary
type CPUProfileSummary struct {
	Duration     time.Duration     `json:"duration"`
	Samples      int               `json:"samples"`
	TopFunctions []FunctionProfile `json:"top_functions"`
	TotalCPUTime time.Duration     `json:"total_cpu_time"`
}

// MemoryProfileSummary provides memory profiling summary
type MemoryProfileSummary struct {
	HeapSize       int64               `json:"heap_size"`
	HeapObjects    int64               `json:"heap_objects"`
	TopAllocators  []AllocationProfile `json:"top_allocators"`
	TotalAllocated int64               `json:"total_allocated"`
}

// HotSpot represents a performance hot spot
type HotSpot struct {
	Type        string  `json:"type"`
	Function    string  `json:"function"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	Impact      float64 `json:"impact"`
	Description string  `json:"description"`
}

// DefaultMonitorConfig returns a default performance monitor configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		SystemMetricsInterval:     time.Second * 10,
		AppMetricsInterval:        time.Second * 5,
		ProfileInterval:           time.Minute * 5,
		OperationHistorySize:      1000,
		ProfileHistorySize:        10,
		CPUThreshold:              80.0,
		MemoryThreshold:           85.0,
		ResponseTimeP95:           time.Second * 2,
		ErrorRateThreshold:        5.0,
		EnableSystemMetrics:       true,
		EnableProfiling:           true,
		EnableGCMetrics:           true,
		EnableMemoryLeakDetection: true,
		ProfileSamplingRate:       0.1,
		MetricsSamplingRate:       1.0,
	}
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(observer observability.Observer, config *MonitorConfig) *PerformanceMonitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	pm := &PerformanceMonitor{
		observer:      observer,
		config:        config,
		systemMetrics: &SystemMetrics{CPUCores: runtime.NumCPU()},
		appMetrics:    &ApplicationMetrics{},
		resourceTracker: &ResourceTracker{
			alertThresholds: map[string]float64{
				"cpu":        config.CPUThreshold,
				"memory":     config.MemoryThreshold,
				"goroutines": 10000,
			},
			leakDetection: &MemoryLeakDetector{
				alertThreshold: 50.0, // 50% growth
			},
		},
		profiler: &Profiler{
			config: &ProfilerConfig{
				CPUProfileDuration:    time.Second * 30,
				MemoryProfileInterval: time.Minute * 5,
				MaxProfiles:           config.ProfileHistorySize,
				EnableCPUProfile:      config.EnableProfiling,
				EnableMemProfile:      config.EnableProfiling,
			},
		},
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}

	// Initialize atomic values
	pm.systemMetrics.CPUUsage.Store(0.0)
	pm.systemMetrics.MemoryUsed.Store(uint64(0))
	pm.systemMetrics.MemoryTotal.Store(uint64(0))
	pm.systemMetrics.MemoryAvailable.Store(uint64(0))
	pm.systemMetrics.MemoryPercent.Store(0.0)
	pm.systemMetrics.GCPauseTotal.Store(time.Duration(0))
	pm.systemMetrics.GCPauseCount.Store(uint64(0))
	pm.systemMetrics.GCFrequency.Store(0.0)
	pm.systemMetrics.GoroutineCount.Store(0)
	pm.systemMetrics.CGOCalls.Store(int64(0))
	pm.systemMetrics.OpenFDs.Store(0)
	pm.systemMetrics.MaxFDs.Store(0)

	pm.appMetrics.RequestCount.Store(int64(0))
	pm.appMetrics.RequestRate.Store(0.0)
	pm.appMetrics.ResponseTimeP50.Store(time.Duration(0))
	pm.appMetrics.ResponseTimeP95.Store(time.Duration(0))
	pm.appMetrics.ResponseTimeP99.Store(time.Duration(0))
	pm.appMetrics.ErrorCount.Store(int64(0))
	pm.appMetrics.ErrorRate.Store(0.0)
	pm.appMetrics.BytesProcessed.Store(int64(0))
	pm.appMetrics.ItemsProcessed.Store(int64(0))
	pm.appMetrics.ProcessingRate.Store(0.0)
	pm.appMetrics.CacheHitRate.Store(0.0)
	pm.appMetrics.CacheSize.Store(int64(0))
	pm.appMetrics.CacheEvictions.Store(int64(0))
	pm.appMetrics.DBConnections.Store(0)
	pm.appMetrics.DBQueryTime.Store(time.Duration(0))
	pm.appMetrics.DBQueryCount.Store(int64(0))

	return pm
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.ticker != nil {
		return // Already started
	}

	// Start metric collection
	pm.ticker = time.NewTicker(pm.config.SystemMetricsInterval)

	go pm.collectMetrics()

	pm.observer.Logger().Info("Performance monitor started",
		"system_metrics_interval", pm.config.SystemMetricsInterval,
		"app_metrics_interval", pm.config.AppMetricsInterval,
		"profile_interval", pm.config.ProfileInterval)
}

// Stop stops performance monitoring
func (pm *PerformanceMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.ticker == nil {
		return // Already stopped
	}

	pm.ticker.Stop()
	close(pm.stopCh)
	pm.ticker = nil

	pm.observer.Logger().Info("Performance monitor stopped")
}

// collectMetrics runs the main metrics collection loop
func (pm *PerformanceMonitor) collectMetrics() {
	systemTicker := time.NewTicker(pm.config.SystemMetricsInterval)
	appTicker := time.NewTicker(pm.config.AppMetricsInterval)
	profileTicker := time.NewTicker(pm.config.ProfileInterval)

	defer systemTicker.Stop()
	defer appTicker.Stop()
	defer profileTicker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-systemTicker.C:
			if pm.config.EnableSystemMetrics {
				pm.collectSystemMetrics()
			}
		case <-appTicker.C:
			pm.collectApplicationMetrics()
		case <-profileTicker.C:
			if pm.config.EnableProfiling {
				pm.collectProfileData()
			}
		}
	}
}

// collectSystemMetrics collects system-level performance metrics
func (pm *PerformanceMonitor) collectSystemMetrics() {
	// Collect runtime metrics
	var memStats runtime.MemStats
	runtime.GC() // Force GC to get accurate stats
	runtime.ReadMemStats(&memStats)

	// Update memory metrics
	pm.systemMetrics.MemoryUsed.Store(memStats.Alloc)
	pm.systemMetrics.MemoryTotal.Store(memStats.Sys)
	memPercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	pm.systemMetrics.MemoryPercent.Store(memPercent)

	// Update GC metrics
	gcPauses := make([]time.Duration, len(memStats.PauseNs))
	for i, pauseNs := range memStats.PauseNs {
		gcPauses[i] = time.Duration(pauseNs)
	}

	pm.systemMetrics.mu.Lock()
	pm.systemMetrics.GCPauses = gcPauses
	pm.systemMetrics.mu.Unlock()

	pm.systemMetrics.GCPauseCount.Store(uint64(memStats.NumGC))

	// Calculate GC frequency
	if memStats.NumGC > 0 {
		gcFreq := float64(memStats.NumGC) / time.Since(pm.startTime).Seconds()
		pm.systemMetrics.GCFrequency.Store(gcFreq)
	}

	// Update goroutine count
	goroutines := runtime.NumGoroutine()
	pm.systemMetrics.GoroutineCount.Store(goroutines)

	// Update CGO calls
	pm.systemMetrics.CGOCalls.Store(runtime.NumCgoCall())

	// Record resource snapshot for trend analysis
	snapshot := ResourceSnapshot{
		Timestamp:     time.Now(),
		Memory:        memStats.Alloc,
		MemoryPercent: memPercent,
		Goroutines:    goroutines,
		GCPauses:      time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256]),
	}

	pm.resourceTracker.mu.Lock()
	pm.resourceTracker.history = append(pm.resourceTracker.history, snapshot)
	// Keep only last 100 snapshots
	if len(pm.resourceTracker.history) > 100 {
		pm.resourceTracker.history = pm.resourceTracker.history[1:]
	}
	pm.resourceTracker.mu.Unlock()

	// Check for memory leaks
	if pm.config.EnableMemoryLeakDetection {
		pm.detectMemoryLeaks(memStats.Alloc)
	}

	// Record metrics with observer
	pm.observer.SetGauge("system_memory_used", float64(memStats.Alloc), map[string]string{"component": "system"})
	pm.observer.SetGauge("system_memory_percent", memPercent, map[string]string{"component": "system"})
	pm.observer.SetGauge("system_goroutines", float64(goroutines), map[string]string{"component": "system"})
	pm.observer.SetGauge("system_gc_frequency", pm.systemMetrics.GCFrequency.Load().(float64), map[string]string{"component": "system"})

	// Check thresholds and generate alerts
	pm.checkSystemThresholds(memPercent, float64(goroutines))
}

// collectApplicationMetrics collects application-level performance metrics
func (pm *PerformanceMonitor) collectApplicationMetrics() {
	// Calculate request rate from operation stats
	var totalRequests int64
	var totalSuccessful int64
	var totalDuration time.Duration
	var durationCount int64
	var responseTimes []time.Duration

	pm.operations.Range(func(key, value interface{}) bool {
		if stats, ok := value.(*OperationProfile); ok {
			stats.mu.RLock()
			totalRequests += stats.Count
			totalSuccessful += stats.SuccessCount
			if stats.Count > 0 {
				avgDuration := stats.TotalDuration / time.Duration(stats.Count)
				responseTimes = append(responseTimes, avgDuration)
				totalDuration += stats.TotalDuration
				durationCount += stats.Count
			}
			stats.mu.RUnlock()
		}
		return true
	})

	// Calculate request rate (requests per second)
	uptime := time.Since(pm.startTime)
	requestRate := float64(totalRequests) / uptime.Seconds()
	pm.appMetrics.RequestRate.Store(requestRate)
	pm.appMetrics.RequestCount.Store(totalRequests)

	// Calculate error rate
	var errorRate float64
	if totalRequests > 0 {
		errorRate = float64(totalRequests-totalSuccessful) / float64(totalRequests)
	}
	pm.appMetrics.ErrorRate.Store(errorRate)
	pm.appMetrics.ErrorCount.Store(totalRequests - totalSuccessful)

	// Calculate response time percentiles
	if len(responseTimes) > 0 {
		// Sort response times for percentile calculation
		for i := 0; i < len(responseTimes)-1; i++ {
			for j := i + 1; j < len(responseTimes); j++ {
				if responseTimes[i] > responseTimes[j] {
					responseTimes[i], responseTimes[j] = responseTimes[j], responseTimes[i]
				}
			}
		}

		// Calculate percentiles
		p50Index := len(responseTimes) * 50 / 100
		p95Index := len(responseTimes) * 95 / 100
		p99Index := len(responseTimes) * 99 / 100

		if p50Index < len(responseTimes) {
			pm.appMetrics.ResponseTimeP50.Store(responseTimes[p50Index])
		}
		if p95Index < len(responseTimes) {
			pm.appMetrics.ResponseTimeP95.Store(responseTimes[p95Index])
		}
		if p99Index < len(responseTimes) {
			pm.appMetrics.ResponseTimeP99.Store(responseTimes[p99Index])
		}
	}

	// Calculate processing rate
	var itemsProcessed int64
	if durationCount > 0 {
		itemsProcessed = durationCount
	}
	processingRate := float64(itemsProcessed) / uptime.Seconds()
	pm.appMetrics.ProcessingRate.Store(processingRate)
	pm.appMetrics.ItemsProcessed.Store(itemsProcessed)

	// Record metrics with observer
	pm.observer.SetGauge("app_request_rate", requestRate, map[string]string{"component": "application"})
	pm.observer.SetGauge("app_error_rate", errorRate*100, map[string]string{"component": "application"})
	if len(responseTimes) > 0 {
		p95 := pm.appMetrics.ResponseTimeP95.Load().(time.Duration)
		pm.observer.SetGauge("app_response_time_p95", float64(p95.Milliseconds()), map[string]string{"component": "application"})
	}
}

// detectMemoryLeaks detects potential memory leaks
func (pm *PerformanceMonitor) detectMemoryLeaks(currentMemory uint64) {
	pm.resourceTracker.leakDetection.mu.Lock()
	defer pm.resourceTracker.leakDetection.mu.Unlock()

	detector := pm.resourceTracker.leakDetection

	// Initialize baseline if not set
	if detector.baseline == 0 {
		detector.baseline = currentMemory
		return
	}

	// Add to trend window
	detector.trendWindow = append(detector.trendWindow, currentMemory)
	// Keep only last 20 measurements
	if len(detector.trendWindow) > 20 {
		detector.trendWindow = detector.trendWindow[1:]
	}

	// Calculate growth rate if we have enough data
	if len(detector.trendWindow) >= 10 {
		// Calculate linear regression to determine growth trend
		n := float64(len(detector.trendWindow))
		var sumX, sumY, sumXY, sumX2 float64

		for i, memory := range detector.trendWindow {
			x := float64(i)
			y := float64(memory)
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}

		// Calculate slope (growth rate)
		slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
		detector.growthRate = slope

		// Calculate percentage growth from baseline
		growthPercent := (float64(currentMemory) - float64(detector.baseline)) / float64(detector.baseline) * 100

		// Check if growth exceeds threshold
		if growthPercent > detector.alertThreshold {
			// Only alert if enough time has passed since last alert
			if time.Since(detector.lastAlert) > time.Minute*5 {
				detector.lastAlert = time.Now()

				// Record memory leak alert
				pm.observer.IncrementCounter("memory_leak_alerts", map[string]string{
					"component": "system",
					"severity":  "warning",
				})

				pm.observer.Logger().Warn("Potential memory leak detected",
					"current_memory", currentMemory,
					"baseline_memory", detector.baseline,
					"growth_percent", growthPercent,
					"growth_rate", detector.growthRate)
			}
		}
	}
}

// checkSystemThresholds checks system metrics against thresholds and generates alerts
func (pm *PerformanceMonitor) checkSystemThresholds(memoryPercent, goroutines float64) {
	// Check memory threshold
	if memoryPercent > pm.config.MemoryThreshold {
		pm.observer.IncrementCounter("threshold_alerts", map[string]string{
			"type":      "memory",
			"severity":  pm.getSeverity(memoryPercent, pm.config.MemoryThreshold),
			"component": "system",
		})
	}

	// Check goroutine threshold
	goroutineThreshold := pm.resourceTracker.alertThresholds["goroutines"]
	if goroutines > goroutineThreshold {
		pm.observer.IncrementCounter("threshold_alerts", map[string]string{
			"type":      "goroutines",
			"severity":  pm.getSeverity(goroutines, goroutineThreshold),
			"component": "system",
		})
	}

	// Check response time threshold
	p95ResponseTime := pm.appMetrics.ResponseTimeP95.Load().(time.Duration)
	if p95ResponseTime > pm.config.ResponseTimeP95 {
		pm.observer.IncrementCounter("threshold_alerts", map[string]string{
			"type":      "response_time",
			"severity":  "warning",
			"component": "application",
		})
	}

	// Check error rate threshold
	errorRate := pm.appMetrics.ErrorRate.Load().(float64) * 100
	if errorRate > pm.config.ErrorRateThreshold {
		pm.observer.IncrementCounter("threshold_alerts", map[string]string{
			"type":      "error_rate",
			"severity":  pm.getSeverity(errorRate, pm.config.ErrorRateThreshold),
			"component": "application",
		})
	}
}

// getSeverity determines alert severity based on threshold breach
func (pm *PerformanceMonitor) getSeverity(value, threshold float64) string {
	if value > threshold*1.5 {
		return "critical"
	} else if value > threshold*1.2 {
		return "warning"
	}
	return "info"
}

// collectProfileData collects CPU and memory profiling data
func (pm *PerformanceMonitor) collectProfileData() {
	if !pm.config.EnableProfiling {
		return
	}

	// Collect CPU profile
	if pm.profiler.config.EnableCPUProfile {
		pm.collectCPUProfile()
	}

	// Collect memory profile
	if pm.profiler.config.EnableMemProfile {
		pm.collectMemoryProfile()
	}
}

// collectCPUProfile collects CPU profiling data
func (pm *PerformanceMonitor) collectCPUProfile() {
	// This would integrate with Go's pprof package to collect CPU profiles
	// For now, we'll create a simple representation
	profile := CPUProfile{
		Timestamp:   time.Now(),
		Duration:    pm.profiler.config.CPUProfileDuration,
		SampleCount: runtime.NumGoroutine() * 10, // Simulated sample count
		TopFunctions: []FunctionProfile{
			{
				Function:   "runtime.schedule",
				File:       "runtime/proc.go",
				Line:       2500,
				CPUTime:    time.Millisecond * 100,
				Percentage: 15.5,
				Calls:      1000,
			},
			// Additional functions would be collected from pprof
		},
	}

	pm.profiler.mu.Lock()
	pm.profiler.cpuProfiles = append(pm.profiler.cpuProfiles, profile)
	// Keep only recent profiles
	if len(pm.profiler.cpuProfiles) > pm.profiler.config.MaxProfiles {
		pm.profiler.cpuProfiles = pm.profiler.cpuProfiles[1:]
	}
	pm.profiler.mu.Unlock()

	// Record profiling metrics
	pm.observer.SetGauge("cpu_profile_samples", float64(profile.SampleCount), map[string]string{"component": "profiler"})
}

// collectMemoryProfile collects memory profiling data
func (pm *PerformanceMonitor) collectMemoryProfile() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	profile := MemoryProfile{
		Timestamp:   time.Now(),
		HeapSize:    int64(memStats.HeapAlloc),
		HeapObjects: int64(memStats.HeapObjects),
		StackSize:   int64(memStats.StackInuse),
		TopAllocators: []AllocationProfile{
			{
				Function:    "container-kit/pkg/mcp/workflow.Execute",
				File:        "workflow/execute.go",
				Line:        150,
				Allocations: 500,
				Bytes:       int64(memStats.HeapAlloc) / 4,
				Percentage:  25.0,
			},
			// Additional allocators would be collected from pprof
		},
	}

	pm.profiler.mu.Lock()
	pm.profiler.memoryProfiles = append(pm.profiler.memoryProfiles, profile)
	// Keep only recent profiles
	if len(pm.profiler.memoryProfiles) > pm.profiler.config.MaxProfiles {
		pm.profiler.memoryProfiles = pm.profiler.memoryProfiles[1:]
	}
	pm.profiler.mu.Unlock()

	// Record profiling metrics
	pm.observer.SetGauge("memory_profile_heap_size", float64(profile.HeapSize), map[string]string{"component": "profiler"})
	pm.observer.SetGauge("memory_profile_heap_objects", float64(profile.HeapObjects), map[string]string{"component": "profiler"})
}

// StartOperation starts tracking an operation
func (pm *PerformanceMonitor) StartOperation(operationName string) *OperationTracker {
	return &OperationTracker{
		name:      operationName,
		startTime: time.Now(),
		monitor:   pm,
	}
}

// OperationTracker tracks individual operation performance
type OperationTracker struct {
	name      string
	startTime time.Time
	monitor   *PerformanceMonitor
	resource  *OperationResourceUsage
}

// EndOperation completes operation tracking
func (ot *OperationTracker) EndOperation(success bool) {
	duration := time.Since(ot.startTime)

	// Get or create operation profile
	key := ot.name
	if existing, loaded := ot.monitor.operations.LoadOrStore(key, &OperationProfile{
		Name:             ot.name,
		Count:            1,
		TotalDuration:    duration,
		MinDuration:      duration,
		MaxDuration:      duration,
		RecentDurations:  []time.Duration{duration},
		ErrorCount:       0,
		SuccessCount:     0,
		LastExecuted:     time.Now(),
		ConcurrentActive: 0,
		MaxConcurrent:    1,
		ResourceUsage:    ot.resource,
	}); loaded {
		if profile, ok := existing.(*OperationProfile); ok {
			profile.mu.Lock()
			profile.Count++
			profile.TotalDuration += duration
			if duration < profile.MinDuration {
				profile.MinDuration = duration
			}
			if duration > profile.MaxDuration {
				profile.MaxDuration = duration
			}

			// Update recent durations (keep last 100)
			profile.RecentDurations = append(profile.RecentDurations, duration)
			if len(profile.RecentDurations) > 100 {
				profile.RecentDurations = profile.RecentDurations[1:]
			}

			if success {
				profile.SuccessCount++
			} else {
				profile.ErrorCount++
			}

			profile.LastExecuted = time.Now()
			atomic.AddInt64(&profile.ConcurrentActive, -1)
			profile.mu.Unlock()
		}
	} else {
		// First time - set success/error count
		if profile, ok := existing.(*OperationProfile); ok {
			if success {
				profile.SuccessCount = 1
			} else {
				profile.ErrorCount = 1
			}
		}
	}

	// Record operation metrics
	ot.monitor.observer.RecordHistogram("operation_duration", float64(duration.Milliseconds()), map[string]string{
		"operation": ot.name,
		"success":   fmt.Sprintf("%v", success),
	})
}

// GetPerformanceReport generates a comprehensive performance report
func (pm *PerformanceMonitor) GetPerformanceReport() *PerformanceReport {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	uptime := now.Sub(pm.startTime)

	report := &PerformanceReport{
		GeneratedAt:     now,
		MonitorUptime:   uptime,
		SystemHealth:    pm.generateSystemHealthReport(),
		AppPerformance:  pm.generateAppPerformanceReport(),
		Operations:      pm.generateOperationReports(),
		ResourceTrends:  pm.generateResourceTrendsReport(),
		Alerts:          pm.generatePerformanceAlerts(),
		Recommendations: pm.generatePerformanceRecommendations(),
	}

	// Add profiling results if enabled
	if pm.config.EnableProfiling {
		report.ProfilingResults = pm.generateProfilingReport()
	}

	return report
}

// generateSystemHealthReport generates system health analysis
func (pm *PerformanceMonitor) generateSystemHealthReport() SystemHealthReport {
	cpuUsage := pm.systemMetrics.CPUUsage.Load().(float64)
	memoryUsage := pm.systemMetrics.MemoryPercent.Load().(float64)
	goroutineCount := pm.systemMetrics.GoroutineCount.Load().(int)
	gcFreq := pm.systemMetrics.GCFrequency.Load().(float64)

	// Calculate health scores
	cpuHealth := pm.calculateHealthStatus(cpuUsage, pm.config.CPUThreshold)
	memoryHealth := pm.calculateHealthStatus(memoryUsage, pm.config.MemoryThreshold)
	gcHealth := pm.calculateGCHealth(gcFreq)
	goroutineHealth := pm.calculateGoroutineHealth(float64(goroutineCount))

	// Calculate overall health score (0-100)
	healthScore := (pm.healthStatusToScore(cpuHealth) +
		pm.healthStatusToScore(memoryHealth) +
		pm.healthStatusToScore(gcHealth) +
		pm.healthStatusToScore(goroutineHealth)) / 4.0

	// Determine overall status
	status := "healthy"
	if healthScore < 50 {
		status = "critical"
	} else if healthScore < 75 {
		status = "warning"
	}

	// Get P95 GC pause time
	var gcPauseP95 time.Duration
	pm.systemMetrics.mu.RLock()
	if len(pm.systemMetrics.GCPauses) > 0 {
		// Simple P95 calculation
		pausesCopy := make([]time.Duration, len(pm.systemMetrics.GCPauses))
		copy(pausesCopy, pm.systemMetrics.GCPauses)
		// Sort pauses
		for i := 0; i < len(pausesCopy)-1; i++ {
			for j := i + 1; j < len(pausesCopy); j++ {
				if pausesCopy[i] > pausesCopy[j] {
					pausesCopy[i], pausesCopy[j] = pausesCopy[j], pausesCopy[i]
				}
			}
		}
		p95Index := len(pausesCopy) * 95 / 100
		if p95Index < len(pausesCopy) {
			gcPauseP95 = pausesCopy[p95Index]
		}
	}
	pm.systemMetrics.mu.RUnlock()

	return SystemHealthReport{
		CPUHealth:       cpuHealth,
		MemoryHealth:    memoryHealth,
		GCHealth:        gcHealth,
		GoroutineHealth: goroutineHealth,
		CPUUsage:        cpuUsage,
		MemoryUsage:     memoryUsage,
		GoroutineCount:  goroutineCount,
		GCPauseP95:      gcPauseP95,
		Score:           healthScore,
		Status:          status,
	}
}

// generateAppPerformanceReport generates application performance analysis
func (pm *PerformanceMonitor) generateAppPerformanceReport() AppPerformanceReport {
	requestRate := pm.appMetrics.RequestRate.Load().(float64)
	responseTimeP95 := pm.appMetrics.ResponseTimeP95.Load().(time.Duration)
	errorRate := pm.appMetrics.ErrorRate.Load().(float64)
	processingRate := pm.appMetrics.ProcessingRate.Load().(float64)

	// Calculate performance score (0-100)
	performanceScore := 100.0

	// Penalize high error rate
	if errorRate > 0.05 { // 5% threshold
		performanceScore -= (errorRate * 100) * 2 // Double penalty for errors
	}

	// Penalize slow response times
	if responseTimeP95 > pm.config.ResponseTimeP95 {
		penalty := float64(responseTimeP95-pm.config.ResponseTimeP95) / float64(pm.config.ResponseTimeP95) * 50
		performanceScore -= penalty
	}

	if performanceScore < 0 {
		performanceScore = 0
	}

	// Determine status
	status := "excellent"
	if performanceScore < 50 {
		status = "poor"
	} else if performanceScore < 75 {
		status = "fair"
	} else if performanceScore < 90 {
		status = "good"
	}

	return AppPerformanceReport{
		RequestRate:      requestRate,
		ResponseTimeP95:  responseTimeP95,
		ErrorRate:        errorRate * 100, // Convert to percentage
		Throughput:       processingRate,
		PerformanceScore: performanceScore,
		Status:           status,
	}
}

// generateOperationReports generates detailed operation analysis
func (pm *PerformanceMonitor) generateOperationReports() []OperationReport {
	var reports []OperationReport

	pm.operations.Range(func(key, value interface{}) bool {
		if name, ok := key.(string); ok {
			if profile, ok := value.(*OperationProfile); ok {
				profile.mu.RLock()

				// Calculate success rate
				var successRate float64
				if profile.Count > 0 {
					successRate = float64(profile.SuccessCount) / float64(profile.Count)
				}

				// Calculate average duration
				var avgDuration time.Duration
				if profile.Count > 0 {
					avgDuration = profile.TotalDuration / time.Duration(profile.Count)
				}

				// Calculate P95 duration from recent durations
				var p95Duration time.Duration
				if len(profile.RecentDurations) > 0 {
					// Sort recent durations
					sortedDurations := make([]time.Duration, len(profile.RecentDurations))
					copy(sortedDurations, profile.RecentDurations)
					for i := 0; i < len(sortedDurations)-1; i++ {
						for j := i + 1; j < len(sortedDurations); j++ {
							if sortedDurations[i] > sortedDurations[j] {
								sortedDurations[i], sortedDurations[j] = sortedDurations[j], sortedDurations[i]
							}
						}
					}
					p95Index := len(sortedDurations) * 95 / 100
					if p95Index < len(sortedDurations) {
						p95Duration = sortedDurations[p95Index]
					}
				}

				// Identify performance issues
				var issues []string
				if successRate < 0.95 {
					issues = append(issues, "Low success rate")
				}
				if avgDuration > time.Second*2 {
					issues = append(issues, "High average response time")
				}
				if profile.MaxConcurrent > 100 {
					issues = append(issues, "High concurrency stress")
				}

				// Calculate performance score
				score := successRate * 100
				if avgDuration > time.Millisecond*500 {
					score -= 20 // Penalty for slow operations
				}

				reports = append(reports, OperationReport{
					Name:              name,
					Count:             profile.Count,
					SuccessRate:       successRate,
					AvgDuration:       avgDuration,
					P95Duration:       p95Duration,
					MaxConcurrent:     profile.MaxConcurrent,
					PerformanceIssues: issues,
					Score:             score,
				})

				profile.mu.RUnlock()
			}
		}
		return true
	})

	return reports
}

// generateResourceTrendsReport generates resource usage trend analysis
func (pm *PerformanceMonitor) generateResourceTrendsReport() ResourceTrendsReport {
	pm.resourceTracker.mu.RLock()
	defer pm.resourceTracker.mu.RUnlock()

	// Analyze trends from resource history
	cpuTrend := pm.analyzeTrend("cpu")
	memoryTrend := pm.analyzeTrend("memory")
	goroutineTrend := pm.analyzeTrend("goroutines")

	// Calculate memory leak risk
	memoryLeakRisk := 0.0
	if pm.resourceTracker.leakDetection.growthRate > 0 {
		memoryLeakRisk = pm.resourceTracker.leakDetection.growthRate * 10 // Scale to percentage
		if memoryLeakRisk > 100 {
			memoryLeakRisk = 100
		}
	}

	// Calculate resource score
	resourceScore := 100.0
	if cpuTrend == TrendIncreasing {
		resourceScore -= 15
	}
	if memoryTrend == TrendIncreasing {
		resourceScore -= 20
	}
	if goroutineTrend == TrendIncreasing {
		resourceScore -= 10
	}
	resourceScore -= memoryLeakRisk * 0.5

	if resourceScore < 0 {
		resourceScore = 0
	}

	return ResourceTrendsReport{
		CPUTrend:       cpuTrend,
		MemoryTrend:    memoryTrend,
		GoroutineTrend: goroutineTrend,
		MemoryLeakRisk: memoryLeakRisk,
		ResourceScore:  resourceScore,
	}
}

// generatePerformanceAlerts generates performance-related alerts
func (pm *PerformanceMonitor) generatePerformanceAlerts() []PerformanceAlert {
	var alerts []PerformanceAlert
	now := time.Now()

	// Check CPU usage
	cpuUsage := pm.systemMetrics.CPUUsage.Load().(float64)
	if cpuUsage > pm.config.CPUThreshold {
		alerts = append(alerts, PerformanceAlert{
			Type:      AlertCPU,
			Severity:  pm.getAlertSeverity(cpuUsage, pm.config.CPUThreshold),
			Message:   fmt.Sprintf("CPU usage is %.1f%%, exceeding threshold of %.1f%%", cpuUsage, pm.config.CPUThreshold),
			Value:     cpuUsage,
			Threshold: pm.config.CPUThreshold,
			Component: "system",
			Timestamp: now,
			Resolved:  false,
		})
	}

	// Check memory usage
	memoryPercent := pm.systemMetrics.MemoryPercent.Load().(float64)
	if memoryPercent > pm.config.MemoryThreshold {
		alerts = append(alerts, PerformanceAlert{
			Type:      AlertMemory,
			Severity:  pm.getAlertSeverity(memoryPercent, pm.config.MemoryThreshold),
			Message:   fmt.Sprintf("Memory usage is %.1f%%, exceeding threshold of %.1f%%", memoryPercent, pm.config.MemoryThreshold),
			Value:     memoryPercent,
			Threshold: pm.config.MemoryThreshold,
			Component: "system",
			Timestamp: now,
			Resolved:  false,
		})
	}

	// Check response time
	responseTimeP95 := pm.appMetrics.ResponseTimeP95.Load().(time.Duration)
	if responseTimeP95 > pm.config.ResponseTimeP95 {
		alerts = append(alerts, PerformanceAlert{
			Type:      AlertResponseTime,
			Severity:  AlertWarning,
			Message:   fmt.Sprintf("P95 response time is %v, exceeding threshold of %v", responseTimeP95, pm.config.ResponseTimeP95),
			Value:     float64(responseTimeP95.Milliseconds()),
			Threshold: float64(pm.config.ResponseTimeP95.Milliseconds()),
			Component: "application",
			Timestamp: now,
			Resolved:  false,
		})
	}

	// Check error rate
	errorRate := pm.appMetrics.ErrorRate.Load().(float64) * 100
	if errorRate > pm.config.ErrorRateThreshold {
		alerts = append(alerts, PerformanceAlert{
			Type:      AlertErrorRate,
			Severity:  pm.getAlertSeverity(errorRate, pm.config.ErrorRateThreshold),
			Message:   fmt.Sprintf("Error rate is %.1f%%, exceeding threshold of %.1f%%", errorRate, pm.config.ErrorRateThreshold),
			Value:     errorRate,
			Threshold: pm.config.ErrorRateThreshold,
			Component: "application",
			Timestamp: now,
			Resolved:  false,
		})
	}

	return alerts
}

// generatePerformanceRecommendations generates actionable performance recommendations
func (pm *PerformanceMonitor) generatePerformanceRecommendations() []PerformanceRecommendation {
	var recommendations []PerformanceRecommendation

	// Analyze system metrics for recommendations
	cpuUsage := pm.systemMetrics.CPUUsage.Load().(float64)
	memoryPercent := pm.systemMetrics.MemoryPercent.Load().(float64)
	goroutineCount := pm.systemMetrics.GoroutineCount.Load().(int)
	gcFreq := pm.systemMetrics.GCFrequency.Load().(float64)
	errorRate := pm.appMetrics.ErrorRate.Load().(float64)

	// CPU optimization recommendations
	if cpuUsage > 70 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:        RecommendationCPU,
			Priority:    PriorityHigh,
			Title:       "High CPU Usage Detected",
			Description: fmt.Sprintf("CPU usage is %.1f%%, consider optimization", cpuUsage),
			Impact:      "Improved response times and system stability",
			Effort:      "Medium",
			Actions: []string{
				"Profile CPU usage to identify hot spots",
				"Optimize algorithms in frequently called functions",
				"Consider adding CPU limits to containers",
				"Implement request throttling",
			},
			Metrics: map[string]float64{
				"current_cpu_usage": cpuUsage,
				"threshold":         pm.config.CPUThreshold,
			},
		})
	}

	// Memory optimization recommendations
	if memoryPercent > 70 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:        RecommendationMemory,
			Priority:    PriorityHigh,
			Title:       "High Memory Usage Detected",
			Description: fmt.Sprintf("Memory usage is %.1f%%, consider optimization", memoryPercent),
			Impact:      "Reduced memory pressure and potential OOM issues",
			Effort:      "Medium",
			Actions: []string{
				"Review memory allocations and implement pooling",
				"Optimize data structures and reduce memory footprint",
				"Implement memory limits and monitoring",
				"Check for memory leaks",
			},
			Metrics: map[string]float64{
				"current_memory_usage": memoryPercent,
				"threshold":            pm.config.MemoryThreshold,
			},
		})
	}

	// GC optimization recommendations
	if gcFreq > 10 { // More than 10 GCs per second
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:        RecommendationGC,
			Priority:    PriorityMedium,
			Title:       "High GC Frequency Detected",
			Description: fmt.Sprintf("GC frequency is %.1f per second, consider tuning", gcFreq),
			Impact:      "Reduced GC overhead and improved performance",
			Effort:      "Low",
			Actions: []string{
				"Tune GOGC environment variable",
				"Reduce allocation rate",
				"Implement object pooling for frequently allocated objects",
				"Consider increasing heap size",
			},
			Metrics: map[string]float64{
				"gc_frequency": gcFreq,
				"recommended":  5.0,
			},
		})
	}

	// Concurrency optimization recommendations
	if goroutineCount > 5000 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:        RecommendationConcurrency,
			Priority:    PriorityMedium,
			Title:       "High Goroutine Count Detected",
			Description: fmt.Sprintf("Running %d goroutines, consider optimization", goroutineCount),
			Impact:      "Reduced memory usage and improved scheduling",
			Effort:      "Medium",
			Actions: []string{
				"Review goroutine lifecycle management",
				"Implement worker pools instead of unlimited goroutines",
				"Add proper context cancellation",
				"Monitor for goroutine leaks",
			},
			Metrics: map[string]float64{
				"goroutine_count": float64(goroutineCount),
				"recommended":     1000.0,
			},
		})
	}

	// Error rate optimization recommendations
	if errorRate > 0.01 { // More than 1% error rate
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:        RecommendationIO,
			Priority:    PriorityHigh,
			Title:       "High Error Rate Detected",
			Description: fmt.Sprintf("Error rate is %.2f%%, investigate and fix", errorRate*100),
			Impact:      "Improved reliability and user experience",
			Effort:      "High",
			Actions: []string{
				"Analyze error patterns and root causes",
				"Implement better error handling and retry logic",
				"Add circuit breakers for external dependencies",
				"Improve input validation and error messages",
			},
			Metrics: map[string]float64{
				"error_rate": errorRate * 100,
				"threshold":  pm.config.ErrorRateThreshold,
			},
		})
	}

	return recommendations
}

// generateProfilingReport generates profiling analysis
func (pm *PerformanceMonitor) generateProfilingReport() *ProfilingReport {
	pm.profiler.mu.RLock()
	defer pm.profiler.mu.RUnlock()

	report := &ProfilingReport{
		HotSpots: []HotSpot{},
	}

	// Add CPU profile summary if available
	if len(pm.profiler.cpuProfiles) > 0 {
		latest := pm.profiler.cpuProfiles[len(pm.profiler.cpuProfiles)-1]
		report.CPUProfile = &CPUProfileSummary{
			Duration:     latest.Duration,
			Samples:      latest.SampleCount,
			TopFunctions: latest.TopFunctions,
			TotalCPUTime: latest.Duration, // Simplified
		}

		// Generate hot spots from CPU profile
		for _, fn := range latest.TopFunctions {
			if fn.Percentage > 5.0 { // Functions using >5% CPU
				report.HotSpots = append(report.HotSpots, HotSpot{
					Type:        "cpu",
					Function:    fn.Function,
					File:        fn.File,
					Line:        fn.Line,
					Impact:      fn.Percentage,
					Description: fmt.Sprintf("Function consuming %.1f%% of CPU time", fn.Percentage),
				})
			}
		}
	}

	// Add memory profile summary if available
	if len(pm.profiler.memoryProfiles) > 0 {
		latest := pm.profiler.memoryProfiles[len(pm.profiler.memoryProfiles)-1]
		report.MemoryProfile = &MemoryProfileSummary{
			HeapSize:       latest.HeapSize,
			HeapObjects:    latest.HeapObjects,
			TopAllocators:  latest.TopAllocators,
			TotalAllocated: latest.HeapSize, // Simplified
		}

		// Generate hot spots from memory profile
		for _, alloc := range latest.TopAllocators {
			if alloc.Percentage > 10.0 { // Functions allocating >10% of memory
				report.HotSpots = append(report.HotSpots, HotSpot{
					Type:        "memory",
					Function:    alloc.Function,
					File:        alloc.File,
					Line:        alloc.Line,
					Impact:      alloc.Percentage,
					Description: fmt.Sprintf("Function allocating %.1f%% of memory", alloc.Percentage),
				})
			}
		}
	}

	return report
}

// Helper methods

func (pm *PerformanceMonitor) calculateHealthStatus(value, threshold float64) HealthStatus {
	if value > threshold*1.5 {
		return HealthCritical
	} else if value > threshold {
		return HealthWarning
	}
	return HealthHealthy
}

func (pm *PerformanceMonitor) calculateGCHealth(gcFreq float64) HealthStatus {
	if gcFreq > 20 {
		return HealthCritical
	} else if gcFreq > 10 {
		return HealthWarning
	}
	return HealthHealthy
}

func (pm *PerformanceMonitor) calculateGoroutineHealth(goroutines float64) HealthStatus {
	if goroutines > 10000 {
		return HealthCritical
	} else if goroutines > 5000 {
		return HealthWarning
	}
	return HealthHealthy
}

func (pm *PerformanceMonitor) healthStatusToScore(status HealthStatus) float64 {
	switch status {
	case HealthHealthy:
		return 100
	case HealthWarning:
		return 60
	case HealthCritical:
		return 20
	default:
		return 50
	}
}

func (pm *PerformanceMonitor) getAlertSeverity(value, threshold float64) AlertSeverity {
	if value > threshold*1.5 {
		return AlertCritical
	} else if value > threshold*1.2 {
		return AlertWarning
	}
	return AlertInfo
}

func (pm *PerformanceMonitor) analyzeTrend(metricType string) TrendDirection {
	// Simple trend analysis based on resource history
	pm.resourceTracker.mu.RLock()
	defer pm.resourceTracker.mu.RUnlock()

	if len(pm.resourceTracker.history) < 5 {
		return TrendStable
	}

	// Get recent snapshots
	recentSnapshots := pm.resourceTracker.history
	if len(recentSnapshots) > 20 {
		recentSnapshots = recentSnapshots[len(recentSnapshots)-20:]
	}

	// Calculate trend based on metric type
	var values []float64
	for _, snapshot := range recentSnapshots {
		switch metricType {
		case "cpu":
			values = append(values, snapshot.CPU)
		case "memory":
			values = append(values, snapshot.MemoryPercent)
		case "goroutines":
			values = append(values, float64(snapshot.Goroutines))
		}
	}

	if len(values) < 5 {
		return TrendStable
	}

	// Simple slope calculation
	first := values[0]
	last := values[len(values)-1]
	middle := values[len(values)/2]

	// Determine trend
	if last > first*1.1 && last > middle*1.05 {
		return TrendIncreasing
	} else if last < first*0.9 && last < middle*0.95 {
		return TrendDecreasing
	} else {
		// Check for volatility
		var variance float64
		mean := (first + last + middle) / 3
		for _, v := range values {
			variance += (v - mean) * (v - mean)
		}
		variance /= float64(len(values))

		if variance > mean*0.2 {
			return TrendVolatile
		}
	}

	return TrendStable
}
