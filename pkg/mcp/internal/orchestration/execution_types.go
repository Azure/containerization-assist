package execution

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
)

// ExecuteToolFunc is the signature for tool execution functions
type ExecuteToolFunc func(
	ctx context.Context,
	toolName string,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
) (interface{}, error)

// ExecutionResult represents the result of executing tools
type ExecutionResult struct {
	Success   bool                        `json:"success"`
	Results   map[string]interface{}      `json:"results"`
	Artifacts []workflow.WorkflowArtifact `json:"artifacts"`
	Metrics   map[string]interface{}      `json:"metrics"`
	Duration  time.Duration               `json:"duration"`
	Error     *ExecutionError             `json:"error,omitempty"`
}

// ExecutionError provides detailed error information
type ExecutionError struct {
	ToolName string `json:"tool_name"`
	Index    int    `json:"index"`
	Error    error  `json:"error"`
	Type     string `json:"type"`
}

// Executor interface for different execution strategies
type Executor interface {
	Execute(
		ctx context.Context,
		stage *workflow.WorkflowStage,
		session *workflow.WorkflowSession,
		toolNames []string,
		executeToolFunc ExecuteToolFunc,
	) (*ExecutionResult, error)
}

// Helper function to extract artifacts from tool results
func extractArtifacts(toolResult interface{}) []workflow.WorkflowArtifact {
	if toolResult == nil {
		return nil
	}

	// Try to extract artifacts from the result
	if resultMap, ok := toolResult.(map[string]interface{}); ok {
		if artifacts, exists := resultMap["artifacts"]; exists {
			if artifactList, ok := artifacts.([]workflow.WorkflowArtifact); ok {
				return artifactList
			}
			// Try to convert []interface{} to []WorkflowArtifact
			if artifactInterfaces, ok := artifacts.([]interface{}); ok {
				var result []workflow.WorkflowArtifact
				for _, a := range artifactInterfaces {
					if artifact, ok := a.(workflow.WorkflowArtifact); ok {
						result = append(result, artifact)
					}
				}
				return result
			}
		}
	}

	return nil
}
