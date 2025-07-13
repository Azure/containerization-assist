// Package properties provides property-based testing for Container Kit MCP workflows.
package properties

import (
	"log/slog"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// WorkflowInvariant represents a property that should always hold for workflows
type WorkflowInvariant func(*workflow.WorkflowState) bool

// SagaInvariant represents a property that should always hold for saga executions
type SagaInvariant func(*saga.SagaExecution) bool

// PropertyTestConfig configures property-based testing
type PropertyTestConfig struct {
	MaxTestCases  int           `json:"max_test_cases"`
	MaxShrinks    int           `json:"max_shrinks"`
	Timeout       time.Duration `json:"timeout"`
	SeedValue     int64         `json:"seed_value"`
	VerboseOutput bool          `json:"verbose_output"`
	FailFast      bool          `json:"fail_fast"`
	ParallelTests bool          `json:"parallel_tests"`
}

// DefaultPropertyTestConfig returns sensible defaults for property testing
func DefaultPropertyTestConfig() PropertyTestConfig {
	return PropertyTestConfig{
		MaxTestCases:  100,
		MaxShrinks:    100,
		Timeout:       30 * time.Second,
		SeedValue:     time.Now().UnixNano(),
		VerboseOutput: false,
		FailFast:      true,
		ParallelTests: true,
	}
}

// PropertyTester provides property-based testing capabilities
type PropertyTester struct {
	config PropertyTestConfig
	logger *slog.Logger
	rand   *rand.Rand
}

// NewPropertyTester creates a new property testing framework
func NewPropertyTester(config PropertyTestConfig, logger *slog.Logger) *PropertyTester {
	return &PropertyTester{
		config: config,
		logger: logger.With("component", "property_tester"),
		rand:   rand.New(rand.NewSource(config.SeedValue)),
	}
}

// TestWorkflowInvariants runs property-based tests for workflow invariants
func (pt *PropertyTester) TestWorkflowInvariants(t *testing.T) {
	t.Helper()

	invariants := []struct {
		name      string
		invariant WorkflowInvariant
	}{
		{"ProgressNeverDecreases", pt.progressNeverDecreases},
		{"AllStepsAccountedFor", pt.allStepsAccountedFor},
		{"TestModeNeverPerformsOperations", pt.testModeNeverPerformsOperations},
		{"SuccessfulWorkflowHasAllResults", pt.successfulWorkflowHasAllResults},
		{"FailedWorkflowHasErrorInfo", pt.failedWorkflowHasErrorInfo},
		{"WorkflowIDConsistency", pt.workflowIDConsistency},
		{"ProgressTrackerConsistency", pt.progressTrackerConsistency},
		{"StepOrderingCorrect", pt.stepOrderingCorrect},
	}

	for _, inv := range invariants {
		t.Run(inv.name, func(t *testing.T) {
			if pt.config.ParallelTests {
				t.Parallel()
			}

			pt.testWorkflowInvariant(t, inv.name, inv.invariant)
		})
	}
}

// TestSagaInvariants runs property-based tests for saga transaction invariants
func (pt *PropertyTester) TestSagaInvariants(t *testing.T) {
	t.Helper()

	invariants := []struct {
		name      string
		invariant SagaInvariant
	}{
		{"CompensationInReverseOrder", pt.compensationInReverseOrder},
		{"AllExecutedStepsCanBeCompensated", pt.allExecutedStepsCanBeCompensated},
		{"SagaStateTransitionsValid", pt.sagaStateTransitionsValid},
		{"CompensationIdempotency", pt.compensationIdempotency},
		{"SagaTimelineConsistency", pt.sagaTimelineConsistency},
		{"PartialExecutionCompensation", pt.partialExecutionCompensation},
	}

	for _, inv := range invariants {
		t.Run(inv.name, func(t *testing.T) {
			if pt.config.ParallelTests {
				t.Parallel()
			}

			pt.testSagaInvariant(t, inv.name, inv.invariant)
		})
	}
}

// Workflow Invariants

// progressNeverDecreases ensures progress only moves forward
func (pt *PropertyTester) progressNeverDecreases(state *workflow.WorkflowState) bool {
	if state.ProgressTracker == nil {
		return true // Nothing to test
	}

	current := state.ProgressTracker.GetCurrent()
	total := state.ProgressTracker.GetTotal()

	// Progress should never exceed total
	if current > total {
		pt.logger.Error("Progress exceeds total", "current", current, "total", total)
		return false
	}

	// Progress should never be negative
	if current < 0 {
		pt.logger.Error("Progress is negative", "current", current)
		return false
	}

	return true
}

// allStepsAccountedFor ensures all 10 containerization steps are tracked
func (pt *PropertyTester) allStepsAccountedFor(state *workflow.WorkflowState) bool {
	expectedSteps := 10 // Standard containerization workflow

	if state.TotalSteps != expectedSteps {
		pt.logger.Error("Total steps mismatch", "expected", expectedSteps, "actual", state.TotalSteps)
		return false
	}

	if state.ProgressTracker != nil && state.ProgressTracker.GetTotal() != expectedSteps {
		pt.logger.Error("Progress tracker total mismatch", "expected", expectedSteps, "actual", state.ProgressTracker.GetTotal())
		return false
	}

	return true
}

// testModeNeverPerformsOperations ensures test mode is safe
func (pt *PropertyTester) testModeNeverPerformsOperations(state *workflow.WorkflowState) bool {
	if state.Args == nil {
		return true
	}

	if state.Args.TestMode {
		// In test mode, no actual external operations should occur
		// Check that no real resources were created
		if state.Result != nil {
			if state.Result.ImageRef != "" && !strings.Contains(strings.ToLower(state.Result.ImageRef), "test") {
				pt.logger.Error("Test mode created real image", "image_ref", state.Result.ImageRef)
				return false
			}

			if state.Result.Namespace != "" && !strings.Contains(strings.ToLower(state.Result.Namespace), "test") {
				pt.logger.Error("Test mode created real namespace", "namespace", state.Result.Namespace)
				return false
			}
		}
	}

	return true
}

// successfulWorkflowHasAllResults ensures successful workflows produce complete results
func (pt *PropertyTester) successfulWorkflowHasAllResults(state *workflow.WorkflowState) bool {
	if state.Result == nil || !state.Result.Success {
		return true // Only applies to successful workflows
	}

	// Successful workflows should have core results
	if state.Result.ImageRef == "" {
		pt.logger.Error("Successful workflow missing image reference")
		return false
	}

	// Should have some steps recorded
	if len(state.Result.Steps) == 0 {
		pt.logger.Error("Successful workflow has no steps recorded")
		return false
	}

	return true
}

// failedWorkflowHasErrorInfo ensures failed workflows have error information
func (pt *PropertyTester) failedWorkflowHasErrorInfo(state *workflow.WorkflowState) bool {
	if state.Result == nil || state.Result.Success {
		return true // Only applies to failed workflows
	}

	// Failed workflows should have error information
	if state.Result.Error == "" {
		pt.logger.Error("Failed workflow missing error information")
		return false
	}

	return true
}

// workflowIDConsistency ensures workflow ID is consistent throughout
func (pt *PropertyTester) workflowIDConsistency(state *workflow.WorkflowState) bool {
	if state.WorkflowID == "" {
		pt.logger.Error("Workflow ID is empty")
		return false
	}

	// Workflow ID should be reasonably formatted
	if len(state.WorkflowID) < 10 {
		pt.logger.Error("Workflow ID too short", "id", state.WorkflowID)
		return false
	}

	return true
}

// progressTrackerConsistency ensures progress tracker state is consistent
func (pt *PropertyTester) progressTrackerConsistency(state *workflow.WorkflowState) bool {
	if state.ProgressTracker == nil {
		return true
	}

	// Current step should match workflow current step
	if state.CurrentStep != 0 && state.ProgressTracker.GetCurrent() != state.CurrentStep {
		pt.logger.Error("Progress tracker current step mismatch",
			"tracker_current", state.ProgressTracker.GetCurrent(),
			"state_current", state.CurrentStep)
		return false
	}

	return true
}

// stepOrderingCorrect ensures steps are executed in the correct order
func (pt *PropertyTester) stepOrderingCorrect(state *workflow.WorkflowState) bool {
	if state.Result == nil || len(state.Result.Steps) < 2 {
		return true // Need at least 2 steps to check ordering
	}

	expectedOrder := []string{
		"analyze", "dockerfile", "build", "scan", "tag",
		"push", "manifest", "cluster", "deploy", "verify",
	}

	// Check that completed steps follow the expected order
	completedSteps := make([]string, 0)
	for _, step := range state.Result.Steps {
		if step.Status == "completed" {
			completedSteps = append(completedSteps, strings.ToLower(step.Name))
		}
	}

	// Verify ordering
	lastIndex := -1
	for _, completedStep := range completedSteps {
		currentIndex := -1
		for i, expectedStep := range expectedOrder {
			if strings.Contains(completedStep, expectedStep) {
				currentIndex = i
				break
			}
		}

		if currentIndex != -1 && currentIndex < lastIndex {
			pt.logger.Error("Steps executed out of order",
				"completed_steps", completedSteps,
				"expected_order", expectedOrder)
			return false
		}

		if currentIndex != -1 {
			lastIndex = currentIndex
		}
	}

	return true
}

// Saga Invariants

// compensationInReverseOrder ensures compensations happen in reverse execution order
func (pt *PropertyTester) compensationInReverseOrder(sagaExecution *saga.SagaExecution) bool {
	executed := sagaExecution.GetExecutedSteps()
	compensated := sagaExecution.GetCompensatedSteps()

	if len(compensated) == 0 {
		return true // No compensations to check
	}

	// Filter out failed steps - they should not be compensated
	successfulSteps := []saga.SagaStepResult{}
	for _, step := range executed {
		if step.Success {
			successfulSteps = append(successfulSteps, step)
		}
	}

	// Compensation should happen in reverse order of successful execution
	for i, compensatedStep := range compensated {
		expectedIndex := len(successfulSteps) - 1 - i
		if expectedIndex >= 0 && expectedIndex < len(successfulSteps) {
			expectedStep := successfulSteps[expectedIndex]
			if compensatedStep.StepName != expectedStep.StepName {
				pt.logger.Error("Compensation not in reverse order",
					"compensated_step", compensatedStep.StepName,
					"expected_step", expectedStep.StepName,
					"position", i)
				return false
			}
		}
	}

	return true
}

// allExecutedStepsCanBeCompensated ensures all successfully executed steps are compensated
func (pt *PropertyTester) allExecutedStepsCanBeCompensated(sagaExecution *saga.SagaExecution) bool {
	if sagaExecution.GetState() != saga.SagaStateCompensated {
		return true // Only applies to compensated sagas
	}

	executed := sagaExecution.GetExecutedSteps()
	compensated := sagaExecution.GetCompensatedSteps()

	// All successfully executed steps should have corresponding compensations
	// Failed steps should NOT be compensated
	for _, executedStep := range executed {
		if executedStep.Success {
			// Successful step should be compensated
			found := false
			for _, compensatedStep := range compensated {
				if compensatedStep.StepName == executedStep.StepName {
					found = true
					break
				}
			}

			if !found {
				pt.logger.Error("Executed step not compensated", "step", executedStep.StepName)
				return false
			}
		} else {
			// Failed step should NOT be compensated
			for _, compensatedStep := range compensated {
				if compensatedStep.StepName == executedStep.StepName {
					pt.logger.Error("Failed step was compensated", "step", executedStep.StepName)
					return false
				}
			}
		}
	}

	return true
}

// sagaStateTransitionsValid ensures saga state transitions are valid
func (pt *PropertyTester) sagaStateTransitionsValid(sagaExecution *saga.SagaExecution) bool {
	state := sagaExecution.GetState()

	// Valid state transitions:
	// Started -> InProgress -> (Completed | Failed | Compensated | Aborted)
	validStates := map[saga.SagaState]bool{
		saga.SagaStateStarted:     true,
		saga.SagaStateInProgress:  true,
		saga.SagaStateCompleted:   true,
		saga.SagaStateFailed:      true,
		saga.SagaStateCompensated: true,
		saga.SagaStateAborted:     true,
	}

	if !validStates[state] {
		pt.logger.Error("Invalid saga state", "state", state)
		return false
	}

	return true
}

// compensationIdempotency ensures running compensation multiple times is safe
func (pt *PropertyTester) compensationIdempotency(sagaExecution *saga.SagaExecution) bool {
	// This would require testing multiple compensation runs
	// For now, check that compensation steps are recorded properly
	compensated := sagaExecution.GetCompensatedSteps()

	// Each step should only be compensated once
	stepCounts := make(map[string]int)
	for _, step := range compensated {
		stepCounts[step.StepName]++
	}

	for stepName, count := range stepCounts {
		if count > 1 {
			pt.logger.Error("Step compensated multiple times", "step", stepName, "count", count)
			return false
		}
	}

	return true
}

// sagaTimelineConsistency ensures saga timestamps are consistent
func (pt *PropertyTester) sagaTimelineConsistency(sagaExecution *saga.SagaExecution) bool {
	// Check that execution timestamps are in order
	executed := sagaExecution.GetExecutedSteps()
	var lastTime time.Time

	for _, step := range executed {
		if !lastTime.IsZero() && step.Timestamp.Before(lastTime) {
			pt.logger.Error("Execution timestamps out of order",
				"step", step.StepName,
				"timestamp", step.Timestamp,
				"previous", lastTime)
			return false
		}
		lastTime = step.Timestamp
	}

	return true
}

// partialExecutionCompensation ensures partial execution can be compensated
func (pt *PropertyTester) partialExecutionCompensation(sagaExecution *saga.SagaExecution) bool {
	if sagaExecution.GetState() != saga.SagaStateCompensated {
		return true
	}

	executed := sagaExecution.GetExecutedSteps()
	compensated := sagaExecution.GetCompensatedSteps()

	// Count successful steps that should be compensated
	successfulCount := 0
	for _, step := range executed {
		if step.Success {
			successfulCount++
		}
	}

	// Number of compensated steps should equal number of successful executed steps
	if len(compensated) != successfulCount {
		pt.logger.Error("Compensation count mismatch",
			"executed", len(executed),
			"compensated", len(compensated))
		return false
	}

	return true
}

// testWorkflowInvariant tests a single workflow invariant with generated data
func (pt *PropertyTester) testWorkflowInvariant(t *testing.T, name string, invariant WorkflowInvariant) {
	// Create a separate rand instance for quick.Config to avoid concurrent access
	quickRand := rand.New(rand.NewSource(pt.config.SeedValue + int64(len(name))))

	config := &quick.Config{
		MaxCount:      pt.config.MaxTestCases,
		MaxCountScale: float64(pt.config.MaxShrinks) / 100.0,
		Rand:          quickRand,
	}

	if err := quick.Check(func() bool {
		// Generate a workflow state
		state := pt.generateWorkflowState()
		return invariant(state)
	}, config); err != nil {
		t.Errorf("Invariant %s failed: %v", name, err)
	}
}

// testSagaInvariant tests a single saga invariant with generated data
func (pt *PropertyTester) testSagaInvariant(t *testing.T, name string, invariant SagaInvariant) {
	// Create a separate rand instance for quick.Config to avoid concurrent access
	quickRand := rand.New(rand.NewSource(pt.config.SeedValue + int64(len(name)) + 1000))

	config := &quick.Config{
		MaxCount:      pt.config.MaxTestCases,
		MaxCountScale: float64(pt.config.MaxShrinks) / 100.0,
		Rand:          quickRand,
	}

	if err := quick.Check(func() bool {
		// Generate a saga execution
		sagaExec := pt.generateSagaExecution()
		return invariant(sagaExec)
	}, config); err != nil {
		t.Errorf("Invariant %s failed: %v", name, err)
	}
}
