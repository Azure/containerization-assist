// Package observability provides unified logging and metrics capabilities
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/errors"
)

// UnifiedLogger provides enhanced logging with automatic metric extraction and correlation
type UnifiedLogger struct {
	observer Observer
	logger   *slog.Logger
	config   *LoggerConfig

	// Metric extraction patterns
	metricPatterns map[string]*MetricPattern

	// Correlation tracking
	correlations sync.Map // map[string]*CorrelationContext

	// Log enrichment
	enrichers []LogEnricher

	// Performance tracking
	logMetrics *LogMetrics
	mu         sync.RWMutex
}

// LoggerConfig provides configuration for the unified logger
type LoggerConfig struct {
	// Automatic metric extraction
	EnableMetricExtraction bool
	EnableLogCorrelation   bool
	EnablePerformanceLog   bool

	// Log enrichment
	EnableContextEnrichment bool
	EnableErrorEnrichment   bool
	EnableTraceEnrichment   bool

	// Sampling and filtering
	LogSamplingRate      float64
	ErrorLogSamplingRate float64
	PerformanceLogLevel  slog.Level

	// Retention and limits
	CorrelationTTL  time.Duration
	MaxCorrelations int
	MaxLogEnrichers int

	// Output formatting
	EnableStructuredOutput bool
	EnableCompactOutput    bool
	IncludeStackTrace      bool
}

// MetricPattern defines how to extract metrics from log messages
type MetricPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	MetricType  MetricType
	ValueGroup  int            // Regex group number for the metric value
	TagGroups   map[string]int // Tag name to regex group number mapping
	Unit        string
	Description string
}

// MetricType defines the type of metric to extract
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeTiming    MetricType = "timing"
)

// CorrelationContext tracks related log entries and operations
type CorrelationContext struct {
	ID          string
	SessionID   string
	WorkflowID  string
	OperationID string
	UserID      string
	StartTime   time.Time
	LastSeen    time.Time
	LogCount    int64
	ErrorCount  int64
	Properties  map[string]interface{}
	mu          sync.RWMutex
}

// LogEnricher provides additional context to log entries
type LogEnricher interface {
	Enrich(ctx context.Context, record *LogRecord) error
	Name() string
	Priority() int
}

// LogRecord represents an enhanced log record with extracted metrics and context
type LogRecord struct {
	// Standard log fields
	Time    time.Time
	Level   slog.Level
	Message string
	Source  string

	// Enhanced fields
	SessionID   string
	WorkflowID  string
	OperationID string
	TraceID     string
	SpanID      string

	// Extracted metrics
	ExtractedMetrics []ExtractedMetric

	// Correlation data
	CorrelationID string
	Correlation   *CorrelationContext

	// Additional context
	Properties map[string]interface{}
	Tags       map[string]string
	StackTrace string

	// Performance data
	Duration        time.Duration
	MemoryDelta     int64
	GoroutinesDelta int
}

// ExtractedMetric represents a metric extracted from a log message
type ExtractedMetric struct {
	Name      string
	Type      MetricType
	Value     float64
	Unit      string
	Tags      map[string]string
	Timestamp time.Time
}

// LogMetrics tracks logging performance and statistics
type LogMetrics struct {
	TotalLogs         int64
	LogsByLevel       map[slog.Level]int64
	ExtractedMetrics  int64
	CorrelationHits   int64
	EnrichmentErrors  int64
	AverageLogSize    float64
	LogProcessingTime time.Duration
	mu                sync.RWMutex
}

// DefaultLoggerConfig returns a default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		EnableMetricExtraction:  true,
		EnableLogCorrelation:    true,
		EnablePerformanceLog:    true,
		EnableContextEnrichment: true,
		EnableErrorEnrichment:   true,
		EnableTraceEnrichment:   true,
		LogSamplingRate:         1.0,
		ErrorLogSamplingRate:    1.0,
		PerformanceLogLevel:     slog.LevelDebug,
		CorrelationTTL:          time.Hour,
		MaxCorrelations:         10000,
		MaxLogEnrichers:         10,
		EnableStructuredOutput:  true,
		EnableCompactOutput:     false,
		IncludeStackTrace:       false,
	}
}

// NewUnifiedLogger creates a new unified logger
func NewUnifiedLogger(observer Observer, logger *slog.Logger, config *LoggerConfig) *UnifiedLogger {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	ul := &UnifiedLogger{
		observer:       observer,
		logger:         logger.With("component", "unified_logger"),
		config:         config,
		metricPatterns: make(map[string]*MetricPattern),
		enrichers:      make([]LogEnricher, 0, config.MaxLogEnrichers),
		logMetrics: &LogMetrics{
			LogsByLevel: make(map[slog.Level]int64),
		},
	}

	// Register default metric patterns
	ul.registerDefaultMetricPatterns()

	return ul
}

// registerDefaultMetricPatterns registers common metric extraction patterns
func (ul *UnifiedLogger) registerDefaultMetricPatterns() {
	// Response time pattern: "request completed in 150ms"
	ul.RegisterMetricPattern(&MetricPattern{
		Name:        "response_time",
		Pattern:     regexp.MustCompile(`(?i)(?:request|operation|task)\s+(?:completed|finished|took)\s+(?:in\s+)?(\d+(?:\.\d+)?)(ms|s|seconds?|milliseconds?)`),
		MetricType:  MetricTypeTiming,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "milliseconds",
		Description: "Request response time",
	})

	// Error count pattern: "failed with 5 errors"
	ul.RegisterMetricPattern(&MetricPattern{
		Name:        "error_count",
		Pattern:     regexp.MustCompile(`(?i)(?:failed|error|errors?)\s+(?:with\s+)?(\d+)\s+(?:errors?|failures?)`),
		MetricType:  MetricTypeCounter,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "count",
		Description: "Number of errors",
	})

	// Memory usage pattern: "using 256MB memory"
	ul.RegisterMetricPattern(&MetricPattern{
		Name:        "memory_usage",
		Pattern:     regexp.MustCompile(`(?i)(?:using|consumed|allocated)\s+(\d+(?:\.\d+)?)(MB|GB|KB|bytes?)`),
		MetricType:  MetricTypeGauge,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "bytes",
		Description: "Memory usage",
	})

	// Processing rate pattern: "processed 1000 items/sec"
	ul.RegisterMetricPattern(&MetricPattern{
		Name:        "processing_rate",
		Pattern:     regexp.MustCompile(`(?i)processed\s+(\d+(?:\.\d+)?)\s+(?:items?|requests?|operations?)(?:/sec|/second|per\s+second)?`),
		MetricType:  MetricTypeGauge,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "per_second",
		Description: "Processing rate",
	})

	// Queue size pattern: "queue size: 42"
	ul.RegisterMetricPattern(&MetricPattern{
		Name:        "queue_size",
		Pattern:     regexp.MustCompile(`(?i)queue\s+size[:=]?\s+(\d+)`),
		MetricType:  MetricTypeGauge,
		ValueGroup:  1,
		TagGroups:   map[string]int{},
		Unit:        "count",
		Description: "Queue size",
	})
}

// RegisterMetricPattern registers a new metric extraction pattern
func (ul *UnifiedLogger) RegisterMetricPattern(pattern *MetricPattern) error {
	ul.mu.Lock()
	defer ul.mu.Unlock()

	if pattern.Pattern == nil {
		return fmt.Errorf("metric pattern regex cannot be nil")
	}

	ul.metricPatterns[pattern.Name] = pattern
	return nil
}

// AddEnricher adds a log enricher
func (ul *UnifiedLogger) AddEnricher(enricher LogEnricher) error {
	ul.mu.Lock()
	defer ul.mu.Unlock()

	if len(ul.enrichers) >= ul.config.MaxLogEnrichers {
		return fmt.Errorf("maximum number of enrichers (%d) reached", ul.config.MaxLogEnrichers)
	}

	// Insert enricher based on priority (higher priority first)
	inserted := false
	for i, existing := range ul.enrichers {
		if enricher.Priority() > existing.Priority() {
			// Insert at position i
			ul.enrichers = append(ul.enrichers[:i], append([]LogEnricher{enricher}, ul.enrichers[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		ul.enrichers = append(ul.enrichers, enricher)
	}

	return nil
}

// Info logs an info message with automatic metric extraction and correlation
func (ul *UnifiedLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	ul.log(ctx, slog.LevelInfo, msg, args...)
}

// Warn logs a warning message with automatic metric extraction and correlation
func (ul *UnifiedLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	ul.log(ctx, slog.LevelWarn, msg, args...)
}

// Error logs an error message with automatic metric extraction and correlation
func (ul *UnifiedLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	ul.log(ctx, slog.LevelError, msg, args...)
}

// Debug logs a debug message with automatic metric extraction and correlation
func (ul *UnifiedLogger) Debug(ctx context.Context, msg string, args ...interface{}) {
	ul.log(ctx, slog.LevelDebug, msg, args...)
}

// LogWithStructuredError logs a structured error with enhanced context
func (ul *UnifiedLogger) LogWithStructuredError(ctx context.Context, err *errors.StructuredError) {
	// Create enhanced log record
	record := &LogRecord{
		Time:       err.Timestamp,
		Level:      ul.mapErrorSeverityToLogLevel(err.Severity),
		Message:    err.Message,
		SessionID:  err.SessionID,
		WorkflowID: err.WorkflowID,
		Properties: make(map[string]interface{}),
		Tags:       make(map[string]string),
	}

	// Add error-specific context
	record.Properties["error_id"] = err.ID
	record.Properties["error_category"] = string(err.Category)
	record.Properties["error_severity"] = string(err.Severity)
	record.Properties["error_recoverable"] = err.Recoverable
	record.Properties["error_component"] = err.Component
	record.Properties["error_operation"] = err.Operation

	// Add error context
	for k, v := range err.Context {
		record.Properties[fmt.Sprintf("error_context_%s", k)] = v
	}

	// Add tags
	record.Tags["error_category"] = string(err.Category)
	record.Tags["error_severity"] = string(err.Severity)
	record.Tags["component"] = err.Component

	// Process the log record
	ul.processLogRecord(ctx, record)

	// Update log metrics
	ul.updateLogMetrics(record.Level, time.Since(err.Timestamp))

	// Also track the error with the observer
	ul.observer.TrackStructuredError(ctx, err)
}

// LogOperation logs an operation with automatic timing and metric extraction
func (ul *UnifiedLogger) LogOperation(ctx context.Context, operation string, duration time.Duration, success bool, properties map[string]interface{}) {
	level := slog.LevelInfo
	if !success {
		level = slog.LevelWarn
	}

	msg := fmt.Sprintf("Operation %s completed in %v", operation, duration)
	if !success {
		msg = fmt.Sprintf("Operation %s failed after %v", operation, duration)
	}

	// Create enhanced log record
	record := &LogRecord{
		Time:       time.Now(),
		Level:      level,
		Message:    msg,
		Duration:   duration,
		Properties: make(map[string]interface{}),
		Tags:       make(map[string]string),
	}

	// Add operation context
	record.Properties["operation"] = operation
	record.Properties["duration_ms"] = duration.Milliseconds()
	record.Properties["success"] = success
	record.Tags["operation"] = operation
	record.Tags["success"] = fmt.Sprintf("%v", success)

	// Add custom properties
	for k, v := range properties {
		record.Properties[k] = v
	}

	// Process the log record
	ul.processLogRecord(ctx, record)

	// Update log metrics
	ul.updateLogMetrics(record.Level, time.Since(record.Time))

	// Record operation metrics
	ul.observer.RecordHistogram("operation_duration", float64(duration.Milliseconds()), map[string]string{
		"operation": operation,
		"success":   fmt.Sprintf("%v", success),
	})
}

// log is the core logging method that handles all log processing
func (ul *UnifiedLogger) log(ctx context.Context, level slog.Level, msg string, args ...interface{}) {
	startTime := time.Now()

	// Create log record
	record := &LogRecord{
		Time:       startTime,
		Level:      level,
		Message:    msg,
		Properties: make(map[string]interface{}),
		Tags:       make(map[string]string),
	}

	// Add arguments as properties
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			record.Properties[key] = args[i+1]
		}
	}

	// Process the log record
	ul.processLogRecord(ctx, record)

	// Update log metrics
	ul.updateLogMetrics(level, time.Since(startTime))

	// Forward to underlying logger
	ul.logger.Log(ctx, level, msg, args...)
}

// processLogRecord processes a log record through all enhancement stages
func (ul *UnifiedLogger) processLogRecord(ctx context.Context, record *LogRecord) {
	// Extract context information
	if ul.config.EnableContextEnrichment {
		ul.extractContextInfo(ctx, record)
	}

	// Extract correlation information
	if ul.config.EnableLogCorrelation {
		ul.handleCorrelation(ctx, record)
	}

	// Extract metrics from log message
	if ul.config.EnableMetricExtraction {
		ul.extractMetrics(record)
	}

	// Apply enrichers
	ul.applyEnrichers(ctx, record)

	// Record extracted metrics
	ul.recordExtractedMetrics(record)

	// Track enhanced event
	ul.trackEnhancedEvent(ctx, record)
}

// extractContextInfo extracts context information from the log record
func (ul *UnifiedLogger) extractContextInfo(ctx context.Context, record *LogRecord) {
	// Extract session ID from context or properties
	if sessionID, ok := record.Properties["session_id"].(string); ok {
		record.SessionID = sessionID
	}

	// Extract workflow ID from context or properties
	if workflowID, ok := record.Properties["workflow_id"].(string); ok {
		record.WorkflowID = workflowID
	}

	// Extract operation ID from context or properties
	if operationID, ok := record.Properties["operation_id"].(string); ok {
		record.OperationID = operationID
	}

	// Extract trace information
	if ul.config.EnableTraceEnrichment {
		if traceID, ok := record.Properties["trace_id"].(string); ok {
			record.TraceID = traceID
		}
		if spanID, ok := record.Properties["span_id"].(string); ok {
			record.SpanID = spanID
		}
	}
}

// handleCorrelation manages log correlation and context tracking
func (ul *UnifiedLogger) handleCorrelation(ctx context.Context, record *LogRecord) {
	// Generate or extract correlation ID
	correlationID := ul.generateCorrelationID(record)
	record.CorrelationID = correlationID

	// Get or create correlation context
	correlation := ul.getOrCreateCorrelation(correlationID, record)
	record.Correlation = correlation

	// Update correlation context
	correlation.mu.Lock()
	correlation.LastSeen = record.Time
	correlation.LogCount++
	if record.Level >= slog.LevelError {
		correlation.ErrorCount++
	}

	// Add new properties to correlation context
	for k, v := range record.Properties {
		if _, exists := correlation.Properties[k]; !exists {
			correlation.Properties[k] = v
		}
	}
	correlation.mu.Unlock()

	// Update metrics
	ul.logMetrics.mu.Lock()
	ul.logMetrics.CorrelationHits++
	ul.logMetrics.mu.Unlock()
}

// extractMetrics extracts metrics from the log message using registered patterns
func (ul *UnifiedLogger) extractMetrics(record *LogRecord) {
	ul.mu.RLock()
	patterns := make(map[string]*MetricPattern, len(ul.metricPatterns))
	for k, v := range ul.metricPatterns {
		patterns[k] = v
	}
	ul.mu.RUnlock()

	for _, pattern := range patterns {
		matches := pattern.Pattern.FindStringSubmatch(record.Message)
		if len(matches) > pattern.ValueGroup {
			if value, err := ul.parseMetricValue(matches[pattern.ValueGroup], pattern); err == nil {
				// Extract tags
				tags := make(map[string]string)
				for tagName, groupIndex := range pattern.TagGroups {
					if groupIndex < len(matches) {
						tags[tagName] = matches[groupIndex]
					}
				}

				// Add component tag if available
				if record.Properties["component"] != nil {
					tags["component"] = fmt.Sprintf("%v", record.Properties["component"])
				}

				metric := ExtractedMetric{
					Name:      pattern.Name,
					Type:      pattern.MetricType,
					Value:     value,
					Unit:      pattern.Unit,
					Tags:      tags,
					Timestamp: record.Time,
				}

				record.ExtractedMetrics = append(record.ExtractedMetrics, metric)
			}
		}
	}

	// Update metrics count
	if len(record.ExtractedMetrics) > 0 {
		ul.logMetrics.mu.Lock()
		ul.logMetrics.ExtractedMetrics += int64(len(record.ExtractedMetrics))
		ul.logMetrics.mu.Unlock()
	}
}

// applyEnrichers applies all registered enrichers to the log record
func (ul *UnifiedLogger) applyEnrichers(ctx context.Context, record *LogRecord) {
	ul.mu.RLock()
	enrichers := make([]LogEnricher, len(ul.enrichers))
	copy(enrichers, ul.enrichers)
	ul.mu.RUnlock()

	for _, enricher := range enrichers {
		if err := enricher.Enrich(ctx, record); err != nil {
			// Log enrichment error but don't fail the logging
			ul.logger.Debug("Log enrichment failed",
				"enricher", enricher.Name(),
				"error", err.Error())

			ul.logMetrics.mu.Lock()
			ul.logMetrics.EnrichmentErrors++
			ul.logMetrics.mu.Unlock()
		}
	}
}

// recordExtractedMetrics records extracted metrics with the observer
func (ul *UnifiedLogger) recordExtractedMetrics(record *LogRecord) {
	for _, metric := range record.ExtractedMetrics {
		switch metric.Type {
		case MetricTypeCounter:
			ul.observer.IncrementCounter(metric.Name, metric.Tags)
		case MetricTypeGauge:
			ul.observer.SetGauge(metric.Name, metric.Value, metric.Tags)
		case MetricTypeHistogram, MetricTypeTiming:
			ul.observer.RecordHistogram(metric.Name, metric.Value, metric.Tags)
		}
	}
}

// trackEnhancedEvent tracks an enhanced event with the observer
func (ul *UnifiedLogger) trackEnhancedEvent(ctx context.Context, record *LogRecord) {
	event := &Event{
		Name:       "enhanced_log",
		Type:       EventTypeSystem,
		Timestamp:  record.Time,
		Component:  "unified_logger",
		Operation:  "log_processing",
		Success:    record.Level < slog.LevelError,
		WorkflowID: record.WorkflowID,
		SessionID:  record.SessionID,
		Properties: record.Properties,
		Tags:       record.Tags,
	}

	// Add log-specific properties
	event.Properties["log_level"] = record.Level.String()
	event.Properties["log_message"] = record.Message
	event.Properties["extracted_metrics_count"] = len(record.ExtractedMetrics)

	if record.CorrelationID != "" {
		event.Properties["correlation_id"] = record.CorrelationID
	}
	if record.Duration > 0 {
		event.Duration = record.Duration
		event.Properties["duration_ms"] = record.Duration.Milliseconds()
	}

	ul.observer.TrackEvent(ctx, event)
}

// Helper methods

func (ul *UnifiedLogger) generateCorrelationID(record *LogRecord) string {
	// Use existing IDs if available
	if record.SessionID != "" {
		return record.SessionID
	}
	if record.WorkflowID != "" {
		return record.WorkflowID
	}
	if record.OperationID != "" {
		return record.OperationID
	}

	// Generate new correlation ID based on log content
	content := fmt.Sprintf("%s_%s_%d", record.Message, record.Level.String(), record.Time.Unix())
	hash := fmt.Sprintf("%x", content)
	if len(hash) > 16 {
		hash = hash[:16]
	}
	return "corr_" + hash
}

func (ul *UnifiedLogger) getOrCreateCorrelation(correlationID string, record *LogRecord) *CorrelationContext {
	if existing, ok := ul.correlations.Load(correlationID); ok {
		return existing.(*CorrelationContext)
	}

	// Create new correlation context
	correlation := &CorrelationContext{
		ID:          correlationID,
		SessionID:   record.SessionID,
		WorkflowID:  record.WorkflowID,
		OperationID: record.OperationID,
		StartTime:   record.Time,
		LastSeen:    record.Time,
		Properties:  make(map[string]interface{}),
	}

	// Add initial properties
	for k, v := range record.Properties {
		correlation.Properties[k] = v
	}

	// Store correlation context
	ul.correlations.Store(correlationID, correlation)

	// Start cleanup timer
	go ul.scheduleCorrelationCleanup(correlationID)

	return correlation
}

func (ul *UnifiedLogger) scheduleCorrelationCleanup(correlationID string) {
	time.Sleep(ul.config.CorrelationTTL)
	ul.correlations.Delete(correlationID)
}

func (ul *UnifiedLogger) parseMetricValue(valueStr string, pattern *MetricPattern) (float64, error) {
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert units if necessary
	if pattern.MetricType == MetricTypeTiming {
		// Convert to milliseconds
		if strings.Contains(strings.ToLower(pattern.Unit), "second") {
			value *= 1000
		}
	}

	return value, nil
}

func (ul *UnifiedLogger) mapErrorSeverityToLogLevel(severity errors.ErrorSeverity) slog.Level {
	switch severity {
	case errors.SeverityCritical:
		return slog.LevelError
	case errors.SeverityHigh:
		return slog.LevelError
	case errors.SeverityMedium:
		return slog.LevelWarn
	case errors.SeverityLow:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

func (ul *UnifiedLogger) updateLogMetrics(level slog.Level, processingTime time.Duration) {
	ul.logMetrics.mu.Lock()
	defer ul.logMetrics.mu.Unlock()

	ul.logMetrics.TotalLogs++
	ul.logMetrics.LogsByLevel[level]++
	ul.logMetrics.LogProcessingTime += processingTime
}

// GetLogMetrics returns current logging metrics
func (ul *UnifiedLogger) GetLogMetrics() *LogMetrics {
	ul.logMetrics.mu.RLock()
	defer ul.logMetrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &LogMetrics{
		TotalLogs:         ul.logMetrics.TotalLogs,
		LogsByLevel:       make(map[slog.Level]int64),
		ExtractedMetrics:  ul.logMetrics.ExtractedMetrics,
		CorrelationHits:   ul.logMetrics.CorrelationHits,
		EnrichmentErrors:  ul.logMetrics.EnrichmentErrors,
		AverageLogSize:    ul.logMetrics.AverageLogSize,
		LogProcessingTime: ul.logMetrics.LogProcessingTime,
	}

	for level, count := range ul.logMetrics.LogsByLevel {
		metrics.LogsByLevel[level] = count
	}

	return metrics
}

// GetCorrelationCount returns the current number of active correlations
func (ul *UnifiedLogger) GetCorrelationCount() int {
	count := 0
	ul.correlations.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
