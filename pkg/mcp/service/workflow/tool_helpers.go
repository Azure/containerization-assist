package workflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"

	domainworkflow "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolChainHint provides hints for tool chaining
type ToolChainHint struct {
	NextTool       string                 `json:"next_tool,omitempty"`
	NextParameters map[string]interface{} `json:"next_parameters,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
}

// ToolResult represents a standardized tool result with chaining hints
type ToolResult struct {
	Success   bool                   `json:"success"`
	Data      interface{}            `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ChainHint *ToolChainHint         `json:"chain_hint,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// createToolResult creates a consistent MCP tool result
func createToolResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// createErrorResult creates an error result
func createErrorResult(err error, stepName string) (*mcp.CallToolResult, error) {
	result := ToolResult{
		Success: false,
		Error:   err.Error(),
		Metadata: map[string]interface{}{
			"step": stepName,
		},
	}
	return createToolResult(result)
}

const workflowStateKey = "workflow_state"

// SerializableWorkflowState represents WorkflowState in a serializable format
type SerializableWorkflowState struct {
	WorkflowID       string                                      `json:"workflow_id"`
	RepoPath         string                                      `json:"repo_path"`
	RepoURL          string                                      `json:"repo_url,omitempty"`
	Branch           string                                      `json:"branch,omitempty"`
	Deploy           *bool                                       `json:"deploy,omitempty"`
	Scan             bool                                        `json:"scan,omitempty"`
	TestMode         bool                                        `json:"test_mode,omitempty"`
	RepoIdentifier   string                                      `json:"repo_identifier,omitempty"`
	AnalyzeResult    *domainworkflow.AnalyzeResult               `json:"analyze_result,omitempty"`
	DockerfileResult *domainworkflow.DockerfileResult            `json:"dockerfile_result,omitempty"`
	BuildResult      *domainworkflow.BuildResult                 `json:"build_result,omitempty"`
	K8sResult        *domainworkflow.K8sResult                   `json:"k8s_result,omitempty"`
	ScanReport       map[string]interface{}                      `json:"scan_report,omitempty"`
	CompletedSteps   []string                                    `json:"completed_steps,omitempty"`
	CurrentStep      string                                      `json:"current_step,omitempty"`
	Result           *domainworkflow.ContainerizeAndDeployResult `json:"result,omitempty"`
}

// loadOrCreateWorkflowState loads existing workflow state or creates new one
func loadOrCreateWorkflowState(ctx context.Context, sessionID, repoPath string, sessionManager session.OptimizedSessionManager, logger *slog.Logger) (*domainworkflow.WorkflowState, error) {
	// Get or create session
	sessionState, err := sessionManager.GetOrCreate(ctx, sessionID)
	if err != nil {
		logger.Error("Debug: Failed to get/create session", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("failed to get/create session: %w", err)
	}
	logger.Info("Debug: Session retrieved/created successfully", "session_id", sessionID, "session_exists", sessionState != nil)

	// Debug session metadata
	logger.Info("Debug: Session metadata check",
		"session_id", sessionID,
		"metadata_exists", sessionState.Metadata != nil,
		"metadata_keys", len(sessionState.Metadata),
		"workflow_key_exists", sessionState.Metadata != nil && sessionState.Metadata[workflowStateKey] != nil)

	// Check if workflow state exists in session metadata
	if workflowData, exists := sessionState.Metadata[workflowStateKey]; exists {
		logger.Info("Debug: Found workflow data in session", "session_id", sessionID, "data_type", fmt.Sprintf("%T", workflowData))

		if workflowBytes, ok := workflowData.([]byte); ok {
			logger.Info("Debug: Workflow data is []byte", "session_id", sessionID, "byte_length", len(workflowBytes))
			var serializable SerializableWorkflowState
			if err := json.Unmarshal(workflowBytes, &serializable); err == nil {
				logger.Info("Debug: Successfully loaded existing workflow state", "session_id", sessionID,
					"completed_steps", len(serializable.CompletedSteps),
					"has_analyze_result", serializable.AnalyzeResult != nil,
					"has_dockerfile_result", serializable.DockerfileResult != nil,
					"repo_path", serializable.RepoPath)
				return deserializeWorkflowState(serializable, logger), nil
			}
			logger.Warn("Failed to unmarshal workflow state from bytes, creating new", "session_id", sessionID, "error", err)
		} else if workflowMap, ok := workflowData.(map[string]interface{}); ok {
			logger.Info("Debug: Workflow data is map[string]interface{}", "session_id", sessionID, "map_keys", len(workflowMap))
			// Handle case where JSON was unmarshaled as map[string]interface{}
			workflowBytes, _ := json.Marshal(workflowMap)
			var serializable SerializableWorkflowState
			if err := json.Unmarshal(workflowBytes, &serializable); err == nil {
				logger.Info("Debug: Successfully loaded existing workflow state from map", "session_id", sessionID,
					"completed_steps", len(serializable.CompletedSteps),
					"has_analyze_result", serializable.AnalyzeResult != nil,
					"has_dockerfile_result", serializable.DockerfileResult != nil)
				return deserializeWorkflowState(serializable, logger), nil
			}
			logger.Warn("Failed to unmarshal workflow state from map, creating new", "session_id", sessionID, "error", err)
		} else if workflowString, ok := workflowData.(string); ok {
			logger.Info("Debug: Workflow data is string", "session_id", sessionID, "string_length", len(workflowString))
			previewLen := 200
			if len(workflowString) < previewLen {
				previewLen = len(workflowString)
			}
			logger.Info("Debug: First 200 chars of workflow string", "session_id", sessionID, "data_preview", workflowString[:previewLen])

			// Handle case where data was stored as string (session manager Base64 encoded it)
			// First try to decode as Base64
			workflowBytes, err := base64.StdEncoding.DecodeString(workflowString)
			if err != nil {
				logger.Warn("Failed to decode Base64 workflow string, trying as raw JSON", "session_id", sessionID, "error", err)
				workflowBytes = []byte(workflowString)
			} else {
				logger.Info("Debug: Successfully decoded Base64 workflow data", "session_id", sessionID, "decoded_length", len(workflowBytes))
			}

			var serializable SerializableWorkflowState
			if err := json.Unmarshal(workflowBytes, &serializable); err == nil {
				logger.Info("Debug: Successfully loaded existing workflow state from string", "session_id", sessionID,
					"completed_steps", len(serializable.CompletedSteps),
					"has_analyze_result", serializable.AnalyzeResult != nil,
					"has_dockerfile_result", serializable.DockerfileResult != nil,
					"repo_path", serializable.RepoPath)
				return deserializeWorkflowState(serializable, logger), nil
			}
			logger.Error("Failed to unmarshal workflow state from string", "session_id", sessionID, "error", err, "json_length", len(workflowBytes))
		} else {
			logger.Warn("Debug: Workflow data has unexpected type", "session_id", sessionID, "type", fmt.Sprintf("%T", workflowData))
		}
	}

	// Create new workflow state
	args := &domainworkflow.ContainerizeAndDeployArgs{
		RepoPath: repoPath,
	}

	// Calculate repo identifier the same way as the orchestrator
	repoIdentifier := domainworkflow.GetRepositoryIdentifier(args)

	state := &domainworkflow.WorkflowState{
		WorkflowID:     sessionID,
		Args:           args,
		RepoIdentifier: repoIdentifier,
		Result: &domainworkflow.ContainerizeAndDeployResult{
			Steps: []domainworkflow.WorkflowStep{},
		},
		Logger: logger,
	}

	logger.Info("Debug: Created new workflow state", "session_id", sessionID, "repo_path", repoPath)
	return state, nil
}

// saveWorkflowState saves the workflow state to session storage
func saveWorkflowState(ctx context.Context, sessionID string, state *domainworkflow.WorkflowState, sessionManager session.OptimizedSessionManager, logger *slog.Logger) error {
	// Serialize the workflow state
	serializable := serializeWorkflowState(state)
	workflowBytes, err := json.Marshal(serializable)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	// Update session metadata
	logger.Info("Debug: Attempting to save workflow state", "session_id", sessionID, "data_size", len(workflowBytes))
	err = sessionManager.Update(ctx, sessionID, func(sessionState *session.SessionState) error {
		if sessionState.Metadata == nil {
			sessionState.Metadata = make(map[string]interface{})
			logger.Info("Debug: Created new metadata map for session", "session_id", sessionID)
		}
		sessionState.Metadata[workflowStateKey] = workflowBytes
		logger.Info("Debug: Set workflow state in metadata", "session_id", sessionID, "key", workflowStateKey)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to save workflow state to session: %w", err)
	}

	logger.Info("Debug: Saved workflow state", "session_id", sessionID,
		"completed_steps", len(serializable.CompletedSteps),
		"has_analyze_result", serializable.AnalyzeResult != nil,
		"has_dockerfile_result", serializable.DockerfileResult != nil,
		"repo_path", serializable.RepoPath)
	return nil
}

// serializeWorkflowState converts WorkflowState to serializable format
func serializeWorkflowState(state *domainworkflow.WorkflowState) SerializableWorkflowState {
	serializable := SerializableWorkflowState{
		WorkflowID:     state.WorkflowID,
		RepoIdentifier: state.RepoIdentifier,
		Result:         state.Result,
	}

	if state.Args != nil {
		serializable.RepoPath = state.Args.RepoPath
		serializable.RepoURL = state.Args.RepoURL
		serializable.Branch = state.Args.Branch
		serializable.Deploy = state.Args.Deploy
		serializable.Scan = state.Args.Scan
		serializable.TestMode = state.Args.TestMode
	}

	if state.AnalyzeResult != nil {
		serializable.AnalyzeResult = state.AnalyzeResult
	}
	if state.DockerfileResult != nil {
		serializable.DockerfileResult = state.DockerfileResult
	}
	if state.BuildResult != nil {
		serializable.BuildResult = state.BuildResult
	}
	if state.K8sResult != nil {
		serializable.K8sResult = state.K8sResult
	}
	if state.ScanReport != nil {
		serializable.ScanReport = state.ScanReport
	}

	return serializable
}

// deserializeWorkflowState converts serializable format back to WorkflowState
func deserializeWorkflowState(serializable SerializableWorkflowState, logger *slog.Logger) *domainworkflow.WorkflowState {
	args := &domainworkflow.ContainerizeAndDeployArgs{
		RepoPath: serializable.RepoPath,
		RepoURL:  serializable.RepoURL,
		Branch:   serializable.Branch,
		Deploy:   serializable.Deploy,
		Scan:     serializable.Scan,
		TestMode: serializable.TestMode,
	}

	result := serializable.Result
	if result == nil {
		result = &domainworkflow.ContainerizeAndDeployResult{
			Steps: []domainworkflow.WorkflowStep{},
		}
	}

	state := &domainworkflow.WorkflowState{
		WorkflowID:       serializable.WorkflowID,
		RepoIdentifier:   serializable.RepoIdentifier,
		Args:             args,
		Result:           result,
		AnalyzeResult:    serializable.AnalyzeResult,
		DockerfileResult: serializable.DockerfileResult,
		BuildResult:      serializable.BuildResult,
		K8sResult:        serializable.K8sResult,
		ScanReport:       serializable.ScanReport,
		Logger:           logger,
	}

	return state
}
