package orchestration

import (
	"context"
)

// ToolRegistry interface for the orchestration package
// This is a local interface to avoid import cycles with pkg/mcp
type ToolRegistry interface {
	// GetTool retrieves a tool instance by name
	GetTool(name string) (interface{}, error)
}

// ToolOrchestrator interface for the orchestration package
// This is a local interface to avoid import cycles with pkg/mcp
type ToolOrchestrator interface {
	// ExecuteTool executes a tool with the given arguments
	ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
}
