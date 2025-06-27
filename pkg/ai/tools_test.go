package ai

import (
	"encoding/json"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
)

func TestMkSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]interface{}{"type": "string"},
			expected: `{"type":"string"}`,
		},
		{
			name:     "complex object",
			input:    map[string]interface{}{"type": "object", "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}}},
			expected: `{"properties":{"name":{"type":"string"}},"type":"object"}`,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mkSchema(tt.input)
			if string(result) != tt.expected {
				t.Errorf("mkSchema() = %s, expected %s", string(result), tt.expected)
			}
		})
	}
}

func TestCreateReadFileTool(t *testing.T) {
	tool := CreateReadFileTool()

	// Check tool type
	if tool.Type == nil || *tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got %v", tool.Type)
	}

	// Check function definition
	if tool.Function == nil {
		t.Fatal("Function definition should not be nil")
	}

	if tool.Function.Name == nil || *tool.Function.Name != "read_file" {
		t.Errorf("Expected function name 'read_file', got %v", tool.Function.Name)
	}

	if tool.Function.Description == nil || *tool.Function.Description != "Read file contents from the repository" {
		t.Errorf("Expected correct description, got %v", tool.Function.Description)
	}

	// Check parameters schema
	if tool.Function.Parameters == nil {
		t.Fatal("Parameters should not be nil")
	}

	var params map[string]interface{}
	err := json.Unmarshal(tool.Function.Parameters, &params)
	if err != nil {
		t.Fatalf("Failed to unmarshal parameters: %v", err)
	}

	if params["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", params["type"])
	}

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	filePath, ok := properties["filePath"].(map[string]interface{})
	if !ok {
		t.Fatal("filePath property should be a map")
	}

	if filePath["type"] != "string" {
		t.Errorf("Expected filePath type 'string', got %v", filePath["type"])
	}

	required, ok := params["required"].([]interface{})
	if !ok {
		t.Fatal("Required should be an array")
	}

	if len(required) != 1 || required[0] != "filePath" {
		t.Errorf("Expected required to contain 'filePath', got %v", required)
	}
}

func TestCreateListDirectoryTool(t *testing.T) {
	tool := CreateListDirectoryTool()

	// Check tool type
	if tool.Type == nil || *tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got %v", tool.Type)
	}

	// Check function definition
	if tool.Function == nil {
		t.Fatal("Function definition should not be nil")
	}

	if tool.Function.Name == nil || *tool.Function.Name != "list_directory" {
		t.Errorf("Expected function name 'list_directory', got %v", tool.Function.Name)
	}

	if tool.Function.Description == nil || *tool.Function.Description != "List files in a directory" {
		t.Errorf("Expected correct description, got %v", tool.Function.Description)
	}

	// Check parameters schema
	var params map[string]interface{}
	err := json.Unmarshal(tool.Function.Parameters, &params)
	if err != nil {
		t.Fatalf("Failed to unmarshal parameters: %v", err)
	}

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	dirPath, ok := properties["dirPath"].(map[string]interface{})
	if !ok {
		t.Fatal("dirPath property should be a map")
	}

	if dirPath["type"] != "string" {
		t.Errorf("Expected dirPath type 'string', got %v", dirPath["type"])
	}

	required, ok := params["required"].([]interface{})
	if !ok {
		t.Fatal("Required should be an array")
	}

	if len(required) != 1 || required[0] != "dirPath" {
		t.Errorf("Expected required to contain 'dirPath', got %v", required)
	}
}

func TestCreateFileExistsTool(t *testing.T) {
	tool := CreateFileExistsTool()

	// Check tool type
	if tool.Type == nil || *tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got %v", tool.Type)
	}

	// Check function definition
	if tool.Function == nil {
		t.Fatal("Function definition should not be nil")
	}

	if tool.Function.Name == nil || *tool.Function.Name != "file_exists" {
		t.Errorf("Expected function name 'file_exists', got %v", tool.Function.Name)
	}

	if tool.Function.Description == nil || *tool.Function.Description != "Check if a file exists" {
		t.Errorf("Expected correct description, got %v", tool.Function.Description)
	}

	// Check parameters schema
	var params map[string]interface{}
	err := json.Unmarshal(tool.Function.Parameters, &params)
	if err != nil {
		t.Fatalf("Failed to unmarshal parameters: %v", err)
	}

	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	filePath, ok := properties["filePath"].(map[string]interface{})
	if !ok {
		t.Fatal("filePath property should be a map")
	}

	if filePath["type"] != "string" {
		t.Errorf("Expected filePath type 'string', got %v", filePath["type"])
	}
}

func TestGetFileSystemTools(t *testing.T) {
	tools := GetFileSystemTools()

	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Verify all tools are function tools
	for i, toolDef := range tools {
		tool, ok := toolDef.(*azopenai.ChatCompletionsFunctionToolDefinition)
		if !ok {
			t.Errorf("Tool %d is not a ChatCompletionsFunctionToolDefinition", i)
			continue
		}

		if tool.Type == nil || *tool.Type != "function" {
			t.Errorf("Tool %d: expected type 'function', got %v", i, tool.Type)
		}

		if tool.Function == nil {
			t.Errorf("Tool %d: function definition should not be nil", i)
			continue
		}

		if tool.Function.Name == nil {
			t.Errorf("Tool %d: function name should not be nil", i)
		}
	}

	// Verify we have the expected tool names
	expectedNames := []string{"read_file", "list_directory", "file_exists"}
	actualNames := make([]string, len(tools))

	for i, toolDef := range tools {
		if tool, ok := toolDef.(*azopenai.ChatCompletionsFunctionToolDefinition); ok && tool.Function != nil && tool.Function.Name != nil {
			actualNames[i] = *tool.Function.Name
		}
	}

	for _, expected := range expectedNames {
		found := false
		for _, actual := range actualNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool name '%s' not found in tools", expected)
		}
	}
}

func TestToolParametersConsistency(t *testing.T) {
	// Test that all tools have consistent parameter structure
	tools := []struct {
		name     string
		toolFunc func() azopenai.ChatCompletionsFunctionToolDefinition
	}{
		{"read_file", CreateReadFileTool},
		{"list_directory", CreateListDirectoryTool},
		{"file_exists", CreateFileExistsTool},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.toolFunc()

			var params map[string]interface{}
			err := json.Unmarshal(tool.Function.Parameters, &params)
			if err != nil {
				t.Fatalf("Failed to unmarshal parameters for %s: %v", tt.name, err)
			}

			// All tools should have object type
			if params["type"] != "object" {
				t.Errorf("Tool %s: expected type 'object', got %v", tt.name, params["type"])
			}

			// All tools should have properties
			if _, ok := params["properties"]; !ok {
				t.Errorf("Tool %s: missing properties", tt.name)
			}

			// All tools should have required fields
			if _, ok := params["required"]; !ok {
				t.Errorf("Tool %s: missing required fields", tt.name)
			}
		})
	}
}
