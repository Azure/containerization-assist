package messaging

import (
	"context"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/stretchr/testify/assert"
)

func TestNewNoOpEmitter(t *testing.T) {
	emitter := NewNoOpEmitter()

	assert.NotNil(t, emitter)
}

func TestNoOpEmitter_Emit(t *testing.T) {
	emitter := NewNoOpEmitter()

	err := emitter.Emit(context.Background(), "test_stage", 50, "Test message")
	assert.NoError(t, err)

	// Should always succeed regardless of input
	err = emitter.Emit(context.Background(), "", -1, "")
	assert.NoError(t, err)

	err = emitter.Emit(context.Background(), "stage", 150, "Over 100%")
	assert.NoError(t, err)
}

func TestNoOpEmitter_EmitDetailed(t *testing.T) {
	emitter := NewNoOpEmitter()

	update := api.ProgressUpdate{
		Stage:      "test_stage",
		Percentage: 75,
		Message:    "Detailed test",
		Status:     "running",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)

	// Should work with empty update
	emptyUpdate := api.ProgressUpdate{}
	err = emitter.EmitDetailed(context.Background(), emptyUpdate)
	assert.NoError(t, err)
}

func TestNoOpEmitter_Close(t *testing.T) {
	emitter := NewNoOpEmitter()

	err := emitter.Close()
	assert.NoError(t, err)

	// Should be safe to call multiple times
	err = emitter.Close()
	assert.NoError(t, err)
}

func TestNoOpEmitter_ImplementsInterface(t *testing.T) {
	emitter := NewNoOpEmitter()

	// Verify it implements the ProgressEmitter interface
	var _ api.ProgressEmitter = emitter
}

func TestNoOpEmitter_ConcurrentAccess(t *testing.T) {
	emitter := NewNoOpEmitter()

	// Test concurrent access - should be safe since it does nothing
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				_ = emitter.Emit(context.Background(), "concurrent", j, "test")

				update := api.ProgressUpdate{
					Stage:      "concurrent_detailed",
					Percentage: j,
					Message:    "concurrent test",
					Status:     "running",
				}
				_ = emitter.EmitDetailed(context.Background(), update)

				_ = emitter.Close()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should not have panicked
	assert.True(t, true)
}

func TestNoOpEmitter_ContextCancellation(t *testing.T) {
	emitter := NewNoOpEmitter()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should work fine with cancelled context
	err := emitter.Emit(ctx, "cancelled_test", 50, "Should work")
	assert.NoError(t, err)

	update := api.ProgressUpdate{
		Stage:   "cancelled_detailed",
		Message: "Should work with cancelled context",
		Status:  "running",
	}
	err = emitter.EmitDetailed(ctx, update)
	assert.NoError(t, err)
}
