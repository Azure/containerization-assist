package manifestpipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// TestManifestPipeline_Initialize tests the Initialize method
func TestManifestPipeline_Initialize(t *testing.T) {
	// Create a test pipeline
	p := &ManifestPipeline{
		AIClient: nil, 
	}

	// Create a test state
	state := &pipeline.PipelineState{
		K8sObjects: make(map[string]*k8s.K8sObject),
	}

	// Create temp dir for testing
	tmpDir := t.TempDir()

	// Initialize should now succeed with no manifests
	err := p.Initialize(context.Background(), state, tmpDir)
	if err != nil {
		t.Errorf("Initialize should succeed with no manifests, got error: %v", err)
	}

	// The K8sObjects map should still be initialized
	if state.K8sObjects == nil {
		t.Errorf("K8sObjects should be initialized to an empty map, not nil")
	}
}

// TestManifestPipeline_GetErrors tests the GetErrors method
func TestManifestPipeline_GetErrors(t *testing.T) {
	// Create a test pipeline
	p := &ManifestPipeline{}

	// Create a test state with errors
	state := &pipeline.PipelineState{
		K8sObjects: map[string]*k8s.K8sObject{
			"test-deployment": {
				ErrorLog: "test error",
			},
		},
	}

	// Test getting errors
	errors := p.GetErrors(state)
	if errors == "" {
		t.Errorf("GetErrors should return errors when present")
	}
}

// TestManifestPipeline_Generate tests basic functionality of Generate
func TestManifestPipeline_Generate(t *testing.T) {
	t.Skip("Skipping test that would require docker package and Draft integration")
}

// TestManifestPipeline_WriteSuccessfulFiles tests the WriteSuccessfulFiles method
func TestManifestPipeline_WriteSuccessfulFiles(t *testing.T) {
	// Create a test pipeline
	p := &ManifestPipeline{}

	// Create a temp dir for testing
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "deployment.yaml")

	// Test with unsuccessful state (should do nothing)
	state := &pipeline.PipelineState{
		Success: false,
		K8sObjects: map[string]*k8s.K8sObject{
			"test-deployment": {
				ManifestPath:           manifestPath,
				Content:                []byte("apiVersion: apps/v1\nkind: Deployment"),
				IsSuccessfullyDeployed: true,
			},
		},
	}

	err := p.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles should succeed with unsuccessful state, got error: %v", err)
	}

	// File should not exist
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Errorf("WriteSuccessfulFiles should not create file with unsuccessful state")
	}

	// Test with successful state
	state.Success = true
	err = p.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles failed with successful state: %v", err)
	}

	// File should exist with correct content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}

	expectedContent := state.K8sObjects["test-deployment"].Content
	if string(content) != string(expectedContent) {
		t.Errorf("WriteSuccessfulFiles should write correct content, expected: %s, got: %s",
			string(expectedContent), string(content))
	}
}

// TestManifestPipeline_Run is a basic test for the Run method
func TestManifestPipeline_Run(t *testing.T) {
	t.Skip("Skipping test that would require services")
}

// TestManifestPipeline_Deploy is a basic test for the Deploy method
func TestManifestPipeline_Deploy(t *testing.T) {
	t.Skip("Skipping test that would require services")
}

// TestGetPendingManifests tests the GetPendingManifests function
func TestGetPendingManifests(t *testing.T) {
	// Create a test state with some manifests
	state := &pipeline.PipelineState{
		K8sObjects: make(map[string]*k8s.K8sObject),
	}

	pending := GetPendingManifests(state)
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending manifests, got: %d", len(pending))
	}

	// Add a pending manifest
	state.K8sObjects["test-deployment"] = &k8s.K8sObject{
		IsSuccessfullyDeployed: false,
	}

	pending = GetPendingManifests(state)
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending manifest, got: %d", len(pending))
	}
}

// TestAnalyzeKubernetesManifest is a basic test for the analyzeKubernetesManifest function
func TestAnalyzeKubernetesManifest(t *testing.T) {
	t.Skip("Skipping test that would require AI service")
}

// TestDeployStateManifests is a basic test for the DeployStateManifests function
func TestDeployStateManifests(t *testing.T) {
	t.Skip("Skipping test that would require services")
}
