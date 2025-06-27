package build

import (
	"testing"
)

// Test testPipelineAdapter GetSessionWorkspace
func TestTestPipelineAdapterGetSessionWorkspace(t *testing.T) {
	// Test with empty workspace dir
	adapter := &testPipelineAdapter{}

	result := adapter.GetSessionWorkspace("test-session")
	expected := "/workspace/test-session"

	if result != expected {
		t.Errorf("Expected workspace to be '%s', got '%s'", expected, result)
	}

	// Test with custom workspace dir
	adapter.workspaceDir = "/custom/workspace"
	result = adapter.GetSessionWorkspace("test-session")
	expected = "/custom/workspace"

	if result != expected {
		t.Errorf("Expected workspace to be '%s', got '%s'", expected, result)
	}
}

// Test testPipelineAdapter UpdateSessionFromDockerResults
func TestTestPipelineAdapterUpdateSessionFromDockerResults(t *testing.T) {
	adapter := &testPipelineAdapter{}

	err := adapter.UpdateSessionFromDockerResults("test-session", nil)

	if err != nil {
		t.Errorf("UpdateSessionFromDockerResults should not return error, got %v", err)
	}

	// Test with some result object
	err = adapter.UpdateSessionFromDockerResults("test-session", "some-result")

	if err != nil {
		t.Errorf("UpdateSessionFromDockerResults should not return error, got %v", err)
	}
}

// Test testPipelineAdapter BuildDockerImage
func TestTestPipelineAdapterBuildDockerImage(t *testing.T) {
	adapter := &testPipelineAdapter{}

	result, err := adapter.BuildDockerImage("test-session", "test-image:latest", "/path/to/Dockerfile")

	if err != nil {
		t.Errorf("BuildDockerImage should not return error, got %v", err)
	}
	if result == nil {
		t.Error("BuildDockerImage should return a result")
	}
	if result.ImageRef != "test-image:latest" {
		t.Errorf("Expected ImageRef to be 'test-image:latest', got '%s'", result.ImageRef)
	}
	if !result.Success {
		t.Error("Expected Success to be true")
	}
}

// Test testPipelineAdapter PullDockerImage
func TestTestPipelineAdapterPullDockerImage(t *testing.T) {
	adapter := &testPipelineAdapter{}

	err := adapter.PullDockerImage("test-session", "nginx:latest")

	if err != nil {
		t.Errorf("PullDockerImage should not return error, got %v", err)
	}
}

// Test testPipelineAdapter PushDockerImage
func TestTestPipelineAdapterPushDockerImage(t *testing.T) {
	adapter := &testPipelineAdapter{}

	err := adapter.PushDockerImage("test-session", "test-image:latest")

	if err != nil {
		t.Errorf("PushDockerImage should not return error, got %v", err)
	}
}

// Test testPipelineAdapter TagDockerImage
func TestTestPipelineAdapterTagDockerImage(t *testing.T) {
	adapter := &testPipelineAdapter{}

	err := adapter.TagDockerImage("test-session", "source:latest", "target:v1.0")

	if err != nil {
		t.Errorf("TagDockerImage should not return error, got %v", err)
	}
}
