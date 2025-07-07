package interfaces

import (
	"context"
	"testing"
	"time"
)

// TestToolInterface tests that Tool interface can be implemented
func TestToolInterface(t *testing.T) {
	mockTool := &MockTool{
		name:        "test-tool",
		description: "A test tool",
	}
	if mockTool.Name() != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", mockTool.Name())
	}

	if mockTool.Description() != "A test tool" {
		t.Errorf("Expected description 'A test tool', got %s", mockTool.Description())
	}
	input := ToolInput{
		SessionID: "test-session",
		Data:      map[string]interface{}{"test": "data"},
	}

	output, err := mockTool.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !output.Success {
		t.Error("Expected success to be true")
	}
}

// TestToolInputValidation tests ToolInput validation
func TestToolInputValidation(t *testing.T) {
	validInput := &ToolInput{
		SessionID: "test-session",
		Data:      map[string]interface{}{"key": "value"},
	}

	if err := validInput.Validate(); err != nil {
		t.Errorf("Expected valid input to pass validation, got error: %v", err)
	}
	invalidInput := &ToolInput{
		SessionID: "",
		Data:      map[string]interface{}{"key": "value"},
	}

	if err := invalidInput.Validate(); err == nil {
		t.Error("Expected invalid input to fail validation")
	}
}

// TestSession tests Session structure
func TestSession(t *testing.T) {
	session := &Session{
		ID:        "test-id",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  map[string]interface{}{"key": "value"},
		State:     map[string]interface{}{"state": "active"},
	}

	if session.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", session.ID)
	}

	if session.Metadata["key"] != "value" {
		t.Errorf("Expected metadata key 'value', got %v", session.Metadata["key"])
	}
}

// TestWorkflowStep tests WorkflowStep structure
func TestWorkflowStep(t *testing.T) {
	step := &WorkflowStep{
		ID:      "step-1",
		Name:    "Test Step",
		Tool:    "test-tool",
		Input:   map[string]interface{}{"param": "value"},
		Timeout: 30 * time.Second,
	}

	if step.ID != "step-1" {
		t.Errorf("Expected ID 'step-1', got %s", step.ID)
	}

	if step.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", step.Timeout)
	}
}

// MockTool implements the Tool interface for testing
type MockTool struct {
	name        string
	description string
}

// Name returns the name of the mock tool
func (m *MockTool) Name() string {
	return m.name
}

// Description returns the description of the mock tool
func (m *MockTool) Description() string {
	return m.description
}

// Execute executes the mock tool
func (m *MockTool) Execute(_ context.Context, _ ToolInput) (ToolOutput, error) {
	return ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "success"},
	}, nil
}

// Schema returns the schema of the mock tool
func (m *MockTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        m.name,
		Description: m.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"test": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
}
