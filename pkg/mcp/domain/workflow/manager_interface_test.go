package workflow

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestManagerInterface tests the unified Manager interface
func TestManagerInterface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("basic progress flow", func(t *testing.T) {
		m := NewManager(context.Background(), nil, 10, logger)

		// Test initial state
		assert.Equal(t, 0, m.GetCurrent())
		assert.Equal(t, 10, m.GetTotal())
		assert.False(t, m.IsComplete())
		assert.NotEmpty(t, m.GetTraceID())

		// Test progress flow
		m.Begin("Starting work")
		m.Update(1, "Step 1", nil)

		// No delay needed now that atomic value is updated immediately
		assert.Equal(t, 1, m.GetCurrent())

		m.Update(5, "Step 5", nil)
		assert.Equal(t, 5, m.GetCurrent())

		m.Complete("Work complete")
		m.Finish()
	})

	t.Run("error budget interface", func(t *testing.T) {
		m := NewManager(context.Background(), nil, 5, logger)

		// Initially no circuit breaker
		assert.False(t, m.IsCircuitOpen())

		// Record some errors
		err1 := errors.New("test error 1")
		within := m.RecordError(err1)
		assert.True(t, within) // Should be within budget initially

		// Record success
		m.RecordSuccess()

		// Test error handling in updates
		err2 := errors.New("test error 2")
		success := m.UpdateWithErrorHandling(1, "Step with error", nil, err2)
		assert.False(t, success) // Should return false due to error

		// Test successful update
		success = m.UpdateWithErrorHandling(2, "Successful step", nil, nil)
		assert.True(t, success) // Should return true with no error

		// Check error budget status
		status := m.GetErrorBudgetStatus()
		assert.NotNil(t, status)
	})

	t.Run("set current", func(t *testing.T) {
		m := NewManager(context.Background(), nil, 10, logger)

		m.SetCurrent(7)
		assert.Equal(t, 7, m.GetCurrent())

		m.SetCurrent(10)
		assert.Equal(t, 10, m.GetCurrent())
		assert.True(t, m.IsComplete())
	})
}
