// Package workflow provides DAG-based workflow orchestration
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
)

// DAGOrchestrator implements workflow orchestration using a DAG execution model
type DAGOrchestrator struct {
	dag            *DAGWorkflow
	emitterFactory ProgressEmitterFactory
	logger         *slog.Logger
	stepProvider   StepProvider
}

// NewDAGOrchestrator creates a new DAG-based orchestrator
func NewDAGOrchestrator(
	stepProvider StepProvider,
	emitterFactory ProgressEmitterFactory,
	logger *slog.Logger,
) (*DAGOrchestrator, error) {
	// Build the DAG with the standard containerization workflow
	dag, err := buildContainerizationDAG(stepProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to build workflow DAG: %w", err)
	}

	return &DAGOrchestrator{
		dag:            dag,
		emitterFactory: emitterFactory,
		logger:         logger,
		stepProvider:   stepProvider,
	}, nil
}

// Execute runs the containerization workflow using DAG execution
func (o *DAGOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Initialize context with workflow ID
	workflowID, ctx := o.initContext(ctx, args)

	// Create progress emitter
	emitter := o.newEmitter(ctx, req)
	defer emitter.Close()

	// Create workflow state
	state := o.newState(workflowID, args, emitter)

	// Log workflow start
	o.logger.Info("Starting DAG-based containerize_and_deploy workflow",
		"workflow_id", workflowID,
		"steps_count", len(o.dag.steps),
		"repo_url", args.RepoURL,
		"repo_path", args.RepoPath,
		"branch", args.Branch,
	)

	// Execute the DAG workflow
	startTime := time.Now()
	err := o.dag.Execute(ctx, state)
	duration := time.Since(startTime)

	// Log completion
	o.logger.Info("DAG workflow completed",
		"workflow_id", workflowID,
		"success", err == nil,
		"duration", duration,
		"steps_executed", len(o.dag.steps),
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
func (o *DAGOrchestrator) initContext(ctx context.Context, args *ContainerizeAndDeployArgs) (string, context.Context) {
	workflowID, ok := GetWorkflowID(ctx)
	if !ok {
		repoIdentifier := GetRepositoryIdentifier(args)
		workflowID = GenerateWorkflowID(repoIdentifier)
		ctx = WithWorkflowID(ctx, workflowID)
	}
	return workflowID, ctx
}

// newEmitter creates a progress emitter for the workflow
func (o *DAGOrchestrator) newEmitter(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
	if o.emitterFactory != nil {
		return o.emitterFactory.CreateEmitter(ctx, req, len(o.dag.steps))
	}
	// Fallback to no-op emitter
	return &NoOpEmitter{}
}

// newState creates the workflow state with all necessary components
func (o *DAGOrchestrator) newState(workflowID string, args *ContainerizeAndDeployArgs, emitter api.ProgressEmitter) *WorkflowState {
	repoIdentifier := GetRepositoryIdentifier(args)

	state := &WorkflowState{
		WorkflowID:       workflowID,
		Args:             args,
		RepoIdentifier:   repoIdentifier,
		Result:           &ContainerizeAndDeployResult{},
		Logger:           o.logger,
		ProgressEmitter:  emitter,
		TotalSteps:       len(o.dag.steps),
		CurrentStep:      0,
		WorkflowProgress: NewWorkflowProgress(workflowID, "containerize_and_deploy", len(o.dag.steps)),
	}

	return state
}

// buildContainerizationDAG creates the DAG for the containerization workflow
func buildContainerizationDAG(provider StepProvider) (*DAGWorkflow, error) {
	builder := NewDAGBuilder()

	// Add all workflow steps with standard retry policies
	defaultRetry := DAGRetryPolicy{
		MaxAttempts: 3,
		BackoffBase: 2 * time.Second,
		BackoffMax:  30 * time.Second,
	}

	// Add steps - wrapping existing Step implementations
	builder.
		AddStep(&DAGStep{
			Name:    "analyze",
			Execute: wrapStep(provider.GetAnalyzeStep()),
			Retry:   defaultRetry,
			Timeout: 5 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "dockerfile",
			Execute: wrapStep(provider.GetDockerfileStep()),
			Retry:   defaultRetry,
			Timeout: 2 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "build",
			Execute: wrapStep(provider.GetBuildStep()),
			Retry:   defaultRetry,
			Timeout: 15 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "scan",
			Execute: wrapStep(provider.GetScanStep()),
			Retry:   defaultRetry,
			Timeout: 10 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "tag",
			Execute: wrapStep(provider.GetTagStep()),
			Retry:   defaultRetry,
			Timeout: 1 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "push",
			Execute: wrapStep(provider.GetPushStep()),
			Retry:   defaultRetry,
			Timeout: 10 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "manifest",
			Execute: wrapStep(provider.GetManifestStep()),
			Retry:   defaultRetry,
			Timeout: 2 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "cluster",
			Execute: wrapStep(provider.GetClusterStep()),
			Retry:   defaultRetry,
			Timeout: 5 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "deploy",
			Execute: wrapStep(provider.GetDeployStep()),
			Retry:   defaultRetry,
			Timeout: 10 * time.Minute,
		}).
		AddStep(&DAGStep{
			Name:    "verify",
			Execute: wrapStep(provider.GetVerifyStep()),
			Retry:   defaultRetry,
			Timeout: 5 * time.Minute,
		})

	// Define sequential dependencies (as per current workflow)
	builder.
		AddDependency("analyze", "dockerfile").
		AddDependency("dockerfile", "build").
		AddDependency("build", "scan").
		AddDependency("scan", "tag").
		AddDependency("tag", "push").
		AddDependency("push", "manifest").
		AddDependency("manifest", "cluster").
		AddDependency("cluster", "deploy").
		AddDependency("deploy", "verify")

	return builder.Build()
}

// wrapStep wraps an existing Step interface into a DAG-compatible function
func wrapStep(step Step) func(context.Context, *WorkflowState) error {
	return func(ctx context.Context, state *WorkflowState) error {
		// Update current step number for progress tracking
		state.CurrentStep++

		// Update progress
		percentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)

		// Execute the step
		err := step.Execute(ctx, state)

		// Emit progress update
		if state.ProgressEmitter != nil {
			status := "completed"
			if err != nil {
				status = "failed"
			}

			// Emit progress update to notify listeners of step completion status
			message := fmt.Sprintf("Step %s %s", step.Name(), status)
			if err != nil {
				message = fmt.Sprintf("Step %s failed: %v", step.Name(), err)
			}

			if emitErr := state.ProgressEmitter.Emit(ctx, step.Name(), percentage, message); emitErr != nil {
				// Log but don't fail on progress emission errors
				state.Logger.Warn("Failed to emit progress",
					"step", step.Name(),
					"error", emitErr)
			}
		}

		return err
	}
}
