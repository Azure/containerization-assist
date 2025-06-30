package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLocalProgressStage tests the LocalProgressStage struct
func TestLocalProgressStage(t *testing.T) {
	stage := LocalProgressStage{
		Name:        "Test Stage",
		Weight:      0.5,
		Description: "A test stage",
	}

	assert.Equal(t, "Test Stage", stage.Name)
	assert.Equal(t, 0.5, stage.Weight)
	assert.Equal(t, "A test stage", stage.Description)
}

// TestGoMCPProgressAdapter_BasicFunctionality tests basic adapter functionality
func TestGoMCPProgressAdapter_BasicFunctionality(t *testing.T) {
	// Create test stages
	stages := []LocalProgressStage{
		{Name: "Stage 1", Weight: 0.3, Description: "First stage"},
		{Name: "Stage 2", Weight: 0.7, Description: "Second stage"},
	}

	// Test without actual server context (will handle nil gracefully)
	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    stages,
		current:   0,
	}

	// Test ReportStage - should not panic with nil context
	adapter.ReportStage(0.5, "Test progress")

	// Test NextStage - should move from stage 0 to stage 1
	adapter.NextStage("Moving to next stage")
	currentIndex, _ := adapter.GetCurrentStage()
	assert.Equal(t, 1, currentIndex)

	// Test SetStage
	adapter.SetStage(0, "Back to first stage")
	currentIndex, currentStage := adapter.GetCurrentStage()
	assert.Equal(t, 0, currentIndex)
	assert.Equal(t, "Stage 1", currentStage.Name)

	// Test ReportOverall
	adapter.ReportOverall(0.75, "75% complete")

	// Test bounds checking - SetStage ignores invalid indices
	originalIndex, _ := adapter.GetCurrentStage()
	adapter.SetStage(-1, "Invalid negative")
	currentIndex, _ = adapter.GetCurrentStage()
	assert.Equal(t, originalIndex, currentIndex) // Should not change

	adapter.SetStage(100, "Invalid high")
	currentIndex, _ = adapter.GetCurrentStage()
	assert.Equal(t, originalIndex, currentIndex) // Should not change
}

// TestGoMCPProgressAdapter_EmptyStages tests adapter with no stages
func TestGoMCPProgressAdapter_EmptyStages(t *testing.T) {
	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    []LocalProgressStage{},
		current:   0,
	}

	// Should handle empty stages gracefully
	adapter.ReportStage(0.5, "Some progress")
	adapter.NextStage("Next stage")
	adapter.ReportOverall(0.5, "Overall progress")

	currentIndex, currentStage := adapter.GetCurrentStage()
	assert.Equal(t, -1, currentIndex) // -1 for empty stages
	assert.Equal(t, LocalProgressStage{}, currentStage)
}

// TestGoMCPProgressAdapter_SingleStage tests adapter with single stage
func TestGoMCPProgressAdapter_SingleStage(t *testing.T) {
	stages := []LocalProgressStage{
		{Name: "Only Stage", Weight: 1.0, Description: "The only stage"},
	}

	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    stages,
		current:   0,
	}

	// Test progression within single stage
	adapter.ReportStage(0.5, "Half done")

	// Try to move beyond - should stay at current stage
	adapter.NextStage("Trying to go beyond")
	currentIndex, _ := adapter.GetCurrentStage()
	assert.Equal(t, 0, currentIndex)
}

// TestGoMCPProgressAdapter_ProgressSequence simulates a realistic progress sequence
func TestGoMCPProgressAdapter_ProgressSequence(t *testing.T) {
	stages := []LocalProgressStage{
		{Name: "Initialize", Weight: 0.1, Description: "Setting up"},
		{Name: "Process", Weight: 0.8, Description: "Main processing"},
		{Name: "Finalize", Weight: 0.1, Description: "Cleanup"},
	}

	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "workflow-token",
		stages:    stages,
		current:   0,
	}

	// Stage 1: Initialize
	adapter.ReportStage(0.0, "Starting initialization")
	currentIndex, currentStage := adapter.GetCurrentStage()
	assert.Equal(t, 0, currentIndex)
	assert.Equal(t, "Initialize", currentStage.Name)

	adapter.ReportStage(1.0, "Initialization complete")

	// Move to Stage 2
	adapter.NextStage("Starting main processing")
	currentIndex, currentStage = adapter.GetCurrentStage()
	assert.Equal(t, 1, currentIndex)
	assert.Equal(t, "Process", currentStage.Name)

	// Stage 2: Process - multiple updates
	adapter.ReportStage(0.25, "25% of processing done")
	adapter.ReportStage(0.50, "50% of processing done")
	adapter.ReportStage(0.75, "75% of processing done")
	adapter.ReportStage(1.0, "Processing complete")

	// Move to final stage
	adapter.NextStage("Starting finalization")
	currentIndex, currentStage = adapter.GetCurrentStage()
	assert.Equal(t, 2, currentIndex)
	assert.Equal(t, "Finalize", currentStage.Name)

	// Stage 3: Finalize
	adapter.ReportStage(1.0, "All done")

	// Verify we're at the final stage
	currentIndex, currentStage = adapter.GetCurrentStage()
	assert.Equal(t, 2, currentIndex)
	assert.Equal(t, "Finalize", currentStage.Name)
	assert.Equal(t, 0.1, currentStage.Weight)
}

// TestGoMCPProgressAdapter_SetStageValidation tests stage index validation
func TestGoMCPProgressAdapter_SetStageValidation(t *testing.T) {
	stages := []LocalProgressStage{
		{Name: "Stage 1", Weight: 0.5, Description: "First"},
		{Name: "Stage 2", Weight: 0.5, Description: "Second"},
	}

	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    stages,
		current:   0,
	}

	// Test valid stage index
	adapter.SetStage(1, "Jump to stage 2")
	currentIndex, _ := adapter.GetCurrentStage()
	assert.Equal(t, 1, currentIndex)

	// Test negative index (should be ignored, stay at current stage)
	currentStage := currentIndex
	adapter.SetStage(-5, "Negative index")
	newIndex, _ := adapter.GetCurrentStage()
	assert.Equal(t, currentStage, newIndex) // Should not change

	// Test index beyond range (should be ignored)
	adapter.SetStage(10, "Beyond range")
	newIndex, _ = adapter.GetCurrentStage()
	assert.Equal(t, currentStage, newIndex) // Should not change

	// Test exact boundary values
	adapter.SetStage(0, "First stage")
	currentIndex, _ = adapter.GetCurrentStage()
	assert.Equal(t, 0, currentIndex)

	adapter.SetStage(len(stages)-1, "Last stage")
	currentIndex, _ = adapter.GetCurrentStage()
	assert.Equal(t, len(stages)-1, currentIndex)
}

// TestGoMCPProgressAdapter_WeightValidation tests stage weight handling
func TestGoMCPProgressAdapter_WeightValidation(t *testing.T) {
	// Test with weights that don't sum to 1.0
	stages := []LocalProgressStage{
		{Name: "Stage 1", Weight: 0.3, Description: "30%"},
		{Name: "Stage 2", Weight: 0.4, Description: "40%"},
		{Name: "Stage 3", Weight: 0.5, Description: "50%"}, // Total = 1.2
	}

	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    stages,
		current:   0,
	}

	// Should still work even if weights don't sum to 1.0
	adapter.ReportStage(1.0, "Complete stage 1")
	adapter.NextStage("Move to stage 2")

	currentIndex, currentStage := adapter.GetCurrentStage()
	assert.Equal(t, 1, currentIndex)
	assert.Equal(t, "Stage 2", currentStage.Name)
	assert.Equal(t, 0.4, currentStage.Weight)
}

// TestLocalProgressReporter_Interface tests that adapter implements the interface
func TestLocalProgressReporter_Interface(t *testing.T) {
	stages := []LocalProgressStage{
		{Name: "Test", Weight: 1.0, Description: "Test stage"},
	}

	adapter := &GoMCPProgressAdapter{
		serverCtx: nil,
		token:     "test-token",
		stages:    stages,
		current:   0,
	}

	// Verify it implements LocalProgressReporter interface
	var _ LocalProgressReporter = adapter

	// Test all interface methods
	adapter.ReportStage(0.5, "Test message")
	adapter.NextStage("Next stage message")
	adapter.SetStage(0, "Set stage message")
	adapter.ReportOverall(0.75, "Overall message")

	currentIndex, currentStage := adapter.GetCurrentStage()
	assert.Equal(t, 0, currentIndex)
	assert.Equal(t, "Test", currentStage.Name)
}
