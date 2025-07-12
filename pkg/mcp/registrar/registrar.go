// Package registrar handles tool and prompt registration
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/mark3labs/mcp-go/server"
)

// Registrar manages all registrations (tools, prompts, resources)
type Registrar struct {
	toolRegistrar     *ToolRegistrar
	promptRegistrar   *PromptRegistrar
	resourceRegistrar *ResourceRegistrar
}

// NewRegistrar creates a new unified registrar
func NewRegistrar(logger *slog.Logger, resourceStore *resources.Store) *Registrar {
	return &Registrar{
		toolRegistrar:     NewToolRegistrar(logger),
		promptRegistrar:   NewPromptRegistrar(logger),
		resourceRegistrar: NewResourceRegistrar(logger, resourceStore),
	}
}

// RegisterAll registers all components with the MCP server
func (r *Registrar) RegisterAll(mcpServer *server.MCPServer) error {
	// Register tools
	if err := r.toolRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	// Register prompts
	if err := r.promptRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	// Register resources
	if err := r.resourceRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	return nil
}
