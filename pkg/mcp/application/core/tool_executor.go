package core

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ToolExecutor handles tool execution
type ToolExecutor interface {
	// Execute runs a tool with the given arguments
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error)

	// ValidateInput validates the input for a specific tool
	ValidateInput(toolName string, args map[string]interface{}) error
}

// toolExecutor implements ToolExecutor
type toolExecutor struct {
	service *ToolService
}

// NewToolExecutor creates a new ToolExecutor service
func NewToolExecutor(service *ToolService) ToolExecutor {
	return &toolExecutor{
		service: service,
	}
}

func (t *toolExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	// Validate input first
	if err := t.ValidateInput(toolName, args); err != nil {
		return nil, err
	}

	// Execute using the existing manager method
	return t.service.ExecuteTool(ctx, toolName, args)
}

func (t *toolExecutor) ValidateInput(toolName string, _ map[string]interface{}) error {
	// Basic validation - check if tool exists
	if !t.service.isAtomicTool(toolName) {
		// Check if it's a built-in tool
		switch toolName {
		case "chat", "conversation_history", "workflow", "workflow_status", "list_workflows", "execute_workflow", "get_workflow_status":
			// Built-in tools are valid
			return nil
		default:
			return errors.NewError().
				Messagef("unknown tool: %s", toolName).
				WithLocation().
				Build()
		}
	}

	// Tool-specific validation could be added here
	// For now, we rely on the tool's own validation during execution

	return nil
}
