package progress

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseSink_ExtractMethods(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	tests := []struct {
		name     string
		update   progress.Update
		expected map[string]interface{}
	}{
		{
			name: "extract step name",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
				},
			},
			expected: map[string]interface{}{
				"step_name": "Build Docker Image",
			},
		},
		{
			name: "extract substep name",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"substep_name": "downloading dependencies",
				},
			},
			expected: map[string]interface{}{
				"substep_name": "downloading dependencies",
			},
		},
		{
			name: "extract can abort flag",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"can_abort": true,
				},
			},
			expected: map[string]interface{}{
				"can_abort": true,
			},
		},
		{
			name: "extract attempt number",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"attempt": 3,
				},
			},
			expected: map[string]interface{}{
				"attempt": 3,
			},
		},
		{
			name: "extract error message",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"error": "failed to connect to registry",
				},
			},
			expected: map[string]interface{}{
				"error": "failed to connect to registry",
			},
		},
		{
			name: "missing metadata returns defaults",
			update: progress.Update{
				UserMeta: map[string]interface{}{},
			},
			expected: map[string]interface{}{
				"step_name":    "",
				"substep_name": "",
				"can_abort":    false,
				"attempt":      0,
				"error":        "",
			},
		},
		{
			name: "wrong type returns defaults",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"step_name":    123,           // wrong type
					"substep_name": true,          // wrong type
					"can_abort":    "not a bool",  // wrong type
					"attempt":      "not an int",  // wrong type
					"error":        []string{"x"}, // wrong type
				},
			},
			expected: map[string]interface{}{
				"step_name":    "",
				"substep_name": "",
				"can_abort":    false,
				"attempt":      0,
				"error":        "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if expected, exists := tt.expected["step_name"]; exists {
				assert.Equal(t, expected, sink.extractStepName(tt.update))
			}
			if expected, exists := tt.expected["substep_name"]; exists {
				assert.Equal(t, expected, sink.extractSubstepName(tt.update))
			}
			if expected, exists := tt.expected["can_abort"]; exists {
				assert.Equal(t, expected, sink.extractCanAbort(tt.update))
			}
			if expected, exists := tt.expected["attempt"]; exists {
				assert.Equal(t, expected, sink.extractAttempt(tt.update))
			}
			if expected, exists := tt.expected["error"]; exists {
				assert.Equal(t, expected, sink.extractError(tt.update))
			}
		})
	}
}

func TestBaseSink_FormatETA(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	tests := []struct {
		name     string
		eta      time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			eta:      0,
			expected: "",
		},
		{
			name:     "negative duration",
			eta:      -5 * time.Second,
			expected: "",
		},
		{
			name:     "seconds only",
			eta:      5 * time.Second,
			expected: "ETA: 5s",
		},
		{
			name:     "minutes and seconds",
			eta:      2*time.Minute + 30*time.Second,
			expected: "ETA: 2m30s",
		},
		{
			name:     "hours minutes seconds",
			eta:      1*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "ETA: 1h30m45s",
		},
		{
			name:     "milliseconds rounded to seconds",
			eta:      1500 * time.Millisecond,
			expected: "ETA: 2s",
		},
		{
			name:     "sub-second rounded up",
			eta:      800 * time.Millisecond,
			expected: "ETA: 1s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.formatETA(tt.eta)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseSink_FormatETAMs(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	tests := []struct {
		name     string
		eta      time.Duration
		expected int64
	}{
		{
			name:     "zero duration",
			eta:      0,
			expected: 0,
		},
		{
			name:     "negative duration",
			eta:      -5 * time.Second,
			expected: 0,
		},
		{
			name:     "seconds to milliseconds",
			eta:      5 * time.Second,
			expected: 5000,
		},
		{
			name:     "milliseconds",
			eta:      1500 * time.Millisecond,
			expected: 1500,
		},
		{
			name:     "sub-millisecond",
			eta:      500 * time.Microsecond,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.formatETAMs(tt.eta)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseSink_BuildEnhancedMessage(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	tests := []struct {
		name     string
		update   progress.Update
		expected string
	}{
		{
			name: "basic step name only",
			update: progress.Update{
				Message: "Generic message",
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
				},
			},
			expected: "Build Docker Image",
		},
		{
			name: "step name with substep",
			update: progress.Update{
				Message: "Generic message",
				UserMeta: map[string]interface{}{
					"step_name":    "Build Docker Image",
					"substep_name": "downloading dependencies",
				},
			},
			expected: "Build Docker Image (downloading dependencies)",
		},
		{
			name: "step name with retry attempt",
			update: progress.Update{
				Message: "Generic message",
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
					"attempt":   3,
				},
			},
			expected: "Build Docker Image - Attempt 3",
		},
		{
			name: "step name with substep and retry",
			update: progress.Update{
				Message: "Generic message",
				UserMeta: map[string]interface{}{
					"step_name":    "Build Docker Image",
					"substep_name": "downloading dependencies",
					"attempt":      2,
				},
			},
			expected: "Build Docker Image (downloading dependencies) - Attempt 2",
		},
		{
			name: "failed status with error",
			update: progress.Update{
				Message: "Generic message",
				Status:  "failed",
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
					"error":     "registry connection timeout",
				},
			},
			expected: "Build Docker Image - Error: registry connection timeout",
		},
		{
			name: "failed status with long error truncated",
			update: progress.Update{
				Message: "Generic message",
				Status:  "failed",
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
					"error":     "this is a very long error message that should be truncated because it exceeds fifty characters",
				},
			},
			expected: "Build Docker Image - Error: this is a very long error message that should b...",
		},
		{
			name: "generating status with token info",
			update: progress.Update{
				Message: "Generic message",
				Status:  "generating",
				UserMeta: map[string]interface{}{
					"step_name":        "Generate Dockerfile",
					"tokens_generated": 150,
					"estimated_total":  300,
				},
			},
			expected: "AI generating tokens: 150/300",
		},
		{
			name: "no step name falls back to original message",
			update: progress.Update{
				Message: "Original progress message",
				UserMeta: map[string]interface{}{
					"substep_name": "some substep",
				},
			},
			expected: "Original progress message",
		},
		{
			name: "empty step name falls back to original message",
			update: progress.Update{
				Message: "Original progress message",
				UserMeta: map[string]interface{}{
					"step_name": "",
				},
			},
			expected: "Original progress message",
		},
		{
			name: "attempt of 1 not shown (first attempt)",
			update: progress.Update{
				Message: "Generic message",
				UserMeta: map[string]interface{}{
					"step_name": "Build Docker Image",
					"attempt":   1,
				},
			},
			expected: "Build Docker Image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.buildEnhancedMessage(tt.update)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseSink_GetStatusInfo(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	tests := []struct {
		name             string
		update           progress.Update
		expectedIcon     string
		expectedDisplay  string
		expectedTerminal bool
	}{
		{
			name:             "completed status",
			update:           progress.Update{Status: "completed"},
			expectedIcon:     "✅",
			expectedDisplay:  "Completed",
			expectedTerminal: true,
		},
		{
			name:             "failed status",
			update:           progress.Update{Status: "failed"},
			expectedIcon:     "❌",
			expectedDisplay:  "Failed",
			expectedTerminal: true,
		},
		{
			name:             "retrying without attempt",
			update:           progress.Update{Status: "retrying"},
			expectedIcon:     "🔄",
			expectedDisplay:  "Retrying",
			expectedTerminal: false,
		},
		{
			name: "retrying with attempt",
			update: progress.Update{
				Status: "retrying",
				UserMeta: map[string]interface{}{
					"attempt": 3,
				},
			},
			expectedIcon:     "🔄(3)",
			expectedDisplay:  "Retrying (attempt 3)",
			expectedTerminal: false,
		},
		{
			name:             "started status",
			update:           progress.Update{Status: "started"},
			expectedIcon:     "🚀",
			expectedDisplay:  "Started",
			expectedTerminal: false,
		},
		{
			name:             "running status",
			update:           progress.Update{Status: "running"},
			expectedIcon:     "⚡",
			expectedDisplay:  "Running",
			expectedTerminal: false,
		},
		{
			name:             "generating status",
			update:           progress.Update{Status: "generating"},
			expectedIcon:     "🧠",
			expectedDisplay:  "Generating",
			expectedTerminal: false,
		},
		{
			name:             "unknown status",
			update:           progress.Update{Status: "custom_status"},
			expectedIcon:     "▶️",
			expectedDisplay:  "custom_status",
			expectedTerminal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.getStatusInfo(tt.update)
			assert.Equal(t, tt.expectedIcon, result.Icon)
			assert.Equal(t, tt.expectedDisplay, result.DisplayName)
			assert.Equal(t, tt.expectedTerminal, result.IsTerminal)
		})
	}
}

func TestBaseSink_ShouldThrottleHeartbeat(t *testing.T) {
	logger := slog.Default()
	_ = newBaseSink(logger, "test") // Test base sink creation

	tests := []struct {
		name        string
		update      progress.Update
		throttle    time.Duration
		setup       func(*baseSink)
		expected    bool
		description string
	}{
		{
			name: "non-heartbeat not throttled",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"kind": "regular",
				},
			},
			throttle:    time.Second,
			expected:    false,
			description: "Regular updates should never be throttled",
		},
		{
			name: "heartbeat without previous heartbeat not throttled",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"kind": "heartbeat",
				},
			},
			throttle:    time.Second,
			expected:    false,
			description: "First heartbeat should not be throttled",
		},
		{
			name: "heartbeat within throttle window throttled",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"kind": "heartbeat",
				},
			},
			throttle: 2 * time.Second,
			setup: func(s *baseSink) {
				s.lastHeartbeat = time.Now().Add(-1 * time.Second)
			},
			expected:    true,
			description: "Heartbeat within throttle window should be throttled",
		},
		{
			name: "heartbeat outside throttle window not throttled",
			update: progress.Update{
				UserMeta: map[string]interface{}{
					"kind": "heartbeat",
				},
			},
			throttle: time.Second,
			setup: func(s *baseSink) {
				s.lastHeartbeat = time.Now().Add(-2 * time.Second)
			},
			expected:    false,
			description: "Heartbeat outside throttle window should not be throttled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh sink for each test
			testSink := newBaseSink(logger, "test")

			if tt.setup != nil {
				tt.setup(testSink)
			}

			result := testSink.shouldThrottleHeartbeat(tt.update, tt.throttle)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestBaseSink_BuildBasePayload(t *testing.T) {
	logger := slog.Default()
	sink := newBaseSink(logger, "test")

	update := progress.Update{
		Step:       3,
		Total:      10,
		Percentage: 30,
		Status:     "running",
		Message:    "Processing step 3",
		TraceID:    "trace-123",
		StartedAt:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		ETA:        5 * time.Minute,
		UserMeta: map[string]interface{}{
			"step_name":    "Build Image",
			"substep_name": "downloading",
			"can_abort":    true,
		},
	}

	payload := sink.buildBasePayload(update)

	// Verify top-level fields
	assert.Equal(t, 3, payload["step"])
	assert.Equal(t, 10, payload["total"])
	assert.Equal(t, 30, payload["percentage"])
	assert.Equal(t, "running", payload["status"])
	assert.Equal(t, "Processing step 3", payload["message"])
	assert.Equal(t, "trace-123", payload["trace_id"])
	assert.Equal(t, update.StartedAt, payload["started_at"])

	// Verify enhanced fields
	assert.Equal(t, int64(300000), payload["eta_ms"]) // 5 minutes in ms
	assert.Equal(t, "Build Image", payload["step_name"])
	assert.Equal(t, "downloading", payload["substep_name"])
	assert.Equal(t, true, payload["can_abort"])

	// Verify metadata block for backward compatibility
	metadata, exists := payload["metadata"]
	require.True(t, exists)
	metadataMap := metadata.(map[string]interface{})
	assert.Equal(t, 3, metadataMap["step"])
	assert.Equal(t, 10, metadataMap["total"])
	assert.Equal(t, 30, metadataMap["percentage"])
	assert.Equal(t, "running", metadataMap["status"])
	assert.Equal(t, int64(300000), metadataMap["eta_ms"])
}
