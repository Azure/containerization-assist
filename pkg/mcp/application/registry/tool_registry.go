// Package registry handles tool and resource registration
package registry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/transport"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolRegistry manages tool and resource registration
type ToolRegistry interface {
	RegisterWorkflowTools(trans transport.MCPTransport) error
	RegisterPrompts(trans transport.MCPTransport) error
	RegisterResources(trans transport.MCPTransport, store *resources.Store) error
}

// toolRegistryImpl implements the tool registry
type toolRegistryImpl struct {
	logger *slog.Logger
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(logger *slog.Logger) ToolRegistry {
	return &toolRegistryImpl{
		logger: logger.With("component", "tool_registry"),
	}
}

// RegisterWorkflowTools registers workflow tools with the transport
func (r *toolRegistryImpl) RegisterWorkflowTools(trans transport.MCPTransport) error {
	r.logger.Info("Registering workflow tools")

	// Create a simple adapter to pass the transport to the workflow registration
	adapter := &transportAdapter{
		transport: trans,
		logger:    r.logger,
	}

	// Use the existing workflow registration
	if err := workflow.RegisterWorkflowTools(adapter, r.logger); err != nil {
		return fmt.Errorf("failed to register workflow tools: %w", err)
	}

	r.logger.Info("Workflow tools registered successfully")
	return nil
}

// RegisterPrompts registers prompts with the transport
func (r *toolRegistryImpl) RegisterPrompts(trans transport.MCPTransport) error {
	r.logger.Info("Registering prompts")

	// Get the underlying MCP server (this is a temporary solution)
	stdioTransport, ok := trans.(*transport.StdioTransport)
	if !ok {
		return fmt.Errorf("prompts registration requires stdio transport")
	}

	mcpServer := stdioTransport.GetMCPServer()
	if mcpServer == nil {
		return fmt.Errorf("MCP server not available")
	}

	// Create and register prompts
	promptRegistry := prompts.NewRegistry(mcpServer, r.logger)
	if err := promptRegistry.RegisterAll(); err != nil {
		return fmt.Errorf("failed to register prompts: %w", err)
	}

	r.logger.Info("Prompts registered successfully")
	return nil
}

// RegisterResources registers resources with the transport
func (r *toolRegistryImpl) RegisterResources(trans transport.MCPTransport, store *resources.Store) error {
	r.logger.Info("Registering resources")

	// Get the underlying MCP server (this is a temporary solution)
	stdioTransport, ok := trans.(*transport.StdioTransport)
	if !ok {
		return fmt.Errorf("resources registration requires stdio transport")
	}

	mcpServer := stdioTransport.GetMCPServer()
	if mcpServer == nil {
		return fmt.Errorf("MCP server not available")
	}

	// Register resource providers using the Store's method
	if err := store.RegisterProviders(mcpServer); err != nil {
		return fmt.Errorf("failed to register resource providers: %w", err)
	}

	r.logger.Info("Resources registered successfully")
	return nil
}

// transportAdapter adapts the transport interface for workflow registration
type transportAdapter struct {
	transport transport.MCPTransport
	logger    *slog.Logger
}

// AddTool implements the interface expected by workflow.RegisterWorkflowTools
func (a *transportAdapter) AddTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	if err := a.transport.RegisterTool(tool, handler); err != nil {
		a.logger.Error("Failed to register tool", "name", tool.Name, "error", err)
	}
}

// Registry combines multiple registries
type Registry struct {
	toolRegistry ToolRegistry
	logger       *slog.Logger
}

// NewRegistry creates a new combined registry
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		toolRegistry: NewToolRegistry(logger),
		logger:       logger.With("component", "registry"),
	}
}

// RegisterAll registers all components with the transport
func (r *Registry) RegisterAll(ctx context.Context, trans transport.MCPTransport, resourceStore *resources.Store) error {
	r.logger.Info("Registering all components")

	// Register workflow tools
	if err := r.toolRegistry.RegisterWorkflowTools(trans); err != nil {
		return fmt.Errorf("failed to register workflow tools: %w", err)
	}

	// Register prompts
	if err := r.toolRegistry.RegisterPrompts(trans); err != nil {
		return fmt.Errorf("failed to register prompts: %w", err)
	}

	// Register resources
	if err := r.toolRegistry.RegisterResources(trans, resourceStore); err != nil {
		return fmt.Errorf("failed to register resources: %w", err)
	}

	r.logger.Info("All components registered successfully")
	return nil
}
