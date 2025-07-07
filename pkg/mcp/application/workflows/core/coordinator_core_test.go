package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowTypes_BasicStructures(t *testing.T) {
	t.Run("WorkflowCheckpoint", func(t *testing.T) {
		checkpoint := WorkflowCheckpoint{
			ID:      "cp-123",
			StageID: "stage-456",
			Created: time.Now(),
		}

		assert.Equal(t, "cp-123", checkpoint.ID)
		assert.Equal(t, "stage-456", checkpoint.StageID)
		assert.NotZero(t, checkpoint.Created)
	})

	t.Run("StageResult", func(t *testing.T) {
		result := StageResult{
			StageName: "validation",
			Success:   true,
			Results:   map[string]interface{}{"validated": true},
			Duration:  2 * time.Minute,
		}

		assert.Equal(t, "validation", result.StageName)
		assert.True(t, result.Success)
		assert.Equal(t, true, result.Results["validated"])
		assert.Equal(t, 2*time.Minute, result.Duration)
	})

	t.Run("ExecutionOptions", func(t *testing.T) {
		options := ExecutionOptions{
			SessionID:         "session-123",
			EnableParallel:    true,
			CreateCheckpoints: false,
			Variables:         map[string]interface{}{"debug": true},
		}

		assert.Equal(t, "session-123", options.SessionID)
		assert.True(t, options.EnableParallel)
		assert.False(t, options.CreateCheckpoints)
		assert.Equal(t, true, options.Variables["debug"])
	})
}

func TestWorkflowStatus_Validation(t *testing.T) {
	// Test that we have all expected status values
	statuses := []WorkflowStatus{
		WorkflowStatusPending,
		WorkflowStatusRunning,
		WorkflowStatusPaused,
		WorkflowStatusCompleted,
		WorkflowStatusFailed,
		WorkflowStatusCancelled,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, string(status), "Status should have a string value")
	}

	// Test specific status values
	assert.Equal(t, "pending", string(WorkflowStatusPending))
	assert.Equal(t, "running", string(WorkflowStatusRunning))
	assert.Equal(t, "paused", string(WorkflowStatusPaused))
	assert.Equal(t, "completed", string(WorkflowStatusCompleted))
	assert.Equal(t, "failed", string(WorkflowStatusFailed))
	assert.Equal(t, "cancelled", string(WorkflowStatusCancelled))
}

func TestWorkflowResult_Construction(t *testing.T) {
	result := WorkflowResult{
		WorkflowID:      "wf-123",
		SessionID:       "sess-456",
		Status:          WorkflowStatusCompleted,
		Success:         true,
		Message:         "All stages completed successfully",
		Duration:        15 * time.Minute,
		Results:         map[string]interface{}{"output": "success"},
		Artifacts:       []WorkflowArtifact{},
		StagesExecuted:  5,
		StagesCompleted: 5,
		StagesFailed:    0,
	}

	assert.Equal(t, "wf-123", result.WorkflowID)
	assert.Equal(t, "sess-456", result.SessionID)
	assert.Equal(t, WorkflowStatusCompleted, result.Status)
	assert.True(t, result.Success)
	assert.Equal(t, "All stages completed successfully", result.Message)
	assert.Equal(t, 15*time.Minute, result.Duration)
	assert.Equal(t, "success", result.Results["output"])
	assert.Equal(t, 5, result.StagesExecuted)
	assert.Equal(t, 5, result.StagesCompleted)
	assert.Equal(t, 0, result.StagesFailed)
}

func TestWorkflowArtifact_Fields(t *testing.T) {
	artifact := WorkflowArtifact{
		Name: "build-output.tar.gz",
		Path: "/tmp/artifacts/build-output.tar.gz",
	}

	assert.Equal(t, "build-output.tar.gz", artifact.Name)
	assert.Equal(t, "/tmp/artifacts/build-output.tar.gz", artifact.Path)
}

func TestWorkflowMetrics_Structure(t *testing.T) {
	metrics := WorkflowMetrics{
		TotalDuration: 30 * time.Minute,
		StageDurations: map[string]time.Duration{
			"build":  10 * time.Minute,
			"test":   5 * time.Minute,
			"deploy": 15 * time.Minute,
		},
		ToolExecutionCounts: map[string]int{
			"docker":  3,
			"kubectl": 2,
		},
	}

	assert.Equal(t, 30*time.Minute, metrics.TotalDuration)
	assert.Equal(t, 10*time.Minute, metrics.StageDurations["build"])
	assert.Equal(t, 5*time.Minute, metrics.StageDurations["test"])
	assert.Equal(t, 15*time.Minute, metrics.StageDurations["deploy"])
	assert.Equal(t, 3, metrics.ToolExecutionCounts["docker"])
	assert.Equal(t, 2, metrics.ToolExecutionCounts["kubectl"])
}
