// Package observability provides the unified observer implementation
package observability

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metric name constants
const (
	MetricErrorsTotal       = "errors_total"
	MetricEventsTotal       = "events_total"
	MetricHealthCheck       = "health_check"
	MetricOperationDuration = "operation_duration"
	MetricResourceUsage     = "resource_usage"
)

// ObserverImpl implements the Observer interface with comprehensive observability
type ObserverImpl struct {
	logger *slog.Logger
	config *ObserverConfig

	// Event tracking
	events     sync.Map // map[string]*Event
	eventCount int64

	// Metrics storage
	counters   sync.Map // map[string]*CounterMetric
	gauges     sync.Map // map[string]*GaugeMetric
	histograms sync.Map // map[string]*HistogramMetric

	// Health monitoring
	healthChecks sync.Map // map[string]*ComponentHealth

	// Resource monitoring
	resourceUsage sync.Map // map[string]*ResourceUsage

	// Performance tracking
	operations sync.Map // map[string]*OperationStats

	// Configuration
	samplingRate atomic.Value // float64
	logLevel     atomic.Value // slog.Level

	// Lifecycle
	startTime     time.Time
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}
}

// ObserverConfig provides configuration for the unified observer
type ObserverConfig struct {
	SamplingRate       float64
	MaxEvents          int
	MaxErrors          int
	RetentionPeriod    time.Duration
	MetricsEnabled     bool
	TracingEnabled     bool
	HealthCheckEnabled bool
	LogLevel           slog.Level
	FlushInterval      time.Duration
}

// CounterMetric represents a counter metric
type CounterMetric struct {
	Name  string
	Value int64
	Tags  map[string]string
	mu    sync.RWMutex
}

// GaugeMetric represents a gauge metric
type GaugeMetric struct {
	Name  string
	Value float64
	Tags  map[string]string
	mu    sync.RWMutex
}

// HistogramMetric represents a histogram metric with percentiles
type HistogramMetric struct {
	Name   string
	Values []float64
	Tags   map[string]string
	mu     sync.RWMutex
}

// OperationStats tracks statistics for operations
type OperationStats struct {
	Name          string
	Count         int64
	SuccessCount  int64
	TotalDuration time.Duration
	MinDuration   time.Duration
	MaxDuration   time.Duration
	LastExecuted  time.Time
	mu            sync.RWMutex
}

// DefaultObserverConfig returns a default observer configuration
func DefaultObserverConfig() *ObserverConfig {
	return &ObserverConfig{
		SamplingRate:       1.0,
		MaxEvents:          10000,
		MaxErrors:          1000,
		RetentionPeriod:    time.Hour * 24,
		MetricsEnabled:     true,
		TracingEnabled:     true,
		HealthCheckEnabled: true,
		LogLevel:           slog.LevelInfo,
		FlushInterval:      time.Minute * 5,
	}
}

// NewObserverImpl creates a new unified observer
func NewObserverImpl(logger *slog.Logger, config *ObserverConfig) *ObserverImpl {
	if config == nil {
		config = DefaultObserverConfig()
	}

	observer := &ObserverImpl{
		logger:      logger.With("component", "observer"),
		config:      config,
		startTime:   time.Now(),
		cleanupStop: make(chan struct{}),
	}

	observer.samplingRate.Store(config.SamplingRate)
	observer.logLevel.Store(config.LogLevel)

	// Start cleanup ticker
	observer.cleanupTicker = time.NewTicker(config.FlushInterval)
	go observer.cleanupWorker()

	return observer
}

// TrackEvent tracks an event in the observability system
func (o *ObserverImpl) TrackEvent(ctx context.Context, event *Event) {
	if !o.shouldSample() {
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate unique event ID
	eventID := o.generateEventID()

	// Store event
	o.events.Store(eventID, event)
	atomic.AddInt64(&o.eventCount, 1)

	// Update operation stats if this is an operation event
	if event.Type == EventTypeOperation {
		o.updateOperationStats(event)
	}

	// Log the event
	o.logEvent(event)
}

// TrackError tracks a standard error
func (o *ObserverImpl) TrackError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	// Create simple error event
	event := &Event{
		Name:      "error",
		Type:      EventTypeError,
		Timestamp: time.Now(),
		Component: "system",
		Operation: "error_tracking",
		Success:   false,
		Properties: map[string]interface{}{
			"message": err.Error(),
		},
		Tags: map[string]string{
			"error_type": "generic",
		},
	}

	o.TrackEvent(ctx, event)

	// Update error metrics
	o.IncrementCounter(MetricErrorsTotal, map[string]string{
		"type": "generic",
	})

	o.logger.Error("Error tracked", "error", err.Error())
}

// StartOperation starts tracking an operation
func (o *ObserverImpl) StartOperation(ctx context.Context, operation string) *OperationContext {
	return &OperationContext{
		Name:      operation,
		StartTime: time.Now(),
		Context:   ctx,
		observer:  o,
	}
}

// StartSpan starts a distributed tracing span
func (o *ObserverImpl) StartSpan(ctx context.Context, name string) *SpanContext {
	return &SpanContext{
		TraceID:   o.generateTraceID(),
		SpanID:    o.generateSpanID(),
		Name:      name,
		StartTime: time.Now(),
		Context:   ctx,
		observer:  o,
	}
}

// RecordHealthCheck records a health check result
func (o *ObserverImpl) RecordHealthCheck(component string, status HealthStatus, latency time.Duration) {
	health := &ComponentHealth{
		Status:       status,
		LastCheck:    time.Now(),
		ResponseTime: latency,
	}

	// Load existing health to preserve uptime and error count
	if existing, ok := o.healthChecks.Load(component); ok {
		if existingHealth, ok := existing.(*ComponentHealth); ok {
			health.Uptime = existingHealth.Uptime
			health.ErrorCount = existingHealth.ErrorCount

			if status != HealthStatusHealthy {
				health.ErrorCount++
			}
		}
	}

	o.healthChecks.Store(component, health)

	// Track as event
	event := &Event{
		Name:      "health_check",
		Type:      EventTypeHealth,
		Timestamp: time.Now(),
		Component: component,
		Operation: "health_check",
		Success:   status == HealthStatusHealthy,
		Properties: map[string]interface{}{
			"status":  string(status),
			"latency": latency,
		},
		Metrics: map[string]float64{
			"latency_ms": float64(latency.Milliseconds()),
		},
		Tags: map[string]string{
			"component": component,
			"status":    string(status),
		},
	}

	o.TrackEvent(context.Background(), event)
}

// RecordMetric records a generic metric
func (o *ObserverImpl) RecordMetric(name string, value float64, tags map[string]string) {
	// Track as histogram for general metrics
	o.RecordHistogram(name, value, tags)
}

// IncrementCounter increments a counter metric
func (o *ObserverImpl) IncrementCounter(name string, tags map[string]string) {
	key := o.buildMetricKey(name, tags)

	if existing, loaded := o.counters.LoadOrStore(key, &CounterMetric{
		Name:  name,
		Value: 1,
		Tags:  tags,
	}); loaded {
		if counter, ok := existing.(*CounterMetric); ok {
			counter.mu.Lock()
			counter.Value++
			counter.mu.Unlock()
		}
	}
}

// SetGauge sets a gauge metric value
func (o *ObserverImpl) SetGauge(name string, value float64, tags map[string]string) {
	key := o.buildMetricKey(name, tags)

	gauge := &GaugeMetric{
		Name:  name,
		Value: value,
		Tags:  tags,
	}

	o.gauges.Store(key, gauge)
}

// RecordHistogram records a value in a histogram
func (o *ObserverImpl) RecordHistogram(name string, value float64, tags map[string]string) {
	key := o.buildMetricKey(name, tags)

	if existing, loaded := o.histograms.LoadOrStore(key, &HistogramMetric{
		Name:   name,
		Values: []float64{value},
		Tags:   tags,
	}); loaded {
		if histogram, ok := existing.(*HistogramMetric); ok {
			histogram.mu.Lock()
			histogram.Values = append(histogram.Values, value)
			// Keep only last 1000 values to prevent memory issues
			if len(histogram.Values) > 1000 {
				histogram.Values = histogram.Values[len(histogram.Values)-1000:]
			}
			histogram.mu.Unlock()
		}
	}
}

// RecordResourceUsage records resource usage metrics
func (o *ObserverImpl) RecordResourceUsage(ctx context.Context, resource *ResourceUsage) {
	if resource.Timestamp.IsZero() {
		resource.Timestamp = time.Now()
	}

	o.resourceUsage.Store(resource.Component, resource)

	// Track as event
	event := &Event{
		Name:      "resource_usage",
		Type:      EventTypeResource,
		Timestamp: resource.Timestamp,
		Component: resource.Component,
		Operation: "resource_monitoring",
		Success:   true,
		Properties: map[string]interface{}{
			"resource": resource,
		},
		Tags: map[string]string{
			"component": resource.Component,
		},
	}

	// Add resource metrics
	if resource.CPU != nil {
		event.Metrics = make(map[string]float64)
		event.Metrics["cpu_percent"] = resource.CPU.Percent
		event.Metrics["cpu_used"] = resource.CPU.Used
	}
	if resource.Memory != nil {
		if event.Metrics == nil {
			event.Metrics = make(map[string]float64)
		}
		event.Metrics["memory_percent"] = resource.Memory.Percent
		event.Metrics["memory_used"] = resource.Memory.Used
	}

	o.TrackEvent(ctx, event)
}

// Logger returns the configured logger
func (o *ObserverImpl) Logger() *slog.Logger {
	return o.logger
}

// GetObservabilityReport generates a comprehensive observability report
func (o *ObserverImpl) GetObservabilityReport() *ObservabilityReport {
	o.mu.RLock()
	defer o.mu.RUnlock()

	now := time.Now()
	period := TimePeriod{
		Start:    o.startTime,
		End:      now,
		Duration: now.Sub(o.startTime),
	}

	report := &ObservabilityReport{
		GeneratedAt:     now,
		Period:          period,
		EventSummary:    o.generateEventSummary(),
		ErrorAnalysis:   o.generateErrorAnalysis(),
		Performance:     o.generatePerformanceMetrics(),
		HealthStatus:    o.generateHealthStatus(),
		ResourceUsage:   o.generateResourceSummary(),
		Trends:          o.generateTrendAnalysis(),
		Recommendations: o.generateRecommendations(),
	}

	return report
}

// SetSamplingRate sets the sampling rate for events
func (o *ObserverImpl) SetSamplingRate(rate float64) {
	o.samplingRate.Store(rate)
}

// SetLogLevel sets the logging level
func (o *ObserverImpl) SetLogLevel(level slog.Level) {
	o.logLevel.Store(level)
}

// Private helper methods

func (o *ObserverImpl) shouldSample() bool {
	rate := o.samplingRate.Load().(float64)
	if rate >= 1.0 {
		return true
	}

	// Simple sampling - in production might use more sophisticated sampling
	return rand.Float64() < rate
}

func (o *ObserverImpl) generateEventID() string {
	bytes := make([]byte, 4)
	if _, err := cryptorand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	return "evt_" + hex.EncodeToString(bytes)
}

func (o *ObserverImpl) generateTraceID() string {
	bytes := make([]byte, 8)
	if _, err := cryptorand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (o *ObserverImpl) generateSpanID() string {
	bytes := make([]byte, 4)
	if _, err := cryptorand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(bytes)
}

func (o *ObserverImpl) buildMetricKey(name string, tags map[string]string) string {
	key := name

	// Sort tag keys for deterministic ordering
	if len(tags) > 0 {
		keys := make([]string, 0, len(tags))
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			key += "|" + k + "=" + tags[k]
		}
	}

	return key
}

func (o *ObserverImpl) updateOperationStats(event *Event) {
	if event.Duration == 0 {
		return
	}

	key := event.Operation
	if event.Component != "" {
		key = event.Component + "." + event.Operation
	}

	if existing, loaded := o.operations.LoadOrStore(key, &OperationStats{
		Name:          key,
		Count:         1,
		SuccessCount:  0,
		TotalDuration: event.Duration,
		MinDuration:   event.Duration,
		MaxDuration:   event.Duration,
		LastExecuted:  event.Timestamp,
	}); loaded {
		if stats, ok := existing.(*OperationStats); ok {
			stats.mu.Lock()
			stats.Count++
			if event.Success {
				stats.SuccessCount++
			}
			stats.TotalDuration += event.Duration
			if event.Duration < stats.MinDuration {
				stats.MinDuration = event.Duration
			}
			if event.Duration > stats.MaxDuration {
				stats.MaxDuration = event.Duration
			}
			stats.LastExecuted = event.Timestamp
			stats.mu.Unlock()
		}
	} else {
		// First time - set success count
		if stats, ok := existing.(*OperationStats); ok && event.Success {
			stats.SuccessCount = 1
		}
	}
}

func (o *ObserverImpl) logEvent(event *Event) {
	level := o.logLevel.Load().(slog.Level)

	// Determine log level based on event type and success
	var logLevel slog.Level
	switch event.Type {
	case EventTypeError:
		logLevel = slog.LevelError
	case EventTypeHealth:
		if event.Success {
			logLevel = slog.LevelDebug
		} else {
			logLevel = slog.LevelWarn
		}
	default:
		if event.Success {
			logLevel = slog.LevelInfo
		} else {
			logLevel = slog.LevelWarn
		}
	}

	// Only log if above configured level
	if logLevel < level {
		return
	}

	args := []interface{}{
		"event_name", event.Name,
		"event_type", string(event.Type),
		"component", event.Component,
		"operation", event.Operation,
		"success", event.Success,
		"timestamp", event.Timestamp,
	}

	if event.Duration > 0 {
		args = append(args, "duration", event.Duration)
	}
	if event.WorkflowID != "" {
		args = append(args, "workflow_id", event.WorkflowID)
	}
	if event.SessionID != "" {
		args = append(args, "session_id", event.SessionID)
	}

	// Add properties
	for k, v := range event.Properties {
		args = append(args, k, v)
	}

	// Add metrics
	for k, v := range event.Metrics {
		args = append(args, k, v)
	}

	// Add tags
	for k, v := range event.Tags {
		args = append(args, "tag_"+k, v)
	}

	message := "Event tracked"
	if !event.Success {
		message = "Event failed"
	}

	o.logger.Log(context.Background(), logLevel, message, args...)
}

func (o *ObserverImpl) cleanupOldEvents() {
	cutoff := time.Now().Add(-o.config.RetentionPeriod)

	o.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Timestamp.Before(cutoff) {
				o.events.Delete(key)
			}
		}
		return true
	})
}

// Report generation methods (simplified implementations)

func (o *ObserverImpl) generateEventSummary() EventSummary {
	totalEvents := atomic.LoadInt64(&o.eventCount)
	eventsByType := make(map[EventType]int64)
	eventsByComponent := make(map[string]int64)
	var successCount int64
	var totalDuration time.Duration
	var durationCount int64

	o.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			eventsByType[event.Type]++
			eventsByComponent[event.Component]++
			if event.Success {
				successCount++
			}
			if event.Duration > 0 {
				totalDuration += event.Duration
				durationCount++
			}
		}
		return true
	})

	var successRate float64
	if totalEvents > 0 {
		successRate = float64(successCount) / float64(totalEvents)
	}

	var avgDuration time.Duration
	if durationCount > 0 {
		avgDuration = totalDuration / time.Duration(durationCount)
	}

	return EventSummary{
		TotalEvents:       totalEvents,
		EventsByType:      eventsByType,
		EventsByComponent: eventsByComponent,
		SuccessRate:       successRate,
		AvgDuration:       avgDuration,
	}
}

func (o *ObserverImpl) generateErrorAnalysis() ErrorAnalysis {
	totalEvents := atomic.LoadInt64(&o.eventCount)
	errorCount := int64(0)
	recoverableCount := int64(0)
	criticalCount := int64(0)
	errorMessages := make(map[string]int)

	// Count and classify error events
	o.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Type == EventTypeError {
				errorCount++

				// Classify error based on properties
				if severity, ok := event.Properties["severity"].(string); ok {
					switch severity {
					case "critical", "fatal":
						criticalCount++
					default:
						recoverableCount++
					}
				} else {
					// Default to recoverable if no severity specified
					recoverableCount++
				}

				// Track error messages for top errors
				if msg, ok := event.Properties["error"].(string); ok {
					errorMessages[msg]++
				}
			}
		}
		return true
	})

	errorRate := float64(0)
	if totalEvents > 0 {
		errorRate = float64(errorCount) / float64(totalEvents)
	}

	// Get top 5 errors
	topErrors := o.getTopErrors(errorMessages, 5)

	return ErrorAnalysis{
		TotalErrors:       errorCount,
		RecoverableErrors: recoverableCount,
		CriticalErrors:    criticalCount,
		ErrorRate:         errorRate,
		TopErrors:         topErrors,
	}
}

// getTopErrors returns the top N most frequent error messages
func (o *ObserverImpl) getTopErrors(errorMessages map[string]int, n int) []string {
	// Convert map to slice for sorting
	type errorCount struct {
		message string
		count   int
	}
	errors := make([]errorCount, 0, len(errorMessages))
	for msg, count := range errorMessages {
		errors = append(errors, errorCount{msg, count})
	}

	// Sort by count descending
	sort.Slice(errors, func(i, j int) bool {
		return errors[i].count > errors[j].count
	})

	// Get top N
	topErrors := make([]string, 0, n)
	for i := 0; i < n && i < len(errors); i++ {
		topErrors = append(topErrors, fmt.Sprintf("%s (count: %d)", errors[i].message, errors[i].count))
	}

	return topErrors
}

func (o *ObserverImpl) generatePerformanceMetrics() PerformanceMetrics {
	operationMetrics := make(map[string]OperationMetrics)

	o.operations.Range(func(key, value interface{}) bool {
		if name, ok := key.(string); ok {
			if stats, ok := value.(*OperationStats); ok {
				stats.mu.RLock()

				var successRate float64
				if stats.Count > 0 {
					successRate = float64(stats.SuccessCount) / float64(stats.Count)
				}

				var avgDuration time.Duration
				if stats.Count > 0 {
					avgDuration = stats.TotalDuration / time.Duration(stats.Count)
				}

				var errorRate float64
				if stats.Count > 0 {
					errorRate = float64(stats.Count-stats.SuccessCount) / float64(stats.Count)
				}

				operationMetrics[name] = OperationMetrics{
					Count:        stats.Count,
					SuccessRate:  successRate,
					AvgDuration:  avgDuration,
					MinDuration:  stats.MinDuration,
					MaxDuration:  stats.MaxDuration,
					ErrorRate:    errorRate,
					LastExecuted: stats.LastExecuted,
				}

				stats.mu.RUnlock()
			}
		}
		return true
	})

	return PerformanceMetrics{
		OperationMetrics: operationMetrics,
		// Other metrics would be calculated from operation stats
	}
}

func (o *ObserverImpl) generateHealthStatus() map[string]ComponentHealth {
	healthStatus := make(map[string]ComponentHealth)

	o.healthChecks.Range(func(key, value interface{}) bool {
		if component, ok := key.(string); ok {
			if health, ok := value.(*ComponentHealth); ok {
				healthStatus[component] = *health
			}
		}
		return true
	})

	return healthStatus
}

func (o *ObserverImpl) generateResourceSummary() ResourceSummary {
	componentUsage := make(map[string]ResourceUsage)

	o.resourceUsage.Range(func(key, value interface{}) bool {
		if component, ok := key.(string); ok {
			if usage, ok := value.(*ResourceUsage); ok {
				componentUsage[component] = *usage
			}
		}
		return true
	})

	return ResourceSummary{
		ComponentUsage: componentUsage,
		// Other fields would be calculated from usage data
	}
}

func (o *ObserverImpl) generateTrendAnalysis() TrendAnalysis {
	return TrendAnalysis{
		ErrorTrends:       make(map[string]string),
		PerformanceTrends: make(map[string]string),
		UsageTrends:       make(map[string]string),
		Predictions:       make(map[string]TrendPrediction),
	}
}

func (o *ObserverImpl) generateRecommendations() []Recommendation {
	var recommendations []Recommendation

	// Generate simple recommendations based on system state
	recommendations = append(recommendations, Recommendation{
		Type:        RecommendationTypePerformance,
		Priority:    PriorityMedium,
		Title:       "Monitor System Performance",
		Description: "Continue monitoring system performance metrics",
		Actions: []Action{
			{
				Description: "Review performance metrics regularly",
				Type:        "monitoring",
				Automated:   false,
			},
		},
		Impact: "Improved system visibility",
		Effort: "Low",
	})

	return recommendations
}

// cleanupWorker runs periodic cleanup tasks
func (o *ObserverImpl) cleanupWorker() {
	for {
		select {
		case <-o.cleanupTicker.C:
			o.cleanupOldEvents()
		case <-o.cleanupStop:
			return
		}
	}
}

// Close shuts down the observer gracefully
func (o *ObserverImpl) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Stop the cleanup ticker
	if o.cleanupTicker != nil {
		o.cleanupTicker.Stop()
	}

	// Signal cleanup worker to stop
	close(o.cleanupStop)

	// Final cleanup
	o.cleanupOldEvents()

	o.logger.Info("Observer shut down gracefully")
	return nil
}
