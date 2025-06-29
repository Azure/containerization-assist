package analyze

import (
	"context"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// Test NewAnalyzeRepositoryRedirectTool constructor
func TestNewAnalyzeRepositoryRedirectTool_Simple(t *testing.T) {
	logger := zerolog.Nop()

	// Create a basic atomic tool for testing
	atomicTool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)

	if tool == nil {
		t.Error("NewAnalyzeRepositoryRedirectTool should not return nil")
		return
	}
	if tool.atomicTool != atomicTool {
		t.Error("Expected atomicTool to be set correctly")
	}
	if tool.logger.GetLevel() < 0 {
		t.Log("Logger level is set to a specific value")
	}
}

// Test AnalyzeRepositoryRedirectTool Execute with invalid argument type
func TestAnalyzeRepositoryRedirectTool_Execute_InvalidArgType_Simple(t *testing.T) {
	logger := zerolog.Nop()

	atomicTool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)

	// Test with non-map argument
	result, err := tool.Execute(context.Background(), "invalid-args")
	if err == nil {
		t.Error("Execute should return error for invalid argument type")
	}
	if result != nil {
		t.Error("Execute should not return result for invalid argument type")
	}

	// Check that it's a RichError with correct type
	if richErr, ok := err.(*mcp.RichError); ok {
		if richErr.Code != "INVALID_ARGUMENTS" {
			t.Errorf("Expected error code 'INVALID_ARGUMENTS', got '%s'", richErr.Code)
		}
		if richErr.Type != "validation_error" {
			t.Errorf("Expected error type 'validation_error', got '%s'", richErr.Type)
		}
	} else {
		t.Error("Expected error to be a RichError")
	}
}

// Test AnalyzeRepositoryRedirectTool Execute without repo_path or path
func TestAnalyzeRepositoryRedirectTool_Execute_MissingPath_Simple(t *testing.T) {
	logger := zerolog.Nop()

	atomicTool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)

	// Test without repo_path or path field
	args := map[string]interface{}{
		"session_id": "test-session",
		"branch":     "main",
	}

	result, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("Execute should return error when repo_path is missing")
	}
	if result != nil {
		t.Error("Execute should not return result when repo_path is missing")
	}

	// Check that it's a RichError with correct message
	if richErr, ok := err.(*mcp.RichError); ok {
		if richErr.Code != "INVALID_ARGUMENTS" {
			t.Errorf("Expected error code 'INVALID_ARGUMENTS', got '%s'", richErr.Code)
		}
		if !containsText(richErr.Message, "repo_path is required") {
			t.Errorf("Expected error message to contain 'repo_path is required', got '%s'", richErr.Message)
		}
	} else {
		t.Error("Expected error to be a RichError")
	}
}

// Test argument processing without actual execution
func TestAnalyzeRepositoryRedirectTool_ArgumentProcessing(t *testing.T) {
	logger := zerolog.Nop()

	atomicTool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)

	testCases := []struct {
		name        string
		args        interface{}
		expectError bool
		errorCode   string
	}{
		{
			name:        "nil args",
			args:        nil,
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
		{
			name:        "string args",
			args:        "string-args",
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
		{
			name:        "int args",
			args:        123,
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
		{
			name:        "slice args",
			args:        []string{"item1", "item2"},
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
		{
			name:        "empty map",
			args:        map[string]interface{}{},
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
		{
			name: "map with non-string repo_path",
			args: map[string]interface{}{
				"repo_path": 123,
			},
			expectError: true,
			errorCode:   "INVALID_ARGUMENTS",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tc.args)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got none", tc.name)
				}
				if result != nil {
					t.Errorf("Expected no result for error case '%s', but got result", tc.name)
				}

				if richErr, ok := err.(*mcp.RichError); ok {
					if richErr.Code != tc.errorCode {
						t.Errorf("Expected error code '%s' for test case '%s', got '%s'", tc.errorCode, tc.name, richErr.Code)
					}
				} else {
					t.Errorf("Expected RichError for test case '%s', got %T", tc.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case '%s', but got: %v", tc.name, err)
				}
			}
		})
	}
}

// Helper function to check if string contains substring
func containsText(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
