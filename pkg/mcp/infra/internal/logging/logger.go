package logging

import (
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Logger provides structured logging with ring buffer capability.
// This combines the best features from both common/logging.go and utils/logging.go.
type Logger struct {
	zerologLogger zerolog.Logger
	ringBuffer    *RingBuffer
	config        Config
	capture       *LogCapture
	mu            sync.RWMutex
	metrics       *LogMetrics
}

// EventAdapter adapts zerolog.Event to the public LogEvent interface.
type EventAdapter struct {
	event *zerolog.Event
}

// Str adds a string field to the log event.
func (lea *EventAdapter) Str(key, value string) Event {
	lea.event = lea.event.Str(key, value)
	return lea
}

// Err adds an error field to the log event.
func (lea *EventAdapter) Err(err error) Event {
	lea.event = lea.event.Err(err)
	return lea
}

// Int adds an integer field to the log event.
func (lea *EventAdapter) Int(key string, value int) Event {
	lea.event = lea.event.Int(key, value)
	return lea
}

// Float64 adds a float64 field to the log event.
func (lea *EventAdapter) Float64(key string, value float64) Event {
	lea.event = lea.event.Float64(key, value)
	return lea
}

// Bool adds a boolean field to the log event.
func (lea *EventAdapter) Bool(key string, value bool) Event {
	lea.event = lea.event.Bool(key, value)
	return lea
}

// Msg logs the event with the given message.
func (lea *EventAdapter) Msg(msg string) {
	lea.event.Msg(msg)
}

// Event defines the interface for log events in the internal package.
type Event interface {
	// Str adds a string field to the log event.
	Str(key, value string) Event

	// Err adds an error field to the log event.
	Err(err error) Event

	// Int adds an integer field to the log event.
	Int(key string, value int) Event

	// Float64 adds a float64 field to the log event.
	Float64(key string, value float64) Event

	// Bool adds a boolean field to the log event.
	Bool(key string, value bool) Event

	// Msg logs the event with the given message.
	Msg(msg string)
}

// LogCapture provides log capture functionality with ring buffer storage.
type LogCapture struct {
	ringBuffer *RingBuffer
	config     LogCaptureConfig
	active     bool
}

// LogCaptureConfig defines log capture configuration.
type LogCaptureConfig struct {
	MaxEntries    int           `json:"max_entries"`
	MinLevel      Level         `json:"min_level"`
	RetentionTime time.Duration `json:"retention_time"`
	BufferSize    int           `json:"buffer_size"`
}

// LogMetrics tracks logging performance metrics.
type LogMetrics struct {
	TotalLogs   int64         `json:"total_logs"`
	ErrorLogs   int64         `json:"error_logs"`
	WarningLogs int64         `json:"warning_logs"`
	DebugLogs   int64         `json:"debug_logs"`
	InfoLogs    int64         `json:"info_logs"`
	AverageTime time.Duration `json:"average_time"`
	LastLogTime time.Time     `json:"last_log_time"`
}

// LogEntry represents a captured log entry.
type LogEntry struct {
	// Timestamp is when the log entry was created.
	Timestamp time.Time

	// Level is the log level.
	Level Level

	// Message is the log message.
	Message string

	// Fields contains structured fields.
	Fields map[string]interface{}

	// Caller contains caller information if enabled.
	Caller string

	// Error contains error information if present.
	Error error
}

// UnifiedLogger is an alias for Logger to match the public API
type UnifiedLogger = Logger

// NewLogger creates a new unified logger with the specified configuration.
func NewLogger(config Config) *Logger {
	var ringBuffer *RingBuffer
	var logCapture *LogCapture
	var output io.Writer = config.Output

	// Initialize ring buffer if enabled
	if config.EnableRingBuffer {
		ringBuffer = NewRingBuffer(config.BufferSize)
		// Create multi-writer (output + ring buffer)
		output = io.MultiWriter(config.Output, ringBuffer)
	}

	// Initialize log capture if enabled
	if config.EnableRingBuffer {
		captureConfig := LogCaptureConfig{
			MaxEntries:    config.BufferSize,
			MinLevel:      config.Level,
			RetentionTime: 24 * time.Hour, // Default 24 hour retention
			BufferSize:    config.BufferSize,
		}

		logCapture = &LogCapture{
			ringBuffer: ringBuffer,
			config:     captureConfig,
			active:     true,
		}
	}

	// Configure zerolog
	zerologLogger := zerolog.New(output).Level(config.Level.toZerologLevel())

	// Add timestamp
	zerologLogger = zerologLogger.With().Timestamp().Logger()

	// Add caller information if enabled
	if config.EnableCaller {
		zerologLogger = zerologLogger.With().Caller().Logger()
	}

	// Add configured fields
	for k, v := range config.Fields {
		zerologLogger = zerologLogger.With().Interface(k, v).Logger()
	}

	// Initialize metrics
	metrics := &LogMetrics{
		LastLogTime: time.Now(),
	}

	return &Logger{
		zerologLogger: zerologLogger,
		ringBuffer:    ringBuffer,
		capture:       logCapture,
		config:        config,
		metrics:       metrics,
	}
}

// GetRecentLogs returns recent log entries from the ring buffer.
func (ul *Logger) GetRecentLogs() []LogEntry {
	if ul.ringBuffer == nil {
		return nil
	}

	ul.mu.RLock()
	defer ul.mu.RUnlock()
	return ul.ringBuffer.GetAll()
}

// GetLogsSince returns log entries since the specified time.
func (ul *Logger) GetLogsSince(since time.Time) []LogEntry {
	if ul.ringBuffer == nil {
		return nil
	}

	ul.mu.RLock()
	defer ul.mu.RUnlock()

	allEntries := ul.ringBuffer.GetAll()
	var result []LogEntry

	for _, entry := range allEntries {
		if entry.Timestamp.After(since) {
			result = append(result, entry)
		}
	}

	return result
}

// GetLogsByLevel returns log entries with the specified level.
func (ul *Logger) GetLogsByLevel(level Level) []LogEntry {
	if ul.ringBuffer == nil {
		return nil
	}

	ul.mu.RLock()
	defer ul.mu.RUnlock()

	allEntries := ul.ringBuffer.GetAll()
	var result []LogEntry

	for _, entry := range allEntries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}

	return result
}

// Clear clears the captured logs.
func (ul *Logger) Clear() {
	if ul.ringBuffer == nil {
		return
	}

	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.ringBuffer.Clear()
}

// Size returns the number of captured log entries.
func (ul *Logger) Size() int {
	if ul.ringBuffer == nil {
		return 0
	}

	ul.mu.RLock()
	defer ul.mu.RUnlock()
	return ul.ringBuffer.Size()
}

// WithField adds a structured field to the logger context.
func (ul *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		zerologLogger: ul.zerologLogger.With().Interface(key, value).Logger(),
		ringBuffer:    ul.ringBuffer,
		capture:       ul.capture,
		config:        ul.config,
		metrics:       ul.metrics,
	}
}

// WithFields adds multiple fields to the logger context.
func (ul *Logger) WithFields(fields map[string]interface{}) *Logger {
	logger := ul.zerologLogger
	for k, v := range fields {
		logger = logger.With().Interface(k, v).Logger()
	}

	return &Logger{
		zerologLogger: logger,
		ringBuffer:    ul.ringBuffer,
		capture:       ul.capture,
		config:        ul.config,
		metrics:       ul.metrics,
	}
}

// WithError adds an error field to the logger context.
func (ul *Logger) WithError(err error) *Logger {
	return &Logger{
		zerologLogger: ul.zerologLogger.With().Err(err).Logger(),
		ringBuffer:    ul.ringBuffer,
		capture:       ul.capture,
		config:        ul.config,
		metrics:       ul.metrics,
	}
}

// WithComponent adds a component name to the logger context.
func (ul *Logger) WithComponent(component string) *Logger {
	return ul.WithField("component", component)
}

// WithTraceID adds a trace ID to the logger context.
func (ul *Logger) WithTraceID(traceID string) *Logger {
	return ul.WithField("trace_id", traceID)
}

// WithSpanID adds a span ID to the logger context.
func (ul *Logger) WithSpanID(spanID string) *Logger {
	return ul.WithField("span_id", spanID)
}

// WithRequestID adds a request ID to the logger context.
func (ul *Logger) WithRequestID(requestID string) *Logger {
	return ul.WithField("request_id", requestID)
}

// WithUserID adds a user ID to the logger context.
func (ul *Logger) WithUserID(userID string) *Logger {
	return ul.WithField("user_id", userID)
}

// Info returns a log event for info level logging.
func (ul *Logger) Info() Event {
	ul.updateMetrics(LevelInfo)
	return &EventAdapter{event: ul.zerologLogger.Info()}
}

// Error returns a log event for error level logging.
func (ul *Logger) Error() Event {
	ul.updateMetrics(LevelError)
	return &EventAdapter{event: ul.zerologLogger.Error()}
}

// Debug returns a log event for debug level logging.
func (ul *Logger) Debug() Event {
	ul.updateMetrics(LevelDebug)
	return &EventAdapter{event: ul.zerologLogger.Debug()}
}

// Warn returns a log event for warning level logging.
func (ul *Logger) Warn() Event {
	ul.updateMetrics(LevelWarn)
	return &EventAdapter{event: ul.zerologLogger.Warn()}
}

// updateMetrics updates internal logging metrics.
func (ul *Logger) updateMetrics(level Level) {
	if ul.metrics == nil {
		return
	}

	ul.mu.Lock()
	defer ul.mu.Unlock()

	ul.metrics.TotalLogs++
	ul.metrics.LastLogTime = time.Now()

	switch level {
	case LevelDebug:
		ul.metrics.DebugLogs++
	case LevelInfo:
		ul.metrics.InfoLogs++
	case LevelWarn:
		ul.metrics.WarningLogs++
	case LevelError:
		ul.metrics.ErrorLogs++
	}
}

// GetMetrics returns current logging metrics.
func (ul *Logger) GetMetrics() LogMetrics {
	if ul.metrics == nil {
		return LogMetrics{}
	}

	ul.mu.RLock()
	defer ul.mu.RUnlock()
	return *ul.metrics
}

// Counter logs a counter metric.
func (ul *Logger) Counter(name string, value int64, tags map[string]string) {
	event := ul.zerologLogger.Info().
		Str("metric_type", "counter").
		Str("metric_name", name).
		Int64("metric_value", value)

	for k, v := range tags {
		event = event.Str("tag_"+k, v)
	}

	event.Msg("metric recorded")
}

// Gauge logs a gauge metric.
func (ul *Logger) Gauge(name string, value float64, tags map[string]string) {
	event := ul.zerologLogger.Info().
		Str("metric_type", "gauge").
		Str("metric_name", name).
		Float64("metric_value", value)

	for k, v := range tags {
		event = event.Str("tag_"+k, v)
	}

	event.Msg("metric recorded")
}

// Timer logs a timer metric.
func (ul *Logger) Timer(name string, duration time.Duration, tags map[string]string) {
	event := ul.zerologLogger.Info().
		Str("metric_type", "timer").
		Str("metric_name", name).
		Dur("metric_value", duration)

	for k, v := range tags {
		event = event.Str("tag_"+k, v)
	}

	event.Msg("metric recorded")
}

// Histogram logs a histogram metric.
func (ul *Logger) Histogram(name string, value float64, tags map[string]string) {
	event := ul.zerologLogger.Info().
		Str("metric_type", "histogram").
		Str("metric_name", name).
		Float64("metric_value", value)

	for k, v := range tags {
		event = event.Str("tag_"+k, v)
	}

	event.Msg("metric recorded")
}
