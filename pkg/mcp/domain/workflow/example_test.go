package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleBaseOrchestrator_Execute demonstrates testing the orchestrator with mocks
func ExampleBaseOrchestrator_Execute() {
	// This example shows how to test the BaseOrchestrator using our test utilities

	// Create test logger that captures output
	logger := testutil.NewDiscardLogger()

	// Create mock step provider
	stepProvider := testutil.NewMockStepProvider()

	// Create orchestrator with mocked dependencies
	factory := workflow.NewStepFactory(stepProvider, nil, nil, logger)
	orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger)

	// Create test context and arguments
	ctx := context.Background()
	args := &workflow.ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/example/repo",
	}

	// Execute workflow
	result, err := orchestrator.Execute(ctx, nil, args)
	if err != nil {
		panic(err)
	}

	// Use result
	_ = result
}

// TestOrchestrator_SuccessfulWorkflow demonstrates testing a complete successful workflow
func TestOrchestrator_SuccessfulWorkflow(t *testing.T) {
	// Arrange
	logger := testutil.NewTestLogger(t)

	// Create mock steps with successful execution
	stepProvider := testutil.NewMockStepProvider()

	// Mock analyze step
	analyzeStep := testutil.NewMockStep("analyze")
	analyzeStep.ExecuteFunc = func(ctx context.Context, state *workflow.WorkflowState) error {
		// Simulate successful analysis
		state.CurrentStep++
		return nil
	}
	stepProvider.SetStep("analyze", analyzeStep)

	// Mock dockerfile step
	dockerfileStep := testutil.NewMockStep("dockerfile")
	dockerfileStep.ExecuteFunc = func(ctx context.Context, state *workflow.WorkflowState) error {
		// Simulate successful dockerfile generation
		state.CurrentStep++
		return nil
	}
	stepProvider.SetStep("dockerfile", dockerfileStep)

	// Create orchestrator
	factory := workflow.NewStepFactory(stepProvider, nil, nil, logger.Logger)
	orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger.Logger)

	// Create test data using fixture builder
	ctx, cancel := testutil.NewFixtureBuilder().
		WithWorkflowID("test-success-123").
		BuildContext()
	defer cancel()

	args := testutil.NewFixtureBuilder().
		WithRepoURL("https://github.com/test/successful-repo").
		BuildArgs()

	// Act
	result, err := orchestrator.Execute(ctx, nil, args)

	// Assert
	testutil.AssertWorkflowSuccess(t, result, err)
	assert.Equal(t, 1, analyzeStep.GetExecuteCount(), "analyze step should be executed")
	assert.Equal(t, 1, dockerfileStep.GetExecuteCount(), "dockerfile step should be executed")
	testutil.AssertLogged(t, logger, "Starting containerize_and_deploy workflow")
	testutil.AssertLogged(t, logger, "analyze")
}

// TestOrchestrator_StepFailure demonstrates testing workflow failure scenarios
func TestOrchestrator_StepFailure(t *testing.T) {
	tests := []struct {
		name        string
		failingStep string
		expectedErr string
		expectRetry bool
	}{
		{
			name:        "analyze step fails",
			failingStep: "analyze",
			expectedErr: "repository analysis failed",
			expectRetry: false,
		},
		{
			name:        "build step fails with retry",
			failingStep: "build",
			expectedErr: "docker build failed",
			expectRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewTestLogger(t)
			stepProvider := testutil.NewMockStepProvider()

			// Create failing step
			failingStep := testutil.MockStepWithError(tt.failingStep, errors.New(tt.expectedErr))
			if tt.expectRetry {
				failingStep.MaxRetriesFunc = func() int { return 3 }
			}
			stepProvider.SetStep(tt.failingStep, failingStep)

			factory := workflow.NewStepFactory(stepProvider, nil, nil, logger.Logger)
			orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger.Logger)

			ctx, cancel := testutil.NewFixtureBuilder().BuildContext()
			defer cancel()
			args := testutil.NewFixtureBuilder().BuildArgs()

			// Act
			result, err := orchestrator.Execute(ctx, nil, args)

			// Assert
			testutil.AssertWorkflowError(t, result, err, tt.expectedErr)
			if tt.expectRetry {
				// With retry middleware, step might be executed multiple times
				assert.GreaterOrEqual(t, failingStep.GetExecuteCount(), 1)
			}
			testutil.AssertLogged(t, logger, "failed")
		})
	}
}

// TestOrchestrator_ContextCancellation demonstrates testing context cancellation
func TestOrchestrator_ContextCancellation(t *testing.T) {
	// Arrange
	logger := testutil.NewTestLogger(t)
	stepProvider := testutil.NewMockStepProvider()

	// Create a slow step that will be cancelled
	slowStep := testutil.MockStepWithDelay("analyze", 5*time.Second)
	stepProvider.SetStep("analyze", slowStep)

	factory := workflow.NewStepFactory(stepProvider, nil, nil, logger.Logger)
	orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger.Logger)

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	args := testutil.NewFixtureBuilder().BuildArgs()

	// Cancel context after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Act
	result, err := orchestrator.Execute(ctx, nil, args)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.False(t, result.Success)
}

// TestWorkflowState_ProgressTracking demonstrates testing progress tracking
func TestWorkflowState_ProgressTracking(t *testing.T) {
	// Arrange
	mockTracker := testutil.NewMockProgressTracker()
	_ = testutil.NewFixtureBuilder().BuildWorkflowState() // State would be used in real workflow

	// Simulate progress updates
	mockTracker.Update("analyze", "Analyzing repository", 0.1)
	mockTracker.Update("analyze", "Detected Go project", 0.5)
	mockTracker.Update("analyze", "Analysis complete", 1.0)

	// Assert
	updates := mockTracker.GetUpdates()
	assert.Len(t, updates, 3)
	testutil.AssertProgressUpdate(t, mockTracker, "analyze", 1.0)

	// Verify progress increases
	for i := 1; i < len(updates); i++ {
		assert.GreaterOrEqual(t, updates[i].Progress, updates[i-1].Progress,
			"progress should increase or stay same")
	}
}

// TestWorkflowMiddleware_Example demonstrates testing with middleware
func TestWorkflowMiddleware_Example(t *testing.T) {
	// Arrange
	var executionOrder []string

	// Create test middleware
	testMiddleware := func(next workflow.StepHandler) workflow.StepHandler {
		return func(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) error {
			executionOrder = append(executionOrder, "before-"+step.Name())
			err := next(ctx, step, state)
			executionOrder = append(executionOrder, "after-"+step.Name())
			return err
		}
	}

	// Create mock step
	mockStep := testutil.NewMockStep("test-step")
	mockStep.ExecuteFunc = func(ctx context.Context, state *workflow.WorkflowState) error {
		executionOrder = append(executionOrder, "execute-test-step")
		return nil
	}

	// Create a proper StepHandler that wraps the mockStep.Execute
	baseHandler := func(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	handler := testMiddleware(baseHandler)
	state := testutil.NewFixtureBuilder().BuildWorkflowState()

	// Act
	err := handler(context.Background(), mockStep, state)

	// Assert
	testutil.AssertNoError(t, err, "middleware execution")
	assert.Equal(t, []string{
		"before-test-step",
		"execute-test-step",
		"after-test-step",
	}, executionOrder)
}

// BenchmarkOrchestrator_Execute demonstrates performance testing
func BenchmarkOrchestrator_Execute(b *testing.B) {
	// Setup
	logger := testutil.NewDiscardLogger()
	stepProvider := testutil.NewMockStepProvider()
	factory := workflow.NewStepFactory(stepProvider, nil, nil, logger)
	orchestrator := workflow.NewBaseOrchestrator(factory, nil, logger)

	ctx := context.Background()
	args := testutil.NewFixtureBuilder().BuildArgs()

	// Reset timer to exclude setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = orchestrator.Execute(ctx, nil, args)
	}
}
