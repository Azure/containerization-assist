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

// TestProgressManagerErrorBudgetIntegration tests integration of error budget with progress manager
func TestProgressManagerErrorBudgetIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("error budget tracking", func(t *testing.T) {
		m := New(context.Background(), nil, 10, logger)

		// Start with clean slate
		assert.False(t, m.IsCircuitOpen())
		assert.Equal(t, 0, m.GetErrorBudgetStatus().CurrentErrors)

		// Record some errors
		err1 := errors.New("test error 1")
		within := m.RecordError(err1)
		assert.True(t, within)
		assert.Equal(t, 1, m.GetErrorBudgetStatus().CurrentErrors)

		// Update with error handling
		success := m.UpdateWithErrorHandling(1, "First step", nil, err1)
		assert.False(t, success) // Should return false due to error

		// Record success
		success = m.UpdateWithErrorHandling(2, "Second step", nil, nil)
		assert.True(t, success)

		// Error count should decrease due to success (from 2 to 1)
		assert.Equal(t, 1, m.GetErrorBudgetStatus().CurrentErrors)
	})

	t.Run("circuit breaker activation", func(t *testing.T) {
		m := New(context.Background(), nil, 10, logger)

		// Record errors until circuit opens (budget allows 5 errors)
		for i := 0; i < 6; i++ {
			err := errors.New("repeated error")
			within := m.RecordError(err)

			if i < 4 {
				assert.True(t, within, "Should be within budget for error %d", i+1)
			} else {
				assert.False(t, within, "Should exceed budget at error %d", i+1)
				assert.True(t, m.IsCircuitOpen(), "Circuit should be open")
			}
		}

		// Updates with circuit open should still work but flag the state
		metadata := make(map[string]interface{})
		success := m.UpdateWithErrorHandling(5, "Step with circuit open", metadata, errors.New("another error"))
		assert.False(t, success)
		assert.True(t, metadata["circuit_open"].(bool))
	})

	t.Run("error budget reset over time", func(t *testing.T) {
		// Create manager with very short reset period for testing
		m := New(context.Background(), nil, 10, logger)
		m.errorBudget = NewErrorBudget(3, 100*time.Millisecond) // Reset every 100ms

		// Record errors
		for i := 0; i < 3; i++ {
			within := m.RecordError(errors.New("test error"))
			if i < 2 {
				assert.True(t, within)
			} else {
				assert.False(t, within) // Third error should exceed budget
				assert.True(t, m.IsCircuitOpen())
			}
		}

		// Wait for reset
		time.Sleep(150 * time.Millisecond)

		// Error budget should reset and circuit should be closed
		within := m.RecordError(errors.New("error after reset"))
		assert.True(t, within)
		assert.False(t, m.IsCircuitOpen())
	})
}

// TestErrorBudgetRetryWithBackoff tests retry functionality with backoff
func TestErrorBudgetRetryWithBackoff(t *testing.T) {
	t.Run("retry with exponential backoff", func(t *testing.T) {
		budget := NewErrorBudget(5, 1*time.Minute)

		attemptCount := 0
		operation := func() error {
			attemptCount++
			if attemptCount < 3 {
				return errors.New("temporary failure")
			}
			return nil // Success on third attempt
		}

		ctx := context.Background()
		start := time.Now()

		err := RetryWithErrorBudget(ctx, budget, operation)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, 3, attemptCount)
		assert.True(t, duration > 1*time.Second)      // Should have waited for backoff (1s + 2s)
		assert.Equal(t, 1, budget.GetCurrentErrors()) // Errors should be decremented on success (2 errors - 1 success)
	})

	t.Run("retry fails when budget exceeded", func(t *testing.T) {
		budget := NewErrorBudget(2, 1*time.Minute) // Very small budget

		attemptCount := 0
		operation := func() error {
			attemptCount++
			return errors.New("persistent failure")
		}

		ctx := context.Background()
		err := RetryWithErrorBudget(ctx, budget, operation)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error budget exceeded")
		assert.Equal(t, 2, attemptCount) // Should stop after budget exceeded
	})

	t.Run("retry respects context timeout", func(t *testing.T) {
		budget := NewErrorBudget(10, 1*time.Minute)

		operation := func() error {
			return errors.New("always fails")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := RetryWithErrorBudget(ctx, budget, operation)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
		assert.Less(t, duration, 200*time.Millisecond) // Should timeout quickly
	})
}

// TestMultiErrorBudget tests multiple error budgets for different operation types
func TestMultiErrorBudget(t *testing.T) {
	t.Run("separate budgets for different operations", func(t *testing.T) {
		multibudget := NewMultiErrorBudget()

		// Create separate budgets for different operations
		dockerBudget := multibudget.GetOrCreateBudget("docker", 3, 5*time.Minute)
		k8sBudget := multibudget.GetOrCreateBudget("kubernetes", 2, 3*time.Minute)

		// Fill docker budget
		for i := 0; i < 3; i++ {
			dockerBudget.RecordError(errors.New("docker error"))
		}
		assert.True(t, dockerBudget.IsCircuitOpen())

		// K8s budget should still be open
		assert.False(t, k8sBudget.IsCircuitOpen())
		within := k8sBudget.RecordError(errors.New("k8s error"))
		assert.True(t, within)

		// Get all statuses
		statuses := multibudget.GetAllStatuses()
		assert.Len(t, statuses, 2)
		assert.True(t, statuses["docker"].CircuitOpen)
		assert.False(t, statuses["kubernetes"].CircuitOpen)
	})
}

// TestErrorBudgetHealthPercentage tests health percentage calculation
func TestErrorBudgetHealthPercentage(t *testing.T) {
	budget := NewErrorBudget(10, 5*time.Minute)

	// Start at 100% health
	status := budget.GetStatus()
	assert.Equal(t, 100.0, status.HealthPercentage)

	// Add 3 errors - should be 70% healthy
	for i := 0; i < 3; i++ {
		budget.RecordError(errors.New("test error"))
	}
	status = budget.GetStatus()
	assert.Equal(t, 70.0, status.HealthPercentage)

	// Add 7 more errors - should be 0% healthy
	for i := 0; i < 7; i++ {
		budget.RecordError(errors.New("test error"))
	}
	status = budget.GetStatus()
	assert.Equal(t, 0.0, status.HealthPercentage)
	assert.True(t, status.CircuitOpen)
}

// BenchmarkErrorBudgetOperations benchmarks error budget operations
func BenchmarkErrorBudgetOperations(b *testing.B) {
	budget := NewErrorBudget(1000, 1*time.Hour)

	b.Run("RecordError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			budget.RecordError(errors.New("test error"))
		}
	})

	b.Run("RecordSuccess", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			budget.RecordSuccess()
		}
	})

	b.Run("GetStatus", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = budget.GetStatus()
		}
	})
}
