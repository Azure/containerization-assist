package workflow

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// ToolRegistry provides simple tool registration
type ToolRegistry struct {
	tools map[string]api.Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]api.Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(name string, tool api.Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (api.Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *ToolRegistry) List() map[string]api.Tool {
	result := make(map[string]api.Tool)
	for name, tool := range r.tools {
		result[name] = tool
	}
	return result
}

// ExecutionContext provides simple execution context
type ExecutionContext struct {
	Context context.Context
	Data    map[string]interface{}
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(ctx context.Context) *ExecutionContext {
	return &ExecutionContext{
		Context: ctx,
		Data:    make(map[string]interface{}),
	}
}
