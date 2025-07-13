// Package workflow provides saga-aware orchestration for containerization workflows.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/mark3labs/mcp-go/mcp"
)

// SagaOrchestrator extends EventOrchestrator with saga transaction support
type SagaOrchestrator struct {
	*EventOrchestrator
	sagaCoordinator *saga.SagaCoordinator
}

// NewSagaOrchestrator creates a new saga-aware workflow orchestrator
func NewSagaOrchestrator(
	logger *slog.Logger,
	eventPublisher *events.Publisher,
	sagaCoordinator *saga.SagaCoordinator,
) *SagaOrchestrator {
	return &SagaOrchestrator{
		EventOrchestrator: NewEventOrchestrator(logger, eventPublisher),
		sagaCoordinator:   sagaCoordinator,
	}
}

// ExecuteWithSaga runs the complete workflow with saga transaction support
func (o *SagaOrchestrator) ExecuteWithSaga(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	workflowID := o.generateWorkflowID(args)
	sagaID := fmt.Sprintf("saga-%s", workflowID)
	startTime := time.Now()

	o.logger.Info("Starting saga-aware containerize_and_deploy workflow",
		"workflow_id", workflowID,
		"saga_id", sagaID,
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"scan", args.Scan)

	// Publish workflow started event (same as EventOrchestrator)
	startEvent := events.WorkflowStartedEvent{
		ID:        o.eventUtils.GenerateEventID(),
		Timestamp: startTime,
		Workflow:  workflowID,
		RepoURL:   args.RepoURL,
		Branch:    args.Branch,
		UserID:    o.extractUserID(ctx),
	}

	if err := o.eventPublisher.Publish(ctx, startEvent); err != nil {
		o.logger.Error("Failed to publish workflow started event", "error", err)
	}

	// Initialize workflow state
	state := NewWorkflowState(ctx, req, args, o.logger)
	state.WorkflowID = workflowID
	defer state.ProgressTracker.Finish()

	// Begin progress tracking
	state.ProgressTracker.Begin("Starting containerization and deployment workflow with saga support")

	// Create saga steps inline to avoid circular imports
	sagaStepsList := o.createWorkflowSagaSteps()

	// Start the saga transaction
	sagaExecution, err := o.sagaCoordinator.StartSaga(ctx, sagaID, workflowID, sagaStepsList)
	if err != nil {
		o.logger.Error("Failed to start saga transaction", "error", err)

		// Fallback to regular execution without saga
		o.logger.Info("Falling back to regular workflow execution")
		return o.EventOrchestrator.Execute(ctx, req, args)
	}

	// Prepare saga data with workflow state
	sagaData := map[string]interface{}{
		"workflow_state": state,
		"request":        req,
		"args":           args,
	}

	// Execute the saga (this will handle compensation automatically on failure)
	err = o.executeSagaWithProgress(ctx, sagaExecution, sagaData, state)

	// Build final result
	result := state.Result
	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Publish workflow failed event
		o.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), false, result, err.Error())
	} else {
		result.Success = true
		state.ProgressTracker.Complete("Containerization and deployment completed successfully with saga support")

		// Publish workflow completed event
		o.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), true, result, "")
	}

	o.logger.Info("Saga-aware workflow completed",
		"workflow_id", workflowID,
		"saga_id", sagaID,
		"success", result.Success,
		"saga_state", sagaExecution.GetState(),
		"duration", time.Since(startTime))

	return result, nil
}

// executeSagaWithProgress executes saga steps while updating progress
func (o *SagaOrchestrator) executeSagaWithProgress(ctx context.Context, sagaExecution *saga.SagaExecution, sagaData map[string]interface{}, state *WorkflowState) error {
	steps := sagaExecution.Steps
	totalSteps := len(steps)

	for i, step := range steps {
		stepNumber := i + 1
		stepStartTime := time.Now()

		o.logger.Info("Executing saga step",
			"step", stepNumber,
			"step_name", step.Name(),
			"total_steps", totalSteps)

		// Update progress tracker
		state.ProgressTracker.Update(stepNumber,
			fmt.Sprintf("Executing step %d/%d: %s", stepNumber, totalSteps, step.Name()),
			map[string]interface{}{"saga_step": step.Name()})

		// Execute the step through saga
		err := step.Execute(ctx, sagaData)
		duration := time.Since(stepStartTime)

		if err != nil {
			o.logger.Error("Saga step failed",
				"step_name", step.Name(),
				"error", err,
				"duration", duration)

			// Progress tracker error
			state.ProgressTracker.Error(stepNumber,
				fmt.Sprintf("Step %s failed", step.Name()), err)

			// Publish step failed event
			progress := float64(state.ProgressTracker.GetCurrent()) / float64(state.ProgressTracker.GetTotal()) * 100
			o.publishStepCompletedEvent(ctx, state.WorkflowID, &sagaStepWrapper{step}, stepNumber, totalSteps,
				duration, false, err.Error(), progress)

			// The saga coordinator will handle compensation
			return err
		}

		// Publish step completed event
		progress := float64(state.ProgressTracker.GetCurrent()) / float64(state.ProgressTracker.GetTotal()) * 100
		o.publishStepCompletedEvent(ctx, state.WorkflowID, &sagaStepWrapper{step}, stepNumber, totalSteps,
			duration, true, "", progress)

		o.logger.Info("Saga step completed successfully",
			"step_name", step.Name(),
			"duration", duration)
	}

	return nil
}

// sagaStepWrapper adapts saga.SagaStep to workflow.Step interface for event publishing
type sagaStepWrapper struct {
	sagaStep saga.SagaStep
}

func (w *sagaStepWrapper) Name() string {
	return w.sagaStep.Name()
}

func (w *sagaStepWrapper) Execute(ctx context.Context, state *WorkflowState) error {
	// This method won't be called, it's just for interface compliance
	return nil
}

func (w *sagaStepWrapper) MaxRetries() int {
	// Default retry count for saga steps
	return 3
}

// CancelWorkflow cancels a running workflow and its associated saga
func (o *SagaOrchestrator) CancelWorkflow(ctx context.Context, workflowID string) error {
	sagaID := fmt.Sprintf("saga-%s", workflowID)

	o.logger.Info("Cancelling workflow and associated saga",
		"workflow_id", workflowID,
		"saga_id", sagaID)

	// Cancel the saga transaction (this will trigger compensation)
	err := o.sagaCoordinator.CancelSaga(ctx, sagaID)
	if err != nil {
		o.logger.Error("Failed to cancel saga", "error", err, "saga_id", sagaID)
		return fmt.Errorf("failed to cancel workflow saga: %w", err)
	}

	o.logger.Info("Workflow and saga cancelled successfully",
		"workflow_id", workflowID,
		"saga_id", sagaID)

	return nil
}

// GetWorkflowSagaStatus returns the status of a workflow's saga transaction
func (o *SagaOrchestrator) GetWorkflowSagaStatus(workflowID string) (*saga.SagaExecution, error) {
	sagaID := fmt.Sprintf("saga-%s", workflowID)
	return o.sagaCoordinator.GetSaga(sagaID)
}

// ListWorkflowSagas returns all saga transactions for a workflow
func (o *SagaOrchestrator) ListWorkflowSagas(workflowID string) []*saga.SagaExecution {
	return o.sagaCoordinator.ListSagas(workflowID)
}

// createWorkflowSagaSteps creates saga steps for the containerization workflow
func (o *SagaOrchestrator) createWorkflowSagaSteps() []saga.SagaStep {
	return []saga.SagaStep{
		&workflowSagaStep{
			name:   "analyze",
			logger: o.logger.With("saga_step", "analyze"),
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				state := data["workflow_state"].(*WorkflowState)
				return o.executeRegularStep(ctx, state, 1, "analyze")
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				// Analysis is read-only, no compensation needed
				return nil
			},
			canCompensate: true,
		},
		&workflowSagaStep{
			name:   "dockerfile",
			logger: o.logger.With("saga_step", "dockerfile"),
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				state := data["workflow_state"].(*WorkflowState)
				err := o.executeRegularStep(ctx, state, 2, "dockerfile")
				if err == nil {
					data["dockerfile_created"] = true
				}
				return err
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				if created, exists := data["dockerfile_created"]; exists && created.(bool) {
					if err := os.Remove("Dockerfile"); err != nil && !os.IsNotExist(err) {
						return fmt.Errorf("failed to remove Dockerfile: %w", err)
					}
				}
				return nil
			},
			canCompensate: true,
		},
		&workflowSagaStep{
			name:   "build",
			logger: o.logger.With("saga_step", "build"),
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				state := data["workflow_state"].(*WorkflowState)
				err := o.executeRegularStep(ctx, state, 3, "build")
				if err == nil {
					data["image_built"] = state.Result.ImageRef
				}
				return err
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				if imageRef, exists := data["image_built"]; exists {
					if ref := imageRef.(string); ref != "" {
						cmd := exec.Command("docker", "rmi", "-f", ref)
						cmd.Run() // Ignore errors, image might not exist
					}
				}
				return nil
			},
			canCompensate: true,
		},
		&workflowSagaStep{
			name:   "deploy",
			logger: o.logger.With("saga_step", "deploy"),
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				state := data["workflow_state"].(*WorkflowState)
				err := o.executeRegularStep(ctx, state, 4, "deploy")
				if err == nil {
					data["deployment_created"] = map[string]string{
						"namespace": state.Result.Namespace,
						"name":      o.extractDeploymentName(state.Result.ImageRef),
					}
				}
				return err
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				if deployment, exists := data["deployment_created"]; exists {
					if depMap := deployment.(map[string]string); depMap != nil {
						ns := depMap["namespace"]
						name := depMap["name"]
						if ns != "" && name != "" {
							cmd := exec.Command("kubectl", "delete", "deployment", name, "-n", ns, "--ignore-not-found=true")
							cmd.Run() // Ignore errors
							cmd = exec.Command("kubectl", "delete", "service", name, "-n", ns, "--ignore-not-found=true")
							cmd.Run() // Ignore errors
						}
					}
				}
				return nil
			},
			canCompensate: true,
		},
	}
}

// executeRegularStep executes a regular workflow step by index
func (o *SagaOrchestrator) executeRegularStep(ctx context.Context, state *WorkflowState, stepIndex int, stepName string) error {
	if stepIndex <= len(o.steps) {
		step := o.steps[stepIndex-1]
		return step.Execute(ctx, state)
	}
	return fmt.Errorf("step index %d out of range", stepIndex)
}

// extractDeploymentName extracts deployment name from image reference
func (o *SagaOrchestrator) extractDeploymentName(imageRef string) string {
	if imageRef == "" {
		return "unknown-deployment"
	}

	parts := strings.Split(imageRef, "/")
	lastPart := parts[len(parts)-1]

	if idx := strings.LastIndex(lastPart, ":"); idx != -1 {
		lastPart = lastPart[:idx]
	}

	lastPart = strings.ToLower(lastPart)
	lastPart = strings.ReplaceAll(lastPart, "_", "-")

	return lastPart
}

// workflowSagaStep implements saga.SagaStep for workflow steps
type workflowSagaStep struct {
	name           string
	logger         *slog.Logger
	executeFunc    func(context.Context, map[string]interface{}) error
	compensateFunc func(context.Context, map[string]interface{}) error
	canCompensate  bool
}

func (s *workflowSagaStep) Name() string { return s.name }

func (s *workflowSagaStep) Execute(ctx context.Context, data map[string]interface{}) error {
	return s.executeFunc(ctx, data)
}

func (s *workflowSagaStep) Compensate(ctx context.Context, data map[string]interface{}) error {
	return s.compensateFunc(ctx, data)
}

func (s *workflowSagaStep) CanCompensate() bool { return s.canCompensate }
