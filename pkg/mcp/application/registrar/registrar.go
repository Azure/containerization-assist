// Package registrar handles MCP tool and resource registration
//
// The registrar package is responsible for registering tools and resources
// with the MCP server. It should not be confused with the registry package,
// which provides a generic thread-safe map implementation.
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/server"
)

// MCPRegistrar manages all MCP registrations (tools, resources)
type MCPRegistrar struct {
	toolRegistrar     *ToolRegistrar
	resourceRegistrar *ResourceRegistrar
	tools             *registry.Registry[func() error]
	resources         *registry.Registry[func() error]
}

// NewMCPRegistrar creates a new unified MCP registrar
func NewMCPRegistrar(logger *slog.Logger, resourceStore resources.Store, orchestrator workflow.WorkflowOrchestrator) *MCPRegistrar {
	return &MCPRegistrar{
		toolRegistrar:     NewToolRegistrar(logger, orchestrator),
		resourceRegistrar: NewResourceRegistrar(logger, resourceStore),
		tools:             registry.New[func() error](),
		resources:         registry.New[func() error](),
	}
}

// RegisterAll registers all components with the MCP server
func (r *MCPRegistrar) RegisterAll(mcpServer *server.MCPServer) error {
	// Register tools
	if err := r.toolRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	// Register resources
	if err := r.resourceRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	return nil
}
