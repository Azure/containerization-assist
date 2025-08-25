package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Azure/containerization-assist/pkg/api"
	domainworkflow "github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/messaging"
	"github.com/Azure/containerization-assist/pkg/service/session"
)

// SimpleWorkflowState represents a simplified workflow state for tool operations
type SimpleWorkflowState struct {
	SessionID      string                        `json:"session_id"`
	RepoPath       string                        `json:"repo_path"`
	Status         string                        `json:"status"`
	CurrentStep    string                        `json:"current_step"`
	CompletedSteps []string                      `json:"completed_steps"`
	FailedSteps    []string                      `json:"failed_steps"`
	SkipSteps      []string                      `json:"skip_steps"`
	Artifacts      *WorkflowArtifacts            `json:"artifacts"`
	Metadata       *ToolMetadata                 `json:"metadata"`
	Error          *domainworkflow.WorkflowError `json:"error,omitempty"`
}

func (s *SimpleWorkflowState) MarkStepCompleted(stepName string) {
	for _, completed := range s.CompletedSteps {
		if completed == stepName {
			return
		}
	}
	s.removeFromFailedSteps(stepName)
	s.CompletedSteps = append(s.CompletedSteps, stepName)
}

func (s *SimpleWorkflowState) MarkStepFailed(stepName string) {
	for _, failed := range s.FailedSteps {
		if failed == stepName {
			return
		}
	}
	s.removeFromCompletedSteps(stepName)
	s.FailedSteps = append(s.FailedSteps, stepName)
}

func (s *SimpleWorkflowState) removeFromCompletedSteps(stepName string) {
	for i, completed := range s.CompletedSteps {
		if completed == stepName {
			s.CompletedSteps = append(s.CompletedSteps[:i], s.CompletedSteps[i+1:]...)
			return
		}
	}
}

func (s *SimpleWorkflowState) removeFromFailedSteps(stepName string) {
	for i, failed := range s.FailedSteps {
		if failed == stepName {
			s.FailedSteps = append(s.FailedSteps[:i], s.FailedSteps[i+1:]...)
			return
		}
	}
}

func (s *SimpleWorkflowState) IsStepCompleted(stepName string) bool {
	for _, completed := range s.CompletedSteps {
		if completed == stepName {
			return true
		}
	}
	return false
}

func (s *SimpleWorkflowState) IsStepFailed(stepName string) bool {
	for _, failed := range s.FailedSteps {
		if failed == stepName {
			return true
		}
	}
	return false
}

func (s *SimpleWorkflowState) GetStepStatus(stepName string) string {
	if s.IsStepCompleted(stepName) {
		return "completed"
	}
	if s.IsStepFailed(stepName) {
		return "failed"
	}
	return "not_started"
}

func (s *SimpleWorkflowState) SetError(err *domainworkflow.WorkflowError) {
	s.Error = err
	s.Status = "error"
}

func (s *SimpleWorkflowState) UpdateArtifacts(artifacts *WorkflowArtifacts) {
	if s.Artifacts == nil {
		s.Artifacts = &WorkflowArtifacts{}
	}
	if artifacts.AnalyzeResult != nil {
		s.Artifacts.AnalyzeResult = artifacts.AnalyzeResult
	}
	if artifacts.DockerfileResult != nil {
		s.Artifacts.DockerfileResult = artifacts.DockerfileResult
	}
	if artifacts.BuildResult != nil {
		s.Artifacts.BuildResult = artifacts.BuildResult
	}
	if artifacts.K8sResult != nil {
		s.Artifacts.K8sResult = artifacts.K8sResult
	}
	if artifacts.ScanResult != nil {
		s.Artifacts.ScanResult = artifacts.ScanResult
	}
}

// LoadWorkflowState loads workflow state from session (creates session if needed)
func LoadWorkflowState(ctx context.Context, sessionManager session.OptimizedSessionManager, sessionID string) (*SimpleWorkflowState, error) {
	// Use GetOrCreate to ensure session exists
	_, err := sessionManager.GetOrCreate(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create session: %w", err)
	}

	var metadata map[string]interface{}

	// Use concurrent-safe read if available
	if concurrentAdapter, ok := sessionManager.(*session.ConcurrentBoltAdapter); ok {
		metadata, err = concurrentAdapter.GetWorkflowState(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow state: %w", err)
		}
	} else {
		// Fallback to regular Get for other implementations
		sessionData, err := sessionManager.Get(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		if sessionData == nil {
			return nil, errors.New("session not found")
		}
		metadata = sessionData.Metadata
	}

	// Extract workflow state from metadata
	workflowData, exists := metadata["workflow_state"]
	if !exists {
		return &SimpleWorkflowState{
			SessionID:      sessionID,
			Status:         "initialized",
			CompletedSteps: []string{},
			FailedSteps:    []string{},
			Artifacts:      &WorkflowArtifacts{},
			Metadata:       &ToolMetadata{},
		}, nil
	}

	workflowMap, ok := workflowData.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid workflow state format in session")
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
	if artifactsData, ok := workflowMap["artifacts"].(map[string]interface{}); ok {
		state.Artifacts = deserializeArtifacts(artifactsData)
	} else {
		state.Artifacts = &WorkflowArtifacts{}
	}
	if metadataData, ok := workflowMap["metadata"].(map[string]interface{}); ok {
		state.Metadata = deserializeMetadata(metadataData)
	} else {
		state.Metadata = &ToolMetadata{}
	}

	return state, nil
}

// SaveWorkflowState saves workflow state to session
func SaveWorkflowState(ctx context.Context, sessionManager session.OptimizedSessionManager, state *SimpleWorkflowState) error {
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

	_, err := sessionManager.GetOrCreate(ctx, state.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	// Use concurrent-safe update if available
	if concurrentAdapter, ok := sessionManager.(*session.ConcurrentBoltAdapter); ok {
		err = concurrentAdapter.UpdateWorkflowState(ctx, state.SessionID, func(metadata map[string]interface{}) error {
			metadata["workflow_state"] = workflowData
			return nil
		})
	} else {
		err = sessionManager.Update(ctx, state.SessionID, func(sessionState *session.SessionState) error {
			if sessionState.Metadata == nil {
				sessionState.Metadata = make(map[string]interface{})
			}
			sessionState.Metadata["workflow_state"] = workflowData
			return nil
		})
	}

	if err != nil {
		return fmt.Errorf("failed to update session with workflow state: %w", err)
	}

	return nil
}

func GenerateSessionID() string {
	return fmt.Sprintf("wf_%s", uuid.New().String())
}

// CreateProgressEmitter creates a progress emitter using the messaging package
func CreateProgressEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) api.ProgressEmitter {
	return messaging.CreateProgressEmitter(ctx, req, totalSteps, logger)
}

// ExtractStringParam safely extracts a string parameter from arguments
func ExtractStringParam(args map[string]interface{}, key string) (string, error) {
	value, exists := args[key]
	if !exists {
		return "", fmt.Errorf("missing parameter: %s", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}

	if str == "" {
		return "", fmt.Errorf("parameter %s cannot be empty", key)
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
		return nil, nil
	}

	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %s must be an array of strings", key)
			}
			result[i] = str
		}
		return result, nil
	default:
		return nil, fmt.Errorf("parameter %s must be an array", key)
	}
}

func MarshalJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("error marshaling data: %v", err)
	}
	return string(bytes)
}

// serializeArtifacts converts WorkflowArtifacts to map for storage
func serializeArtifacts(artifacts *WorkflowArtifacts) map[string]interface{} {
	if artifacts == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})

	if artifacts.AnalyzeResult != nil {
		result["analyze_result"] = artifacts.AnalyzeResult
	}
	if artifacts.DockerfileResult != nil {
		result["dockerfile_result"] = artifacts.DockerfileResult
	}
	if artifacts.BuildResult != nil {
		result["build_result"] = artifacts.BuildResult
	}
	if artifacts.K8sResult != nil {
		result["k8s_result"] = artifacts.K8sResult
	}
	if artifacts.ScanResult != nil {
		result["scan_result"] = artifacts.ScanResult
	}

	return result
}

// deserializeArtifacts converts map to WorkflowArtifacts
func deserializeArtifacts(data map[string]interface{}) *WorkflowArtifacts {
	artifacts := &WorkflowArtifacts{}

	if analyzeData, ok := data["analyze_result"].(map[string]interface{}); ok {
		artifacts.AnalyzeResult = &AnalyzeArtifact{}
		if lang, ok := analyzeData["language"].(string); ok {
			artifacts.AnalyzeResult.Language = lang
		}
		if framework, ok := analyzeData["framework"].(string); ok {
			artifacts.AnalyzeResult.Framework = framework
		}
		if port, ok := analyzeData["port"].(float64); ok {
			artifacts.AnalyzeResult.Port = int(port)
		}
		if buildCmd, ok := analyzeData["build_command"].(string); ok {
			artifacts.AnalyzeResult.BuildCommand = buildCmd
		}
		if startCmd, ok := analyzeData["start_command"].(string); ok {
			artifacts.AnalyzeResult.StartCommand = startCmd
		}
		if repoPath, ok := analyzeData["repo_path"].(string); ok {
			artifacts.AnalyzeResult.RepoPath = repoPath
		}
		if metadata, ok := analyzeData["metadata"].(map[string]interface{}); ok {
			artifacts.AnalyzeResult.Metadata = metadata
		}
	}

	if dockerfileData, ok := data["dockerfile_result"].(map[string]interface{}); ok {
		artifacts.DockerfileResult = &DockerfileArtifact{}
		if content, ok := dockerfileData["content"].(string); ok {
			artifacts.DockerfileResult.Content = content
		}
		if path, ok := dockerfileData["path"].(string); ok {
			artifacts.DockerfileResult.Path = path
		}
	}

	if buildData, ok := data["build_result"].(map[string]interface{}); ok {
		artifacts.BuildResult = &BuildArtifact{}
		if imageID, ok := buildData["image_id"].(string); ok {
			artifacts.BuildResult.ImageID = imageID
		}
		if imageRef, ok := buildData["image_ref"].(string); ok {
			artifacts.BuildResult.ImageRef = imageRef
		}
		if imageSize, ok := buildData["image_size"].(float64); ok {
			artifacts.BuildResult.ImageSize = int64(imageSize)
		}
		if buildTime, ok := buildData["build_time"].(string); ok {
			artifacts.BuildResult.BuildTime = buildTime
		}
	}

	if k8sData, ok := data["k8s_result"].(map[string]interface{}); ok {
		artifacts.K8sResult = &K8sArtifact{}
		if namespace, ok := k8sData["namespace"].(string); ok {
			artifacts.K8sResult.Namespace = namespace
		}
		if endpoint, ok := k8sData["endpoint"].(string); ok {
			artifacts.K8sResult.Endpoint = endpoint
		}
		if manifests, ok := k8sData["manifests"].([]interface{}); ok {
			manifestStrs := make([]string, 0, len(manifests))
			for _, m := range manifests {
				if mStr, ok := m.(string); ok {
					manifestStrs = append(manifestStrs, mStr)
				}
			}
			artifacts.K8sResult.Manifests = manifestStrs
		}
	}

	return artifacts
}

// serializeMetadata converts ToolMetadata to map for storage
func serializeMetadata(metadata *ToolMetadata) map[string]interface{} {
	if metadata == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})

	if metadata.SessionID != "" {
		result["session_id"] = metadata.SessionID
	}
	if metadata.WorkflowID != "" {
		result["workflow_id"] = metadata.WorkflowID
	}
	if metadata.Step != "" {
		result["step"] = metadata.Step
	}
	if !metadata.Timestamp.IsZero() {
		result["timestamp"] = metadata.Timestamp.Format(time.RFC3339)
	}
	if metadata.Version != "" {
		result["version"] = metadata.Version
	}
	if metadata.Custom != nil && len(metadata.Custom) > 0 {
		result["custom"] = metadata.Custom
	}

	return result
}

// deserializeMetadata converts map to ToolMetadata
func deserializeMetadata(data map[string]interface{}) *ToolMetadata {
	metadata := &ToolMetadata{}

	if sessionID, ok := data["session_id"].(string); ok {
		metadata.SessionID = sessionID
	}
	if workflowID, ok := data["workflow_id"].(string); ok {
		metadata.WorkflowID = workflowID
	}
	if step, ok := data["step"].(string); ok {
		metadata.Step = step
	}
	if timestampStr, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			metadata.Timestamp = t
		}
	}
	if version, ok := data["version"].(string); ok {
		metadata.Version = version
	}
	if custom, ok := data["custom"].(map[string]interface{}); ok {
		metadata.Custom = make(map[string]string)
		for k, v := range custom {
			if str, ok := v.(string); ok {
				metadata.Custom[k] = str
			}
		}
	} else if custom, ok := data["custom"].(map[string]string); ok {
		metadata.Custom = custom
	}

	return metadata
}
