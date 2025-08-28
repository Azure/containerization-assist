package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/mark3labs/mcp-go/mcp"
)

type Orchestrator struct {
	steps                  []Step
	logger                 *slog.Logger
	stepProvider           StepProvider
	progressEmitterFactory func(context.Context, *mcp.CallToolRequest) api.ProgressEmitter
}

func NewOrchestrator(
	stepProvider StepProvider,
	logger *slog.Logger,
	progressEmitterFactory func(context.Context, *mcp.CallToolRequest) api.ProgressEmitter,
) (*Orchestrator, error) {
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

func (o *Orchestrator) GetStepProvider() StepProvider {
	return o.stepProvider
}

func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	workflowID, ctx := o.initContext(ctx, args)

	emitter := o.progressEmitterFactory(ctx, req)
	defer func() {
		if err := emitter.Close(); err != nil {
			o.logger.Error("Failed to close progress emitter", slog.String("error", err.Error()))
		}
	}()

	state := o.newState(workflowID, args, emitter)

	startTime := time.Now()
	err := o.executeSequentially(ctx, state)
	_ = time.Since(startTime)

	if err != nil {
		state.Result.Success = false
		state.Result.Error = fmt.Sprintf("Workflow failed: %v", err)
		return state.Result, err
	}

	state.Result.Success = true

	return state.Result, nil
}

func (o *Orchestrator) initContext(ctx context.Context, args *ContainerizeAndDeployArgs) (string, context.Context) {
	workflowID, ok := GetWorkflowID(ctx)
	if !ok {
		repoIdentifier := GetRepositoryIdentifier(args)
		workflowID = GenerateWorkflowID(repoIdentifier)
		ctx = WithWorkflowID(ctx, workflowID)
	}
	return workflowID, ctx
}

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

func (o *Orchestrator) executeSequentially(ctx context.Context, state *WorkflowState) error {
	state.SetAllSteps(o.steps)

	for _, step := range o.steps {
		state.CurrentStep++

		_, err := step.Execute(ctx, state)

		if state.ProgressEmitter != nil {
			percentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)
			message := fmt.Sprintf("Step %s completed", step.Name())

			if err != nil {
				message = fmt.Sprintf("Step %s failed: %v", step.Name(), err)
			}

			if emitErr := state.ProgressEmitter.Emit(ctx, step.Name(), percentage, message); emitErr != nil {
			}
		}

		if err != nil {
			return fmt.Errorf("step %s failed: %w", step.Name(), err)
		}
	}

	return nil
}

func buildContainerizationSteps(provider StepProvider) ([]Step, error) {
	stepKeys := []string{
		StepAnalyzeRepository,
		StepResolveBaseImages,
		StepVerifyDockerfile,
		StepBuildImage,
		StepSecurityScan,
		StepTagImage,
		StepPushImage,
		StepVerifyManifests,
		StepSetupCluster,
		StepDeployApplication,
		StepVerifyDeployment,
	}

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
