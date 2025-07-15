// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/server"
)

// ToolRegistrarType is a type alias for the generic registry
type ToolRegistrarType = registry.Registry[func() error]

// ResourceRegistrarType is a type alias for the generic registry
type ResourceRegistrarType = registry.Registry[func() error]

// Registrar manages all registrations (tools, resources)
type Registrar struct {
	toolRegistrar     *ToolRegistrar
	resourceRegistrar *ResourceRegistrar
	tools             *ToolRegistrarType
	resources         *ResourceRegistrarType
}

// NewRegistrar creates a new unified registrar
func NewRegistrar(logger *slog.Logger, resourceStore resources.Store, orchestrator workflow.WorkflowOrchestrator) *Registrar {
	return &Registrar{
		toolRegistrar:     NewToolRegistrar(logger, orchestrator),
		resourceRegistrar: NewResourceRegistrar(logger, resourceStore),
		tools:             registry.New[func() error](),
		resources:         registry.New[func() error](),
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
