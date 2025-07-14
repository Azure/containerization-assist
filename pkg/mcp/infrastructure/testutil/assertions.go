package testutil

import (
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// AssertWorkflowSuccess verifies that a workflow completed successfully
func AssertWorkflowSuccess(t *testing.T, result *workflow.ContainerizeAndDeployResult, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if !result.Success {
		t.Fatalf("Expected success=true, got false. Error: %s", result.Error)
	}
}

// AssertWorkflowError verifies that a workflow failed with expected error
func AssertWorkflowError(t *testing.T, result *workflow.ContainerizeAndDeployResult, err error, expectedError string) {
	t.Helper()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error containing %q, got: %v", expectedError, err)
	}
}

// AssertRichError verifies Rich error properties
func AssertRichError(t *testing.T, err error, expectedCode errors.Code) {
	t.Helper()

	richErr, ok := err.(*errors.Rich)
	if !ok {
		t.Fatalf("Expected Rich error, got %T", err)
	}

	if richErr.Code != expectedCode {
		t.Errorf("Expected code %v, got %v", expectedCode, richErr.Code)
	}
}

// AssertStepResult verifies a step result
func AssertStepResult(t *testing.T, state *workflow.WorkflowState, stepName string, expectSuccess bool) {
	t.Helper()

	// Check the overall result
	if state.Result == nil {
		t.Fatal("WorkflowState.Result is nil")
	}

	// Check specific step results based on step name
	switch stepName {
	case "analyze":
		if state.AnalyzeResult == nil && expectSuccess {
			t.Errorf("Step %q: expected result, got nil", stepName)
		}
	case "dockerfile":
		if state.DockerfileResult == nil && expectSuccess {
			t.Errorf("Step %q: expected result, got nil", stepName)
		}
	case "build":
		if state.BuildResult == nil && expectSuccess {
			t.Errorf("Step %q: expected result, got nil", stepName)
		}
	case "scan":
		if state.ScanReport == nil && expectSuccess {
			t.Errorf("Step %q: expected result, got nil", stepName)
		}
	case "manifest", "deploy":
		if state.K8sResult == nil && expectSuccess {
			t.Errorf("Step %q: expected result, got nil", stepName)
		}
	default:
		// For other steps, just check overall success
		if state.Result.Success != expectSuccess {
			t.Errorf("Step %q: expected success=%v, got %v", stepName, expectSuccess, state.Result.Success)
		}
	}
}

// AssertDuration verifies that a duration is within expected bounds
func AssertDuration(t *testing.T, actual, expected, tolerance time.Duration) {
	t.Helper()

	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}

	if diff > tolerance {
		t.Errorf("Duration %v not within %v of expected %v", actual, tolerance, expected)
	}
}

// AssertImageRef verifies a Docker image reference format
func AssertImageRef(t *testing.T, imageRef string) {
	t.Helper()

	if imageRef == "" {
		t.Error("Image reference is empty")
		return
	}

	// Basic validation - should contain registry/repo:tag or repo:tag
	parts := strings.Split(imageRef, ":")
	if len(parts) != 2 {
		t.Errorf("Invalid image reference format: %s", imageRef)
	}
}

// AssertProgressUpdate verifies progress tracking
func AssertProgressUpdate(t *testing.T, tracker *MockProgressTracker, stepName string, minProgress float64) {
	t.Helper()

	updates := tracker.GetUpdates()
	found := false
	var lastProgress float64

	// Find the last update for this step
	for _, update := range updates {
		if update.Step == stepName {
			found = true
			lastProgress = update.Progress
		}
	}

	if found && lastProgress < minProgress {
		t.Errorf("Step %q progress %v < expected minimum %v", stepName, lastProgress, minProgress)
	}

	if !found {
		t.Errorf("No progress update found for step %q", stepName)
	}
}

// AssertEventPublished verifies that an event was published
func AssertEventPublished(t *testing.T, events []interface{}, eventType string) {
	t.Helper()

	found := false
	for _, event := range events {
		if e, ok := event.(interface{ EventType() string }); ok {
			if e.EventType() == eventType {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("Event type %q not found in published events", eventType)
	}
}

// AssertNoError is a simple helper for checking no error occurred
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// AssertError is a simple helper for checking an error occurred
func AssertError(t *testing.T, err error, context string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s: expected error, got nil", context)
	}
}

// AssertContains verifies a string contains a substring
func AssertContains(t *testing.T, haystack, needle, context string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected to contain %q, got %q", context, needle, haystack)
	}
}

// AssertMapHasKey verifies a map contains a key
func AssertMapHasKey(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()

	if _, exists := m[key]; !exists {
		t.Errorf("Map missing expected key %q", key)
	}
}
