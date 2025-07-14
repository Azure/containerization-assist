// Package workflow provides the base orchestrator implementation
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow/common"
	"github.com/mark3labs/mcp-go/mcp"
)

// BaseOrchestrator provides the core workflow orchestration functionality
// It executes steps in sequence with configurable middleware
type BaseOrchestrator struct {
	steps           []Step
	middlewares     []StepMiddleware
	progressFactory ProgressTrackerFactory
	logger          *slog.Logger
}

// OrchestratorOption configures a BaseOrchestrator during construction
type OrchestratorOption func(*BaseOrchestrator)

// WithMiddleware registers one or more step middlewares
func WithMiddleware(middlewares ...StepMiddleware) OrchestratorOption {
	return func(o *BaseOrchestrator) {
		o.middlewares = append(o.middlewares, middlewares...)
	}
}

// NewBaseOrchestrator creates a new base orchestrator with the supplied options
func NewBaseOrchestrator(
	factory *StepFactory,
	progressFactory ProgressTrackerFactory,
	logger *slog.Logger,
	opts ...OrchestratorOption,
) *BaseOrchestrator {
	// Get all steps and filter out nil values
	allSteps := factory.CreateAllSteps()
	steps := make([]Step, 0, len(allSteps))
	for _, step := range allSteps {
		if step != nil {
			steps = append(steps, step)
		}
	}

	orchestrator := &BaseOrchestrator{
		steps:           steps,
		middlewares:     []StepMiddleware{}, // Initialize empty slice
		progressFactory: progressFactory,
		logger:          logger.With("component", "base_orchestrator"),
	}

	// Apply all options
	for _, opt := range opts {
		opt(orchestrator)
	}

	return orchestrator
}

// Execute runs the containerization workflow
func (o *BaseOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	startTime := time.Now()

	// Generate workflow ID if not in context
	workflowID, ok := GetWorkflowID(ctx)
	if !ok {
		workflowID = common.GenerateWorkflowID(args.RepoURL)
		ctx = WithWorkflowID(ctx, workflowID)
	}

	o.logger.Info("Starting containerize_and_deploy workflow",
		"workflow_id", workflowID,
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"steps_count", len(o.steps))

	// Create progress tracker
	var progressTracker *progress.Tracker
	if o.progressFactory != nil {
		progressTracker = o.progressFactory.CreateTracker(ctx, req, len(o.steps))
	} else {
		// Fallback: create a minimal tracker if no factory provided
		sink := &common.NoOpSink{}
		progressTracker = progress.NewTracker(ctx, len(o.steps), sink)
		progressTracker.Begin("Starting workflow")
	}
	defer progressTracker.Finish()

	// Create workflow state
	state := &WorkflowState{
		WorkflowID:       workflowID,
		Args:             args,
		Result:           &ContainerizeAndDeployResult{},
		Logger:           o.logger,
		ProgressTracker:  progressTracker,
		TotalSteps:       len(o.steps),
		CurrentStep:      0,
		WorkflowProgress: NewWorkflowProgress(workflowID, "containerize_and_deploy", len(o.steps)),
	}

	// Build middleware chain
	stepExecutor := o.buildStepExecutor()

	// Execute workflow steps
	for i, step := range o.steps {
		state.CurrentStep = i + 1

		o.logger.Info("Executing workflow step",
			"step", step.Name(),
			"step_number", state.CurrentStep,
			"total_steps", state.TotalSteps)

		// Update progress
		progressTracker.Update(
			state.CurrentStep,
			fmt.Sprintf("Executing %s", step.Name()),
			nil,
		)

		// Execute step through middleware chain
		if err := stepExecutor(ctx, step, state); err != nil {
			o.logger.Error("Workflow step failed",
				"step", step.Name(),
				"step_number", state.CurrentStep,
				"error", err)

			// Update result with error
			state.Result.Success = false
			state.Result.Error = err.Error()

			// Report failure
			progressTracker.Update(
				state.CurrentStep,
				fmt.Sprintf("%s failed: %v", step.Name(), err),
				map[string]interface{}{"error": err.Error()},
			)

			return state.Result, err
		}

		o.logger.Info("Workflow step completed",
			"step", step.Name(),
			"step_number", state.CurrentStep)
	}

	// Workflow completed successfully
	state.Result.Success = true

	duration := time.Since(startTime)
	o.logger.Info("Workflow completed successfully",
		"workflow_id", workflowID,
		"duration", duration,
		"steps_executed", len(o.steps))

	progressTracker.Complete("Workflow completed successfully")

	return state.Result, nil
}

// buildStepExecutor builds the middleware chain for step execution
func (o *BaseOrchestrator) buildStepExecutor() StepHandler {
	// Base handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware in reverse order (innermost first)
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		handler = o.middlewares[i](handler)
	}

	return handler
}
