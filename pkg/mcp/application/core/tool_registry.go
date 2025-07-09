package core

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// CoreToolRegistry manages tool registration and discovery (internal core version)
// CoreToolRegistry - Use services.ToolRegistry for the canonical interface
// This version is simplified for core tool management
// Deprecated: Use services.ToolRegistry for new code
type CoreToolRegistry interface {
	// Register adds a new tool to the registry
	Register(tool api.Tool) error

	// Get retrieves a tool by name
	Get(name string) (api.Tool, error)

	// List returns all registered tool names
	List() []string

	// ListByMode returns tools available for a specific server mode
	ListByMode(mode ServerMode) []ToolDefinition
}

// toolRegistry implements CoreToolRegistry
type toolRegistry struct {
	service *ToolService
}

// NewCoreToolRegistry creates a new CoreToolRegistry service
func NewCoreToolRegistry(service *ToolService) CoreToolRegistry {
	return &toolRegistry{
		service: service,
	}
}

func (t *toolRegistry) Register(tool api.Tool) error {
	// This would need to be implemented based on the actual tool storage mechanism
	// For now, tools are registered through the orchestrator
	if t.service.server.toolOrchestrator != nil {
		return t.service.server.toolOrchestrator.RegisterTool(tool.Name(), tool)
	}
	return ErrOrchestratorNotAvailable
}

func (t *toolRegistry) Get(name string) (api.Tool, error) {
	if t.service.server.toolOrchestrator != nil {
		if tool, ok := t.service.server.toolOrchestrator.GetTool(name); ok {
			return tool, nil
		}
	}
	return nil, ErrToolNotFound
}

func (t *toolRegistry) List() []string {
	if t.service.server.toolOrchestrator != nil {
		return t.service.server.toolOrchestrator.ListTools()
	}
	return []string{}
}

func (t *toolRegistry) ListByMode(mode ServerMode) []ToolDefinition {
	// Save current mode, switch temporarily, get tools, then restore
	originalMode := t.service.server.currentMode
	t.service.server.currentMode = mode
	tools := t.service.GetAvailableTools()
	t.service.server.currentMode = originalMode
	return tools
}
