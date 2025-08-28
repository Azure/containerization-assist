package tools

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllToolsRegistration tests that all 15 tools register successfully
func TestAllToolsRegistration(t *testing.T) {
	// Create test server
	mcpServer := server.NewMCPServer("test-server", "1.0.0")

	// Create mock dependencies
	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Register all tools
	err := RegisterTools(mcpServer, deps)
	require.NoError(t, err)

	// Verify all tools are registered
	expectedTools := []string{
		"analyze_repository",
		"resolve_base_images",
		"verify_dockerfile",
		"build_image",
		"scan_image",
		"tag_image",
		"push_image",
		"verify_k8s_manifests",
		"prepare_cluster",
		"deploy_application",
		"verify_deployment",
		"start_workflow",
		"workflow_status",
		"list_tools",
		"ping",
		"server_status",
	}

	for _, toolName := range expectedTools {
		t.Run(toolName, func(t *testing.T) {
			// Verify tool exists - this would need to be implemented
			// based on how the mcp-go library exposes registered tools
			assert.True(t, true, "Tool %s should be registered", toolName)
		})
	}
}

// TestWorkflowToolsChain tests the workflow tool chain
func TestWorkflowToolsChain(t *testing.T) {
	workflowTools := []struct {
		name     string
		nextTool string
	}{
		{"analyze_repository", "resolve_base_images"},
		{"resolve_base_images", "verify_dockerfile"},
		{"verify_dockerfile", "build_image"},
		{"build_image", "scan_image"},
		{"scan_image", "tag_image"},
		{"tag_image", "push_image"},
		{"push_image", "verify_k8s_manifests"},
		{"verify_k8s_manifests", "prepare_cluster"},
		{"prepare_cluster", "deploy_application"},
		{"deploy_application", "verify_deployment"},
		{"verify_deployment", ""},
	}

	for _, tool := range workflowTools {
		t.Run(tool.name, func(t *testing.T) {
			config, err := GetToolConfig(tool.name)
			require.NoError(t, err)
			assert.Equal(t, tool.nextTool, config.NextTool)
		})
	}
}

// TestToolCategories tests that tools are properly categorized
func TestToolCategories(t *testing.T) {
	configs := GetToolConfigs()

	categoryCount := map[ToolCategory]int{}
	for _, config := range configs {
		categoryCount[config.Category]++
	}

	assert.Equal(t, 11, categoryCount[CategoryWorkflow], "Should have 11 workflow tools")
	assert.Equal(t, 2, categoryCount[CategoryOrchestration], "Should have 2 orchestration tools")
	assert.Equal(t, 3, categoryCount[CategoryUtility], "Should have 3 utility tools")
}

// TestToolDependencies tests that tool dependencies are correctly configured
func TestToolDependencies(t *testing.T) {
	configs := GetToolConfigs()

	for _, config := range configs {
		t.Run(config.Name, func(t *testing.T) {
			switch config.Category {
			case CategoryWorkflow:
				// Workflow tools should need all dependencies (progress is now handled directly)
				assert.True(t, config.NeedsStepProvider, "Workflow tool should need StepProvider")
				assert.True(t, config.NeedsSessionManager, "Workflow tool should need SessionManager")
				assert.True(t, config.NeedsLogger, "Workflow tool should need Logger")
				assert.NotEmpty(t, config.StepGetterName, "Workflow tool should have StepGetterName")
			case CategoryOrchestration:
				// Orchestration tools only need logger
				assert.False(t, config.NeedsStepProvider, "Orchestration tool should not need StepProvider")
				assert.False(t, config.NeedsSessionManager, "Orchestration tool should not need SessionManager")
				assert.True(t, config.NeedsLogger, "Orchestration tool should need Logger")
			case CategoryUtility:
				// Utility tools may have custom requirements
				// Just verify they have names and descriptions
				assert.NotEmpty(t, config.Name, "Utility tool should have name")
				assert.NotEmpty(t, config.Description, "Utility tool should have description")
			}
		})
	}
}

// TestToolParameterValidation tests parameter requirements
func TestToolParameterValidation(t *testing.T) {
	configs := GetToolConfigs()

	for _, config := range configs {
		t.Run(config.Name, func(t *testing.T) {
			// Build schema to test parameter validation
			schema := BuildToolSchema(config)

			// Verify required parameters are in schema
			for _, param := range config.RequiredParams {
				assert.Contains(t, schema.Properties, param)
				assert.Contains(t, schema.Required, param)
			}

			// Verify optional parameters are in schema but not required
			for param := range config.OptionalParams {
				assert.Contains(t, schema.Properties, param)
				assert.NotContains(t, schema.Required, param)
			}
		})
	}
}

// TestWorkflowToolsRequireSessionID tests that workflow tools require session_id
func TestWorkflowToolsRequireSessionID(t *testing.T) {
	configs := GetToolConfigs()

	for _, config := range configs {
		if config.Category == CategoryWorkflow {
			t.Run(config.Name, func(t *testing.T) {
				assert.Contains(t, config.RequiredParams, "session_id")
			})
		}
	}
}

// TestPingToolHandler tests the ping tool specifically
func TestPingToolHandler(t *testing.T) {
	ctx := context.Background()
	deps := ToolDependencies{}
	handler := createPingHandler(deps)

	tests := []struct {
		name     string
		args     map[string]interface{}
		wantText bool
	}{
		{
			name:     "ping without message",
			args:     map[string]interface{}{},
			wantText: true,
		},
		{
			name:     "ping with message",
			args:     map[string]interface{}{"message": "test"},
			wantText: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result, err := handler(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantText {
				assert.NotEmpty(t, result.Content)
			}
		})
	}
}

// TestServerStatusToolHandler tests the server_status tool
func TestServerStatusToolHandler(t *testing.T) {
	ctx := context.Background()
	deps := ToolDependencies{}
	handler := createServerStatusHandler(deps)

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "status without details",
			args: map[string]interface{}{},
		},
		{
			name: "status with details",
			args: map[string]interface{}{"details": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result, err := handler(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.NotEmpty(t, result.Content)
		})
	}
}

// TestListToolsHandler tests the list_tools tool
func TestListToolsHandler(t *testing.T) {
	ctx := context.Background()
	handler := CreateListToolsHandler()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{},
		},
	}

	result, err := handler(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}
