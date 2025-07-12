package application

import (
	"github.com/Azure/container-kit/pkg/ai"
)

// ChatModeConfig defines the configuration for a custom chat mode
type ChatModeConfig struct {
	Mode        string   `json:"mode"`
	Description string   `json:"description"`
	Functions   []string `json:"functions"`
}

// RegisterChatModes registers custom chat modes for Copilot integration
func (s *serverImpl) RegisterChatModes() error {
	// Note: gomcp doesn't have ChatMode support yet, so this is prepared for future use
	// When available, it would look something like:
	//
	// s.gomcpServer.ChatMode("copilot", ChatModeConfig{
	//     Mode:        "custom",
	//     Description: "Enhanced Copilot experience with file tools",
	//     Functions:   GetChatModeFunctions(),
	// })
	
	s.logger.Info("Chat mode registration prepared for future gomcp support",
		"mode", "copilot",
		"description", "Enhanced Copilot experience with file tools")
	
	return nil
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"read_file",
		"list_directory",
		"file_exists",
		"containerize_and_deploy",
	}
}

// ConvertAIToolsToMCP converts Azure OpenAI tool definitions to MCP format
// This will be used when gomcp supports function calling in chat modes
func ConvertAIToolsToMCP() []ToolDefinition {
	aiTools := ai.GetFileSystemTools()
	mcpTools := make([]ToolDefinition, 0, len(aiTools))
	
	// When gomcp supports this, we'll convert the tools here
	// For now, just log that we're ready
	_ = aiTools
	
	return mcpTools
}

// ToolDefinition represents an MCP tool definition
// This is a placeholder until gomcp provides the actual type
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}