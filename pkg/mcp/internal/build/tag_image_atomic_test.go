package build

import (
	"testing"
)

// Test standardTagStages function
func TestStandardTagStages(t *testing.T) {
	stages := standardTagStages()
	
	if len(stages) == 0 {
		t.Error("standardTagStages should return at least one stage")
	}

	expectedStages := []struct {
		name        string
		weight      float64
		description string
	}{
		{"Initialize", 0.10, "Loading session and validating inputs"},
		{"Check", 0.30, "Checking source image availability"},
		{"Tag", 0.40, "Tagging Docker image"},
		{"Verify", 0.15, "Verifying tag operation"},
		{"Finalize", 0.05, "Updating session state"},
	}

	if len(stages) != len(expectedStages) {
		t.Errorf("Expected %d stages, got %d", len(expectedStages), len(stages))
	}

	for i, expected := range expectedStages {
		if i >= len(stages) {
			t.Errorf("Missing stage %d", i)
			continue
		}

		stage := stages[i]
		if stage.Name != expected.name {
			t.Errorf("Stage %d: expected name '%s', got '%s'", i, expected.name, stage.Name)
		}
		if stage.Weight != expected.weight {
			t.Errorf("Stage %d: expected weight %f, got %f", i, expected.weight, stage.Weight)
		}
		if stage.Description != expected.description {
			t.Errorf("Stage %d: expected description '%s', got '%s'", i, expected.description, stage.Description)
		}
	}

	// Test that weights sum up to approximately 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("Expected total weight to be ~1.0, got %f", totalWeight)
	}
}

// Test AtomicTagImageArgs type
func TestAtomicTagImageArgs(t *testing.T) {
	args := AtomicTagImageArgs{
		SourceImage: "nginx:latest",
		TargetImage: "myregistry.com/nginx:production",
		Force:       true,
	}

	if args.SourceImage != "nginx:latest" {
		t.Errorf("Expected SourceImage to be 'nginx:latest', got '%s'", args.SourceImage)
	}
	if args.TargetImage != "myregistry.com/nginx:production" {
		t.Errorf("Expected TargetImage to be 'myregistry.com/nginx:production', got '%s'", args.TargetImage)
	}
	if !args.Force {
		t.Error("Expected Force to be true")
	}

	// Test with minimal args
	minimalArgs := AtomicTagImageArgs{
		SourceImage: "alpine:3.14",
		TargetImage: "myapp:latest",
	}

	if minimalArgs.SourceImage != "alpine:3.14" {
		t.Errorf("Expected SourceImage to be 'alpine:3.14', got '%s'", minimalArgs.SourceImage)
	}
	if minimalArgs.TargetImage != "myapp:latest" {
		t.Errorf("Expected TargetImage to be 'myapp:latest', got '%s'", minimalArgs.TargetImage)
	}
	if minimalArgs.Force {
		t.Error("Expected Force to be false by default")
	}
}

// Test AtomicTagImageResult type
func TestAtomicTagImageResult(t *testing.T) {
	result := AtomicTagImageResult{
		Success:      true,
		SessionID:    "session-123",
		WorkspaceDir: "/tmp/workspace",
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", result.SessionID)
	}
	if result.WorkspaceDir != "/tmp/workspace" {
		t.Errorf("Expected WorkspaceDir to be '/tmp/workspace', got '%s'", result.WorkspaceDir)
	}

	// Test with failure result
	failureResult := AtomicTagImageResult{
		Success:      false,
		SessionID:    "session-456",
		WorkspaceDir: "",
	}

	if failureResult.Success {
		t.Error("Expected Success to be false")
	}
	if failureResult.SessionID != "session-456" {
		t.Errorf("Expected SessionID to be 'session-456', got '%s'", failureResult.SessionID)
	}
	if failureResult.WorkspaceDir != "" {
		t.Errorf("Expected WorkspaceDir to be empty, got '%s'", failureResult.WorkspaceDir)
	}
}