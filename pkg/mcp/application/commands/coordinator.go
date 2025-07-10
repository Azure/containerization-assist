package commands

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// ToolCoordinator orchestrates tool execution at the application layer
type ToolCoordinator struct {
	registry api.ToolRegistry
	logger   *slog.Logger
}

// NewToolCoordinator creates a new tool coordinator
func NewToolCoordinator(registry api.ToolRegistry, logger *slog.Logger) *ToolCoordinator {
	return &ToolCoordinator{
		registry: registry,
		logger:   logger,
	}
}

// ExecuteTool executes a tool by name with the provided input
func (c *ToolCoordinator) ExecuteTool(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	c.logger.Info("executing tool", "name", name, "session_id", input.SessionID)

	// Execute tool using registry
	result, err := c.registry.Execute(ctx, name, input)
	if err != nil {
		c.logger.Error("tool execution failed", "name", name, "error", err)
		return api.ToolOutput{}, err
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
	return c.registry.Metadata(name)
}

// RegisterTool registers a new tool with the coordinator
func (c *ToolCoordinator) RegisterTool(name string, tool api.Tool) error {
	factory := func() api.Tool { return tool }
	return c.registry.Register(name, factory)
}

// UnregisterTool removes a tool from the coordinator
func (c *ToolCoordinator) UnregisterTool(name string) error {
	return c.registry.Unregister(name)
}

// IsToolRegistered checks if a tool is registered
func (c *ToolCoordinator) IsToolRegistered(name string) bool {
	_, err := c.registry.Discover(name)
	return err == nil
}
