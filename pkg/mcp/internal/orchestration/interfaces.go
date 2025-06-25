package orchestration

import (
	"context"
)

// InternalToolRegistry interface for the orchestration package
// This is a local interface to avoid import cycles with pkg/mcp
type InternalToolRegistry interface {
	// GetTool retrieves a tool instance by name
	GetTool(name string) (interface{}, error)
}

// InternalToolOrchestrator interface for the orchestration package
// This is a local interface to avoid import cycles with pkg/mcp
type InternalToolOrchestrator interface {
	// ExecuteTool executes a tool with the given arguments
	ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
}
