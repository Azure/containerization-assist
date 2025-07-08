package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ToolCoordinator orchestrates tool execution at the application layer
type ToolCoordinator struct {
	registry *UnifiedRegistry
	logger   *slog.Logger
}

// NewToolCoordinator creates a new tool coordinator
func NewToolCoordinator(registry *UnifiedRegistry, logger *slog.Logger) *ToolCoordinator {
	return &ToolCoordinator{
		registry: registry,
		logger:   logger,
	}
}

// ExecuteTool executes a tool by name with the provided input
func (c *ToolCoordinator) ExecuteTool(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	c.logger.Info("executing tool", "name", name, "session_id", input.SessionID)

	// Get tool from registry
	tool, err := c.registry.Get(name)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeToolNotFound).
			Message(fmt.Sprintf("tool '%s' not found", name)).
			Context("tool_name", name).
			Context("available_tools", c.registry.List()).
			Cause(err).
			Build()
	}

	// Execute tool with context
	result, err := tool.Execute(ctx, input)
	if err != nil {
		c.logger.Error("tool execution failed", "name", name, "error", err)
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeToolExecutionFailed).
			Message("tool execution failed").
			Context("tool_name", name).
			Context("session_id", input.SessionID).
			Cause(err).
			Build()
	}

	c.logger.Info("tool execution completed", "name", name, "session_id", input.SessionID)
	return result, nil
}

// GetAvailableTools returns a list of available tools
func (c *ToolCoordinator) GetAvailableTools() []string {
	return c.registry.List()
}

// GetToolInfo returns information about a specific tool
func (c *ToolCoordinator) GetToolInfo(name string) (api.ToolMetadata, error) {
	return c.registry.GetMetadata(name)
}

// RegisterTool registers a new tool with the coordinator
func (c *ToolCoordinator) RegisterTool(tool api.Tool, options ...RegistryOption) error {
	return c.registry.Register(tool, options...)
}

// UnregisterTool removes a tool from the coordinator
func (c *ToolCoordinator) UnregisterTool(name string) error {
	return c.registry.Unregister(name)
}

// IsToolRegistered checks if a tool is registered
func (c *ToolCoordinator) IsToolRegistered(name string) bool {
	_, err := c.registry.Get(name)
	return err == nil
}