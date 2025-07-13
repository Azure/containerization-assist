// Package properties provides integration tests for property-based testing and fuzzing.
package properties

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// TestWorkflowProperties tests all workflow invariants with property-based testing
func TestWorkflowProperties(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultPropertyTestConfig()
	config.MaxTestCases = 50 // Reduce for faster tests

	tester := NewPropertyTester(config, logger)
	tester.TestWorkflowInvariants(t)
}

// TestSagaProperties tests all saga invariants with property-based testing
func TestSagaProperties(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultPropertyTestConfig()
	config.MaxTestCases = 50 // Reduce for faster tests

	tester := NewPropertyTester(config, logger)
	tester.TestSagaInvariants(t)
}

// TestMCPToolFuzzing tests MCP tools with fuzzing
func TestMCPToolFuzzing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzzing in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultFuzzConfig()
	config.MaxIterations = 100 // Reduce for CI
	config.TimeoutPerTest = 5 * time.Second

	fuzzer := NewMCPFuzzer(config, logger)

	t.Run("ContainerizeAndDeploy", func(t *testing.T) {
		fuzzer.FuzzMCPToolArguments(t, "containerize_and_deploy")
	})

	t.Run("Chat", func(t *testing.T) {
		fuzzer.FuzzMCPToolArguments(t, "chat")
	})

	t.Run("WorkflowStatus", func(t *testing.T) {
		fuzzer.FuzzMCPToolArguments(t, "workflow_status")
	})
}

// TestSagaScenarioFuzzing tests saga scenarios with fuzzing
func TestSagaScenarioFuzzing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping saga fuzzing in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultFuzzConfig()
	config.MaxIterations = 100 // Reduce for CI

	fuzzer := NewMCPFuzzer(config, logger)
	fuzzer.FuzzSagaScenarios(t)
}

// TestSagaCompensationScenarios tests specific saga compensation scenarios
func TestSagaCompensationScenarios(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultPropertyTestConfig()
	tester := NewPropertyTester(config, logger)

	// Test specific compensation scenarios
	scenarios := []struct {
		name        string
		description string
		generator   func() *saga.SagaExecution
	}{
		{
			"PartialExecution",
			"Test saga with partial execution requiring compensation",
			func() *saga.SagaExecution {
				return tester.generatePartialExecutionSaga()
			},
		},
		{
			"MultiStepFailure",
			"Test saga with multiple step failures",
			func() *saga.SagaExecution {
				return tester.generateMultiStepFailureSaga()
			},
		},
		{
			"CompensationChain",
			"Test complex compensation chain",
			func() *saga.SagaExecution {
				return tester.generateCompensationChainSaga()
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			saga := scenario.generator()

			// Test all saga invariants
			invariants := []struct {
				name      string
				invariant SagaInvariant
			}{
				{"CompensationInReverseOrder", tester.compensationInReverseOrder},
				{"AllExecutedStepsCanBeCompensated", tester.allExecutedStepsCanBeCompensated},
				{"SagaStateTransitionsValid", tester.sagaStateTransitionsValid},
				{"CompensationIdempotency", tester.compensationIdempotency},
				{"SagaTimelineConsistency", tester.sagaTimelineConsistency},
				{"PartialExecutionCompensation", tester.partialExecutionCompensation},
			}

			for _, inv := range invariants {
				if !inv.invariant(saga) {
					t.Errorf("Invariant %s failed for scenario %s", inv.name, scenario.description)
				}
			}
		})
	}
}

// TestWorkflowProgressInvariants tests workflow progress invariants
func TestWorkflowProgressInvariants(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultPropertyTestConfig()
	tester := NewPropertyTester(config, logger)

	// Test specific workflow scenarios
	scenarios := []struct {
		name        string
		description string
		generator   func() *workflow.WorkflowState
	}{
		{
			"TestModeWorkflow",
			"Test workflow running in test mode",
			func() *workflow.WorkflowState {
				return tester.generateTestModeWorkflow()
			},
		},
		{
			"FailedWorkflow",
			"Test workflow that failed midway",
			func() *workflow.WorkflowState {
				return tester.generateFailedWorkflow()
			},
		},
		{
			"SuccessfulWorkflow",
			"Test completely successful workflow",
			func() *workflow.WorkflowState {
				return tester.generateSuccessfulWorkflow()
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			state := scenario.generator()

			// Test all workflow invariants
			invariants := []struct {
				name      string
				invariant WorkflowInvariant
			}{
				{"ProgressNeverDecreases", tester.progressNeverDecreases},
				{"AllStepsAccountedFor", tester.allStepsAccountedFor},
				{"TestModeNeverPerformsOperations", tester.testModeNeverPerformsOperations},
				{"SuccessfulWorkflowHasAllResults", tester.successfulWorkflowHasAllResults},
				{"FailedWorkflowHasErrorInfo", tester.failedWorkflowHasErrorInfo},
				{"WorkflowIDConsistency", tester.workflowIDConsistency},
				{"ProgressTrackerConsistency", tester.progressTrackerConsistency},
				{"StepOrderingCorrect", tester.stepOrderingCorrect},
			}

			for _, inv := range invariants {
				if !inv.invariant(state) {
					t.Errorf("Invariant %s failed for scenario %s", inv.name, scenario.description)
				}
			}
		})
	}
}

// Benchmark property testing performance
func BenchmarkPropertyTesting(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultPropertyTestConfig()
	config.MaxTestCases = 10 // Small number for benchmarking
	config.VerboseOutput = false

	tester := NewPropertyTester(config, logger)

	b.Run("WorkflowInvariants", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			state := tester.generateWorkflowState()
			_ = tester.progressNeverDecreases(state)
		}
	})

	b.Run("SagaInvariants", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			saga := tester.generateSagaExecution()
			_ = tester.compensationInReverseOrder(saga)
		}
	})
}

// Helper methods for specific scenario generation

// generatePartialExecutionSaga creates a saga with partial execution
func (pt *PropertyTester) generatePartialExecutionSaga() *saga.SagaExecution {
	steps := pt.generateSagaSteps()
	executeCount := pt.randIntn(len(steps)/2 + 1) // Execute less than half

	executed := make([]saga.SagaStepResult, executeCount)
	baseTime := time.Now().Add(-time.Hour)

	for i := 0; i < executeCount; i++ {
		executed[i] = saga.SagaStepResult{
			StepName:  steps[i].Name(),
			Success:   true,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Duration:  time.Duration(pt.randIntn(60)+10) * time.Second,
		}
	}

	// Compensate all executed steps
	compensated := make([]saga.SagaStepResult, executeCount)
	compensationTime := time.Now().Add(-30 * time.Minute)

	for i := executeCount - 1; i >= 0; i-- {
		compensatedIndex := executeCount - 1 - i
		compensated[compensatedIndex] = saga.SagaStepResult{
			StepName:  executed[i].StepName,
			Success:   true,
			Timestamp: compensationTime.Add(time.Duration(compensatedIndex) * time.Minute),
			Duration:  time.Duration(pt.randIntn(30)+5) * time.Second,
		}
	}

	return &saga.SagaExecution{
		ID:               pt.generateSagaID(),
		WorkflowID:       pt.generateWorkflowID(),
		State:            saga.SagaStateCompensated,
		Steps:            steps,
		ExecutedSteps:    executed,
		CompensatedSteps: compensated,
	}
}

// generateMultiStepFailureSaga creates a saga with multiple failure points
func (pt *PropertyTester) generateMultiStepFailureSaga() *saga.SagaExecution {
	// Generate a fixed set of steps for predictable testing
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag"}
	steps := make([]saga.SagaStep, len(stepNames))
	for i, name := range stepNames {
		steps[i] = &TestSagaStep{
			name:          name,
			canCompensate: true,
		}
	}

	// Execute 4 steps, with the last one failing
	executeCount := 4

	executed := make([]saga.SagaStepResult, executeCount)
	baseTime := time.Now().Add(-time.Hour)

	for i := 0; i < executeCount; i++ {
		success := true
		if i == executeCount-1 {
			success = false // Last executed step failed
		}

		executed[i] = saga.SagaStepResult{
			StepName:  steps[i].Name(),
			Success:   success,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Duration:  time.Duration(pt.randIntn(60)+10) * time.Second,
		}
	}

	// Compensate all successfully executed steps (including successful ones before failure)
	// Only the last failed step is not compensated
	successfulCount := 0
	for _, step := range executed {
		if step.Success {
			successfulCount++
		}
	}

	compensated := make([]saga.SagaStepResult, successfulCount)
	compensationTime := time.Now().Add(-20 * time.Minute)
	compensatedIndex := 0

	// Compensate in reverse order (successful steps only)
	for i := len(executed) - 1; i >= 0; i-- {
		if executed[i].Success {
			compensated[compensatedIndex] = saga.SagaStepResult{
				StepName:  executed[i].StepName,
				Success:   true,
				Timestamp: compensationTime.Add(time.Duration(compensatedIndex) * time.Minute),
				Duration:  time.Duration(pt.randIntn(20)+5) * time.Second,
			}
			compensatedIndex++
		}
	}

	return &saga.SagaExecution{
		ID:               pt.generateSagaID(),
		WorkflowID:       pt.generateWorkflowID(),
		State:            saga.SagaStateCompensated,
		Steps:            steps,
		ExecutedSteps:    executed,
		CompensatedSteps: compensated,
	}
}

// generateCompensationChainSaga creates a saga with complex compensation logic
func (pt *PropertyTester) generateCompensationChainSaga() *saga.SagaExecution {
	// Create a longer saga with more steps
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	steps := make([]saga.SagaStep, len(stepNames))

	for i, name := range stepNames {
		steps[i] = &TestSagaStep{
			name:          name,
			canCompensate: pt.randFloat64() < 0.9, // 90% can be compensated
		}
	}

	// Execute most steps successfully
	executeCount := len(steps) - 2 // Leave last 2 steps unexecuted
	executed := make([]saga.SagaStepResult, executeCount)
	baseTime := time.Now().Add(-2 * time.Hour)

	for i := 0; i < executeCount; i++ {
		executed[i] = saga.SagaStepResult{
			StepName:  steps[i].Name(),
			Success:   true,
			Timestamp: baseTime.Add(time.Duration(i*10) * time.Minute),
			Duration:  time.Duration(pt.randIntn(300)+60) * time.Second,
		}
	}

	// Compensate all in reverse order
	compensated := make([]saga.SagaStepResult, executeCount)
	compensationTime := time.Now().Add(-time.Hour)

	for i := executeCount - 1; i >= 0; i-- {
		compensatedIndex := executeCount - 1 - i
		compensated[compensatedIndex] = saga.SagaStepResult{
			StepName:  executed[i].StepName,
			Success:   true,
			Timestamp: compensationTime.Add(time.Duration(compensatedIndex*5) * time.Minute),
			Duration:  time.Duration(pt.randIntn(120)+30) * time.Second,
		}
	}

	return &saga.SagaExecution{
		ID:               pt.generateSagaID(),
		WorkflowID:       pt.generateWorkflowID(),
		State:            saga.SagaStateCompensated,
		Steps:            steps,
		ExecutedSteps:    executed,
		CompensatedSteps: compensated,
	}
}

// generateTestModeWorkflow creates a workflow in test mode
func (pt *PropertyTester) generateTestModeWorkflow() *workflow.WorkflowState {
	state := pt.generateWorkflowState()

	// Ensure test mode is enabled
	if state.Args != nil {
		state.Args.TestMode = true
	}

	// Ensure test mode characteristics
	if state.Result != nil {
		if state.Result.Success {
			state.Result.ImageRef = "test-registry/test-app:test-tag"
			state.Result.Namespace = "test-namespace"
			state.Result.Endpoint = "http://test-service.test-namespace.svc.cluster.local:8080"
		}
	}

	return state
}

// generateFailedWorkflow creates a workflow that failed
func (pt *PropertyTester) generateFailedWorkflow() *workflow.WorkflowState {
	state := pt.generateWorkflowState()

	// Ensure workflow failed
	if state.Result != nil {
		state.Result.Success = false
		state.Result.Error = "Build failed: dependency resolution error"
		state.Result.ImageRef = ""
		state.Result.Endpoint = ""

		// Add some completed steps and one failed step
		// Use all step names to avoid duplicates
		stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
		stepCount := pt.randIntn(8) + 3 // 3-10 steps (can fail at any point)
		if stepCount > 10 {
			stepCount = 10
		}
		state.Result.Steps = make([]workflow.WorkflowStep, stepCount)

		for i := 0; i < stepCount; i++ {
			status := "completed"
			var errorMsg string

			if i == stepCount-1 { // Last step failed
				status = "failed"
				errorMsg = "Step failed: " + pt.generateErrorMessage()
			}

			state.Result.Steps[i] = workflow.WorkflowStep{
				Name:     stepNames[i], // Use direct indexing to ensure proper order
				Status:   status,
				Duration: fmt.Sprintf("%ds", pt.randIntn(120)+10),
				Progress: fmt.Sprintf("%d/10", i+1),
				Message:  fmt.Sprintf("Step %s %s", stepNames[i], status),
				Retries:  pt.randIntn(3),
				Error:    errorMsg,
			}
		}

		// Update current step to reflect failure point
		state.CurrentStep = stepCount
	}

	return state
}

// generateSuccessfulWorkflow creates a completely successful workflow
func (pt *PropertyTester) generateSuccessfulWorkflow() *workflow.WorkflowState {
	state := pt.generateWorkflowState()

	// Ensure workflow succeeded
	if state.Result != nil {
		state.Result.Success = true
		state.Result.Error = ""

		// Respect test mode from args
		if state.Args != nil && state.Args.TestMode {
			state.Result.ImageRef = fmt.Sprintf("test-registry/test-%s:test-%s",
				pt.generateRandomString(8),
				pt.generateRandomString(6))
			state.Result.Namespace = "test-namespace"
			state.Result.Endpoint = fmt.Sprintf("http://test-%s.test-namespace.svc.cluster.local:8080",
				pt.generateRandomString(10))
		} else {
			state.Result.ImageRef = pt.generateImageRef()
			state.Result.Endpoint = pt.generateEndpoint()
			state.Result.Namespace = pt.generateRandomString(8)
		}

		// All 10 steps completed successfully
		state.Result.Steps = make([]workflow.WorkflowStep, 10)
		stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}

		for i := 0; i < 10; i++ {
			state.Result.Steps[i] = workflow.WorkflowStep{
				Name:     stepNames[i],
				Status:   "completed",
				Duration: fmt.Sprintf("%ds", pt.randIntn(180)+30),
				Progress: fmt.Sprintf("%d/10", i+1),
				Message:  fmt.Sprintf("Step %s completed successfully", stepNames[i]),
				Retries:  pt.randIntn(2), // Successful workflows can still have retries
			}
		}

		// Workflow completed all steps
		state.CurrentStep = 10
	}

	return state
}
