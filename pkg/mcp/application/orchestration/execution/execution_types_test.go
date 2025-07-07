package execution

import (
	"testing"
	"time"
)

// Test ExecutionStage type
func TestExecutionStage(t *testing.T) {
	timeout := time.Minute * 5
	stage := ExecutionStage{
		ID:         "stage-1",
		Name:       "Build Stage",
		Type:       "build",
		Tools:      []string{"docker", "build-tool"},
		DependsOn:  []string{"stage-0"},
		Variables:  map[string]interface{}{"image": "nginx", "tag": "latest"},
		Timeout:    &timeout,
		MaxRetries: 3,
		Parallel:   false,
		Conditions: []StageCondition{},
	}

	if stage.ID != "stage-1" {
		t.Errorf("Expected ID to be 'stage-1', got '%s'", stage.ID)
	}
	if stage.Name != "Build Stage" {
		t.Errorf("Expected Name to be 'Build Stage', got '%s'", stage.Name)
	}
	if stage.Type != "build" {
		t.Errorf("Expected Type to be 'build', got '%s'", stage.Type)
	}
	if len(stage.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(stage.Tools))
	}
	if stage.Tools[0] != "docker" {
		t.Errorf("Expected first tool to be 'docker', got '%s'", stage.Tools[0])
	}
	if len(stage.DependsOn) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(stage.DependsOn))
	}
	if stage.DependsOn[0] != "stage-0" {
		t.Errorf("Expected dependency to be 'stage-0', got '%s'", stage.DependsOn[0])
	}
	if stage.Variables["image"] != "nginx" {
		t.Errorf("Expected Variables['image'] to be 'nginx', got '%v'", stage.Variables["image"])
	}
	if stage.Timeout == nil {
		t.Error("Expected Timeout to not be nil")
	} else if *stage.Timeout != timeout {
		t.Errorf("Expected Timeout to be %v, got %v", timeout, *stage.Timeout)
	}
	if stage.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", stage.MaxRetries)
	}
	if stage.Parallel {
		t.Error("Expected Parallel to be false")
	}
}

// Test ExecutionSession type
func TestExecutionSession(t *testing.T) {
	startTime := time.Now()
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	session := ExecutionSession{
		SessionID:                "session-123",
		ID:                       "legacy-id",
		WorkflowID:               "workflow-456",
		WorkflowName:             "Build and Deploy",
		Variables:                map[string]interface{}{"env": "production"},
		Context:                  map[string]interface{}{"user": "admin"},
		StartTime:                startTime,
		Status:                   "running",
		CurrentStage:             "build",
		CompletedStages:          []string{"prepare"},
		FailedStages:             []string{},
		SkippedStages:            []string{"test"},
		SharedContext:            map[string]interface{}{"registry": "docker.io"},
		ResourceBindings:         map[string]interface{}{"cpu": "2", "memory": "4Gi"},
		LastActivity:             updatedAt,
		StageResults:             map[string]interface{}{"prepare": "success"},
		CreatedAt:                createdAt,
		UpdatedAt:                updatedAt,
		Checkpoints:              []WorkflowCheckpoint{},
		ConsolidatedErrorContext: map[string]interface{}{},
		WorkflowVersion:          "v1.0",
		Labels:                   map[string]string{"environment": "prod", "team": "platform"},
	}

	if session.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", session.SessionID)
	}
	if session.ID != "legacy-id" {
		t.Errorf("Expected ID to be 'legacy-id', got '%s'", session.ID)
	}
	if session.WorkflowID != "workflow-456" {
		t.Errorf("Expected WorkflowID to be 'workflow-456', got '%s'", session.WorkflowID)
	}
	if session.WorkflowName != "Build and Deploy" {
		t.Errorf("Expected WorkflowName to be 'Build and Deploy', got '%s'", session.WorkflowName)
	}
	if session.Variables["env"] != "production" {
		t.Errorf("Expected Variables['env'] to be 'production', got '%v'", session.Variables["env"])
	}
	if session.Status != "running" {
		t.Errorf("Expected Status to be 'running', got '%s'", session.Status)
	}
	if session.CurrentStage != "build" {
		t.Errorf("Expected CurrentStage to be 'build', got '%s'", session.CurrentStage)
	}
	if len(session.CompletedStages) != 1 {
		t.Errorf("Expected 1 completed stage, got %d", len(session.CompletedStages))
	}
	if len(session.FailedStages) != 0 {
		t.Errorf("Expected 0 failed stages, got %d", len(session.FailedStages))
	}
	if len(session.SkippedStages) != 1 {
		t.Errorf("Expected 1 skipped stage, got %d", len(session.SkippedStages))
	}
	if session.WorkflowVersion != "v1.0" {
		t.Errorf("Expected WorkflowVersion to be 'v1.0', got '%s'", session.WorkflowVersion)
	}
	if session.Labels["environment"] != "prod" {
		t.Errorf("Expected Labels['environment'] to be 'prod', got '%s'", session.Labels["environment"])
	}
}

// Test ExecutionArtifact type
func TestExecutionArtifact(t *testing.T) {
	createdAt := time.Now()

	artifact := ExecutionArtifact{
		ID:        "artifact-789",
		Name:      "build-output.tar",
		Type:      "archive",
		Path:      "/artifacts/build-output.tar",
		Size:      1024000,
		Metadata:  map[string]interface{}{"compression": "gzip", "checksum": "sha256:abc123"},
		CreatedAt: createdAt,
	}

	if artifact.ID != "artifact-789" {
		t.Errorf("Expected ID to be 'artifact-789', got '%s'", artifact.ID)
	}
	if artifact.Name != "build-output.tar" {
		t.Errorf("Expected Name to be 'build-output.tar', got '%s'", artifact.Name)
	}
	if artifact.Type != "archive" {
		t.Errorf("Expected Type to be 'archive', got '%s'", artifact.Type)
	}
	if artifact.Path != "/artifacts/build-output.tar" {
		t.Errorf("Expected Path to be '/artifacts/build-output.tar', got '%s'", artifact.Path)
	}
	if artifact.Size != 1024000 {
		t.Errorf("Expected Size to be 1024000, got %d", artifact.Size)
	}
	if artifact.Metadata["compression"] != "gzip" {
		t.Errorf("Expected Metadata['compression'] to be 'gzip', got '%v'", artifact.Metadata["compression"])
	}
	if artifact.CreatedAt != createdAt {
		t.Errorf("Expected CreatedAt to match, got %v", artifact.CreatedAt)
	}
}

// Test WorkflowSpec type
func TestWorkflowSpec(t *testing.T) {
	spec := WorkflowSpec{
		ID:         "workflow-spec-123",
		Name:       "CI/CD Pipeline",
		Version:    "v2.0",
		Stages:     []ExecutionStage{{ID: "stage-1", Name: "Build"}},
		Variables:  map[string]interface{}{"branch": "main"},
		APIVersion: "v1",
		Kind:       "Workflow",
	}

	if spec.ID != "workflow-spec-123" {
		t.Errorf("Expected ID to be 'workflow-spec-123', got '%s'", spec.ID)
	}
	if spec.Name != "CI/CD Pipeline" {
		t.Errorf("Expected Name to be 'CI/CD Pipeline', got '%s'", spec.Name)
	}
	if spec.Version != "v2.0" {
		t.Errorf("Expected Version to be 'v2.0', got '%s'", spec.Version)
	}
	if len(spec.Stages) != 1 {
		t.Errorf("Expected 1 stage, got %d", len(spec.Stages))
	}
	if spec.Stages[0].ID != "stage-1" {
		t.Errorf("Expected stage ID to be 'stage-1', got '%s'", spec.Stages[0].ID)
	}
	if spec.Variables["branch"] != "main" {
		t.Errorf("Expected Variables['branch'] to be 'main', got '%v'", spec.Variables["branch"])
	}
	if spec.APIVersion != "v1" {
		t.Errorf("Expected APIVersion to be 'v1', got '%s'", spec.APIVersion)
	}
	if spec.Kind != "Workflow" {
		t.Errorf("Expected Kind to be 'Workflow', got '%s'", spec.Kind)
	}
}

// Test WorkflowCheckpoint type
func TestWorkflowCheckpoint(t *testing.T) {
	timestamp := time.Now()

	checkpoint := WorkflowCheckpoint{
		ID:           "checkpoint-456",
		WorkflowID:   "workflow-123",
		SessionID:    "session-789",
		StageID:      "stage-2",
		StageName:    "Deploy Stage",
		Timestamp:    timestamp,
		State:        map[string]interface{}{"deployed": true},
		SessionState: map[string]interface{}{"user": "admin"},
		StageResults: map[string]interface{}{"deployment_id": "deploy-123"},
		Message:      "Stage completed successfully",
	}

	if checkpoint.ID != "checkpoint-456" {
		t.Errorf("Expected ID to be 'checkpoint-456', got '%s'", checkpoint.ID)
	}
	if checkpoint.WorkflowID != "workflow-123" {
		t.Errorf("Expected WorkflowID to be 'workflow-123', got '%s'", checkpoint.WorkflowID)
	}
	if checkpoint.SessionID != "session-789" {
		t.Errorf("Expected SessionID to be 'session-789', got '%s'", checkpoint.SessionID)
	}
	if checkpoint.StageID != "stage-2" {
		t.Errorf("Expected StageID to be 'stage-2', got '%s'", checkpoint.StageID)
	}
	if checkpoint.StageName != "Deploy Stage" {
		t.Errorf("Expected StageName to be 'Deploy Stage', got '%s'", checkpoint.StageName)
	}
	if checkpoint.Timestamp != timestamp {
		t.Errorf("Expected Timestamp to match, got %v", checkpoint.Timestamp)
	}
	if checkpoint.State["deployed"] != true {
		t.Errorf("Expected State['deployed'] to be true, got '%v'", checkpoint.State["deployed"])
	}
	if checkpoint.Message != "Stage completed successfully" {
		t.Errorf("Expected Message to be 'Stage completed successfully', got '%s'", checkpoint.Message)
	}
}
