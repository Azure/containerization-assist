package messaging

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestCLIEmitter() *CLIDirectEmitter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewCLIDirectEmitter(logger)
}

func TestNewCLIDirectEmitter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	emitter := NewCLIDirectEmitter(logger)

	assert.NotNil(t, emitter)
	assert.NotNil(t, emitter.logger)
	assert.Equal(t, -1, emitter.lastPercent)
	assert.Equal(t, "", emitter.lastStage)
	assert.WithinDuration(t, time.Now(), emitter.startTime, time.Second)
}

func TestCLIDirectEmitter_Emit_FirstCall(t *testing.T) {
	emitter := createTestCLIEmitter()

	err := emitter.Emit(context.Background(), "test_stage", 25, "Test message")

	assert.NoError(t, err)
	assert.Equal(t, 25, emitter.lastPercent)
	assert.Equal(t, "test_stage", emitter.lastStage)
}

func TestCLIDirectEmitter_Emit_SkipSmallIncrements(t *testing.T) {
	emitter := createTestCLIEmitter()

	// First call should succeed
	err := emitter.Emit(context.Background(), "test_stage", 10, "First message")
	assert.NoError(t, err)
	assert.Equal(t, 10, emitter.lastPercent)

	// Small increment should be skipped (less than 5%)
	err = emitter.Emit(context.Background(), "test_stage", 12, "Should be skipped")
	assert.NoError(t, err)
	assert.Equal(t, 10, emitter.lastPercent) // Should remain unchanged

	// Larger increment should succeed
	err = emitter.Emit(context.Background(), "test_stage", 16, "Should succeed")
	assert.NoError(t, err)
	assert.Equal(t, 16, emitter.lastPercent)
}

func TestCLIDirectEmitter_Emit_StageChange(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Initial call
	err := emitter.Emit(context.Background(), "stage1", 10, "First message")
	assert.NoError(t, err)

	// Stage change should always emit, even with small percentage change
	err = emitter.Emit(context.Background(), "stage2", 12, "Different stage")
	assert.NoError(t, err)
	assert.Equal(t, 12, emitter.lastPercent)
	assert.Equal(t, "stage2", emitter.lastStage)
}

func TestCLIDirectEmitter_Emit_100Percent(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Set initial state
	err := emitter.Emit(context.Background(), "test_stage", 95, "Almost done")
	assert.NoError(t, err)

	// 100% should always emit regardless of increment
	err = emitter.Emit(context.Background(), "test_stage", 100, "Complete")
	assert.NoError(t, err)
	assert.Equal(t, 100, emitter.lastPercent)
}

func TestCLIDirectEmitter_EmitDetailed_ErrorStatus(t *testing.T) {
	emitter := createTestCLIEmitter()

	update := api.ProgressUpdate{
		Stage:      "error_stage",
		Percentage: 50,
		Message:    "Something went wrong",
		Status:     "error",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_EmitDetailed_FailedStatus(t *testing.T) {
	emitter := createTestCLIEmitter()

	update := api.ProgressUpdate{
		Stage:      "failed_stage",
		Percentage: 75,
		Message:    "Operation failed",
		Status:     "failed",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_EmitDetailed_CompletedStatus(t *testing.T) {
	emitter := createTestCLIEmitter()

	update := api.ProgressUpdate{
		Stage:   "completed_stage",
		Message: "All done",
		Status:  "completed",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_EmitDetailed_WarningStatus(t *testing.T) {
	emitter := createTestCLIEmitter()

	update := api.ProgressUpdate{
		Stage:      "warning_stage",
		Percentage: 60,
		Message:    "Minor issue detected",
		Status:     "warning",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_EmitDetailed_RegularStatus(t *testing.T) {
	emitter := createTestCLIEmitter()

	update := api.ProgressUpdate{
		Stage:      "regular_stage",
		Percentage: 40,
		Message:    "In progress",
		Status:     "in_progress",
	}

	err := emitter.EmitDetailed(context.Background(), update)
	assert.NoError(t, err)
	assert.Equal(t, 40, emitter.lastPercent)
	assert.Equal(t, "regular_stage", emitter.lastStage)
}

func TestCLIDirectEmitter_formatProgressBar_ValidPercentages(t *testing.T) {
	emitter := createTestCLIEmitter()

	tests := []struct {
		percent  int
		expected string
	}{
		{0, "[░░░░░░░░░░░░░░░░░░░░]   0%"},
		{25, "[█████░░░░░░░░░░░░░░░]  25%"},
		{50, "[██████████░░░░░░░░░░]  50%"},
		{75, "[███████████████░░░░░]  75%"},
		{100, "[████████████████████] 100%"},
	}

	for _, test := range tests {
		result := emitter.formatProgressBar(test.percent)
		assert.Equal(t, test.expected, result, "Failed for percent: %d", test.percent)
	}
}

func TestCLIDirectEmitter_formatProgressBar_EdgeCases(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Negative percentage should be treated as 0
	result := emitter.formatProgressBar(-10)
	assert.Equal(t, "[░░░░░░░░░░░░░░░░░░░░]   0%", result)

	// Percentage over 100 should be treated as 100
	result = emitter.formatProgressBar(150)
	assert.Equal(t, "[████████████████████] 100%", result)
}

func TestCLIDirectEmitter_Close(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Set some state
	err := emitter.Emit(context.Background(), "test_stage", 85, "Near completion")
	require.NoError(t, err)

	// Close should succeed
	err = emitter.Close()
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_ImplementsInterface(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Verify it implements the ProgressEmitter interface
	var _ api.ProgressEmitter = emitter
}

func TestCLIDirectEmitter_ContextCancellation(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still work with cancelled context (doesn't check context)
	err := emitter.Emit(ctx, "test_stage", 50, "Test message")
	assert.NoError(t, err)

	update := api.ProgressUpdate{
		Stage:      "test_stage",
		Percentage: 75,
		Message:    "Test detailed",
		Status:     "in_progress",
	}
	err = emitter.EmitDetailed(ctx, update)
	assert.NoError(t, err)
}

func TestCLIDirectEmitter_SequentialEmit(t *testing.T) {
	emitter := createTestCLIEmitter()

	// Test sequential usage pattern (typical use case)
	for i := 0; i < 10; i++ {
		err := emitter.Emit(context.Background(), "sequential_stage", i*10, "Sequential test")
		assert.NoError(t, err)

		update := api.ProgressUpdate{
			Stage:      "sequential_detailed",
			Percentage: i * 10,
			Message:    "Sequential detailed test",
			Status:     "in_progress",
		}
		err = emitter.EmitDetailed(context.Background(), update)
		assert.NoError(t, err)
	}

	// Should have updated state
	assert.Equal(t, 90, emitter.lastPercent)
	assert.Equal(t, "sequential_detailed", emitter.lastStage)
}
