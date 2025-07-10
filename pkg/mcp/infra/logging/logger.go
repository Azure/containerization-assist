package logging

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Logger provides structured logging with ring buffer capability using slog.
type Logger struct {
	slogLogger *slog.Logger
	ringBuffer *RingBuffer
	config     Config
	capture    *LogCapture
	mu         sync.RWMutex
	metrics    *LogMetrics
}

// EventAdapter adapts slog operations to the Event interface.
type EventAdapter struct {
	logger  *slog.Logger
	level   slog.Level
	attrs   []slog.Attr
	message string
}

// Str adds a string field to the log event.
func (ea *EventAdapter) Str(key, value string) Event {
	ea.attrs = append(ea.attrs, slog.String(key, value))
	return ea
}

// Err adds an error field to the log event.
func (ea *EventAdapter) Err(err error) Event {
	if err != nil {
		ea.attrs = append(ea.attrs, slog.Any("error", err))
	}
	return ea
}

// Int adds an integer field to the log event.
func (ea *EventAdapter) Int(key string, value int) Event {
	ea.attrs = append(ea.attrs, slog.Int(key, value))
	return ea
}

// Float64 adds a float64 field to the log event.
func (ea *EventAdapter) Float64(key string, value float64) Event {
	ea.attrs = append(ea.attrs, slog.Float64(key, value))
	return ea
}

// Bool adds a boolean field to the log event.
func (ea *EventAdapter) Bool(key string, value bool) Event {
	ea.attrs = append(ea.attrs, slog.Bool(key, value))
	return ea
}

// Msg logs the event with the given message.
func (ea *EventAdapter) Msg(msg string) {
	ea.logger.LogAttrs(context.Background(), ea.level, msg, ea.attrs...)
}

// Event defines the interface for log events.
type Event interface {
	Str(key, value string) Event
	Err(err error) Event
	Int(key string, value int) Event
	Float64(key string, value float64) Event
	Bool(key string, value bool) Event
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
	Timestamp time.Time
	Level     Level
	Message   string
	Fields    map[string]interface{}
	Caller    string
	Error     error
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
			RetentionTime: 24 * time.Hour,
			BufferSize:    config.BufferSize,
		}

		logCapture = &LogCapture{
			ringBuffer: ringBuffer,
			config:     captureConfig,
			active:     true,
		}
	}

	// Configure slog
	opts := &slog.HandlerOptions{
		Level:     toSlogLevel(config.Level),
		AddSource: config.EnableCaller,
	}

	var handler slog.Handler
	if config.EnableStructuredLogging {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	// Add default fields
	if len(config.Fields) > 0 {
		attrs := make([]slog.Attr, 0, len(config.Fields))
		for k, v := range config.Fields {
			attrs = append(attrs, slog.Any(k, v))
		}
		handler = handler.WithAttrs(attrs)
	}

	slogLogger := slog.New(handler)

	// Initialize metrics
	metrics := &LogMetrics{
		LastLogTime: time.Now(),
	}

	return &Logger{
		slogLogger: slogLogger,
		ringBuffer: ringBuffer,
		capture:    logCapture,
		config:     config,
		metrics:    metrics,
	}
}

// toSlogLevel converts our Level to slog.Level.
func toSlogLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelFatal:
		return slog.LevelError + 4 // slog doesn't have fatal, use higher than error
	default:
		return slog.LevelInfo
	}
}

// GetRecentLogs returns recent log entries from the ring buffer.
func (l *Logger) GetRecentLogs() []LogEntry {
	if l.ringBuffer == nil {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.ringBuffer.GetAll()
}

// GetLogsSince returns log entries since the specified time.
func (l *Logger) GetLogsSince(since time.Time) []LogEntry {
	if l.ringBuffer == nil {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	allEntries := l.ringBuffer.GetAll()
	var result []LogEntry

	for _, entry := range allEntries {
		if entry.Timestamp.After(since) {
			result = append(result, entry)
		}
	}

	return result
}

// GetLogsByLevel returns log entries with the specified level.
func (l *Logger) GetLogsByLevel(level Level) []LogEntry {
	if l.ringBuffer == nil {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	allEntries := l.ringBuffer.GetAll()
	var result []LogEntry

	for _, entry := range allEntries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}

	return result
}

// Clear clears the captured logs.
func (l *Logger) Clear() {
	if l.ringBuffer == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.ringBuffer.Clear()
}

// Size returns the number of captured log entries.
func (l *Logger) Size() int {
	if l.ringBuffer == nil {
		return 0
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.ringBuffer.Size()
}

// WithField adds a structured field to the logger context.
func (l *Logger) WithField(key string, value interface{}) Standards {
	return &Logger{
		slogLogger: l.slogLogger.With(key, value),
		ringBuffer: l.ringBuffer,
		capture:    l.capture,
		config:     l.config,
		metrics:    l.metrics,
	}
}

// WithFields adds multiple fields to the logger context.
func (l *Logger) WithFields(fields map[string]interface{}) Standards {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &Logger{
		slogLogger: l.slogLogger.With(args...),
		ringBuffer: l.ringBuffer,
		capture:    l.capture,
		config:     l.config,
		metrics:    l.metrics,
	}
}

// WithError adds an error field to the logger context.
func (l *Logger) WithError(err error) Standards {
	return &Logger{
		slogLogger: l.slogLogger.With("error", err),
		ringBuffer: l.ringBuffer,
		capture:    l.capture,
		config:     l.config,
		metrics:    l.metrics,
	}
}

// WithComponent adds a component name to the logger context.
func (l *Logger) WithComponent(component string) Standards {
	return l.WithField("component", component)
}

// WithTraceID adds a trace ID to the logger context.
func (l *Logger) WithTraceID(traceID string) Standards {
	return l.WithField("trace_id", traceID)
}

// WithSpanID adds a span ID to the logger context.
func (l *Logger) WithSpanID(spanID string) Standards {
	return l.WithField("span_id", spanID)
}

// WithRequestID adds a request ID to the logger context.
func (l *Logger) WithRequestID(requestID string) Standards {
	return l.WithField("request_id", requestID)
}

// WithUserID adds a user ID to the logger context.
func (l *Logger) WithUserID(userID string) Standards {
	return l.WithField("user_id", userID)
}

// Info returns a log event for info level logging.
func (l *Logger) Info() Event {
	l.updateMetrics(LevelInfo)
	return &EventAdapter{
		logger: l.slogLogger,
		level:  slog.LevelInfo,
		attrs:  []slog.Attr{},
	}
}

// Error returns a log event for error level logging.
func (l *Logger) Error() Event {
	l.updateMetrics(LevelError)
	return &EventAdapter{
		logger: l.slogLogger,
		level:  slog.LevelError,
		attrs:  []slog.Attr{},
	}
}

// Debug returns a log event for debug level logging.
func (l *Logger) Debug() Event {
	l.updateMetrics(LevelDebug)
	return &EventAdapter{
		logger: l.slogLogger,
		level:  slog.LevelDebug,
		attrs:  []slog.Attr{},
	}
}

// Warn returns a log event for warning level logging.
func (l *Logger) Warn() Event {
	l.updateMetrics(LevelWarn)
	return &EventAdapter{
		logger: l.slogLogger,
		level:  slog.LevelWarn,
		attrs:  []slog.Attr{},
	}
}

// updateMetrics updates internal logging metrics.
func (l *Logger) updateMetrics(level Level) {
	if l.metrics == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.metrics.TotalLogs++
	l.metrics.LastLogTime = time.Now()

	switch level {
	case LevelDebug:
		l.metrics.DebugLogs++
	case LevelInfo:
		l.metrics.InfoLogs++
	case LevelWarn:
		l.metrics.WarningLogs++
	case LevelError:
		l.metrics.ErrorLogs++
	}
}

// GetMetrics returns current logging metrics.
func (l *Logger) GetMetrics() LogMetrics {
	if l.metrics == nil {
		return LogMetrics{}
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return *l.metrics
}

// Counter logs a counter metric.
func (l *Logger) Counter(name string, value int64, tags map[string]string) {
	attrs := []slog.Attr{
		slog.String("metric_type", "counter"),
		slog.String("metric_name", name),
		slog.Int64("metric_value", value),
	}

	for k, v := range tags {
		attrs = append(attrs, slog.String("tag_"+k, v))
	}

	l.slogLogger.LogAttrs(context.Background(), slog.LevelInfo, "metric recorded", attrs...)
}

// Gauge logs a gauge metric.
func (l *Logger) Gauge(name string, value float64, tags map[string]string) {
	attrs := []slog.Attr{
		slog.String("metric_type", "gauge"),
		slog.String("metric_name", name),
		slog.Float64("metric_value", value),
	}

	for k, v := range tags {
		attrs = append(attrs, slog.String("tag_"+k, v))
	}

	l.slogLogger.LogAttrs(context.Background(), slog.LevelInfo, "metric recorded", attrs...)
}

// Timer logs a timer metric.
func (l *Logger) Timer(name string, duration time.Duration, tags map[string]string) {
	attrs := []slog.Attr{
		slog.String("metric_type", "timer"),
		slog.String("metric_name", name),
		slog.Duration("metric_value", duration),
	}

	for k, v := range tags {
		attrs = append(attrs, slog.String("tag_"+k, v))
	}

	l.slogLogger.LogAttrs(context.Background(), slog.LevelInfo, "metric recorded", attrs...)
}

// Histogram logs a histogram metric.
func (l *Logger) Histogram(name string, value float64, tags map[string]string) {
	attrs := []slog.Attr{
		slog.String("metric_type", "histogram"),
		slog.String("metric_name", name),
		slog.Float64("metric_value", value),
	}

	for k, v := range tags {
		attrs = append(attrs, slog.String("tag_"+k, v))
	}

	l.slogLogger.LogAttrs(context.Background(), slog.LevelInfo, "metric recorded", attrs...)
}
