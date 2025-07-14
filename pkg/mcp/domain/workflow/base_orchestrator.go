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

// Execute runs the containerization workflow using a 3-stage state machine
func (o *BaseOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Stage 1: Context initialization
	workflowID, ctx := o.initContext(ctx, args)
	tracker := o.newTracker(ctx, req)
	defer tracker.Finish()

	// Stage 2: State and executor setup
	state := o.newState(workflowID, args, tracker)
	executor := o.buildStepExecutor()

	// Stage 3: Step execution
	if err := o.runSteps(ctx, executor, state); err != nil {
		return state.Result, err
	}
	return state.Result, nil
}

// initContext generates workflow ID and adds it to context
func (o *BaseOrchestrator) initContext(ctx context.Context, args *ContainerizeAndDeployArgs) (string, context.Context) {
	workflowID, ok := GetWorkflowID(ctx)
	if !ok {
		workflowID = common.GenerateWorkflowID(args.RepoURL)
		ctx = WithWorkflowID(ctx, workflowID)
	}
	o.logger.Info("Starting containerize_and_deploy workflow",
		"workflow_id", workflowID, "repo_url", args.RepoURL, "branch", args.Branch, "steps_count", len(o.steps))
	return workflowID, ctx
}

// newTracker creates a progress tracker for the workflow
func (o *BaseOrchestrator) newTracker(ctx context.Context, req *mcp.CallToolRequest) *progress.Tracker {
	if o.progressFactory != nil {
		return o.progressFactory.CreateTracker(ctx, req, len(o.steps))
	}
	sink := &common.NoOpSink{}
	tracker := progress.NewTracker(ctx, len(o.steps), sink)
	tracker.Begin("Starting workflow")
	return tracker
}

// newState creates the workflow state with all necessary components
func (o *BaseOrchestrator) newState(workflowID string, args *ContainerizeAndDeployArgs, tracker *progress.Tracker) *WorkflowState {
	return &WorkflowState{
		WorkflowID:       workflowID,
		Args:             args,
		Result:           &ContainerizeAndDeployResult{},
		Logger:           o.logger,
		ProgressTracker:  tracker,
		TotalSteps:       len(o.steps),
		CurrentStep:      0,
		WorkflowProgress: NewWorkflowProgress(workflowID, "containerize_and_deploy", len(o.steps)),
	}
}

// buildStepExecutor builds the middleware chain for step execution
func (o *BaseOrchestrator) buildStepExecutor() StepHandler {
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		handler = o.middlewares[i](handler)
	}
	return handler
}

// runSteps executes all workflow steps with proper error handling
func (o *BaseOrchestrator) runSteps(ctx context.Context, executor StepHandler, state *WorkflowState) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		o.logger.Info("Workflow completed", "workflow_id", state.WorkflowID, "success", state.Result.Success,
			"duration", duration, "steps_executed", len(o.steps))
	}()

	for i, step := range o.steps {
		state.CurrentStep = i + 1
		o.logger.Info("Executing workflow step", "step", step.Name(), "step_number", state.CurrentStep, "total_steps", state.TotalSteps)
		state.ProgressTracker.Update(state.CurrentStep, fmt.Sprintf("Executing %s", step.Name()), nil)

		if err := executor(ctx, step, state); err != nil {
			o.logger.Error("Workflow step failed", "step", step.Name(), "step_number", state.CurrentStep, "error", err)
			state.Result.Success = false
			state.Result.Error = err.Error()
			state.ProgressTracker.Update(state.CurrentStep, fmt.Sprintf("%s failed: %v", step.Name(), err),
				map[string]interface{}{"error": err.Error()})
			return err
		}
		o.logger.Info("Workflow step completed", "step", step.Name(), "step_number", state.CurrentStep)
	}

	state.Result.Success = true
	state.ProgressTracker.Complete("Workflow completed successfully")
	return nil
}
