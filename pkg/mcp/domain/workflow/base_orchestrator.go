// Package workflow provides the base orchestrator implementation
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
)

// BaseOrchestrator provides the core workflow orchestration functionality
type BaseOrchestrator struct {
	steps          []Step
	middlewares    []StepMiddleware
	emitterFactory ProgressEmitterFactory
	logger         *slog.Logger
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
	emitterFactory ProgressEmitterFactory,
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
		steps:          steps,
		middlewares:    []StepMiddleware{}, // Initialize empty slice
		emitterFactory: emitterFactory,
		logger:         logger.With("component", "base_orchestrator"),
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
	emitter := o.newEmitter(ctx, req)
	defer emitter.Close()

	// Stage 2: State and executor setup
	state := o.newState(workflowID, args, emitter)
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
		// Generate workflow ID based on available input (cached for efficiency)
		repoIdentifier := GetRepositoryIdentifier(args)
		workflowID = GenerateWorkflowID(repoIdentifier)
		ctx = WithWorkflowID(ctx, workflowID)
	}

	if args.RepoURL != "" {
		o.logger.Info("Starting containerize_and_deploy workflow",
			"workflow_id", workflowID, "repo_url", args.RepoURL, "branch", args.Branch, "steps_count", len(o.steps))
	} else {
		o.logger.Info("Starting containerize_and_deploy workflow",
			"workflow_id", workflowID, "repo_path", args.RepoPath, "steps_count", len(o.steps))
	}

	return workflowID, ctx
}

// newEmitter creates a progress emitter for the workflow
func (o *BaseOrchestrator) newEmitter(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
	if o.emitterFactory != nil {
		return o.emitterFactory.CreateEmitter(ctx, req, len(o.steps))
	}
	// Fallback to no-op emitter
	return &NoOpEmitter{}
}

// newState creates the workflow state with all necessary components
func (o *BaseOrchestrator) newState(workflowID string, args *ContainerizeAndDeployArgs, emitter api.ProgressEmitter) *WorkflowState {
	repoIdentifier := GetRepositoryIdentifier(args)

	state := &WorkflowState{
		WorkflowID:       workflowID,
		Args:             args,
		RepoIdentifier:   repoIdentifier,
		Result:           &ContainerizeAndDeployResult{},
		Logger:           o.logger,
		ProgressEmitter:  emitter,
		TotalSteps:       len(o.steps),
		CurrentStep:      0,
		WorkflowProgress: NewWorkflowProgress(workflowID, "containerize_and_deploy", len(o.steps)),
	}

	// Set all steps for AI enhancement middleware
	state.SetAllSteps(o.steps)

	return state
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
		percentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)
		o.logger.Info("Executing workflow step", "step", step.Name(), "step_number", state.CurrentStep, "total_steps", state.TotalSteps)

		// Emit progress update
		_ = state.ProgressEmitter.Emit(ctx, step.Name(), percentage, fmt.Sprintf("Executing %s", step.Name()))

		if err := executor(ctx, step, state); err != nil {
			o.logger.Error("Workflow step failed", "step", step.Name(), "step_number", state.CurrentStep, "error", err)
			state.Result.Success = false
			state.Result.Error = err.Error()

			// Emit error progress
			_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
				Step:       state.CurrentStep,
				Total:      state.TotalSteps,
				Stage:      step.Name(),
				Message:    fmt.Sprintf("%s failed: %v", step.Name(), err),
				Percentage: percentage,
				Status:     "failed",
				Metadata:   map[string]interface{}{"error": err.Error()},
			})
			return err
		}
		o.logger.Info("Workflow step completed", "step", step.Name(), "step_number", state.CurrentStep)
	}

	state.Result.Success = true
	// Final completion via Close() which is called in defer
	return nil
}
