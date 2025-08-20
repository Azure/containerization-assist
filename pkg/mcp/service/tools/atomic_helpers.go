package tools

import (
	"context"

	"github.com/pkg/errors"

	domainworkflow "github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/mcp/service/session"
)

// AtomicUpdateWorkflowState performs an atomic update on workflow state
// The updateFunc receives a SimpleWorkflowState and can modify it safely
func AtomicUpdateWorkflowState(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, updateFunc func(*SimpleWorkflowState) error) error {
	// First ensure session exists
	_, err := sessionManager.GetOrCreate(ctx, sessionID)
	if err != nil {
		return errors.Wrap(err, "failed to get or create session")
	}

	// Use concurrent-safe update if available
	if concurrentAdapter, ok := sessionManager.(*session.ConcurrentBoltAdapter); ok {
		return concurrentAdapter.UpdateWorkflowState(ctx, sessionID, func(metadata map[string]interface{}) error {
			// Extract or create workflow state
			state := extractWorkflowState(sessionID, metadata)

			// Apply the update function
			if err := updateFunc(state); err != nil {
				return err
			}

			// Save the updated state back to metadata
			metadata["workflow_state"] = serializeWorkflowState(state)
			return nil
		})
	}

	// Fallback for non-concurrent adapters (still not fully atomic but best effort)
	return sessionManager.Update(ctx, sessionID, func(sessionState *session.SessionState) error {
		if sessionState.Metadata == nil {
			sessionState.Metadata = make(map[string]interface{})
		}

		// Extract or create workflow state
		state := extractWorkflowState(sessionID, sessionState.Metadata)

		// Apply the update function
		if err := updateFunc(state); err != nil {
			return err
		}

		// Save the updated state back
		sessionState.Metadata["workflow_state"] = serializeWorkflowState(state)
		return nil
	})
}

// AtomicMarkStepCompleted atomically marks a step as completed
func AtomicMarkStepCompleted(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, stepName string) error {
	return AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, func(state *SimpleWorkflowState) error {
		state.MarkStepCompleted(stepName)
		state.CurrentStep = stepName
		state.Status = "running"
		return nil
	})
}

// AtomicMarkStepFailed atomically marks a step as failed
func AtomicMarkStepFailed(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, stepName string, err error) error {
	return AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, func(state *SimpleWorkflowState) error {
		state.MarkStepFailed(stepName)
		state.CurrentStep = stepName
		state.Status = "error"
		if err != nil {
			state.SetError(&domainworkflow.WorkflowError{
				Step: stepName,
				Err:  err,
			})
		}
		return nil
	})
}

// AtomicUpdateArtifacts atomically updates workflow artifacts
func AtomicUpdateArtifacts(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, artifacts map[string]interface{}) error {
	return AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, func(state *SimpleWorkflowState) error {
		state.UpdateArtifacts(artifacts)
		return nil
	})
}

// AtomicUpdateMetadata atomically updates workflow metadata
func AtomicUpdateMetadata(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, updateFunc func(map[string]interface{}) error) error {
	return AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, func(state *SimpleWorkflowState) error {
		if state.Metadata == nil {
			state.Metadata = make(map[string]interface{})
		}
		return updateFunc(state.Metadata)
	})
}

// extractWorkflowState extracts workflow state from metadata or creates a new one
func extractWorkflowState(sessionID string, metadata map[string]interface{}) *SimpleWorkflowState {
	workflowData, exists := metadata["workflow_state"]
	if !exists {
		// Create new workflow state if not found
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			SkipSteps:      []string{},
			Artifacts:      make(map[string]interface{}),
			Metadata:       make(map[string]interface{}),
		}
	}

	// Convert interface{} to WorkflowState
	workflowMap, ok := workflowData.(map[string]interface{})
	if !ok {
		// Invalid format, return new state
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			SkipSteps:      []string{},
			Artifacts:      make(map[string]interface{}),
			Metadata:       make(map[string]interface{}),
		}
	}

	// Parse workflow state fields
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

	// Parse arrays with type conversion
	state.CompletedSteps = extractStringArray(workflowMap["completed_steps"])
	state.FailedSteps = extractStringArray(workflowMap["failed_steps"])
	state.SkipSteps = extractStringArray(workflowMap["skip_steps"])

	// Parse maps
	if artifacts, ok := workflowMap["artifacts"].(map[string]interface{}); ok {
		state.Artifacts = artifacts
	} else {
		state.Artifacts = make(map[string]interface{})
	}
	if metadata, ok := workflowMap["metadata"].(map[string]interface{}); ok {
		state.Metadata = metadata
	} else {
		state.Metadata = make(map[string]interface{})
	}

	// Parse error if present
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

// extractStringArray safely extracts a string array from an interface{}
func extractStringArray(data interface{}) []string {
	if data == nil {
		return []string{}
	}

	// Handle []interface{} (from JSON unmarshaling)
	if arr, ok := data.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	// Handle []string (direct type)
	if arr, ok := data.([]string); ok {
		return arr
	}

	return []string{}
}

// serializeWorkflowState converts a SimpleWorkflowState to a map for storage
func serializeWorkflowState(state *SimpleWorkflowState) map[string]interface{} {
	workflowData := map[string]interface{}{
		"repo_path":       state.RepoPath,
		"status":          state.Status,
		"current_step":    state.CurrentStep,
		"completed_steps": state.CompletedSteps,
		"failed_steps":    state.FailedSteps,
		"skip_steps":      state.SkipSteps,
		"artifacts":       state.Artifacts,
		"metadata":        state.Metadata,
	}

	// Add error information if present
	if state.Error != nil {
		workflowData["error"] = map[string]interface{}{
			"step":    state.Error.Step,
			"attempt": state.Error.Attempt,
			"message": state.Error.Error(),
		}
	}

	return workflowData
}

// AtomicIncrementCounter atomically increments a counter in workflow metadata
// This is useful for testing and debugging concurrent operations
func AtomicIncrementCounter(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, counterName string) (int, error) {
	var newValue int
	err := AtomicUpdateMetadata(ctx, sessionManager, sessionID, func(metadata map[string]interface{}) error {
		var counter int
		if val, ok := metadata[counterName].(float64); ok {
			counter = int(val)
		} else if val, ok := metadata[counterName].(int); ok {
			counter = val
		}
		counter++
		metadata[counterName] = counter
		newValue = counter
		return nil
	})
	return newValue, err
}

// AtomicAppendToList atomically appends an item to a list in workflow metadata
func AtomicAppendToList(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string, listName string, item string) error {
	return AtomicUpdateMetadata(ctx, sessionManager, sessionID, func(metadata map[string]interface{}) error {
		list := extractStringArray(metadata[listName])
		list = append(list, item)
		metadata[listName] = list
		return nil
	})
}
