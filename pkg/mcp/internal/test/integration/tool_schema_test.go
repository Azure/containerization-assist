package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// TestToolSchemaIntegration validates tool schema compliance through MCP protocol
func TestToolSchemaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping schema integration tests in short mode")
	}
	// Setup real MCP client
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Get tools via MCP protocol
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, tools, "Server should expose tools")

	// Validate each tool has proper schema
	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			validateToolSchema(t, tool)
		})
	}
}

// TestToolDescriptionSessionManagement validates that tool descriptions contain session management instructions
func TestToolDescriptionSessionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping session management tests in short mode")
	}
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	// Tools that require session management
	sessionTools := []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"generate_manifests",
		"scan_image",
	}

	for _, toolName := range sessionTools {
		t.Run(toolName, func(t *testing.T) {
			tool := findTool(tools, toolName)
			require.NotNil(t, tool, "Tool %s should exist", toolName)

			// Validate session management instructions in description
			desc := tool.Description
			assert.Contains(t, desc, "session", "Tool description should mention session management")

			// Validate session_id parameter if tool supports it
			if toolName != "analyze_repository" { // analyze_repository creates session
				params, ok := tool.Parameters["properties"].(map[string]interface{})
				require.True(t, ok, "Tool should have parameters")

				sessionParam, exists := params["session_id"]
				assert.True(t, exists, "Tool %s should have session_id parameter", toolName)

				if exists {
					sessionParamObj, ok := sessionParam.(map[string]interface{})
					require.True(t, ok, "session_id should be parameter object")

					// Validate session_id is required
					assert.Equal(t, "string", sessionParamObj["type"], "session_id should be string type")
					assert.NotEmpty(t, sessionParamObj["description"], "session_id should have description")
				}
			}
		})
	}
}

// TestToolParameterSchemaValidation validates parameter schemas match RichError types from BETA
func TestToolParameterSchemaValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping parameter schema validation tests in short mode")
	}
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	for _, tool := range tools {
		t.Run(tool.Name+"_parameters", func(t *testing.T) {
			// Validate parameters structure
			params, ok := tool.Parameters["properties"].(map[string]interface{})
			if !ok {
				t.Skip("Tool has no parameters")
				return
			}

			// Validate each parameter has proper type information
			for paramName, paramDef := range params {
				paramObj, ok := paramDef.(map[string]interface{})
				require.True(t, ok, "Parameter %s should be object", paramName)

				// Validate required fields
				assert.NotEmpty(t, paramObj["type"], "Parameter %s should have type", paramName)
				assert.NotEmpty(t, paramObj["description"], "Parameter %s should have description", paramName)

				// Validate type consistency (should match RichError-compatible types)
				paramType, ok := paramObj["type"].(string)
				require.True(t, ok, "Parameter type should be string")

				validTypes := []string{"string", "number", "integer", "boolean", "array", "object"}
				assert.Contains(t, validTypes, paramType, "Parameter %s has invalid type %s", paramName, paramType)
			}

			// Validate required parameters are marked
			if required, exists := tool.Parameters["required"].([]interface{}); exists {
				for _, reqField := range required {
					reqFieldStr, ok := reqField.(string)
					require.True(t, ok, "Required field should be string")

					_, paramExists := params[reqFieldStr]
					assert.True(t, paramExists, "Required parameter %s should exist in properties", reqFieldStr)
				}
			}
		})
	}
}

// TestToolDiscoveryThroughMCP validates tool discovery through MCP protocol
func TestToolDiscoveryThroughMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping tool discovery tests in short mode")
	}
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test multiple discovery calls return consistent results
	tools1, err := client.ListTools(ctx)
	require.NoError(t, err)

	tools2, err := client.ListTools(ctx)
	require.NoError(t, err)

	assert.Equal(t, len(tools1), len(tools2), "Tool discovery should be consistent")

	// Validate core tools are always present
	expectedCoreTools := []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"generate_manifests",
	}

	toolNames := make([]string, len(tools1))
	for i, tool := range tools1 {
		toolNames[i] = tool.Name
	}

	for _, expected := range expectedCoreTools {
		assert.Contains(t, toolNames, expected, "Core tool %s should be discoverable", expected)
	}
}

// TestToolSchemaRichErrorIntegration validates integration with BETA's RichError system
func TestToolSchemaRichErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rich error integration tests in short mode")
	}
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test tool call with invalid parameters to trigger RichError
	invalidArgs := map[string]interface{}{
		"invalid_param": "invalid_value",
	}

	result, err := client.CallTool(ctx, "analyze_repository", invalidArgs)

	// Should get error response with RichError structure
	if err != nil {
		// Error should contain structured information
		assert.Contains(t, err.Error(), "validation", "Error should mention validation")
	} else {
		// Or result should contain error information
		if errorInfo, exists := result["error"]; exists {
			errorObj, ok := errorInfo.(map[string]interface{})
			require.True(t, ok, "Error should be structured object")

			// Validate RichError structure
			assert.NotEmpty(t, errorObj["message"], "Error should have message")
			assert.NotEmpty(t, errorObj["code"], "Error should have code")

			// Validate context information from RichError
			if context, exists := errorObj["context"]; exists {
				contextObj, ok := context.(map[string]interface{})
				require.True(t, ok, "Context should be object")
				assert.NotEmpty(t, contextObj, "Context should contain information")
			}
		}
	}
}

// Helper functions

// setupMCPTestEnvironment creates a real MCP test environment
func setupMCPTestEnvironment(t *testing.T) (testutil.MCPTestClient, *testutil.TestServer, func()) {
	server, err := testutil.NewTestServer()
	require.NoError(t, err)

	client, err := testutil.NewMCPTestClient(server.URL())
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
		server.Close()
	}

	return client, server, cleanup
}

// validateToolSchema validates individual tool schema
func validateToolSchema(t *testing.T, tool testutil.ToolInfo) {
	// Validate required fields
	assert.NotEmpty(t, tool.Name, "Tool should have name")
	assert.NotEmpty(t, tool.Description, "Tool should have description")
	assert.NotNil(t, tool.Parameters, "Tool should have parameters")

	// Validate parameters structure
	if params, ok := tool.Parameters["properties"]; ok {
		paramsObj, ok := params.(map[string]interface{})
		assert.True(t, ok, "Parameters properties should be object")
		assert.NotEmpty(t, paramsObj, "Tool should have at least one parameter")
	}

	// Validate tool naming convention
	assert.Regexp(t, `^[a-z][a-z0-9_]*$`, tool.Name, "Tool name should follow naming convention")
}

// findTool finds a tool by name in the tools list
func findTool(tools []testutil.ToolInfo, name string) *testutil.ToolInfo {
	for _, tool := range tools {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}
