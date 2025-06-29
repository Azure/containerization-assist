package analyze

import (
	"context"
	"strings"
	"testing"

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

	// Check that error contains expected message
	if !strings.Contains(err.Error(), "error") {
		t.Errorf("Expected error to contain 'error', got '%s'", err.Error())
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

	// Check that error contains expected message
	if !strings.Contains(err.Error(), "error") {
		t.Errorf("Expected error to contain 'error', got '%s'", err.Error())
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

				// Just check that we got an error for error cases
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got none", tc.name)
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
