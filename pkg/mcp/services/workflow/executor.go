package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// WorkflowExecutorImpl implements WorkflowExecutor interface
type WorkflowExecutorImpl struct {
	sessionState services.SessionState
	toolRegistry services.ToolRegistry
	workflows    map[string]*api.WorkflowStatus
	workflowsMux sync.RWMutex
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(sessionState services.SessionState, toolRegistry services.ToolRegistry) *WorkflowExecutorImpl {
	return &WorkflowExecutorImpl{
		sessionState: sessionState,
		toolRegistry: toolRegistry,
		workflows:    make(map[string]*api.WorkflowStatus),
	}
}

// Execute implements WorkflowExecutor.Execute
func (w *WorkflowExecutorImpl) Execute(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error) {
	if err := w.Validate(workflow); err != nil {
		return nil, errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow validation failed").
			Context("workflow_id", workflow.ID).
			Context("workflow_name", workflow.Name).
			Cause(err).Build()
	}

	workflowID := workflow.ID
	if workflowID == "" {
		workflowID = generateWorkflowID()
	}

	startTime := time.Now()

	status := &api.WorkflowStatus{
		WorkflowID:     workflowID,
		Status:         "running",
		CurrentStep:    "",
		StartTime:      startTime,
		LastUpdate:     startTime,
		CompletedSteps: 0,
		TotalSteps:     len(workflow.Steps),
	}

	w.updateWorkflowStatus(workflowID, status)

	stepResults := make([]api.StepResult, 0, len(workflow.Steps))
	successSteps := 0
	failedSteps := 0

	for i, step := range workflow.Steps {
		stepStartTime := time.Now()

		status.CurrentStep = step.ID
		status.LastUpdate = time.Now()
		w.updateWorkflowStatus(workflowID, status)

		stepResult, err := w.executeStep(ctx, step, workflow.Variables)
		stepEndTime := time.Now()

		result := api.StepResult{
			StepID:    step.ID,
			StepName:  step.Name,
			StartTime: stepStartTime,
			EndTime:   stepEndTime,
			Duration:  stepEndTime.Sub(stepStartTime),
			Retries:   0,
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			failedSteps++
		} else {
			result.Success = true
			result.Output = stepResult
			successSteps++
		}

		stepResults = append(stepResults, result)
		status.CompletedSteps = i + 1
		status.LastUpdate = time.Now()
		w.updateWorkflowStatus(workflowID, status)

		if err != nil {
			break
		}
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if failedSteps > 0 {
		status.Status = "failed"
	} else {
		status.Status = "completed"
	}
	status.LastUpdate = endTime
	w.updateWorkflowStatus(workflowID, status)

	return &api.WorkflowResult{
		WorkflowID:   workflowID,
		Success:      failedSteps == 0,
		StepResults:  stepResults,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration,
		TotalSteps:   len(workflow.Steps),
		SuccessSteps: successSteps,
		FailedSteps:  failedSteps,
	}, nil
}

// GetStatus implements WorkflowExecutor.GetStatus
func (w *WorkflowExecutorImpl) GetStatus(workflowID string) (*api.WorkflowStatus, error) {
	w.workflowsMux.RLock()
	defer w.workflowsMux.RUnlock()

	status, exists := w.workflows[workflowID]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("workflow not found").
			Context("workflow_id", workflowID).Build()
	}

	return status, nil
}

// Cancel implements WorkflowExecutor.Cancel
func (w *WorkflowExecutorImpl) Cancel(workflowID string) error {
	w.workflowsMux.Lock()
	defer w.workflowsMux.Unlock()

	status, exists := w.workflows[workflowID]
	if !exists {
		return errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Message("workflow not found for cancellation").
			Context("workflow_id", workflowID).Build()
	}

	if status.Status == "completed" || status.Status == "failed" {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Type(errors.ErrTypeValidation).
			Message("cannot cancel completed or failed workflow").
			Context("workflow_id", workflowID).
			Context("current_status", status.Status).Build()
	}

	status.Status = "cancelled"
	status.LastUpdate = time.Now()
	w.workflows[workflowID] = status

	return nil
}

// Validate implements WorkflowExecutor.Validate
func (w *WorkflowExecutorImpl) Validate(workflow *api.Workflow) error {
	if workflow == nil {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("workflow cannot be nil").Build()
	}

	if workflow.Name == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("workflow name is required").Build()
	}

	if len(workflow.Steps) == 0 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow must have at least one step").
			Context("workflow_name", workflow.Name).Build()
	}

	for i, step := range workflow.Steps {
		if step.Tool == "" {
			return errors.NewError().
				Code(errors.CodeMissingParameter).
				Type(errors.ErrTypeValidation).
				Message("step tool is required").
				Context("workflow_name", workflow.Name).
				Context("step_index", i).
				Context("step_name", step.Name).Build()
		}

		if _, err := w.toolRegistry.GetTool(step.Tool); err != nil {
			return errors.NewError().
				Code(errors.CodeToolNotFound).
				Type(errors.ErrTypeTool).
				Message("step references unknown tool").
				Context("workflow_name", workflow.Name).
				Context("step_index", i).
				Context("step_name", step.Name).
				Context("tool_name", step.Tool).
				Cause(err).Build()
		}
	}

	return nil
}

// executeStep executes a single workflow step
func (w *WorkflowExecutorImpl) executeStep(ctx context.Context, step api.WorkflowStep, variables map[string]interface{}) (map[string]interface{}, error) {
	tool, err := w.toolRegistry.GetTool(step.Tool)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Message("failed to get tool for step execution").
			Context("step_name", step.Name).
			Context("tool_name", step.Tool).
			Cause(err).Build()
	}

	toolInput := api.ToolInput{
		SessionID: "", // Will be set by the calling context
		Data:      step.Input,
		Context:   make(map[string]interface{}),
	}

	for key, value := range variables {
		toolInput.Context[key] = value
	}

	output, err := tool.Execute(ctx, toolInput)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeToolExecutionFailed).
			Type(errors.ErrTypeTool).
			Message("tool execution failed").
			Context("step_name", step.Name).
			Context("tool_name", step.Tool).
			Cause(err).Build()
	}

	if !output.Success {
		return nil, errors.NewError().
			Code(errors.CodeToolExecutionFailed).
			Type(errors.ErrTypeTool).
			Message("tool execution returned failure").
			Context("step_name", step.Name).
			Context("tool_name", step.Tool).
			Context("tool_error", output.Error).Build()
	}

	return output.Data, nil
}

// updateWorkflowStatus updates the workflow status in the internal map
func (w *WorkflowExecutorImpl) updateWorkflowStatus(workflowID string, status *api.WorkflowStatus) {
	w.workflowsMux.Lock()
	defer w.workflowsMux.Unlock()
	w.workflows[workflowID] = status
}

// generateWorkflowID generates a unique workflow identifier
func generateWorkflowID() string {
	return fmt.Sprintf("workflow_%d", time.Now().UnixNano())
}

// Close closes the workflow executor
func (w *WorkflowExecutorImpl) Close() error {
	w.workflowsMux.Lock()
	defer w.workflowsMux.Unlock()

	w.workflows = make(map[string]*api.WorkflowStatus)

	return nil
}
