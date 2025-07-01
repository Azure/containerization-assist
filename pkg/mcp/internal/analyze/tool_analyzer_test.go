package analyze

import (
	"testing"
)

// Test DefaultToolAnalyzer
func TestDefaultToolAnalyzer(t *testing.T) {
	// Test constructor
	toolName := "test-tool"
	analyzer := NewDefaultToolAnalyzer(toolName)

	if analyzer == nil {
		t.Error("NewDefaultToolAnalyzer should not return nil")
	}
	if analyzer.GetToolName() != toolName {
		t.Errorf("Expected toolName to be '%s', got '%s'", toolName, analyzer.GetToolName())
	}

	// Test with empty tool name
	emptyAnalyzer := NewDefaultToolAnalyzer("")
	if emptyAnalyzer == nil {
		t.Error("NewDefaultToolAnalyzer should not return nil even with empty name")
	}
	if emptyAnalyzer.GetToolName() != "" {
		t.Errorf("Expected empty toolName, got '%s'", emptyAnalyzer.GetToolName())
	}
}

// Test AnalyzeValidationFailure
func TestAnalyzeValidationFailure(t *testing.T) {
	analyzer := NewDefaultToolAnalyzer("test-tool")

	// Test with valid parameters
	err := analyzer.AnalyzeValidationFailure("/path/to/Dockerfile", "session-123")
	if err != nil {
		t.Errorf("AnalyzeValidationFailure should not return error: %v", err)
	}

	// Test with empty parameters
	err = analyzer.AnalyzeValidationFailure("", "")
	if err != nil {
		t.Errorf("AnalyzeValidationFailure should not return error even with empty params: %v", err)
	}

	// Test with one empty parameter
	err = analyzer.AnalyzeValidationFailure("/path/to/Dockerfile", "")
	if err != nil {
		t.Errorf("AnalyzeValidationFailure should not return error with empty sessionID: %v", err)
	}

	err = analyzer.AnalyzeValidationFailure("", "session-123")
	if err != nil {
		t.Errorf("AnalyzeValidationFailure should not return error with empty dockerfilePath: %v", err)
	}
}

// Test ToolAnalyzer interface compliance
func TestToolAnalyzerInterface(t *testing.T) {
	var analyzer ToolAnalyzer = NewDefaultToolAnalyzer("interface-test")

	// Verify it implements the interface correctly
	err := analyzer.AnalyzeValidationFailure("/test/path", "test-session")
	if err != nil {
		t.Errorf("Interface implementation should not return error: %v", err)
	}
}

// Test tool analyzer with different tool names
func TestToolAnalyzerVariousNames(t *testing.T) {
	testCases := []string{
		"build-tool",
		"deploy-tool",
		"analyze-tool",
		"test_tool_with_underscores",
		"tool-with-123-numbers",
		"UPPERCASE_TOOL",
	}

	for _, toolName := range testCases {
		analyzer := NewDefaultToolAnalyzer(toolName)
		if analyzer.GetToolName() != toolName {
			t.Errorf("Expected toolName '%s', got '%s'", toolName, analyzer.GetToolName())
		}

		// Test that it can analyze failures regardless of tool name
		err := analyzer.AnalyzeValidationFailure("/path/to/file", "session-id")
		if err != nil {
			t.Errorf("Tool '%s' should be able to analyze failures: %v", toolName, err)
		}
	}
}
