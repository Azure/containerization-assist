package orchestration

import (
	"context"
)

// ToolExecutor defines the interface for executing tools
type ToolExecutor interface {
	// ExecuteTool executes a tool with the given input
	ExecuteTool(ctx context.Context, input ToolInput) (interface{}, error)
}

// ToolInput represents the input parameters for tool execution
type ToolInput struct {
	SessionID  string                 `json:"session_id"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}
