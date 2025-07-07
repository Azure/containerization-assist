package api

import (
	"context"
	"encoding/json"
	"testing"
)

// ExampleTool is a test implementation of the canonical api.Tool interface
type ExampleTool struct{}

func (t *ExampleTool) Name() string {
	return "example_tool"
}

func (t *ExampleTool) Description() string {
	return "An example tool for testing the canonical interface"
}

func (t *ExampleTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	return ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"session_id": input.SessionID,
			"message":    "Example tool executed successfully",
			"processed":  true,
		},
		Metadata: map[string]interface{}{
			"execution_time_ms": 100,
			"tool_version":      "1.0.0",
		},
	}, nil
}

func (t *ExampleTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "example_tool",
		Description: "An example tool for testing the canonical interface",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for the operation",
				},
				"data": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Test message",
						},
					},
					"required": []string{"message"},
				},
			},
			"required": []string{"session_id", "data"},
		},
		Tags:     []string{"test", "example", "validation"},
		Category: ToolCategory("test"),
		Version:  "1.0.0",
	}
}

// TestToolContractStability ensures the canonical Tool interface doesn't change unexpectedly
func TestToolContractStability(t *testing.T) {
	// Create an example canonical tool
	tool := &ExampleTool{}

	// Test that it implements the canonical interface
	var _ Tool = tool

	// Test schema generation consistency
	schema := tool.Schema()
	if schema.Name != "example_tool" {
		t.Errorf("Expected name 'example_tool', got '%s'", schema.Name)
	}

	if schema.Category != ToolCategory("test") {
		t.Errorf("Expected category 'test', got '%s'", schema.Category)
	}

	if schema.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", schema.Version)
	}

	// Test execution flow
	input := ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"message": "test message",
		},
	}

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !output.Success {
		t.Errorf("Expected successful execution, got error: %s", output.Error)
	}

	// Validate output structure
	if output.Data == nil {
		t.Error("Expected data in output")
	}

	if output.Metadata == nil {
		t.Error("Expected metadata in output")
	}
}

// TestCanonicalInterfaceCompatibility ensures canonical tools work properly
func TestCanonicalInterfaceCompatibility(t *testing.T) {
	// Test all our canonical tool implementations
	tools := []Tool{
		&ExampleTool{},
	}

	for _, tool := range tools {
		t.Run(tool.Name(), func(t *testing.T) {
			// Test interface compliance
			var _ Tool = tool

			// Test basic properties
			if tool.Name() == "" {
				t.Error("Tool name is empty")
			}

			if tool.Description() == "" {
				t.Error("Tool description is empty")
			}

			schema := tool.Schema()
			if schema.Name != tool.Name() {
				t.Errorf("Schema name mismatch: expected %s, got %s",
					tool.Name(), schema.Name)
			}

			// Verify input schema is valid
			if schema.InputSchema != nil {
				schemaJSON, err := json.Marshal(schema.InputSchema)
				if err != nil {
					t.Errorf("Invalid input schema: %v", err)
				}

				var testSchema map[string]interface{}
				if err := json.Unmarshal(schemaJSON, &testSchema); err != nil {
					t.Errorf("Invalid input schema JSON: %v", err)
				}
			}
		})
	}
}

// TestJSONSchemaStability generates and validates JSON schemas
func TestJSONSchemaStability(t *testing.T) {
	tool := &ExampleTool{}
	schema := tool.Schema()

	// Convert schema to JSON for stability testing
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema to JSON: %v", err)
	}

	// Basic schema validation
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
		t.Fatalf("Generated schema is not valid JSON: %v", err)
	}

	// Check required fields
	requiredFields := []string{"name", "description", "input_schema"}
	for _, field := range requiredFields {
		if _, exists := schemaMap[field]; !exists {
			t.Errorf("Schema missing required field: %s", field)
		}
	}

	// Validate schema structure matches expected format
	expectedName := "example_tool"
	if schemaMap["name"] != expectedName {
		t.Errorf("Expected schema name %s, got %v", expectedName, schemaMap["name"])
	}

	// Log schema for manual review (can be used for approval tests in the future)
	t.Logf("Generated schema:\n%s", string(schemaJSON))
}

// TestToolExecutionStability ensures execution results maintain consistent structure
func TestToolExecutionStability(t *testing.T) {
	tool := &ExampleTool{}

	testInput := ToolInput{
		SessionID: "stability-test",
		Data: map[string]interface{}{
			"message": "stability test message",
		},
		Context: map[string]interface{}{
			"test": true,
		},
	}

	output, err := tool.Execute(context.Background(), testInput)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Validate output structure stability
	if !output.Success {
		t.Errorf("Expected successful execution")
	}

	if output.Data == nil {
		t.Error("Expected data in output")
	}

	if output.Metadata == nil {
		t.Error("Expected metadata in output")
	}

	// Convert output to JSON to ensure it's serializable
	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal output to JSON: %v", err)
	}

	// Log output for manual review
	t.Logf("Generated output:\n%s", string(outputJSON))

	// Verify output can be round-tripped
	var roundTrip ToolOutput
	if err := json.Unmarshal(outputJSON, &roundTrip); err != nil {
		t.Fatalf("Failed to unmarshal output JSON: %v", err)
	}

	if roundTrip.Success != output.Success {
		t.Error("Round-trip failed for Success field")
	}
}
