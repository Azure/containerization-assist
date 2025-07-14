// Package workflow provides event publishing middleware for workflow orchestration
package workflow

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow/common"
	"github.com/mark3labs/mcp-go/mcp"
)

// EventMiddleware creates a middleware that publishes events for step execution
func EventMiddleware(publisher *events.Publisher, logger *slog.Logger) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepStartTime := time.Now()

			// Log step start (since we don't have a separate StepStartedEvent type)
			logger.Info("Step started",
				"step", step.Name(),
				"workflow_id", state.WorkflowID,
				"step_number", state.CurrentStep,
				"total_steps", state.TotalSteps)

			// Execute the step
			err := next(ctx, step, state)

			// Calculate duration
			duration := time.Since(stepStartTime)

			// Publish step completed event (handles both success and failure)
			completedEvent := common.CreateStepCompletedEvent(step.Name(), state.WorkflowID, state.CurrentStep, state.TotalSteps, duration, err)
			publisher.PublishAsync(ctx, completedEvent)

			// Log completion
			if err != nil {
				logger.Error("Step failed",
					"step", step.Name(),
					"workflow_id", state.WorkflowID,
					"duration", duration,
					"error", err)
			} else {
				logger.Info("Step completed",
					"step", step.Name(),
					"workflow_id", state.WorkflowID,
					"duration", duration)
			}

			return err
		}
	}
}

// WorkflowEventMiddleware creates a middleware that publishes workflow-level events
// This is used at the orchestrator level, not step level
func WorkflowEventMiddleware(publisher *events.Publisher, logger *slog.Logger) func(WorkflowHandler) WorkflowHandler {
	return func(next WorkflowHandler) WorkflowHandler {
		return func(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
			workflowStartTime := time.Now()
			workflowID := common.GenerateWorkflowID(args.RepoURL)

			// Store workflow ID in context for other middlewares
			ctx = context.WithValue(ctx, "workflow_id", workflowID)

			// Publish workflow started event
			startEvent := common.CreateWorkflowStartedEvent(workflowID, args.RepoURL, args.Branch, common.ExtractUserID(ctx))
			publisher.PublishAsync(ctx, startEvent)

			// Execute the workflow
			result, err := next(ctx, req, args)

			// Calculate duration
			duration := time.Since(workflowStartTime)

			// Determine success and error message
			success := err == nil && result != nil && result.Success
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			} else if result != nil && !result.Success && result.Error != "" {
				errorMsg = result.Error
			}

			// Extract result fields
			imageRef := ""
			namespace := ""
			endpoint := ""
			if result != nil {
				imageRef = result.ImageRef
				namespace = result.Namespace
				endpoint = result.Endpoint
			}

			// Publish workflow completed event
			completedEvent := common.CreateWorkflowCompletedEvent(workflowID, duration, success, imageRef, namespace, endpoint, errorMsg)
			publisher.PublishAsync(ctx, completedEvent)

			return result, err
		}
	}
}

// WorkflowHandler is a function type for workflow execution
type WorkflowHandler func(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error)
