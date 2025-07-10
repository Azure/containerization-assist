package logging

import "time"

// Standards defines the standard interface that all logging implementations
// must adhere to throughout the Container Kit codebase.
//
// All logging in the codebase should use this interface to ensure consistency
// and allow for easy switching between logging backends if needed.
//
// Standard Usage:
//   - Use Info() for general informational messages
//   - Use Debug() for debugging information (hidden in production)
//   - Use Warn() for warning conditions that should be investigated
//   - Use Error() for error conditions that need immediate attention
//
// Context Fields:
//   - Always include relevant context using structured fields
//   - Use WithComponent() to identify the logging component
//   - Use WithTraceID() for distributed tracing
//   - Use WithRequestID() for request correlation
//   - Use WithError() to attach error details
//
// Example:
//
//	logger.WithComponent("docker-builder").
//	    WithRequestID(reqID).
//	    Info().
//	    Str("image", imageName).
//	    Msg("Building Docker image")
type Standards interface {
	// Core logging methods
	Info() Event
	Debug() Event
	Warn() Event
	Error() Event

	// Context methods for structured logging
	WithField(key string, value interface{}) Standards
	WithFields(fields map[string]interface{}) Standards
	WithError(err error) Standards
	WithComponent(component string) Standards
	WithTraceID(traceID string) Standards
	WithSpanID(spanID string) Standards
	WithRequestID(requestID string) Standards
	WithUserID(userID string) Standards

	// Metrics logging (for observability)
	Counter(name string, value int64, tags map[string]string)
	Gauge(name string, value float64, tags map[string]string)
	Timer(name string, duration time.Duration, tags map[string]string)
	Histogram(name string, value float64, tags map[string]string)

	// Log capture and inspection (for testing and debugging)
	GetRecentLogs() []LogEntry
	GetLogsSince(since time.Time) []LogEntry
	GetLogsByLevel(level Level) []LogEntry
	Clear()
	Size() int
	GetMetrics() LogMetrics
}

// Verify that our Logger implements Standards
var _ Standards = (*Logger)(nil)
