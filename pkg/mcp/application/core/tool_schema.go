package core

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// ToolSchemaProvider manages tool schemas
type ToolSchemaProvider interface {
	// GetInputSchema returns the input schema for a specific tool
	GetInputSchema(toolName string) (map[string]interface{}, error)

	// BuildSchemaForMode returns schemas for all tools in a specific mode
	BuildSchemaForMode(mode ServerMode) map[string]interface{}

	// GetToolDefinitions returns all tool definitions for the current mode
	GetToolDefinitions() []ToolDefinition
}

// toolSchemaProvider implements ToolSchemaProvider
type toolSchemaProvider struct {
	service *ToolService
}

// NewToolSchemaProvider creates a new ToolSchemaProvider service
func NewToolSchemaProvider(service *ToolService) ToolSchemaProvider {
	return &toolSchemaProvider{
		service: service,
	}
}

func (t *toolSchemaProvider) GetInputSchema(toolName string) (map[string]interface{}, error) {
	// Check built-in tools first
	switch toolName {
	case "chat":
		return t.service.buildChatInputSchema(), nil
	case "workflow", "execute_workflow":
		return t.service.buildWorkflowInputSchema(), nil
	case "conversation_history":
		return t.service.buildConversationHistoryInputSchema(), nil
	case "workflow_status", "get_workflow_status":
		return t.service.buildWorkflowStatusInputSchema(), nil
	case "list_workflows":
		return t.service.buildWorkflowListInputSchema(), nil
	}

	// Check atomic tools
	if t.service.server.toolOrchestrator != nil {
		if tool, ok := t.service.server.toolOrchestrator.GetTool(toolName); ok {
			schema := tool.Schema()
			return t.service.buildInputSchema(&api.ToolMetadata{
				Name:        schema.Name,
				Description: schema.Description,
				Version:     schema.Version,
			}), nil
		}
	}

	return nil, ErrToolNotFound
}

func (t *toolSchemaProvider) BuildSchemaForMode(mode ServerMode) map[string]interface{} {
	schemas := make(map[string]interface{})

	// Get tools for the specified mode
	originalMode := t.service.server.currentMode
	t.service.server.currentMode = mode
	tools := t.service.GetAvailableTools()
	t.service.server.currentMode = originalMode

	// Build schemas for each tool
	for _, tool := range tools {
		schemas[tool.Name] = tool.InputSchema
	}

	return schemas
}

func (t *toolSchemaProvider) GetToolDefinitions() []ToolDefinition {
	return t.service.GetAvailableTools()
}
