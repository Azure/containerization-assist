// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/server"
)

// Registrar manages all registrations (tools, resources)
type Registrar struct {
	toolRegistrar     *ToolRegistrar
	resourceRegistrar *ResourceRegistrar
}

// NewRegistrar creates a new unified registrar
func NewRegistrar(logger *slog.Logger, resourceStore resources.Store, orchestrator workflow.WorkflowOrchestrator) *Registrar {
	return &Registrar{
		toolRegistrar:     NewToolRegistrar(logger, orchestrator),
		resourceRegistrar: NewResourceRegistrar(logger, resourceStore),
	}
}

// RegisterAll registers all components with the MCP server
func (r *Registrar) RegisterAll(mcpServer *server.MCPServer) error {
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
