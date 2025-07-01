package observability

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// resetErrorMetrics resets the singleton for testing
func resetErrorMetrics() {
	errorMetricsOnce = sync.Once{}
	errorMetricsInstance = nil
}

func TestErrorMetrics_RecordError(t *testing.T) {
	em := NewErrorMetricsForTesting()
	ctx := context.Background()

	// Create a test error
	richErr := fmt.Errorf("test error")

	// Record the error
	em.RecordError(ctx, richErr)

	// Verify error was recorded
	recent := em.GetRecentErrors(1)
	assert.Equal(t, 1, len(recent))

	// Check patterns
	patterns := em.GetErrorPatterns()
	assert.True(t, len(patterns) > 0)
}

func TestErrorMetrics_RecordResolution(t *testing.T) {
	em := NewErrorMetricsForTesting()
	ctx := context.Background()

	// Create and record an error
	richErr := fmt.Errorf("test error")

	em.RecordError(ctx, richErr)

	// Record resolution
	duration := 5 * time.Second
	em.RecordResolution(ctx, richErr, "automatic", duration)

	// Verify resolution metrics
	assert.True(t, em.resolutionTimes["automatic"] > 0)
}

func TestErrorMetrics_EnrichContext(t *testing.T) {
	em := NewErrorMetricsForTesting()

	// Create context with correlation ID
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")

	// Create error
	richErr := fmt.Errorf("test error")

	// Enrich the context
	em.EnrichContext(ctx, richErr)

	// Basic test - just ensure no panic occurs
	assert.NotNil(t, richErr)
}

func TestErrorMetrics_GetErrorPatterns(t *testing.T) {
	em := NewErrorMetricsForTesting()
	ctx := context.Background()

	// Record multiple errors
	errors := []struct {
		message string
		count   int
	}{
		{"build failed", 3},
		{"network error", 2},
		{"build failed", 1}, // Same pattern, should increment
	}

	for _, e := range errors {
		for i := 0; i < e.count; i++ {
			err := fmt.Errorf("%s", e.message)
			em.RecordError(ctx, err)
		}
	}

	// Check patterns
	patterns := em.GetErrorPatterns()
	assert.True(t, len(patterns) > 0)
}

func TestErrorMetrics_MiddlewareIntegration(t *testing.T) {
	em := NewErrorMetricsForTesting()

	handlerCalled := false
	errorResolved := false

	// Create a middleware that resolves certain errors
	middleware := em.CreateErrorMiddleware(func(ctx context.Context, err error) error {
		handlerCalled = true
		if err.Error() == "test error" {
			errorResolved = true
			return nil // Error resolved
		}
		return err // Error not resolved
	})

	ctx := context.Background()

	// Test with resolvable error
	resolvableErr := fmt.Errorf("test error")

	result := middleware(ctx, resolvableErr)
	assert.Nil(t, result)
	assert.True(t, handlerCalled)
	assert.True(t, errorResolved)

	// Verify error was recorded
	recent := em.GetRecentErrors(1)
	assert.Equal(t, 1, len(recent))
}
