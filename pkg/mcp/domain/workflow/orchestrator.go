// Package workflow provides sequential workflow orchestration
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
)

// Orchestrator implements sequential workflow orchestration
type Orchestrator struct {
	steps                  []Step
	logger                 *slog.Logger
	stepProvider           StepProvider
	progressEmitterFactory func(context.Context, *mcp.CallToolRequest) api.ProgressEmitter
}

// NewOrchestrator creates a new workflow orchestrator
func NewOrchestrator(
	stepProvider StepProvider,
	logger *slog.Logger,
	progressEmitterFactory func(context.Context, *mcp.CallToolRequest) api.ProgressEmitter,
) (*Orchestrator, error) {
	// Build the sequential containerization workflow
	steps, err := buildContainerizationSteps(stepProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to build workflow steps: %w", err)
	}

	return &Orchestrator{
		steps:                  steps,
		logger:                 logger,
		stepProvider:           stepProvider,
		progressEmitterFactory: progressEmitterFactory,
	}, nil
}

// GetStepProvider returns the step provider for accessing individual steps
func (o *Orchestrator) GetStepProvider() StepProvider {
	return o.stepProvider
}

// Execute runs the containerization workflow sequentially
func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Initialize context with workflow ID
	workflowID, ctx := o.initContext(ctx, args)

	// Create progress emitter
	emitter := o.progressEmitterFactory(ctx, req)
	defer emitter.Close()

	// Create workflow state
	state := o.newState(workflowID, args, emitter)

	// Log workflow start
	o.logger.Info("Starting sequential containerization workflow",
		"workflow_id", workflowID,
		"steps_count", len(o.steps),
		"repo_url", args.RepoURL,
		"repo_path", args.RepoPath,
		"branch", args.Branch,
	)

	// Execute steps sequentially
	startTime := time.Now()
	err := o.executeSequentially(ctx, state)
	duration := time.Since(startTime)

	// Log completion
	o.logger.Info("Sequential workflow completed",
		"workflow_id", workflowID,
		"success", err == nil,
		"duration", duration,
		"steps_executed", state.CurrentStep,
	)

	if err != nil {
		state.Result.Success = false
		state.Result.Error = fmt.Sprintf("Workflow failed: %v", err)
		return state.Result, err
	}

	// Mark as successful
	state.Result.Success = true

	return state.Result, nil
}

// initContext generates workflow ID and adds it to context
func (o *Orchestrator) initContext(ctx context.Context, args *ContainerizeAndDeployArgs) (string, context.Context) {
	workflowID, ok := GetWorkflowID(ctx)
	if !ok {
		repoIdentifier := GetRepositoryIdentifier(args)
		workflowID = GenerateWorkflowID(repoIdentifier)
		ctx = WithWorkflowID(ctx, workflowID)
	}
	return workflowID, ctx
}

// newState creates the workflow state with all necessary components
func (o *Orchestrator) newState(workflowID string, args *ContainerizeAndDeployArgs, emitter api.ProgressEmitter) *WorkflowState {
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
		WorkflowProgress: NewWorkflowProgress(workflowID, "containerization", len(o.steps)),
	}

	return state
}

// executeSequentially runs all workflow steps in sequence
func (o *Orchestrator) executeSequentially(ctx context.Context, state *WorkflowState) error {
	// Cache all steps in state for optimization analysis
	state.SetAllSteps(o.steps)

	for _, step := range o.steps {
		// Update current step number for progress tracking
		state.CurrentStep++

		// Log step start
		o.logger.Info("Starting workflow step",
			"step", step.Name(),
			"step_number", state.CurrentStep,
			"total_steps", state.TotalSteps,
		)

		// Execute step with retry logic
		err := o.executeStepWithRetry(ctx, step, state)

		// Emit progress update
		if state.ProgressEmitter != nil {
			percentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)
			message := fmt.Sprintf("Step %s completed", step.Name())

			if err != nil {
				message = fmt.Sprintf("Step %s failed: %v", step.Name(), err)
			}

			if emitErr := state.ProgressEmitter.Emit(ctx, step.Name(), percentage, message); emitErr != nil {
				o.logger.Warn("Failed to emit progress",
					"step", step.Name(),
					"error", emitErr)
			}
		}

		// Log step completion
		o.logger.Info("Workflow step completed",
			"step", step.Name(),
			"success", err == nil,
			"step_number", state.CurrentStep,
		)

		if err != nil {
			return fmt.Errorf("step %s failed: %w", step.Name(), err)
		}
	}

	return nil
}

// executeStepWithRetry executes a step with retry logic
func (o *Orchestrator) executeStepWithRetry(ctx context.Context, step Step, state *WorkflowState) error {
	maxRetries := step.MaxRetries()
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create timeout context for this attempt
		timeoutCtx, cancel := context.WithTimeout(ctx, o.getStepTimeout(step.Name()))
		defer cancel()

		// Execute the step
		err := step.Execute(timeoutCtx, state)
		if err == nil {
			return nil
		}

		lastErr = err
		o.logger.Warn("Step failed, will retry",
			"step", step.Name(),
			"attempt", attempt,
			"max_attempts", maxRetries,
			"error", err,
		)

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			// Exponential backoff with jitter
			backoff := time.Duration(attempt) * 2 * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			time.Sleep(backoff)
		}
	}

	return NewWorkflowError(step.Name(), maxRetries, lastErr)
}

// getStepTimeout returns the timeout for a specific step
func (o *Orchestrator) getStepTimeout(stepName string) time.Duration {
	timeouts := map[string]time.Duration{
		StepAnalyzeRepository:  5 * time.Minute,
		StepGenerateDockerfile: 2 * time.Minute,
		StepBuildImage:         15 * time.Minute,
		StepSecurityScan:       10 * time.Minute,
		StepTagImage:           1 * time.Minute,
		StepPushImage:          10 * time.Minute,
		StepGenerateManifests:  2 * time.Minute,
		StepSetupCluster:       5 * time.Minute,
		StepDeployApplication:  10 * time.Minute,
		StepVerifyDeployment:   5 * time.Minute,
	}

	if timeout, ok := timeouts[stepName]; ok {
		return timeout
	}
	return 5 * time.Minute // default timeout
}

// buildContainerizationSteps creates the sequential list of workflow steps
func buildContainerizationSteps(provider StepProvider) ([]Step, error) {
	// Define the sequential order of steps
	stepKeys := []string{
		StepAnalyzeRepository,
		StepGenerateDockerfile,
		StepBuildImage,
		StepSecurityScan,
		StepTagImage,
		StepPushImage,
		StepGenerateManifests,
		StepSetupCluster,
		StepDeployApplication,
		StepVerifyDeployment,
	}

	// Build the step list
	steps := make([]Step, 0, len(stepKeys))
	for _, stepKey := range stepKeys {
		step, err := provider.GetStep(stepKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get step %s: %w", stepKey, err)
		}
		steps = append(steps, step)
	}

	return steps, nil
}
