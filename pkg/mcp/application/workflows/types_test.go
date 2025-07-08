package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStageTypeSerialization(t *testing.T) {
	// Test StageType serialization
	stageType := StageTypeAnalysis

	data, err := json.Marshal(stageType)
	if err != nil {
		t.Fatalf("Failed to marshal StageType: %v", err)
	}

	var decoded StageType
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal StageType: %v", err)
	}

	if decoded != stageType {
		t.Errorf("Expected StageType %s, got %s", stageType, decoded)
	}
}

func TestExecutionStatusSerialization(t *testing.T) {
	// Test ExecutionStatus
	status := StatusRunning

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal ExecutionStatus: %v", err)
	}

	var decoded ExecutionStatus
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ExecutionStatus: %v", err)
	}

	if decoded != status {
		t.Errorf("Expected ExecutionStatus %s, got %s", status, decoded)
	}
}

func TestRetryPolicySerialization(t *testing.T) {
	// Test RetryPolicy
	policy := &RetryPolicy{
		MaxAttempts:     3,
		InitialDelay:    time.Second,
		MaxDelay:        time.Minute,
		BackoffMode:     BackoffExponential,
		Multiplier:      2.0,
		RetryableErrors: []string{"timeout", "network_error"},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Failed to marshal RetryPolicy: %v", err)
	}

	var decoded RetryPolicy
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal RetryPolicy: %v", err)
	}

	if decoded.MaxAttempts != policy.MaxAttempts {
		t.Errorf("Expected MaxAttempts %d, got %d", policy.MaxAttempts, decoded.MaxAttempts)
	}
	if decoded.BackoffMode != policy.BackoffMode {
		t.Errorf("Expected BackoffMode %s, got %s", policy.BackoffMode, decoded.BackoffMode)
	}
}

func TestServiceIntegrationTypes(t *testing.T) {
	// Test ServiceConfig
	config := &ServiceConfig{
		Type:     "docker",
		Endpoint: "unix:///var/run/docker.sock",
		Options:  map[string]interface{}{"version": "1.40"},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal ServiceConfig: %v", err)
	}

	var decoded ServiceConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServiceConfig: %v", err)
	}

	if decoded.Type != config.Type {
		t.Errorf("Expected Type %s, got %s", config.Type, decoded.Type)
	}

	// Test ToolExecution
	execution := &ToolExecution{
		ID:        "exec-123",
		Tool:      "analyze",
		Stage:     "stage1",
		Input:     map[string]interface{}{"path": "/repo"},
		StartTime: time.Now(),
	}

	data, err = json.Marshal(execution)
	if err != nil {
		t.Fatalf("Failed to marshal ToolExecution: %v", err)
	}

	var decodedExecution ToolExecution
	err = json.Unmarshal(data, &decodedExecution)
	if err != nil {
		t.Fatalf("Failed to unmarshal ToolExecution: %v", err)
	}

	if decodedExecution.ID != execution.ID {
		t.Errorf("Expected ID %s, got %s", execution.ID, decodedExecution.ID)
	}
}
