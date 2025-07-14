// Package workflow provides saga transaction middleware for workflow orchestration
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow/common"
	"github.com/mark3labs/mcp-go/mcp"
)

// CompensatableStep represents a workflow step that can be compensated
type CompensatableStep interface {
	Step
	// Compensate reverses the effects of this step
	Compensate(ctx context.Context, state *WorkflowState) error
	// CanCompensate indicates if this step supports compensation
	CanCompensate() bool
}

// SagaMiddleware creates a middleware that provides saga transaction support
func SagaMiddleware(coordinator *saga.SagaCoordinator, logger *slog.Logger) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			// Check if we're in a saga context
			sagaID, hasSaga := ctx.Value("saga_id").(string)
			if !hasSaga {
				// No saga context, just execute normally
				return next(ctx, step, state)
			}

			// Check if step is compensatable
			compensatable, isCompensatable := step.(CompensatableStep)
			if !isCompensatable || !compensatable.CanCompensate() {
				// Step doesn't support compensation, execute normally
				logger.Debug("Step does not support compensation",
					"step", step.Name(),
					"saga_id", sagaID)
				return next(ctx, step, state)
			}

			// Create saga step wrapper
			sagaStep := &workflowSagaStepAdapter{
				step:     compensatable,
				state:    state,
				stepName: step.Name(),
				logger:   logger,
				executeFunc: func(ctx context.Context, data map[string]interface{}) error {
					// Store state in saga data for compensation
					data["workflow_state"] = state
					data["step_name"] = step.Name()

					// Execute the actual step through the chain
					return next(ctx, step, state)
				},
				compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
					// Retrieve state from saga data
					if savedState, ok := data["workflow_state"].(*WorkflowState); ok {
						return compensatable.Compensate(ctx, savedState)
					}
					return fmt.Errorf("missing workflow state for compensation")
				},
			}

			// Store step data for potential compensation
			if sagaExec, ok := ctx.Value("saga_execution").(*saga.SagaExecution); ok {
				// Store step information in saga data for later compensation
				sagaExec.Data[fmt.Sprintf("step_%s", step.Name())] = map[string]interface{}{
					"compensatable": true,
					"state":         state,
				}
				logger.Info("Marked step as compensatable in saga",
					"step", step.Name(),
					"saga_id", sagaID)
			}

			// Execute through saga step (which calls our executeFunc)
			return sagaStep.Execute(ctx, make(map[string]interface{}))
		}
	}
}

// WorkflowSagaMiddleware creates a middleware that wraps entire workflow execution in a saga
func WorkflowSagaMiddleware(coordinator *saga.SagaCoordinator, logger *slog.Logger) func(WorkflowHandler) WorkflowHandler {
	return func(next WorkflowHandler) WorkflowHandler {
		return func(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
			// Generate saga ID
			workflowID, _ := ctx.Value("workflow_id").(string)
			if workflowID == "" {
				workflowID = common.GenerateWorkflowID(args.RepoURL)
			}
			sagaID := fmt.Sprintf("saga-%s-%d", workflowID, time.Now().Unix())

			// Create empty saga steps (actual steps will be registered during execution)
			sagaSteps := []saga.SagaStep{}

			// Start saga execution
			sagaExec, err := coordinator.StartSaga(ctx, sagaID, workflowID, sagaSteps)
			if err != nil {
				logger.Error("Failed to start saga",
					"saga_id", sagaID,
					"workflow_id", workflowID,
					"error", err)
				// Fall back to normal execution
				return next(ctx, req, args)
			}

			// Add saga context
			ctx = context.WithValue(ctx, "saga_id", sagaID)
			ctx = context.WithValue(ctx, "saga_execution", sagaExec)

			logger.Info("Started saga-enabled workflow",
				"saga_id", sagaID,
				"workflow_id", workflowID)

			// Execute workflow
			result, execErr := next(ctx, req, args)

			// Handle saga completion
			if execErr != nil || (result != nil && !result.Success) {
				// Workflow failed, trigger compensation
				logger.Info("Workflow failed, triggering saga compensation",
					"saga_id", sagaID,
					"error", execErr)

				if cancelErr := coordinator.CancelSaga(ctx, sagaID); cancelErr != nil {
					logger.Error("Failed to cancel saga",
						"saga_id", sagaID,
						"error", cancelErr)
				}
			} else {
				logger.Info("Workflow succeeded, saga completed",
					"saga_id", sagaID)
			}

			return result, execErr
		}
	}
}

// workflowSagaStepAdapter adapts a CompensatableStep to saga.SagaStep interface
type workflowSagaStepAdapter struct {
	step           CompensatableStep
	state          *WorkflowState
	stepName       string
	logger         *slog.Logger
	executeFunc    func(ctx context.Context, data map[string]interface{}) error
	compensateFunc func(ctx context.Context, data map[string]interface{}) error
}

func (s *workflowSagaStepAdapter) Name() string { return s.stepName }

func (s *workflowSagaStepAdapter) Execute(ctx context.Context, data map[string]interface{}) error {
	return s.executeFunc(ctx, data)
}

func (s *workflowSagaStepAdapter) Compensate(ctx context.Context, data map[string]interface{}) error {
	s.logger.Info("Compensating step", "step", s.stepName)
	return s.compensateFunc(ctx, data)
}

func (s *workflowSagaStepAdapter) CanCompensate() bool { return true }
