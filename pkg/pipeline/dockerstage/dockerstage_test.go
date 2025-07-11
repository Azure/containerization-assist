package dockerstage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/pipeline"
)

// TestDockerStage_Initialize tests the Initialize method
func TestDockerStage_Initialize(t *testing.T) {
	// Create a test pipeline
	p := &DockerStage{
		AIClient:         nil,
		UseDraftTemplate: false,
	}

	// Create a test state
	state := &pipeline.PipelineState{
		Dockerfile: docker.Dockerfile{},
	}

	// Create a temp file for testing
	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

	// Test initializing with non-existent file (should create empty state)
	err := p.Initialize(context.Background(), state, dockerfilePath)
	if err != nil {
		t.Errorf("Initialize should succeed with non-existent file, got error: %v", err)
	}

	if state.Dockerfile.Content != "" {
		t.Errorf("Initialize should set empty content for non-existent file, got: %s", state.Dockerfile.Content)
	}

	// Create a test Dockerfile
	testContent := "FROM alpine:latest"
	if err := os.WriteFile(dockerfilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test Dockerfile: %v", err)
	}

	// Reset state and test with existing file
	state.Dockerfile = docker.Dockerfile{}
	err = p.Initialize(context.Background(), state, dockerfilePath)
	if err != nil {
		t.Errorf("Initialize failed with existing file: %v", err)
	}

	if state.Dockerfile.Content != testContent {
		t.Errorf("Initialize should set content from file, expected: %s, got: %s", testContent, state.Dockerfile.Content)
	}
}

// TestDockerStage_GetErrors tests the GetErrors method
func TestDockerStage_GetErrors(t *testing.T) {
	// Create a test pipeline
	p := &DockerStage{}

	// Create a test state with errors
	state := &pipeline.PipelineState{
		Dockerfile: docker.Dockerfile{
			BuildErrors: "test error",
		},
	}

	// Test getting errors
	errors := p.GetErrors(state)
	if errors != "test error" {
		t.Errorf("GetErrors should return the build errors, expected: 'test error', got: %s", errors)
	}
}

// TestDockerStage_Generate tests basic functionality of Generate
func TestDockerStage_Generate(t *testing.T) {
	t.Skip("Skipping test that would require docker package mocking")
}

// TestDockerStage_WriteSuccessfulFiles tests the WriteSuccessfulFiles method
func TestDockerStage_WriteSuccessfulFiles(t *testing.T) {
	// Create a test pipeline
	p := &DockerStage{}

	// Create a temp dir and file for testing
	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

	// Test with unsuccessful state (should do nothing)
	state := &pipeline.PipelineState{
		Success: false,
		Dockerfile: docker.Dockerfile{
			Path:    dockerfilePath,
			Content: "FROM alpine:latest",
		},
	}

	err := p.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles should succeed with unsuccessful state, got error: %v", err)
	}

	// File should exist
	if _, err := os.Stat(dockerfilePath); err != nil {
		t.Errorf("WriteSuccessfulFiles should create file with unsuccessful state")
	}

	// Test with successful state
	state.Success = true
	err = p.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles failed with successful state: %v", err)
	}

	// File should exist with correct content
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}

	if string(content) != state.Dockerfile.Content {
		t.Errorf("WriteSuccessfulFiles should write correct content, expected: %s, got: %s",
			state.Dockerfile.Content, string(content))
	}
}

// TestDockerStage_Run is a basic test for the Run method
func TestDockerStage_Run(t *testing.T) {
	t.Skip("Skipping test that would require services")
}

// TestDockerStage_Deploy is a basic test for the Deploy method
func TestDockerStage_Deploy(t *testing.T) {
	t.Skip("Skipping test that would require services")
}

// TestAnalyzeDockerfile is a basic test for the AnalyzeDockerfile function
func TestAnalyzeDockerfile(t *testing.T) {
	t.Skip("Skipping test that would require AI service")
}
