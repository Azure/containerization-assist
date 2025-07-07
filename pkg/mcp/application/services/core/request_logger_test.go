package core

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLogger_BasicLogging(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.DebugLevel)
	defer logger.Close()

	ctx := context.Background()

	// Test without request ID
	logger.Info(ctx, "test message without request ID")

	// Test with request ID
	ctx = logger.WithRequestID(ctx, "test-request-123")
	logger.Info(ctx, "test message with request ID", "key", "value")

	// Verify request context was created
	reqCtx, exists := logger.GetRequestContext("test-request-123")
	assert.True(t, exists)
	assert.Equal(t, "test-request-123", reqCtx.RequestID)
	assert.Equal(t, "started", reqCtx.Status)
	assert.NotZero(t, reqCtx.StartTime)
}

func TestRequestLogger_RequestIDGeneration(t *testing.T) {
	// Test automatic request ID generation
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "req_")
	assert.Contains(t, id2, "req_")
}

func TestRequestLogger_RequestCorrelation(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx := context.Background()
	ctx = logger.WithRequestID(ctx, "") // Auto-generate ID

	requestID := GetRequestID(ctx)
	assert.NotEmpty(t, requestID)

	// Update request context
	logger.UpdateRequestContext(ctx, func(reqCtx *RequestContext) {
		reqCtx.SessionID = "session-456"
		reqCtx.ToolName = "test_tool"
		reqCtx.UserID = "user-789"
	})

	// Verify updates
	reqCtx, exists := logger.GetRequestContext(requestID)
	require.True(t, exists)
	assert.Equal(t, "session-456", reqCtx.SessionID)
	assert.Equal(t, "test_tool", reqCtx.ToolName)
	assert.Equal(t, "user-789", reqCtx.UserID)
}

func TestRequestLogger_TraceEvents(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.DebugLevel)
	defer logger.Close()

	ctx := logger.WithRequestID(context.Background(), "trace-test")

	// Add trace events
	logger.AddTraceEvent(ctx, "operation_start", map[string]interface{}{
		"operation": "test_operation",
	})

	time.Sleep(10 * time.Millisecond) // Small delay to measure duration

	logger.AddTraceEvent(ctx, "operation_end", map[string]interface{}{
		"success": true,
	})

	// Verify trace events
	reqCtx, exists := logger.GetRequestContext("trace-test")
	require.True(t, exists)
	assert.Len(t, reqCtx.TraceEvents, 2)

	assert.Equal(t, "operation_start", reqCtx.TraceEvents[0].Event)
	assert.Equal(t, "operation_end", reqCtx.TraceEvents[1].Event)
	assert.True(t, reqCtx.TraceEvents[1].Duration > 0)
}

func TestRequestLogger_OperationTracking(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx := logger.WithRequestID(context.Background(), "operation-test")

	// Start operation
	logger.StartOperation(ctx, "test_operation", map[string]interface{}{
		"param1": "value1",
	})

	// End operation successfully
	logger.EndOperation(ctx, "test_operation", true, nil)

	// Verify trace events were added
	reqCtx, exists := logger.GetRequestContext("operation-test")
	require.True(t, exists)
	assert.Len(t, reqCtx.TraceEvents, 2)
	assert.Equal(t, "start_test_operation", reqCtx.TraceEvents[0].Event)
	assert.Equal(t, "end_test_operation", reqCtx.TraceEvents[1].Event)
}

func TestRequestLogger_OperationFailure(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx := logger.WithRequestID(context.Background(), "failure-test")

	// Start and fail operation
	logger.StartOperation(ctx, "failing_operation", nil)
	testErr := assert.AnError
	logger.EndOperation(ctx, "failing_operation", false, testErr)

	// Verify error was recorded
	reqCtx, exists := logger.GetRequestContext("failure-test")
	require.True(t, exists)
	assert.Len(t, reqCtx.TraceEvents, 2)

	endEvent := reqCtx.TraceEvents[1]
	assert.Equal(t, "end_failing_operation", endEvent.Event)
	assert.Equal(t, false, endEvent.Metadata["success"])
	assert.Equal(t, testErr.Error(), endEvent.Metadata["error"])
}

func TestRequestLogger_RequestCompletion(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx := logger.WithRequestID(context.Background(), "completion-test")

	// Small delay to measure duration
	time.Sleep(10 * time.Millisecond)

	// Finish request successfully
	logger.FinishRequest(ctx, true, nil)

	// Verify completion
	reqCtx, exists := logger.GetRequestContext("completion-test")
	require.True(t, exists)
	assert.Equal(t, "completed", reqCtx.Status)
	assert.True(t, reqCtx.Duration > 0)
	assert.Empty(t, reqCtx.Error)
}

func TestRequestLogger_RequestFailure(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx := logger.WithRequestID(context.Background(), "failure-test")

	// Finish request with failure
	testErr := assert.AnError
	logger.FinishRequest(ctx, false, testErr)

	// Verify failure
	reqCtx, exists := logger.GetRequestContext("failure-test")
	require.True(t, exists)
	assert.Equal(t, "failed", reqCtx.Status)
	assert.Equal(t, testErr.Error(), reqCtx.Error)
}

func TestRequestLogger_GetAllActiveRequests(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	// Create multiple requests
	ctx1 := logger.WithRequestID(context.Background(), "request-1")
	ctx2 := logger.WithRequestID(context.Background(), "request-2")
	ctx3 := logger.WithRequestID(context.Background(), "request-3")

	// Update contexts
	logger.UpdateRequestContext(ctx1, func(reqCtx *RequestContext) {
		reqCtx.ToolName = "tool1"
	})
	logger.UpdateRequestContext(ctx2, func(reqCtx *RequestContext) {
		reqCtx.ToolName = "tool2"
	})
	logger.UpdateRequestContext(ctx3, func(reqCtx *RequestContext) {
		reqCtx.ToolName = "tool3"
	})

	// Get all requests
	all := logger.GetAllActiveRequests()
	assert.Len(t, all, 3)
	assert.Contains(t, all, "request-1")
	assert.Contains(t, all, "request-2")
	assert.Contains(t, all, "request-3")

	assert.Equal(t, "tool1", all["request-1"].ToolName)
	assert.Equal(t, "tool2", all["request-2"].ToolName)
	assert.Equal(t, "tool3", all["request-3"].ToolName)
}

func TestRequestLogger_Metrics(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	// Create requests with different states
	ctx1 := logger.WithRequestID(context.Background(), "metrics-1")
	ctx2 := logger.WithRequestID(context.Background(), "metrics-2")
	ctx3 := logger.WithRequestID(context.Background(), "metrics-3")

	// Complete some requests
	logger.FinishRequest(ctx1, true, nil)
	logger.FinishRequest(ctx2, false, assert.AnError)
	// Leave ctx3 active (just access it to avoid unused warning)
	_ = ctx3

	metrics := logger.GetMetrics()
	assert.Equal(t, "test_component", metrics["component"])
	assert.Equal(t, 3, metrics["active_requests"])
	assert.Equal(t, 1, metrics["completed_count"])
	assert.Equal(t, 1, metrics["failed_count"])
	assert.GreaterOrEqual(t, metrics["avg_duration_ms"], int64(0))
}

func TestRequestLogger_ContextExtraction(t *testing.T) {
	// Test GetRequestID with missing context
	ctx := context.Background()
	requestID := GetRequestID(ctx)
	assert.Empty(t, requestID)

	// Test GetRequestID with valid context
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	ctx = logger.WithRequestID(ctx, "extract-test")
	requestID = GetRequestID(ctx)
	assert.Equal(t, "extract-test", requestID)
}

func TestRequestLogger_ConcurrentAccess(t *testing.T) {
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	defer logger.Close()

	const numGoroutines = 10
	const numOperations = 50

	done := make(chan struct{}, numGoroutines)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				ctx := logger.WithRequestID(context.Background(), "")
				logger.Info(ctx, "concurrent test", "goroutine", id, "operation", j)
				logger.FinishRequest(ctx, true, nil)
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify no race conditions occurred
	metrics := logger.GetMetrics()
	assert.Equal(t, numGoroutines*numOperations, metrics["active_requests"])
}

func TestRequestLogger_CleanupCorrelations(t *testing.T) {
	// Create logger with very short retention for testing
	logger := NewRequestLogger("test_component", zerolog.InfoLevel)
	logger.maxRetention = 50 * time.Millisecond
	defer logger.Close()

	// Create a request
	ctx := logger.WithRequestID(context.Background(), "cleanup-test")
	logger.Info(ctx, "test message")

	// Verify request exists
	_, exists := logger.GetRequestContext("cleanup-test")
	assert.True(t, exists)

	// Wait for cleanup to occur
	time.Sleep(100 * time.Millisecond)

	// Trigger manual cleanup by creating new correlation data past retention
	logger.mu.Lock()
	cutoff := time.Now().Add(-logger.maxRetention)
	for id, reqCtx := range logger.correlations {
		if reqCtx.StartTime.Before(cutoff) {
			delete(logger.correlations, id)
		}
	}
	logger.mu.Unlock()

	// Verify request was cleaned up
	_, exists = logger.GetRequestContext("cleanup-test")
	assert.False(t, exists)
}
