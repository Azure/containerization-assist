package build

import (
	"testing"
)

// Test DefaultToolAnalyzer constructor
func TestNewDefaultToolAnalyzer(t *testing.T) {
	toolName := "build-tool"
	analyzer := NewDefaultToolAnalyzer(toolName)

	if analyzer == nil {
		t.Error("NewDefaultToolAnalyzer should not return nil")
	}
	if analyzer.toolName != toolName {
		t.Errorf("Expected toolName to be '%s', got '%s'", toolName, analyzer.toolName)
	}

	// Test with empty tool name
	emptyAnalyzer := NewDefaultToolAnalyzer("")
	if emptyAnalyzer == nil {
		t.Error("NewDefaultToolAnalyzer should not return nil even with empty name")
	}
	if emptyAnalyzer.toolName != "" {
		t.Errorf("Expected empty toolName, got '%s'", emptyAnalyzer.toolName)
	}
}

// Test AnalyzeBuildFailure
func TestAnalyzeBuildFailure(t *testing.T) {
	analyzer := NewDefaultToolAnalyzer("build-tool")

	// Test with valid parameters
	err := analyzer.AnalyzeBuildFailure("session-123", "myapp:latest")
	if err != nil {
		t.Errorf("AnalyzeBuildFailure should not return error: %v", err)
	}

	// Test with empty parameters
	err = analyzer.AnalyzeBuildFailure("", "")
	if err != nil {
		t.Errorf("AnalyzeBuildFailure should not return error even with empty params: %v", err)
	}

	// Test with one empty parameter
	err = analyzer.AnalyzeBuildFailure("session-123", "")
	if err != nil {
		t.Errorf("AnalyzeBuildFailure should not return error with empty imageName: %v", err)
	}

	err = analyzer.AnalyzeBuildFailure("", "myapp:latest")
	if err != nil {
		t.Errorf("AnalyzeBuildFailure should not return error with empty sessionID: %v", err)
	}
}

// Test AnalyzePushFailure
func TestAnalyzePushFailure(t *testing.T) {
	analyzer := NewDefaultToolAnalyzer("push-tool")

	// Test with valid parameters
	err := analyzer.AnalyzePushFailure("myapp:latest", "session-123")
	if err != nil {
		t.Errorf("AnalyzePushFailure should not return error: %v", err)
	}

	// Test with empty parameters
	err = analyzer.AnalyzePushFailure("", "")
	if err != nil {
		t.Errorf("AnalyzePushFailure should not return error even with empty params: %v", err)
	}

	// Test with one empty parameter
	err = analyzer.AnalyzePushFailure("myapp:latest", "")
	if err != nil {
		t.Errorf("AnalyzePushFailure should not return error with empty sessionID: %v", err)
	}

	err = analyzer.AnalyzePushFailure("", "session-123")
	if err != nil {
		t.Errorf("AnalyzePushFailure should not return error with empty imageRef: %v", err)
	}
}

// Test AnalyzePullFailure
func TestAnalyzePullFailure(t *testing.T) {
	analyzer := NewDefaultToolAnalyzer("pull-tool")

	// Test with valid parameters
	err := analyzer.AnalyzePullFailure("nginx:latest", "session-456")
	if err != nil {
		t.Errorf("AnalyzePullFailure should not return error: %v", err)
	}

	// Test with empty parameters
	err = analyzer.AnalyzePullFailure("", "")
	if err != nil {
		t.Errorf("AnalyzePullFailure should not return error even with empty params: %v", err)
	}
}

// Test AnalyzeTagFailure
func TestAnalyzeTagFailure(t *testing.T) {
	analyzer := NewDefaultToolAnalyzer("tag-tool")

	// Test with valid parameters
	err := analyzer.AnalyzeTagFailure("myapp:latest", "myapp:v1.0.0", "session-789")
	if err != nil {
		t.Errorf("AnalyzeTagFailure should not return error: %v", err)
	}

	// Test with empty parameters
	err = analyzer.AnalyzeTagFailure("", "", "")
	if err != nil {
		t.Errorf("AnalyzeTagFailure should not return error even with empty params: %v", err)
	}

	// Test with some empty parameters
	err = analyzer.AnalyzeTagFailure("myapp:latest", "", "session-789")
	if err != nil {
		t.Errorf("AnalyzeTagFailure should not return error with empty targetImage: %v", err)
	}
}

// Test ToolAnalyzer interface compliance
func TestToolAnalyzerInterface(t *testing.T) {
	var analyzer ToolAnalyzer = NewDefaultToolAnalyzer("interface-test")

	// Verify it implements the interface correctly
	err := analyzer.AnalyzeBuildFailure("test-session", "test-image")
	if err != nil {
		t.Errorf("Interface implementation should not return error: %v", err)
	}

	err = analyzer.AnalyzePushFailure("test-image", "test-session")
	if err != nil {
		t.Errorf("Interface implementation should not return error: %v", err)
	}

	err = analyzer.AnalyzePullFailure("test-image", "test-session")
	if err != nil {
		t.Errorf("Interface implementation should not return error: %v", err)
	}

	err = analyzer.AnalyzeTagFailure("source-image", "target-image", "test-session")
	if err != nil {
		t.Errorf("Interface implementation should not return error: %v", err)
	}
}

// Test tool analyzer with different tool names
func TestToolAnalyzerVariousNames(t *testing.T) {
	testCases := []string{
		"docker-build",
		"docker-push",
		"docker-pull",
		"docker-tag",
		"buildkit",
		"kaniko",
	}

	for _, toolName := range testCases {
		analyzer := NewDefaultToolAnalyzer(toolName)
		if analyzer.toolName != toolName {
			t.Errorf("Expected toolName '%s', got '%s'", toolName, analyzer.toolName)
		}

		// Test that all methods work regardless of tool name
		if err := analyzer.AnalyzeBuildFailure("session", "image"); err != nil {
			t.Errorf("Tool '%s' should be able to analyze build failures: %v", toolName, err)
		}
		if err := analyzer.AnalyzePushFailure("image", "session"); err != nil {
			t.Errorf("Tool '%s' should be able to analyze push failures: %v", toolName, err)
		}
		if err := analyzer.AnalyzePullFailure("image", "session"); err != nil {
			t.Errorf("Tool '%s' should be able to analyze pull failures: %v", toolName, err)
		}
		if err := analyzer.AnalyzeTagFailure("src", "dst", "session"); err != nil {
			t.Errorf("Tool '%s' should be able to analyze tag failures: %v", toolName, err)
		}
	}
}
