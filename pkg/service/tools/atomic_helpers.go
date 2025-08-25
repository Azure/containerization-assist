package tools

import (
	"context"
	"errors"
	"fmt"

	domainworkflow "github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/service/session"
)

// AtomicUpdateWorkflowState performs an atomic update on workflow state
// The updateFunc receives a SimpleWorkflowState and can modify it safely
func AtomicUpdateWorkflowState(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, updateFunc func(*SimpleWorkflowState) error) error {
	_, err := sessionManager.GetOrCreate(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	if concurrentAdapter, ok := sessionManager.(*session.ConcurrentBoltAdapter); ok {
		return concurrentAdapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
			state := extractWorkflowState(sessionID, metadata)

			if err := updateFunc(state); err != nil {
				return err
			}

			metadata["workflow_state"] = serializeWorkflowState(state)
			return nil
		})
	}

	return sessionManager.Update(ctx, sessionID, func(sessionState *session.SessionState) error {
		if sessionState.Metadata == nil {
			sessionState.Metadata = make(map[string]interface{})
		}

		state := extractWorkflowState(sessionID, sessionState.Metadata)

		if err := updateFunc(state); err != nil {
			return err
		}

		sessionState.Metadata["workflow_state"] = serializeWorkflowState(state)
		return nil
	})
}

func extractWorkflowState(sessionID string, metadata map[string]interface{}) *SimpleWorkflowState {
	workflowData, exists := metadata["workflow_state"]
	if !exists {
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			SkipSteps:      []string{},
			Artifacts:      &WorkflowArtifacts{},
			Metadata:       &ToolMetadata{},
		}
	}

	workflowMap, ok := workflowData.(map[string]interface{})
	if !ok {
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			SkipSteps:      []string{},
			Artifacts:      &WorkflowArtifacts{},
			Metadata:       &ToolMetadata{},
		}
	}

	state := &SimpleWorkflowState{
		SessionID: sessionID,
	}

	if repoPath, ok := workflowMap["repo_path"].(string); ok {
		state.RepoPath = repoPath
	}
	if status, ok := workflowMap["status"].(string); ok {
		state.Status = status
	}
	if currentStep, ok := workflowMap["current_step"].(string); ok {
		state.CurrentStep = currentStep
	}

	state.CompletedSteps = extractStringArray(workflowMap["completed_steps"])
	state.FailedSteps = extractStringArray(workflowMap["failed_steps"])
	state.SkipSteps = extractStringArray(workflowMap["skip_steps"])

	if artifacts, ok := workflowMap["artifacts"].(map[string]interface{}); ok {
		state.Artifacts = deserializeArtifacts(artifacts)
	} else {
		state.Artifacts = &WorkflowArtifacts{}
	}
	if metadata, ok := workflowMap["metadata"].(map[string]interface{}); ok {
		state.Metadata = deserializeMetadata(metadata)
	} else {
		state.Metadata = &ToolMetadata{}
	}

	if errorData, ok := workflowMap["error"].(map[string]interface{}); ok {
		workflowErr := &domainworkflow.WorkflowError{}
		if step, ok := errorData["step"].(string); ok {
			workflowErr.Step = step
		}
		if attempt, ok := errorData["attempt"].(float64); ok {
			workflowErr.Attempt = int(attempt)
		} else if attempt, ok := errorData["attempt"].(int); ok {
			workflowErr.Attempt = attempt
		}
		if message, ok := errorData["message"].(string); ok {
			workflowErr.Err = errors.New(message)
		}
		state.Error = workflowErr
	}

	return state
}

func extractStringArray(data interface{}) []string {
	if data == nil {
		return []string{}
	}

	if arr, ok := data.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	if arr, ok := data.([]string); ok {
		return arr
	}

	return []string{}
}

func serializeWorkflowState(state *SimpleWorkflowState) map[string]interface{} {
	workflowData := map[string]interface{}{
		"repo_path":       state.RepoPath,
		"status":          state.Status,
		"current_step":    state.CurrentStep,
		"completed_steps": state.CompletedSteps,
		"failed_steps":    state.FailedSteps,
		"skip_steps":      state.SkipSteps,
		"artifacts":       serializeArtifacts(state.Artifacts),
		"metadata":        serializeMetadata(state.Metadata),
	}

	if state.Error != nil {
		workflowData["error"] = map[string]interface{}{
			"step":    state.Error.Step,
			"attempt": state.Error.Attempt,
			"message": state.Error.Error(),
		}
	}

	return workflowData
}
