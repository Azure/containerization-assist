package progress

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLISink_NewCLISink(t *testing.T) {
	logger := slog.Default()
	sink := NewCLISink(logger)

	assert.NotNil(t, sink)
	assert.NotNil(t, sink.baseSink)
	assert.Equal(t, 40, sink.barWidth)
	assert.Len(t, sink.spinner, 10)
	assert.Equal(t, 0, sink.spinIndex)
}

func TestCLISink_CreateProgressBar(t *testing.T) {
	logger := slog.Default()
	sink := NewCLISink(logger)

	tests := []struct {
		name        string
		percentage  int
		expectedLen int
		description string
	}{
		{
			name:        "zero percent",
			percentage:  0,
			expectedLen: len("[░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0%"),
			description: "Should create empty progress bar",
		},
		{
			name:        "fifty percent",
			percentage:  50,
			expectedLen: len("[████████████████████░░░░░░░░░░░░░░░░░░░░]  50%"),
			description: "Should create half-filled progress bar",
		},
		{
			name:        "hundred percent",
			percentage:  100,
			expectedLen: len("[████████████████████████████████████████] 100%"),
			description: "Should create fully filled progress bar",
		},
		{
			name:        "negative percentage",
			percentage:  -10,
			expectedLen: len("[░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0%"),
			description: "Should clamp negative percentage to 0",
		},
		{
			name:        "over hundred percent",
			percentage:  150,
			expectedLen: len("[████████████████████████████████████████] 100%"),
			description: "Should clamp percentage over 100 to 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.createProgressBar(tt.percentage)
			assert.Equal(t, tt.expectedLen, len(result), tt.description)

			// Verify format contains brackets and percentage
			assert.Contains(t, result, "[")
			assert.Contains(t, result, "]")
			assert.Contains(t, result, "%")
		})
	}
}

func TestCLISink_Publish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	sink := NewCLISink(logger)
	ctx := context.Background()

	tests := []struct {
		name        string
		update      progress.Update
		expectError bool
		description string
	}{
		{
			name: "basic progress update",
			update: progress.Update{
				Step:       1,
				Total:      5,
				Percentage: 20,
				Status:     "running",
				Message:    "Processing step 1",
				UserMeta:   map[string]interface{}{},
			},
			expectError: false,
			description: "Should publish basic progress update without error",
		},
		{
			name: "heartbeat update with spinner",
			update: progress.Update{
				Step:       2,
				Total:      5,
				Percentage: 40,
				Status:     "running",
				Message:    "Processing step 2",
				UserMeta: map[string]interface{}{
					"kind": "heartbeat",
				},
			},
			expectError: false,
			description: "Should handle heartbeat updates with spinner",
		},
		{
			name: "enhanced progress with step name",
			update: progress.Update{
				Step:       3,
				Total:      5,
				Percentage: 60,
				Status:     "running",
				Message:    "Generic message",
				UserMeta: map[string]interface{}{
					"step_name":    "Build Docker Image",
					"substep_name": "downloading dependencies",
				},
			},
			expectError: false,
			description: "Should use enhanced message from baseSink",
		},
		{
			name: "completed status",
			update: progress.Update{
				Step:       5,
				Total:      5,
				Percentage: 100,
				Status:     "completed",
				Message:    "All done",
				UserMeta:   map[string]interface{}{},
			},
			expectError: false,
			description: "Should handle completed status",
		},
		{
			name: "failed status with error",
			update: progress.Update{
				Step:       3,
				Total:      5,
				Percentage: 60,
				Status:     "failed",
				Message:    "Step failed",
				UserMeta: map[string]interface{}{
					"step_name": "Build Image",
					"error":     "registry timeout",
				},
			},
			expectError: false,
			description: "Should handle failed status with error details",
		},
		{
			name: "retry status with attempt",
			update: progress.Update{
				Step:       3,
				Total:      5,
				Percentage: 60,
				Status:     "retrying",
				Message:    "Retrying step",
				UserMeta: map[string]interface{}{
					"step_name": "Build Image",
					"attempt":   2,
				},
			},
			expectError: false,
			description: "Should handle retry status with attempt count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sink.Publish(ctx, tt.update)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestCLISink_Close(t *testing.T) {
	logger := slog.Default()
	sink := NewCLISink(logger)

	err := sink.Close()
	assert.NoError(t, err, "Close should not return an error")
}

func TestCLISink_SpinnerProgression(t *testing.T) {
	logger := slog.Default()
	sink := NewCLISink(logger)
	ctx := context.Background()

	// Test that spinner index progresses with heartbeat updates
	initialIndex := sink.spinIndex

	update := progress.Update{
		Step:       1,
		Total:      5,
		Percentage: 20,
		Status:     "running",
		Message:    "Processing",
		UserMeta: map[string]interface{}{
			"kind": "heartbeat",
		},
	}

	err := sink.Publish(ctx, update)
	require.NoError(t, err)

	// Spinner index should have incremented
	assert.Equal(t, initialIndex+1, sink.spinIndex, "Spinner index should increment on heartbeat")

	// Test spinner wraps around
	sink.spinIndex = len(sink.spinner) - 1
	err = sink.Publish(ctx, update)
	require.NoError(t, err)

	// Should wrap to 0
	expectedIndex := len(sink.spinner) % len(sink.spinner) // 0
	actualChar := sink.spinner[sink.spinIndex%len(sink.spinner)]
	expectedChar := sink.spinner[expectedIndex]
	assert.Equal(t, expectedChar, actualChar, "Spinner should wrap around to beginning")
}

func TestCLISink_StatusIcons(t *testing.T) {
	logger := slog.Default()
	sink := NewCLISink(logger)

	tests := []struct {
		name         string
		status       string
		userMeta     map[string]interface{}
		expectedIcon string
		description  string
	}{
		{
			name:         "completed status",
			status:       "completed",
			userMeta:     map[string]interface{}{},
			expectedIcon: "✅",
			description:  "Should use checkmark for completed",
		},
		{
			name:         "failed status",
			status:       "failed",
			userMeta:     map[string]interface{}{},
			expectedIcon: "❌",
			description:  "Should use X mark for failed",
		},
		{
			name:         "retrying status with attempt",
			status:       "retrying",
			userMeta:     map[string]interface{}{"attempt": 3},
			expectedIcon: "🔄(3)",
			description:  "Should show retry icon with attempt number",
		},
		{
			name:         "started status",
			status:       "started",
			userMeta:     map[string]interface{}{},
			expectedIcon: "🚀",
			description:  "Should use rocket for started",
		},
		{
			name:         "running status",
			status:       "running",
			userMeta:     map[string]interface{}{},
			expectedIcon: "⚡",
			description:  "Should use lightning for running",
		},
		{
			name:         "generating status",
			status:       "generating",
			userMeta:     map[string]interface{}{},
			expectedIcon: "🧠",
			description:  "Should use brain for generating",
		},
		{
			name:         "unknown status",
			status:       "custom",
			userMeta:     map[string]interface{}{},
			expectedIcon: "▶️",
			description:  "Should use play button for unknown status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := progress.Update{
				Status:   tt.status,
				UserMeta: tt.userMeta,
			}

			statusInfo := sink.getStatusInfo(update)
			assert.Equal(t, tt.expectedIcon, statusInfo.Icon, tt.description)
		})
	}
}
