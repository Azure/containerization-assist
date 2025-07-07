package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// RequestIDKey is the context key for storing request IDs
type RequestIDKey struct{}

// RequestLogger provides structured logging with request ID correlation
type RequestLogger struct {
	logger        zerolog.Logger
	component     string
	correlations  map[string]*RequestContext
	mu            sync.RWMutex
	maxRetention  time.Duration
	cleanupTicker *time.Ticker
	done          chan struct{}
}

// RequestContext holds context information for request correlation
type RequestContext struct {
	RequestID   string                 `json:"request_id"`
	SessionID   string                 `json:"session_id,omitempty"`
	ToolName    string                 `json:"tool_name,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	TraceEvents []TraceEvent           `json:"trace_events,omitempty"`
}

// TraceEvent represents a trace event within a request
type TraceEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewRequestLogger creates a new request logger with correlation support
func NewRequestLogger(component string, level zerolog.Level) *RequestLogger {
	// Create a zerolog logger with the specified level and component
	logger := zerolog.New(os.Stderr).
		Level(level).
		With().
		Str("component", component).
		Timestamp().
		Logger()

	rl := &RequestLogger{
		logger:       logger,
		component:    component,
		correlations: make(map[string]*RequestContext),
		maxRetention: 1 * time.Hour, // Keep correlation data for 1 hour
		done:         make(chan struct{}),
	}

	rl.cleanupTicker = time.NewTicker(10 * time.Minute)
	go rl.cleanupCorrelations()

	return rl
}

// GenerateRequestID generates a new request ID
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req_%s", hex.EncodeToString(bytes))
}

// WithRequestID adds a request ID to the context and starts correlation tracking
func (rl *RequestLogger) WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = GenerateRequestID()
	}

	reqCtx := &RequestContext{
		RequestID: requestID,
		StartTime: time.Now(),
		Status:    "started",
		Metadata:  make(map[string]interface{}),
	}

	rl.mu.Lock()
	rl.correlations[requestID] = reqCtx
	rl.mu.Unlock()

	return context.WithValue(ctx, RequestIDKey{}, requestID)
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return requestID
	}
	return ""
}

// UpdateRequestContext updates the correlation data for a request
func (rl *RequestLogger) UpdateRequestContext(ctx context.Context, updates func(*RequestContext)) {
	requestID := GetRequestID(ctx)
	if requestID == "" {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if reqCtx, exists := rl.correlations[requestID]; exists {
		updates(reqCtx)
	}
}

// AddTraceEvent adds a trace event to the request context
func (rl *RequestLogger) AddTraceEvent(ctx context.Context, event string, metadata map[string]interface{}) {
	rl.UpdateRequestContext(ctx, func(reqCtx *RequestContext) {
		traceEvent := TraceEvent{
			Timestamp: time.Now(),
			Event:     event,
			Metadata:  metadata,
		}
		if len(reqCtx.TraceEvents) > 0 {
			lastEvent := reqCtx.TraceEvents[len(reqCtx.TraceEvents)-1]
			traceEvent.Duration = time.Since(lastEvent.Timestamp)
		}
		reqCtx.TraceEvents = append(reqCtx.TraceEvents, traceEvent)
	})
}

// LogWithRequestID logs a message with request correlation
func (rl *RequestLogger) LogWithRequestID(ctx context.Context, level zerolog.Level, msg string, args ...interface{}) {
	requestID := GetRequestID(ctx)

	logArgs := []interface{}{"request_id", requestID}
	if requestID != "" {
		rl.mu.RLock()
		if reqCtx, exists := rl.correlations[requestID]; exists {
			if reqCtx.SessionID != "" {
				logArgs = append(logArgs, "session_id", reqCtx.SessionID)
			}
			if reqCtx.ToolName != "" {
				logArgs = append(logArgs, "tool_name", reqCtx.ToolName)
			}
			if reqCtx.UserID != "" {
				logArgs = append(logArgs, "user_id", reqCtx.UserID)
			}
			if reqCtx.Status != "" {
				logArgs = append(logArgs, "status", reqCtx.Status)
			}
		}
		rl.mu.RUnlock()
	}

	logArgs = append(logArgs, args...)
	fields := make(map[string]interface{})
	for i := 0; i < len(logArgs)-1; i += 2 {
		if key, ok := logArgs[i].(string); ok && i+1 < len(logArgs) {
			fields[key] = logArgs[i+1]
		}
	}

	var event *zerolog.Event
	switch level {
	case zerolog.DebugLevel:
		event = rl.logger.Debug()
	case zerolog.InfoLevel:
		event = rl.logger.Info()
	case zerolog.WarnLevel:
		event = rl.logger.Warn()
	case zerolog.ErrorLevel:
		event = rl.logger.Error()
	default:
		event = rl.logger.Info()
	}

	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg(msg)
}

// Info logs an info message with request correlation
func (rl *RequestLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	rl.LogWithRequestID(ctx, zerolog.InfoLevel, msg, args...)
}

// Error logs an error message with request correlation
func (rl *RequestLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	rl.LogWithRequestID(ctx, zerolog.ErrorLevel, msg, args...)
}

// Warn logs a warning message with request correlation
func (rl *RequestLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	rl.LogWithRequestID(ctx, zerolog.WarnLevel, msg, args...)
}

// Debug logs a debug message with request correlation
func (rl *RequestLogger) Debug(ctx context.Context, msg string, args ...interface{}) {
	rl.LogWithRequestID(ctx, zerolog.DebugLevel, msg, args...)
}

// StartOperation logs the start of an operation with timing
func (rl *RequestLogger) StartOperation(ctx context.Context, operation string, metadata map[string]interface{}) {
	rl.AddTraceEvent(ctx, fmt.Sprintf("start_%s", operation), metadata)
	rl.Info(ctx, fmt.Sprintf("Starting %s", operation), "operation", operation)
}

// EndOperation logs the end of an operation with timing and status
func (rl *RequestLogger) EndOperation(ctx context.Context, operation string, success bool, err error) {
	status := "success"
	if !success || err != nil {
		status = "failure"
	}

	metadata := map[string]interface{}{
		"success": success,
	}
	if err != nil {
		metadata["error"] = err.Error()
	}

	rl.AddTraceEvent(ctx, fmt.Sprintf("end_%s", operation), metadata)

	if success {
		rl.Info(ctx, fmt.Sprintf("Completed %s", operation), "operation", operation, "status", status)
	} else {
		rl.Error(ctx, fmt.Sprintf("Failed %s", operation), "operation", operation, "status", status, "error", err)
	}
}

// FinishRequest marks a request as completed and logs final metrics
func (rl *RequestLogger) FinishRequest(ctx context.Context, success bool, err error) {
	requestID := GetRequestID(ctx)
	if requestID == "" {
		return
	}

	rl.UpdateRequestContext(ctx, func(reqCtx *RequestContext) {
		reqCtx.Duration = time.Since(reqCtx.StartTime)
		if success {
			reqCtx.Status = "completed"
		} else {
			reqCtx.Status = "failed"
			if err != nil {
				reqCtx.Error = err.Error()
			}
		}
	})

	if success {
		rl.Info(ctx, "Request completed",
			"success", true,
			"duration_ms", time.Since(rl.getRequestStartTime(requestID)).Milliseconds())
	} else {
		rl.Error(ctx, "Request failed",
			"success", false,
			"duration_ms", time.Since(rl.getRequestStartTime(requestID)).Milliseconds(),
			"error", err)
	}
}

// GetRequestContext retrieves the full context for a request
func (rl *RequestLogger) GetRequestContext(requestID string) (*RequestContext, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	reqCtx, exists := rl.correlations[requestID]
	if !exists {
		return nil, false
	}

	copy := *reqCtx
	copy.TraceEvents = make([]TraceEvent, len(reqCtx.TraceEvents))
	for i, event := range reqCtx.TraceEvents {
		copy.TraceEvents[i] = event
	}

	return &copy, true
}

// GetAllActiveRequests returns all currently tracked requests
func (rl *RequestLogger) GetAllActiveRequests() map[string]*RequestContext {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	result := make(map[string]*RequestContext)
	for id, ctx := range rl.correlations {
		copy := *ctx
		result[id] = &copy
	}

	return result
}

// getRequestStartTime safely retrieves the start time for a request
func (rl *RequestLogger) getRequestStartTime(requestID string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if reqCtx, exists := rl.correlations[requestID]; exists {
		return reqCtx.StartTime
	}
	return time.Now() // Fallback if not found
}

// cleanupCorrelations removes old correlation data to prevent memory leaks
func (rl *RequestLogger) cleanupCorrelations() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.maxRetention)
			for id, reqCtx := range rl.correlations {
				if reqCtx.StartTime.Before(cutoff) {
					delete(rl.correlations, id)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// Close stops the background cleanup routine
func (rl *RequestLogger) Close() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
	close(rl.done)
}

// GetMetrics returns logging and correlation metrics
func (rl *RequestLogger) GetMetrics() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	activeRequests := len(rl.correlations)
	completedCount := 0
	failedCount := 0
	avgDuration := time.Duration(0)
	totalDuration := time.Duration(0)

	for _, reqCtx := range rl.correlations {
		if reqCtx.Status == "completed" {
			completedCount++
		} else if reqCtx.Status == "failed" {
			failedCount++
		}
		if reqCtx.Duration > 0 {
			totalDuration += reqCtx.Duration
		}
	}

	if activeRequests > 0 {
		avgDuration = totalDuration / time.Duration(activeRequests)
	}

	return map[string]interface{}{
		"component":       rl.component,
		"active_requests": activeRequests,
		"completed_count": completedCount,
		"failed_count":    failedCount,
		"avg_duration_ms": avgDuration.Milliseconds(),
		"retention_hours": rl.maxRetention.Hours(),
	}
}
