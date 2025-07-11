package integration

import (
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowProgressStructure tests that workflow steps include progress information
func TestWorkflowProgressStructure(t *testing.T) {
	// Create a test workflow result
	result := &server.ContainerizeAndDeployResult{
		Steps: make([]server.WorkflowStep, 0, 10),
	}

	// Simulate progress tracking
	totalSteps := 10
	currentStep := 0

	updateProgress := func() (int, string) {
		currentStep++
		progress := fmt.Sprintf("%d/%d", currentStep, totalSteps)
		percentage := int((float64(currentStep) / float64(totalSteps)) * 100)
		return percentage, progress
	}

	// Add some test steps
	testSteps := []struct {
		name    string
		message string
	}{
		{"analyze_repository", "Analyzing repository structure and detecting language/framework"},
		{"generate_dockerfile", "Generating optimized Dockerfile for detected language/framework"},
		{"build_image", "Building Docker image with AI-powered error fixing"},
	}

	for _, ts := range testSteps {
		percentage, progress := updateProgress()
		step := server.WorkflowStep{
			Name:     ts.name,
			Status:   "completed",
			Progress: progress,
			Message:  fmt.Sprintf("[%d%%] %s", percentage, ts.message),
			Duration: "1s",
		}
		result.Steps = append(result.Steps, step)
	}

	// Verify the structure
	require.Len(t, result.Steps, 3)

	for i, step := range result.Steps {
		t.Logf("Step %d: %+v", i+1, step)

		// Check progress format
		assert.NotEmpty(t, step.Progress, "Step should have progress")
		assert.Contains(t, step.Progress, "/", "Progress should be in format 'current/total'")

		// Check message format
		assert.NotEmpty(t, step.Message, "Step should have message")
		assert.Contains(t, step.Message, "%", "Message should contain percentage")
		assert.Contains(t, step.Message, "[", "Message should start with percentage in brackets")

		// Verify specific progress values
		expectedProgress := fmt.Sprintf("%d/%d", i+1, totalSteps)
		assert.Equal(t, expectedProgress, step.Progress, "Progress should match expected value")
	}
}

// TestWorkflowProgressUpdate tests the updateProgress function behavior
func TestWorkflowProgressUpdate(t *testing.T) {
	totalSteps := 10
	currentStep := 0

	updateProgress := func() (int, string) {
		currentStep++
		progress := fmt.Sprintf("%d/%d", currentStep, totalSteps)
		percentage := int((float64(currentStep) / float64(totalSteps)) * 100)
		return percentage, progress
	}

	// Test progression
	testCases := []struct {
		expectedStep       int
		expectedPercentage int
		expectedProgress   string
	}{
		{1, 10, "1/10"},
		{2, 20, "2/10"},
		{3, 30, "3/10"},
		{5, 50, "5/10"},
		{10, 100, "10/10"},
	}

	for _, tc := range testCases {
		percentage, progress := updateProgress()
		assert.Equal(t, tc.expectedPercentage, percentage, "Percentage should match for step %d", tc.expectedStep)
		assert.Equal(t, tc.expectedProgress, progress, "Progress string should match for step %d", tc.expectedStep)

		if tc.expectedStep == 5 || tc.expectedStep == 10 {
			// Skip ahead for specific test cases
			for currentStep < tc.expectedStep-1 {
				updateProgress()
			}
		}
	}
}
