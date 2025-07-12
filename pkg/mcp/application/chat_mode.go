package application

// ChatModeConfig defines the configuration for a custom chat mode
type ChatModeConfig struct {
	Mode        string   `json:"mode"`
	Description string   `json:"description"`
	Functions   []string `json:"functions"`
}

// RegisterChatModes registers custom chat modes for Copilot integration
func (s *serverImpl) RegisterChatModes() error {
	s.logger.Info("Chat mode support enabled via standard MCP protocol",
		"available_tools", GetChatModeFunctions())

	return nil
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"containerize_and_deploy",
	}
}

// ConvertWorkflowToolsToChat converts workflow tools to chat-compatible format
// This provides a mapping of available MCP tools for chat interfaces
func ConvertWorkflowToolsToChat() []ToolDefinition {
	// Return the tools that are available for chat mode
	// These correspond to the actual MCP tools registered by the server
	return []ToolDefinition{
		{
			Name:        "containerize_and_deploy",
			Description: "Complete containerization and deployment workflow",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_url": map[string]interface{}{
						"type":        "string",
						"description": "Repository URL to containerize",
					},
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Git branch to use (optional)",
					},
				},
				"required": []string{"repo_url"},
			},
		},
	}
}

// ToolDefinition represents an MCP tool definition
// Using our own type for chat mode compatibility with mcp-go
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
