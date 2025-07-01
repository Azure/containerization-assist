package orchestration

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// Test ToolMetadata type
func TestToolMetadata(t *testing.T) {
	metadata := core.ToolMetadata{
		Name:         "test-tool",
		Description:  "A test tool for validation",
		Version:      "1.0.0",
		Category:     "testing",
		Dependencies: []string{"docker", "kubernetes"},
		Capabilities: []string{"build", "deploy"},
		Requirements: []string{"docker_daemon"},
		Parameters:   map[string]string{"image": "string", "tag": "string"},
		Examples:     []core.ToolExample{{Name: "basic", Description: "Basic usage"}},
	}

	if metadata.Name != "test-tool" {
		t.Errorf("Expected Name to be 'test-tool', got '%s'", metadata.Name)
	}
	if metadata.Description != "A test tool for validation" {
		t.Errorf("Expected Description to be 'A test tool for validation', got '%s'", metadata.Description)
	}
	if metadata.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got '%s'", metadata.Version)
	}
	if metadata.Category != "testing" {
		t.Errorf("Expected Category to be 'testing', got '%s'", metadata.Category)
	}
	if len(metadata.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(metadata.Dependencies))
	}
	if metadata.Dependencies[0] != "docker" {
		t.Errorf("Expected first dependency to be 'docker', got '%s'", metadata.Dependencies[0])
	}
	if len(metadata.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(metadata.Capabilities))
	}
	if len(metadata.Requirements) != 1 {
		t.Errorf("Expected 1 requirement, got %d", len(metadata.Requirements))
	}
	if metadata.Parameters["image"] != "string" {
		t.Errorf("Expected Parameters['image'] to be 'string', got '%v'", metadata.Parameters["image"])
	}
	if len(metadata.Examples) != 1 {
		t.Errorf("Expected 1 example, got %d", len(metadata.Examples))
	}
}

// Test ToolExample type
func TestToolExample(t *testing.T) {
	example := core.ToolExample{
		Name:        "build-image",
		Description: "Build a Docker image",
		Input:       map[string]interface{}{"dockerfile": "Dockerfile", "context": "."},
		Output:      map[string]interface{}{"image_id": "sha256:abc123", "success": true},
	}

	if example.Name != "build-image" {
		t.Errorf("Expected Name to be 'build-image', got '%s'", example.Name)
	}
	if example.Description != "Build a Docker image" {
		t.Errorf("Expected Description to be 'Build a Docker image', got '%s'", example.Description)
	}
	if example.Input == nil {
		t.Error("Expected Input to not be nil")
	}
	if example.Output == nil {
		t.Error("Expected Output to not be nil")
	}

	// Test input map (Input is already map[string]interface{})
	if example.Input["dockerfile"] != "Dockerfile" {
		t.Errorf("Expected Input['dockerfile'] to be 'Dockerfile', got '%v'", example.Input["dockerfile"])
	}

	// Test output map (Output is already map[string]interface{})
	if example.Output["success"] != true {
		t.Errorf("Expected Output['success'] to be true, got '%v'", example.Output["success"])
	}
}
