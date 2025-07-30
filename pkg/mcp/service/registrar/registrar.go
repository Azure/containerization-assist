// Package registrar handles MCP tool and resource registration
//
// The registrar package is responsible for registering tools and resources
// with the MCP server. It should not be confused with the registry package,
// which provides a generic thread-safe map implementation.
package registrar

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	resources "github.com/Azure/container-kit/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/server"
)

// Alias for session manager interface
type OptimizedSessionManager = session.OptimizedSessionManager

// MCPRegistrar manages all MCP registrations (tools, resources)
type MCPRegistrar struct {
	toolRegistrar     *ToolRegistrar
	resourceRegistrar *ResourceRegistrar
	promptRegistrar   *prompts.Registry
	config            workflow.ServerConfig
}

// NewMCPRegistrar creates a new unified MCP registrar
func NewMCPRegistrar(logger *slog.Logger, resourceStore *resources.Store, orchestrator workflow.WorkflowOrchestrator, sessionManager OptimizedSessionManager, config workflow.ServerConfig) *MCPRegistrar {
	// Extract dependencies from orchestrator for individual tools
	var stepProvider workflow.StepProvider

	// Type assert to concrete Orchestrator to access dependencies
	if concreteOrchestrator, ok := orchestrator.(*workflow.Orchestrator); ok {
		stepProvider = concreteOrchestrator.GetStepProvider()
	}

	return &MCPRegistrar{
		toolRegistrar:     NewToolRegistrar(logger, orchestrator, stepProvider, sessionManager, config),
		resourceRegistrar: NewResourceRegistrar(logger, resourceStore),
		promptRegistrar:   prompts.NewRegistry(logger),
		config:            config,
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

	// Register prompts
	if err := r.promptRegistrar.RegisterAll(mcpServer); err != nil {
		return err
	}

	return nil
}
