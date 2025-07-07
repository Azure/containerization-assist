package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowStatus_Constants(t *testing.T) {
	// Verify all status constants are defined
	assert.Equal(t, WorkflowStatus("pending"), WorkflowStatusPending)
	assert.Equal(t, WorkflowStatus("running"), WorkflowStatusRunning)
	assert.Equal(t, WorkflowStatus("paused"), WorkflowStatusPaused)
	assert.Equal(t, WorkflowStatus("completed"), WorkflowStatusCompleted)
	assert.Equal(t, WorkflowStatus("failed"), WorkflowStatusFailed)
	assert.Equal(t, WorkflowStatus("cancelled"), WorkflowStatusCancelled)
}

func TestWorkflowCheckpoint_Structure(t *testing.T) {
	checkpoint := &WorkflowCheckpoint{
		ID:      "cp-123",
		StageID: "stage-789",
		Created: time.Now(),
	}

	assert.Equal(t, "cp-123", checkpoint.ID)
	assert.Equal(t, "stage-789", checkpoint.StageID)
	assert.NotZero(t, checkpoint.Created)
}

func TestStageResult_Structure(t *testing.T) {
	result := StageResult{
		StageName: "test-stage",
		Success:   true,
		Results:   map[string]interface{}{"output": "processed"},
		Duration:  5 * time.Minute,
		Artifacts: []WorkflowArtifact{
			{
				Name: "output.json",
				Path: "/tmp/output.json",
			},
		},
	}

	assert.Equal(t, "test-stage", result.StageName)
	assert.True(t, result.Success)
	assert.Equal(t, "processed", result.Results["output"])
	assert.Equal(t, 5*time.Minute, result.Duration)
	assert.Len(t, result.Artifacts, 1)
}

func TestWorkflowArtifact_Structure(t *testing.T) {
	artifact := WorkflowArtifact{
		Name: "report.pdf",
		Path: "/storage/reports/report.pdf",
	}

	assert.Equal(t, "report.pdf", artifact.Name)
	assert.Equal(t, "/storage/reports/report.pdf", artifact.Path)
}

func TestWorkflowResult_Structure(t *testing.T) {
	result := WorkflowResult{
		WorkflowID:      "workflow-123",
		SessionID:       "session-456",
		Status:          WorkflowStatusCompleted,
		Success:         true,
		Message:         "Workflow completed successfully",
		Duration:        10 * time.Minute,
		Results:         map[string]interface{}{"total": 100},
		Artifacts:       []WorkflowArtifact{},
		StagesExecuted:  3,
		StagesCompleted: 3,
		StagesFailed:    0,
	}

	assert.Equal(t, "workflow-123", result.WorkflowID)
	assert.Equal(t, "session-456", result.SessionID)
	assert.Equal(t, WorkflowStatusCompleted, result.Status)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.StagesExecuted)
	assert.Equal(t, 3, result.StagesCompleted)
	assert.Equal(t, 0, result.StagesFailed)
}

func TestExecutionOptions_Structure(t *testing.T) {
	options := ExecutionOptions{
		SessionID:            "session-123",
		ResumeFromCheckpoint: "cp-456",
		EnableParallel:       true,
		CreateCheckpoints:    true,
		Variables:            map[string]interface{}{"env": "test"},
	}

	assert.Equal(t, "session-123", options.SessionID)
	assert.Equal(t, "cp-456", options.ResumeFromCheckpoint)
	assert.True(t, options.EnableParallel)
	assert.True(t, options.CreateCheckpoints)
	assert.Equal(t, "test", options.Variables["env"])
}
