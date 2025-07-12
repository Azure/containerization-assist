package progress

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetadataFields tests all the new metadata fields
func TestMetadataFields(t *testing.T) {
	t.Run("new metadata fields populated correctly", func(t *testing.T) {
		step := NewStepInfo("test_step", "Test Description", 3, 10)

		// Check initial values
		assert.Equal(t, "progress", step.Metadata.Kind)
		assert.NotEmpty(t, step.Metadata.StageID)
		assert.Contains(t, step.Metadata.StageID, "test_step_3_")
		assert.Equal(t, "test_step", step.Metadata.Step)
		assert.Equal(t, 3, step.Metadata.Current)
		assert.Equal(t, 10, step.Metadata.Total)
		assert.Equal(t, 30, step.Metadata.Percentage)
		assert.Equal(t, 0.3, step.Metadata.Progress)
		assert.Equal(t, StatusRunning, step.Metadata.Status)
		assert.Equal(t, StatusCodeRunning, step.Metadata.StatusCode)
		assert.Equal(t, "Test Description", step.Metadata.Message)
		assert.Equal(t, int64(0), step.Metadata.ETAMS) // No ETA initially
	})

	t.Run("status code mapping", func(t *testing.T) {
		tests := []struct {
			status Status
			code   int
		}{
			{StatusPending, StatusCodePending},
			{StatusRunning, StatusCodeRunning},
			{StatusCompleted, StatusCodeCompleted},
			{StatusFailed, StatusCodeFailed},
			{StatusSkipped, StatusCodeSkipped},
			{StatusRetrying, StatusCodeRetrying},
			{Status("unknown"), 0},
		}

		for _, tt := range tests {
			t.Run(string(tt.status), func(t *testing.T) {
				assert.Equal(t, tt.code, GetStatusCode(tt.status))
			})
		}
	})

	t.Run("complete updates all fields", func(t *testing.T) {
		step := NewStepInfo("complete_test", "Testing completion", 5, 10)
		time.Sleep(10 * time.Millisecond) // Ensure some duration

		step.Complete()

		assert.Equal(t, StatusCompleted, step.Status)
		assert.Equal(t, StatusCompleted, step.Metadata.Status)
		assert.Equal(t, StatusCodeCompleted, step.Metadata.StatusCode)
		assert.Equal(t, 1.0, step.Metadata.Progress)
		assert.Equal(t, 100, step.Metadata.Percentage)
		assert.Equal(t, int64(0), step.Metadata.ETAMS)
		assert.Greater(t, step.Duration, time.Duration(0))
	})

	t.Run("fail updates all fields", func(t *testing.T) {
		step := NewStepInfo("fail_test", "Testing failure", 3, 10)
		err := errors.New("test error")

		step.Fail(err)

		assert.Equal(t, StatusFailed, step.Status)
		assert.Equal(t, StatusFailed, step.Metadata.Status)
		assert.Equal(t, StatusCodeFailed, step.Metadata.StatusCode)
		assert.Equal(t, "test error", step.Metadata.Error)
		assert.Equal(t, int64(0), step.Metadata.ETAMS)
	})

	t.Run("update progress with ETA calculation", func(t *testing.T) {
		step := NewStepInfo("eta_test", "Testing ETA", 0, 100)

		// Simulate some work
		time.Sleep(100 * time.Millisecond)
		step.UpdateProgress(10, "10% complete")

		assert.Equal(t, 10, step.Metadata.Current)
		assert.Equal(t, "10% complete", step.Metadata.Message)
		assert.Equal(t, 10, step.Metadata.Percentage)
		assert.Equal(t, 0.1, step.Metadata.Progress)

		// ETA should be approximately 900ms (100ms elapsed, 10% done = ~1000ms total - 100ms elapsed)
		assert.Greater(t, step.Metadata.ETAMS, int64(800))
		assert.Less(t, step.Metadata.ETAMS, int64(1100))

		// Test edge cases
		step.UpdateProgress(0, "No progress")
		assert.Equal(t, int64(0), step.Metadata.ETAMS)

		step.UpdateProgress(100, "Complete")
		assert.Equal(t, int64(0), step.Metadata.ETAMS)
	})
}

// TestNewStepInfo tests step info creation
func TestNewStepInfo(t *testing.T) {
	tests := []struct {
		name        string
		stepName    string
		description string
		current     int
		total       int
		expPercent  int
	}{
		{
			name:        "normal step",
			stepName:    "build",
			description: "Building Docker image",
			current:     3,
			total:       10,
			expPercent:  30,
		},
		{
			name:        "first step",
			stepName:    "start",
			description: "Starting workflow",
			current:     1,
			total:       5,
			expPercent:  20,
		},
		{
			name:        "last step",
			stepName:    "complete",
			description: "Completing workflow",
			current:     10,
			total:       10,
			expPercent:  100,
		},
		{
			name:        "zero total",
			stepName:    "invalid",
			description: "Invalid step",
			current:     1,
			total:       0,
			expPercent:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := NewStepInfo(tt.stepName, tt.description, tt.current, tt.total)

			assert.Equal(t, tt.stepName, step.Name)
			assert.Equal(t, tt.description, step.Description)
			assert.Equal(t, StatusRunning, step.Status)
			assert.Equal(t, tt.current, step.Metadata.Current)
			assert.Equal(t, tt.total, step.Metadata.Total)
			assert.Equal(t, tt.expPercent, step.Metadata.Percentage)
			assert.NotZero(t, step.StartTime)
			assert.NotNil(t, step.Metadata.Details)
		})
	}
}

// TestStepInfoComplete tests step completion
func TestStepInfoComplete(t *testing.T) {
	step := NewStepInfo("test", "Test step", 1, 10)

	// Wait a bit to ensure duration is measurable
	time.Sleep(10 * time.Millisecond)

	step.Complete()

	assert.Equal(t, StatusCompleted, step.Status)
	assert.Equal(t, StatusCompleted, step.Metadata.Status)
	assert.NotZero(t, step.EndTime)
	assert.NotZero(t, step.Duration)
	assert.Equal(t, step.Duration, step.Metadata.Duration)
}

// TestStepInfoFail tests step failure
func TestStepInfoFail(t *testing.T) {
	step := NewStepInfo("test", "Test step", 1, 10)
	testErr := errors.New("test error")

	step.Fail(testErr)

	assert.Equal(t, StatusFailed, step.Status)
	assert.Equal(t, StatusFailed, step.Metadata.Status)
	assert.Equal(t, testErr, step.Error)
	assert.Equal(t, "test error", step.ErrorMsg)
	assert.Equal(t, "test error", step.Metadata.Error)
	assert.NotZero(t, step.EndTime)
	assert.NotZero(t, step.Duration)
}

// TestStepInfoAddDetail tests adding details
func TestStepInfoAddDetail(t *testing.T) {
	step := NewStepInfo("test", "Test step", 1, 10)

	step.AddDetail("key1", "value1")
	step.AddDetail("key2", 123)
	step.AddDetail("key3", true)

	assert.Equal(t, "value1", step.Metadata.Details["key1"])
	assert.Equal(t, 123, step.Metadata.Details["key2"])
	assert.Equal(t, true, step.Metadata.Details["key3"])
}

// TestMetadataJSON tests JSON marshaling/unmarshaling
func TestMetadataJSON(t *testing.T) {
	t.Run("marshal and unmarshal metadata", func(t *testing.T) {
		original := Metadata{
			Kind:       "progress",
			StageID:    "test_stage_123",
			Step:       "test_step",
			Current:    5,
			Total:      10,
			Percentage: 50,
			Status:     StatusRunning,
			StatusCode: StatusCodeRunning,
			Message:    "Test message",
			Progress:   0.5,
			ETAMS:      5000,
			StartTime:  time.Now(),
			Duration:   5 * time.Second,
			Details: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			Error:      "",
			RetryCount: 0,
		}

		// Marshal to JSON
		data, err := json.Marshal(original)
		require.NoError(t, err)

		// Unmarshal back
		var unmarshaled Metadata
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		// Compare fields (excluding time fields which may have precision differences)
		assert.Equal(t, original.Kind, unmarshaled.Kind)
		assert.Equal(t, original.StageID, unmarshaled.StageID)
		assert.Equal(t, original.Step, unmarshaled.Step)
		assert.Equal(t, original.Current, unmarshaled.Current)
		assert.Equal(t, original.Total, unmarshaled.Total)
		assert.Equal(t, original.Percentage, unmarshaled.Percentage)
		assert.Equal(t, original.Status, unmarshaled.Status)
		assert.Equal(t, original.StatusCode, unmarshaled.StatusCode)
		assert.Equal(t, original.Message, unmarshaled.Message)
		assert.Equal(t, original.Progress, unmarshaled.Progress)
		assert.Equal(t, original.ETAMS, unmarshaled.ETAMS)
		// Details map comparison needs special handling due to JSON number conversion
		assert.Equal(t, len(original.Details), len(unmarshaled.Details))
		assert.Equal(t, original.Details["key1"], unmarshaled.Details["key1"])
		// JSON unmarshaling converts int to float64, so compare as float64
		assert.Equal(t, float64(original.Details["key2"].(int)), unmarshaled.Details["key2"])
		assert.Equal(t, original.Error, unmarshaled.Error)
		assert.Equal(t, original.RetryCount, unmarshaled.RetryCount)

		// Check time fields are close enough
		assert.WithinDuration(t, original.StartTime, unmarshaled.StartTime, time.Second)
		assert.Equal(t, original.Duration, unmarshaled.Duration)
	})

	t.Run("json output snapshot", func(t *testing.T) {
		metadata := Metadata{
			Kind:       "progress",
			StageID:    "build_1_20240112150405",
			Step:       "build",
			Current:    3,
			Total:      10,
			Percentage: 30,
			Status:     StatusRunning,
			StatusCode: StatusCodeRunning,
			Message:    "Building Docker image",
			Progress:   0.3,
			ETAMS:      7000,
			StartTime:  time.Date(2024, 1, 12, 15, 4, 5, 0, time.UTC),
			Duration:   3 * time.Second,
			Details: map[string]interface{}{
				"image": "myapp:latest",
			},
		}

		data, err := json.MarshalIndent(metadata, "", "  ")
		require.NoError(t, err)

		// This serves as a snapshot test - if the JSON structure changes, this test will catch it
		jsonStr := string(data)
		assert.Contains(t, jsonStr, `"kind": "progress"`)
		assert.Contains(t, jsonStr, `"stage_id": "build_1_20240112150405"`)
		assert.Contains(t, jsonStr, `"status": "running"`)
		assert.Contains(t, jsonStr, `"status_code": 1`)
		assert.Contains(t, jsonStr, `"progress": 0.3`)
		assert.Contains(t, jsonStr, `"eta_ms": 7000`)
		assert.Contains(t, jsonStr, `"start_time": "2024-01-12T15:04:05Z"`)
		assert.Contains(t, jsonStr, `"duration": "3s"`)
	})
}

// TestNewWorkflowProgress tests workflow progress creation
func TestNewWorkflowProgress(t *testing.T) {
	wp := NewWorkflowProgress("wf-123", "test-workflow", 5)

	assert.Equal(t, "wf-123", wp.WorkflowID)
	assert.Equal(t, "test-workflow", wp.WorkflowName)
	assert.Equal(t, 5, wp.TotalSteps)
	assert.Equal(t, 0, wp.CurrentStep)
	assert.Equal(t, 0, wp.Percentage)
	assert.Equal(t, StatusRunning, wp.Status)
	assert.NotZero(t, wp.StartTime)
	assert.NotNil(t, wp.Steps)
	assert.Empty(t, wp.Steps)
}

// TestWorkflowProgressAddStep tests adding steps
func TestWorkflowProgressAddStep(t *testing.T) {
	wp := NewWorkflowProgress("wf-123", "test-workflow", 3)

	// Add first step
	step1 := NewStepInfo("step1", "First step", 1, 3)
	wp.AddStep(step1)

	assert.Equal(t, 1, wp.CurrentStep)
	assert.Equal(t, 33, wp.Percentage) // 1/3 = 33%
	assert.Len(t, wp.Steps, 1)

	// Add second step
	step2 := NewStepInfo("step2", "Second step", 2, 3)
	wp.AddStep(step2)

	assert.Equal(t, 2, wp.CurrentStep)
	assert.Equal(t, 66, wp.Percentage) // 2/3 = 66%
	assert.Len(t, wp.Steps, 2)

	// Add third step
	step3 := NewStepInfo("step3", "Third step", 3, 3)
	wp.AddStep(step3)

	assert.Equal(t, 3, wp.CurrentStep)
	assert.Equal(t, 100, wp.Percentage) // 3/3 = 100%
	assert.Len(t, wp.Steps, 3)
}

// TestWorkflowProgressComplete tests workflow completion
func TestWorkflowProgressComplete(t *testing.T) {
	wp := NewWorkflowProgress("wf-123", "test-workflow", 3)

	// Wait to ensure duration
	time.Sleep(10 * time.Millisecond)

	wp.Complete()

	assert.Equal(t, StatusCompleted, wp.Status)
	assert.Equal(t, 100, wp.Percentage)
	assert.NotZero(t, wp.EndTime)
	assert.NotZero(t, wp.Duration)
}

// TestWorkflowProgressFail tests workflow failure
func TestWorkflowProgressFail(t *testing.T) {
	wp := NewWorkflowProgress("wf-123", "test-workflow", 3)

	wp.Fail("Critical error occurred")

	assert.Equal(t, StatusFailed, wp.Status)
	assert.Equal(t, "Critical error occurred", wp.Error)
	assert.NotZero(t, wp.EndTime)
	assert.NotZero(t, wp.Duration)
}

// TestStatusConstants tests that status constants are defined correctly
func TestStatusConstants(t *testing.T) {
	// Ensure all status constants are unique strings
	statuses := []Status{
		StatusPending,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusSkipped,
		StatusRetrying,
	}

	seen := make(map[Status]bool)
	for _, status := range statuses {
		assert.NotEmpty(t, string(status), "Status should not be empty")
		assert.False(t, seen[status], "Status %s is duplicated", status)
		seen[status] = true
	}
}

// TestWorkflowProgress tests workflow progress tracking
func TestWorkflowProgress(t *testing.T) {
	t.Run("workflow progress tracking", func(t *testing.T) {
		workflow := NewWorkflowProgress("wf-123", "containerize", 5)

		assert.Equal(t, "wf-123", workflow.WorkflowID)
		assert.Equal(t, "containerize", workflow.WorkflowName)
		assert.Equal(t, 5, workflow.TotalSteps)
		assert.Equal(t, 0, workflow.CurrentStep)
		assert.Equal(t, StatusRunning, workflow.Status)

		// Add steps
		step1 := NewStepInfo("analyze", "Analyzing repository", 1, 5)
		workflow.AddStep(step1)
		assert.Equal(t, 1, workflow.CurrentStep)
		assert.Equal(t, 20, workflow.Percentage)

		// Complete workflow
		workflow.Complete()
		assert.Equal(t, StatusCompleted, workflow.Status)
		assert.Equal(t, 100, workflow.Percentage)
		assert.Greater(t, workflow.Duration, time.Duration(0))

		// Test failure
		workflow2 := NewWorkflowProgress("wf-456", "deploy", 3)
		workflow2.Fail("Deployment failed")
		assert.Equal(t, StatusFailed, workflow2.Status)
		assert.Equal(t, "Deployment failed", workflow2.Error)
	})
}

// BenchmarkMetadata benchmarks metadata operations
func BenchmarkMetadata(b *testing.B) {
	b.Run("NewStepInfo", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewStepInfo("bench_step", "Benchmark step", i%10, 10)
		}
	})

	b.Run("UpdateProgress", func(b *testing.B) {
		step := NewStepInfo("bench_step", "Benchmark step", 0, 1000)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			step.UpdateProgress(i%1000, "Progress update")
		}
	})

	b.Run("JSON Marshal", func(b *testing.B) {
		metadata := NewStepInfo("bench_step", "Benchmark step", 5, 10).Metadata
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(metadata)
		}
	})
}
