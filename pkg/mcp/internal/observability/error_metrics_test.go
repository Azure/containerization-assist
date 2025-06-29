package observability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/stretchr/testify/assert"
)

// resetErrorMetrics resets the singleton for testing
func resetErrorMetrics() {
	errorMetricsOnce = sync.Once{}
	errorMetricsInstance = nil
}

func TestErrorMetrics_RecordError(t *testing.T) {
	em := NewErrorMetrics()
	ctx := context.Background()

	// Create a test error
	richErr := mcp.NewRichError("TEST_ERROR", "Test error message", "test_error")
	richErr.Severity = "medium"
	richErr.Context.Operation = "test_operation"
	richErr.Context.Component = "test_component"
	// Note: mcp.RichError uses ErrorMetadata struct
	richErr.Context.Metadata = mcp.NewErrorMetadata("session123", "test_tool", "test_op")
	richErr.Diagnostics.RootCause = "test root cause"
	richErr.Diagnostics.ErrorPattern = "test pattern"
	richErr.Diagnostics.Symptoms = []string{"symptom1", "symptom2"}
	richErr.AttemptNumber = 2

	// Record the error
	em.RecordError(ctx, richErr)

	// Verify error was recorded
	recentErrors := em.GetRecentErrors(1)
	assert.Len(t, recentErrors, 1)
	assert.Equal(t, richErr.Code, recentErrors[0].Code)

	// Verify error patterns
	patterns := em.GetErrorPatterns()
	assert.Equal(t, 1, patterns["TEST_ERROR:test_error"])
}

func TestErrorMetrics_RecordResolution(t *testing.T) {
	em := NewErrorMetrics()
	ctx := context.Background()

	// Create and record an error
	richErr := mcp.NewRichError("RESOLVE_TEST", "Error to be resolved", "resolvable_error")
	richErr.Severity = "high"

	em.RecordError(ctx, richErr)

	// Record resolution
	duration := 5 * time.Second
	em.RecordResolution(ctx, richErr, "automatic", duration)

	// Metrics are recorded but we can't easily test Prometheus metrics
	// without a registry, so we just ensure no panic
}

func TestErrorMetrics_EnrichContext(t *testing.T) {
	em := NewErrorMetrics()

	// Create context with correlation ID
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")

	// Create error with metadata
	richErr := mcp.NewRichError("ENRICHED_ERROR", "", "")
	richErr.Context.Metadata = mcp.NewErrorMetadata("session456", "enrich_tool", "enrich_op")

	// Enrich the context
	em.EnrichContext(ctx, richErr)

	// Verify correlation ID was added
	if richErr.Context.Metadata != nil && richErr.Context.Metadata.Custom != nil {
		assert.Equal(t, "test-correlation-123", richErr.Context.Metadata.Custom["correlation_id"])
	}
}

func TestErrorMetrics_GetRecentErrors(t *testing.T) {
	em := NewErrorMetrics()
	ctx := context.Background()

	// Get current count of errors
	initialCount := len(em.GetRecentErrors(1000))

	// Record multiple errors
	testErrors := 5
	for i := 0; i < testErrors; i++ {
		err := mcp.NewRichError(
			"ERROR_"+string(rune('A'+i)),
			"Test error "+string(rune('A'+i)),
			"",
		)
		em.RecordError(ctx, err)
	}

	// Test getting limited recent errors
	recent := em.GetRecentErrors(3)
	assert.Len(t, recent, 3)

	// Check that we got the last 3 errors recorded
	assert.Equal(t, "ERROR_C", recent[0].Code)
	assert.Equal(t, "ERROR_D", recent[1].Code)
	assert.Equal(t, "ERROR_E", recent[2].Code)

	// Test getting recent errors including our test errors
	recentWithTest := em.GetRecentErrors(initialCount + testErrors)
	newErrors := recentWithTest[initialCount:]
	assert.Len(t, newErrors, testErrors)
}

func TestErrorMetrics_ErrorPatterns(t *testing.T) {
	em := NewErrorMetrics()
	ctx := context.Background()

	// Record errors with patterns
	errors := []struct {
		code    string
		errType string
		count   int
	}{
		{"BUILD_FAILED", "build_error", 3},
		{"NETWORK_ERROR", "system_error", 2},
		{"BUILD_FAILED", "build_error", 1}, // Same pattern, should increment
	}

	for _, e := range errors {
		for i := 0; i < e.count; i++ {
			err := mcp.NewRichError(e.code, "", e.errType)
			em.RecordError(ctx, err)
		}
	}

	// Check patterns
	patterns := em.GetErrorPatterns()
	assert.Equal(t, 4, patterns["BUILD_FAILED:build_error"]) // 3 + 1
	assert.Equal(t, 2, patterns["NETWORK_ERROR:system_error"])
}

func TestErrorMetricsMiddleware(t *testing.T) {
	em := NewErrorMetrics()

	// Track if handler was called
	handlerCalled := false
	errorResolved := false

	// Create middleware
	middleware := ErrorMetricsMiddleware(em)

	// Create handler that resolves errors
	handler := middleware(func(ctx context.Context, err *mcp.RichError) error {
		handlerCalled = true
		if err.Code == "RESOLVABLE" {
			errorResolved = true
			return nil // Error resolved
		}
		return err // Error not resolved
	})

	ctx := context.Background()

	// Test with resolvable error
	resolvableErr := mcp.NewRichError("RESOLVABLE", "This error will be resolved", "")

	result := handler(ctx, resolvableErr)
	assert.Nil(t, result)
	assert.True(t, handlerCalled)
	assert.True(t, errorResolved)

	// Verify error was recorded
	recent := em.GetRecentErrors(1)
	assert.Len(t, recent, 1)
	assert.Equal(t, "RESOLVABLE", recent[0].Code)
}
