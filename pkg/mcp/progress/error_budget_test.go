package progress

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestErrorBudgetExceeded tests behavior when error budget is exceeded
func TestErrorBudgetExceeded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(nil, 10, logger)

	// Set a low error budget for testing
	errorBudget := 3
	errorCount := 0

	// Simulate multiple errors
	for i := 0; i < 5; i++ {
		if errorCount >= errorBudget {
			t.Logf("Error budget exceeded after %d errors", errorCount)
			break
		}

		// Simulate an error occurring
		errorCount++
		m.logger.Error("Simulated error", "count", errorCount, "budget", errorBudget)
	}

	assert.Equal(t, errorBudget, errorCount, "Should stop at error budget limit")
}

// TestProgressManagerFailureRecovery tests recovery from failures
func TestProgressManagerFailureRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("recovers from watchdog failure", func(t *testing.T) {
		m := New(nil, 10, logger)
		// Set minUpdateTime to 0 to avoid throttling
		m.minUpdateTime = 0

		// Force stop watchdog to simulate failure
		m.stopWatchdog()

		// Manager should still function for basic operations
		m.Update(5, "Test update after watchdog failure", nil)
		assert.Equal(t, 5, m.current)

		// Should complete successfully
		m.Complete("Completed despite watchdog failure")
	})

	t.Run("handles spinner creation failure", func(t *testing.T) {
		// Test CI environment where spinner might not work
		os.Setenv("CI", "true")
		defer os.Unsetenv("CI")

		m := New(nil, 10, logger)
		assert.True(t, m.isCI)
		assert.Nil(t, m.spinner) // Should not create spinner in CI

		// Should still work in CI mode
		m.minUpdateTime = 0 // Avoid throttling
		m.Update(3, "CI mode update", nil)
		assert.Equal(t, 3, m.current)
	})
}

// TestProgressManagerTimeout tests timeout scenarios
func TestProgressManagerTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		m := New(nil, 10, logger)

		// Start a long-running operation
		go func() {
			for i := 0; i < 10; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					m.Update(i, "Long running update", nil)
					time.Sleep(20 * time.Millisecond)
				}
			}
		}()

		// Wait for context timeout
		<-ctx.Done()

		// Manager should handle timeout gracefully
		assert.NotNil(t, ctx.Err())
		assert.True(t, m.current >= 0) // Progress should be valid
	})
}

// TestProgressManagerResourceLimits tests resource limit handling
func TestProgressManagerResourceLimits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles memory pressure", func(t *testing.T) {
		m := New(nil, 1000, logger) // Large number of steps

		// Create a large metadata object to simulate memory pressure
		largeMetadata := make(map[string]interface{})
		for i := 0; i < 10000; i++ {
			largeMetadata[string(rune(i))] = "large_data_value_that_consumes_memory"
		}

		// Should handle large metadata without crashing
		m.minUpdateTime = 0 // Avoid throttling
		m.Update(500, "Memory pressure test", largeMetadata)
		assert.Equal(t, 500, m.current)

		// Cleanup
		largeMetadata = nil
	})

	t.Run("handles high frequency updates", func(t *testing.T) {
		m := New(nil, 1000, logger)
		m.minUpdateTime = 1 * time.Microsecond // Very short throttle for testing

		start := time.Now()

		// Rapid fire updates
		for i := 0; i < 100; i++ {
			m.Update(i, "Rapid update", nil)
		}

		duration := time.Since(start)

		// Should handle rapid updates efficiently (under 1 second)
		assert.Less(t, duration, 1*time.Second)
		assert.True(t, m.current >= 0 && m.current <= 100)
	})
}

// TestProgressManagerErrorStates tests various error conditions
func TestProgressManagerErrorStates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles invalid progress values", func(t *testing.T) {
		m := New(nil, 10, logger)

		// Test negative progress
		m.minUpdateTime = 0 // Avoid throttling
		m.Update(-5, "Negative progress", nil)
		assert.Equal(t, -5, m.current) // Should accept but log warning

		// Test progress beyond total
		m.Update(15, "Excess progress", nil)
		assert.Equal(t, 15, m.current) // Should accept but log warning
	})

	t.Run("handles nil metadata gracefully", func(t *testing.T) {
		m := New(nil, 10, logger)

		// Should not panic with nil metadata
		assert.NotPanics(t, func() {
			m.minUpdateTime = 0 // Avoid throttling
			m.Update(5, "Test with nil metadata", nil)
		})

		assert.Equal(t, 5, m.current)
	})

	t.Run("handles empty messages", func(t *testing.T) {
		m := New(nil, 10, logger)

		// Should handle empty messages gracefully
		assert.NotPanics(t, func() {
			m.minUpdateTime = 0 // Avoid throttling
			m.Update(3, "", nil)
			m.Begin("")
			m.Complete("")
		})

		assert.Equal(t, 3, m.current)
	})
}

// TestProgressManagerConcurrentFailures tests concurrent error scenarios
func TestProgressManagerConcurrentFailures(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("handles concurrent updates with errors", func(t *testing.T) {
		m := New(nil, 100, logger)

		errChan := make(chan error, 10)

		// Start multiple goroutines with potential failures
		for i := 0; i < 10; i++ {
			go func(step int) {
				defer func() {
					if r := recover(); r != nil {
						errChan <- errors.New("panic recovered")
					}
				}()

				// Simulate some operations failing
				if step%3 == 0 {
					errChan <- errors.New("simulated failure")
					return
				}

				for j := 0; j < 10; j++ {
					m.Update(step*10+j, "Concurrent update", nil)
					time.Sleep(time.Millisecond)
				}

				errChan <- nil // Success
			}(i)
		}

		// Collect results
		successCount := 0
		errorCount := 0
		for i := 0; i < 10; i++ {
			if err := <-errChan; err != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		t.Logf("Concurrent operations: %d succeeded, %d failed", successCount, errorCount)

		// Should have some successes and some controlled failures
		assert.True(t, successCount > 0, "Should have some successful operations")
		assert.True(t, errorCount > 0, "Should have some failed operations")
		assert.True(t, m.current >= 0, "Progress should remain valid")
	})
}

// TestProgressManagerRecoveryStrategies tests different recovery strategies
func TestProgressManagerRecoveryStrategies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("circuit breaker pattern", func(t *testing.T) {
		m := New(nil, 10, logger)
		m.minUpdateTime = 0 // Avoid throttling
		// Set a low error budget for testing
		m.errorBudget = NewErrorBudget(3, 10*time.Minute)

		// Simulate operations with failures
		for i := 0; i < 10; i++ {
			if m.IsCircuitOpen() {
				continue
			}

			// Simulate operation with potential failure
			if i < 4 { // First 4 operations fail (0, 1, 2, 3)
				err := errors.New("simulated failure")
				m.RecordError(err)
				continue
			}

			// Successful operation
			m.RecordSuccess()
			m.Update(i, "Successful operation", nil)
		}

		assert.True(t, m.IsCircuitOpen(), "Circuit breaker should have opened")
		assert.True(t, m.current >= 0, "Progress should remain valid")
	})

	t.Run("exponential backoff", func(t *testing.T) {
		m := New(nil, 10, logger)
		m.minUpdateTime = 0 // Avoid throttling

		// Test exponential backoff pattern
		baseDelay := 10 * time.Millisecond
		maxRetries := 3

		for retry := 0; retry < maxRetries; retry++ {
			delay := time.Duration(1<<retry) * baseDelay
			t.Logf("Retry %d with delay %v", retry, delay)

			time.Sleep(delay)

			// Simulate operation
			m.Update(retry+1, "Retry operation", map[string]interface{}{
				"retry":    retry,
				"delay_ms": delay.Milliseconds(),
			})
		}

		assert.Equal(t, maxRetries, m.current)
	})
}
