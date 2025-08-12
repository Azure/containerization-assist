package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"

	"github.com/Azure/container-kit/pkg/mcp/api"
	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
)

// WorkflowStateHelpers provides utility functions for workflow state management
type WorkflowStateHelpers struct {
	sessionManager session.SessionManager
}

// NewWorkflowStateHelpers creates a new instance of workflow state helpers
func NewWorkflowStateHelpers(sessionManager session.SessionManager) *WorkflowStateHelpers {
	return &WorkflowStateHelpers{
		sessionManager: sessionManager,
	}
}

// SimpleWorkflowState represents a simplified workflow state for tool operations
type SimpleWorkflowState struct {
	SessionID      string                        `json:"session_id"`
	RepoPath       string                        `json:"repo_path"`
	Status         string                        `json:"status"`
	CurrentStep    string                        `json:"current_step"`
	CompletedSteps []string                      `json:"completed_steps"`
	FailedSteps    []string                      `json:"failed_steps"`
	SkipSteps      []string                      `json:"skip_steps"`
	Artifacts      map[string]interface{}        `json:"artifacts"`
	Metadata       map[string]interface{}        `json:"metadata"`
	Error          *domainworkflow.WorkflowError `json:"error,omitempty"`
}

// MarkStepCompleted marks a step as completed
func (s *SimpleWorkflowState) MarkStepCompleted(stepName string) {
	for _, completed := range s.CompletedSteps {
		if completed == stepName {
			return // Already completed
		}
	}
	// Remove from failed steps if it was previously failed
	s.removeFromFailedSteps(stepName)
	s.CompletedSteps = append(s.CompletedSteps, stepName)
}

// MarkStepFailed marks a step as failed
func (s *SimpleWorkflowState) MarkStepFailed(stepName string) {
	for _, failed := range s.FailedSteps {
		if failed == stepName {
			return // Already marked as failed
		}
	}
	// Remove from completed steps if it was previously completed
	s.removeFromCompletedSteps(stepName)
	s.FailedSteps = append(s.FailedSteps, stepName)
}

// removeFromCompletedSteps removes a step from the completed steps list
func (s *SimpleWorkflowState) removeFromCompletedSteps(stepName string) {
	for i, completed := range s.CompletedSteps {
		if completed == stepName {
			s.CompletedSteps = append(s.CompletedSteps[:i], s.CompletedSteps[i+1:]...)
			return
		}
	}
}

// removeFromFailedSteps removes a step from the failed steps list
func (s *SimpleWorkflowState) removeFromFailedSteps(stepName string) {
	for i, failed := range s.FailedSteps {
		if failed == stepName {
			s.FailedSteps = append(s.FailedSteps[:i], s.FailedSteps[i+1:]...)
			return
		}
	}
}

// IsStepCompleted checks if a step is completed
func (s *SimpleWorkflowState) IsStepCompleted(stepName string) bool {
	for _, completed := range s.CompletedSteps {
		if completed == stepName {
			return true
		}
	}
	return false
}

// IsStepFailed checks if a step has failed
func (s *SimpleWorkflowState) IsStepFailed(stepName string) bool {
	for _, failed := range s.FailedSteps {
		if failed == stepName {
			return true
		}
	}
	return false
}

// GetStepStatus returns the status of a specific step
func (s *SimpleWorkflowState) GetStepStatus(stepName string) string {
	if s.IsStepCompleted(stepName) {
		return "completed"
	}
	if s.IsStepFailed(stepName) {
		return "failed"
	}
	return "not_started"
}

// SetError sets a workflow error
func (s *SimpleWorkflowState) SetError(err *domainworkflow.WorkflowError) {
	s.Error = err
	s.Status = "error"
}

// UpdateArtifacts updates the workflow artifacts
func (s *SimpleWorkflowState) UpdateArtifacts(result map[string]interface{}) {
	if s.Artifacts == nil {
		s.Artifacts = make(map[string]interface{})
	}
	for k, v := range result {
		s.Artifacts[k] = v
	}
}

// LoadWorkflowState loads workflow state from session
func LoadWorkflowState(ctx context.Context, sessionManager session.SessionManager, sessionID string) (*SimpleWorkflowState, error) {
	sessionData, err := sessionManager.Get(ctx, sessionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session")
	}

	if sessionData == nil {
		return nil, errors.New("session not found")
	}

	// Extract workflow state from session metadata
	workflowData, exists := sessionData.Metadata["workflow_state"]
	if !exists {
		// Create new workflow state if not found
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			Artifacts:      make(map[string]interface{}),
			Metadata:       make(map[string]interface{}),
		}, nil
	}

	// Convert interface{} to WorkflowState
	workflowMap, ok := workflowData.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid workflow state format in session")
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
	if completedSteps, ok := workflowMap["completed_steps"].([]interface{}); ok {
		steps := make([]string, len(completedSteps))
		for i, step := range completedSteps {
			if s, ok := step.(string); ok {
				steps[i] = s
			}
		}
		state.CompletedSteps = steps
	}
	if failedSteps, ok := workflowMap["failed_steps"].([]interface{}); ok {
		steps := make([]string, len(failedSteps))
		for i, step := range failedSteps {
			if s, ok := step.(string); ok {
				steps[i] = s
			}
		}
		state.FailedSteps = steps
	}
	if skipSteps, ok := workflowMap["skip_steps"].([]interface{}); ok {
		steps := make([]string, len(skipSteps))
		for i, step := range skipSteps {
			if s, ok := step.(string); ok {
				steps[i] = s
			}
		}
		state.SkipSteps = steps
	}
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

	return state, nil
}

// SaveWorkflowState saves workflow state to session
func SaveWorkflowState(ctx context.Context, sessionManager session.SessionManager, state *SimpleWorkflowState) error {
	// Create workflow state map for serialization
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

	// Save workflow state in session metadata
	err := sessionManager.Update(ctx, state.SessionID, func(sessionState *session.SessionState) error {
		if sessionState.Metadata == nil {
			sessionState.Metadata = make(map[string]interface{})
		}
		sessionState.Metadata["workflow_state"] = workflowData
		return nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to update session with workflow state")
	}

	return nil
}

// GenerateSessionID generates a new session ID
func GenerateSessionID() string {
	return fmt.Sprintf("wf_%s", uuid.New().String())
}

// convertWorkflowError converts domain workflow error to a simple map (no longer needed as separate function)
// This is kept for compatibility but workflow errors are now stored directly in metadata

// CreateProgressEmitter creates a progress emitter using the messaging package
func CreateProgressEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) api.ProgressEmitter {
	return messaging.CreateProgressEmitter(ctx, req, totalSteps, logger)
}

// ExtractStringParam safely extracts a string parameter from arguments
func ExtractStringParam(args map[string]interface{}, key string) (string, error) {
	value, exists := args[key]
	if !exists {
		return "", errors.Errorf("missing parameter: %s", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", errors.Errorf("parameter %s must be a string", key)
	}

	if str == "" {
		return "", errors.Errorf("parameter %s cannot be empty", key)
	}

	return str, nil
}

// ExtractOptionalStringParam safely extracts an optional string parameter
func ExtractOptionalStringParam(args map[string]interface{}, key string, defaultValue string) string {
	value, exists := args[key]
	if !exists {
		return defaultValue
	}

	str, ok := value.(string)
	if !ok || str == "" {
		return defaultValue
	}

	return str
}

// ExtractStringArrayParam safely extracts a string array parameter
func ExtractStringArrayParam(args map[string]interface{}, key string) ([]string, error) {
	value, exists := args[key]
	if !exists {
		return nil, nil // Optional parameter
	}

	// Handle different array representations
	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, errors.Errorf("parameter %s must be an array of strings", key)
			}
			result[i] = str
		}
		return result, nil
	default:
		return nil, errors.Errorf("parameter %s must be an array", key)
	}
}

// NoOpProgressEmitter provides a no-op implementation of ProgressEmitter
type NoOpProgressEmitter struct{}

func (e *NoOpProgressEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return nil
}

func (e *NoOpProgressEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	return nil
}

func (e *NoOpProgressEmitter) Close() error {
	return nil
}

// MarshalJSON marshals data to JSON string
func MarshalJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("error marshaling data: %v", err)
	}
	return string(bytes)
}
