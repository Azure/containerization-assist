package server

import (
	"context"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestServerMode_Constants(t *testing.T) {
	assert.Equal(t, ServerMode("dual"), ModeDual)
	assert.Equal(t, ServerMode("chat"), ModeChat)
	assert.Equal(t, ServerMode("workflow"), ModeWorkflow)
}

func TestServerCapabilities_Structure(t *testing.T) {
	caps := ServerCapabilities{
		ChatSupport:     true,
		WorkflowSupport: false,
		AvailableModes:  []string{"chat"},
		SharedTools:     []string{"tool1", "tool2"},
	}

	assert.True(t, caps.ChatSupport)
	assert.False(t, caps.WorkflowSupport)
	assert.Len(t, caps.AvailableModes, 1)
	assert.Len(t, caps.SharedTools, 2)
}

func TestUnifiedMCPServer_GetCapabilities(t *testing.T) {
	// Test the capabilities logic with real registries
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("chat mode capabilities", func(t *testing.T) {
		registry := orchestration.NewMCPToolRegistry(logger)
		server := &UnifiedMCPServer{
			currentMode:  ModeChat,
			toolRegistry: registry,
		}

		capabilities := server.GetCapabilities()

		assert.True(t, capabilities.ChatSupport)
		assert.False(t, capabilities.WorkflowSupport)
		assert.Equal(t, []string{"chat"}, capabilities.AvailableModes)
		// SharedTools can be nil for empty registry, which is acceptable
		assert.True(t, len(capabilities.SharedTools) == 0) // Either nil or empty slice
	})

	t.Run("workflow mode capabilities", func(t *testing.T) {
		registry := orchestration.NewMCPToolRegistry(logger)
		server := &UnifiedMCPServer{
			currentMode:  ModeWorkflow,
			toolRegistry: registry,
		}

		capabilities := server.GetCapabilities()

		assert.False(t, capabilities.ChatSupport)
		assert.True(t, capabilities.WorkflowSupport)
		assert.Equal(t, []string{"workflow"}, capabilities.AvailableModes)
	})

	t.Run("dual mode capabilities", func(t *testing.T) {
		registry := orchestration.NewMCPToolRegistry(logger)
		server := &UnifiedMCPServer{
			currentMode:  ModeDual,
			toolRegistry: registry,
		}

		capabilities := server.GetCapabilities()

		assert.True(t, capabilities.ChatSupport)
		assert.True(t, capabilities.WorkflowSupport)
		assert.Equal(t, []string{"chat", "workflow"}, capabilities.AvailableModes)
	})
}

func TestUnifiedMCPServer_getChatModeTools(t *testing.T) {
	server := &UnifiedMCPServer{
		currentMode: ModeChat,
		logger:      zerolog.New(nil).Level(zerolog.Disabled),
	}

	tools := server.getChatModeTools()

	assert.Len(t, tools, 2)

	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	assert.Contains(t, toolNames, "chat")
	assert.Contains(t, toolNames, "list_conversation_history")

	// Check tool structure
	chatTool := tools[0]
	assert.Equal(t, "chat", chatTool.Name)
	assert.NotEmpty(t, chatTool.Description)
	assert.NotNil(t, chatTool.InputSchema)

	// Check input schema structure
	schema := chatTool.InputSchema
	assert.Equal(t, "object", schema["type"])
	assert.NotNil(t, schema["properties"])
	assert.NotNil(t, schema["required"])
}

func TestUnifiedMCPServer_getWorkflowModeTools(t *testing.T) {
	server := &UnifiedMCPServer{
		currentMode: ModeWorkflow,
		logger:      zerolog.New(nil).Level(zerolog.Disabled),
	}

	tools := server.getWorkflowModeTools()

	assert.GreaterOrEqual(t, len(tools), 3) // At least execute_workflow, list_workflows, get_workflow_status

	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	assert.Contains(t, toolNames, "execute_workflow")
	assert.Contains(t, toolNames, "list_workflows")
	assert.Contains(t, toolNames, "get_workflow_status")

	// Check tool structure
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}
}

func TestUnifiedMCPServer_isAtomicTool(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	registry := orchestration.NewMCPToolRegistry(logger)

	server := &UnifiedMCPServer{
		toolRegistry: registry,
	}

	// Test with empty registry
	assert.False(t, server.isAtomicTool("any_tool"))
	assert.False(t, server.isAtomicTool("chat"))
	assert.False(t, server.isAtomicTool("unknown_tool"))

	// Note: To properly test with registered tools, we'd need to register actual tools
	// which is complex in this test environment
}

func TestUnifiedMCPServer_ExecuteTool_Validation(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	registry := orchestration.NewMCPToolRegistry(logger)
	server := &UnifiedMCPServer{
		currentMode:  ModeChat,
		logger:       logger,
		toolRegistry: registry,
	}

	tests := []struct {
		name          string
		toolName      string
		mode          ServerMode
		expectError   bool
		errorContains string
	}{
		{
			name:          "chat in workflow mode",
			toolName:      "chat",
			mode:          ModeWorkflow,
			expectError:   true,
			errorContains: "chat mode not available",
		},
		{
			name:          "workflow in chat mode",
			toolName:      "execute_workflow",
			mode:          ModeChat,
			expectError:   true,
			errorContains: "workflow mode not available",
		},
		{
			name:          "unknown tool",
			toolName:      "unknown_tool",
			mode:          ModeDual,
			expectError:   true,
			errorContains: "unknown tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.currentMode = tt.mode
			_, err := server.ExecuteTool(context.Background(), tt.toolName, map[string]interface{}{})

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnifiedMCPServer_buildInputSchema(t *testing.T) {
	server := &UnifiedMCPServer{
		logger: zerolog.New(nil).Level(zerolog.Disabled),
	}

	metadata := &orchestration.ToolMetadata{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"fields": map[string]interface{}{
				"test_param": map[string]interface{}{
					"type":        "string",
					"description": "A test parameter",
				},
			},
		},
	}

	schema := server.buildInputSchema(metadata)

	assert.Equal(t, "object", schema["type"])
	assert.NotNil(t, schema["properties"])
	assert.NotNil(t, schema["required"])

	properties := schema["properties"].(map[string]interface{})
	assert.Contains(t, properties, "session_id")
	assert.Contains(t, properties, "test_param")

	required := schema["required"].([]string)
	assert.Contains(t, required, "session_id")
}

func TestToolDefinition_Structure(t *testing.T) {
	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	assert.Equal(t, "test_tool", tool.Name)
	assert.Equal(t, "A test tool", tool.Description)
	assert.NotNil(t, tool.InputSchema)
}

func TestConvertParametersMapToString(t *testing.T) {
	input := map[string]interface{}{
		"string_param": "hello",
		"int_param":    42,
		"bool_param":   true,
	}

	result := convertParametersMapToString(input)

	assert.Equal(t, "hello", result["string_param"])
	assert.Equal(t, "42", result["int_param"])
	assert.Equal(t, "true", result["bool_param"])
}

func TestConvertExamplesToTypes(t *testing.T) {
	input := []orchestration.ToolExample{
		{
			Name:        "test_example",
			Description: "A test example",
			Input:       map[string]interface{}{"param": "value"},
			Output:      map[string]interface{}{"result": "success"},
		},
	}

	result := convertExamplesToTypes(input)

	assert.Len(t, result, 1)
	assert.Equal(t, "test_example", result[0].Name)
	assert.Equal(t, "A test example", result[0].Description)
	assert.NotNil(t, result[0].Input)
	assert.NotNil(t, result[0].Output)
}

func TestConvertToMapStringInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "valid map",
			input:    map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
		},
		{
			name:     "invalid input",
			input:    "not a map",
			expected: map[string]interface{}{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToMapStringInterface(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegistryAdapter_Basic(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	registry := orchestration.NewMCPToolRegistry(logger)

	adapter := &RegistryAdapter{
		registry: registry,
	}

	// Test basic structure
	assert.NotNil(t, adapter.registry)

	// Test List (should be empty initially)
	tools := adapter.List()
	assert.Empty(t, tools) // More flexible than checking specific length

	// Test Exists
	assert.False(t, adapter.Exists("nonexistent_tool"))

	// Test GetMetadata
	metadata := adapter.GetMetadata()
	assert.NotNil(t, metadata)
	assert.IsType(t, map[string]core.ToolMetadata{}, metadata)
}

// Note: Mock registries removed as they were incompatible with the concrete types expected

// Simple test for DirectSessionManager structure
func TestDirectSessionManager_Structure(t *testing.T) {
	// Create a minimal test to verify the structure exists
	// without requiring complex session manager setup

	// Test that the types exist and can be created
	dsm := &directSessionManager{
		sessionManager: nil, // Would be a real session manager in practice
	}

	assert.NotNil(t, dsm)
	// Cannot test methods without real session manager due to complexity
}

// ConversationOrchestratorAdapter test removed - adapter eliminated in interface simplification
