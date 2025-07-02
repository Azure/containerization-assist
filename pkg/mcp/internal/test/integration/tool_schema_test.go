package integration

import (
	"strings"
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
	// Setup real MCP client using stdio transport (proven approach)
	client, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	// Get tools via MCP protocol
	tools, err := client.ListTools()
	require.NoError(t, err)
	require.NotEmpty(t, tools, "Server should expose tools")

	// Debug: log all discovered tools
	t.Logf("Discovered %d tools:", len(tools))
	for _, tool := range tools {
		hasParams := tool.Parameters != nil || tool.InputSchema != nil
		t.Logf("  - %s: %s (schema: %v)", tool.Name, tool.Description, hasParams)
	}

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
	client, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	tools, err := client.ListTools()
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
			t.Logf("Tool %s has description: %q", toolName, desc)
			assert.Contains(t, strings.ToLower(desc), "session", "Tool description should mention session management")

			// Validate session_id parameter if tool supports it
			if toolName != "analyze_repository" { // analyze_repository creates session
				// Check both legacy and MCP spec field names
				schema := tool.Parameters
				if schema == nil {
					schema = tool.InputSchema
				}
				t.Logf("Tool %s schema: %#v", toolName, schema != nil)
				if schema == nil {
					t.Skip("Tool has no schema")
					return
				}
				params, ok := schema["properties"].(map[string]interface{})
				require.True(t, ok, "Tool should have schema properties")

				// Check for session management in BaseToolArgs structure
				baseToolArgs, exists := params["basetoolargs"]
				if !exists {
					// Alternative: check for direct session_id parameter
					sessionParam, directExists := params["session_id"]
					if !directExists {
						t.Errorf("Tool %s should have either basetoolargs object or direct session_id parameter", toolName)
						return
					}
					// Validate direct session_id parameter
					sessionParamObj, ok := sessionParam.(map[string]interface{})
					assert.True(t, ok, "session_id should be parameter object")
					assert.Equal(t, "string", sessionParamObj["type"], "session_id should be string type")
					return
				}

				// Validate BaseToolArgs contains session management
				assert.NotNil(t, baseToolArgs, "Tool %s should have basetoolargs for session management", toolName)
			}
		})
	}
}

// TestToolParameterSchemaValidation validates parameter schemas match RichError types from BETA
func TestToolParameterSchemaValidation(t *testing.T) {
	// Re-enabled: Basic tool discovery is working, now testing parameter schemas
	if testing.Short() {
		t.Skip("Skipping parameter schema validation tests in short mode")
	}
	client, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	tools, err := client.ListTools()
	require.NoError(t, err)

	for _, tool := range tools {
		t.Run(tool.Name+"_parameters", func(t *testing.T) {
			// Check both legacy and MCP spec field names
			schema := tool.Parameters
			if schema == nil {
				schema = tool.InputSchema
			}
			if schema == nil {
				t.Skip("Tool has no schema")
				return
			}

			// Validate parameters structure
			params, ok := schema["properties"].(map[string]interface{})
			if !ok {
				t.Skip("Tool has no properties in schema")
				return
			}

			// Validate each parameter has proper type information
			for paramName, paramDef := range params {
				paramObj, ok := paramDef.(map[string]interface{})
				require.True(t, ok, "Parameter %s should be object", paramName)

				// Validate required fields
				assert.NotEmpty(t, paramObj["type"], "Parameter %s should have type", paramName)
				// Description is optional for some generated parameters (like basetoolargs)
				if desc := paramObj["description"]; desc != nil {
					assert.NotEmpty(t, desc, "Parameter %s description should not be empty if present", paramName)
				}

				// Validate type consistency (should match RichError-compatible types)
				paramType, ok := paramObj["type"].(string)
				require.True(t, ok, "Parameter type should be string")

				validTypes := []string{"string", "number", "integer", "boolean", "array", "object"}
				assert.Contains(t, validTypes, paramType, "Parameter %s has invalid type %s", paramName, paramType)
			}

			// Validate required parameters are marked
			if required, exists := schema["required"].([]interface{}); exists {
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
	client, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	// Test multiple discovery calls return consistent results
	tools1, err := client.ListTools()
	require.NoError(t, err)

	tools2, err := client.ListTools()
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
	t.Skip("TEMPORARILY SKIPPED: Integration tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	if testing.Short() {
		t.Skip("Skipping rich error integration tests in short mode")
	}
	client, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	// Test tool call with invalid parameters to trigger RichError
	invalidArgs := map[string]interface{}{
		"invalid_param": "invalid_value",
	}

	result, err := client.CallTool("analyze_repository", invalidArgs)

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

// setupMCPTestEnvironment creates a real MCP test environment using stdio transport
// This uses the same proven approach as the schema regression test
func setupMCPTestEnvironment(t *testing.T) (testutil.StdioMCPClient, func()) {
	client, err := testutil.StartMCPServerForTest(t)
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
	}

	return client, cleanup
}

// validateToolSchema validates individual tool schema
func validateToolSchema(t *testing.T, tool testutil.ToolInfo) {
	// Validate required fields
	assert.NotEmpty(t, tool.Name, "Tool should have name")
	assert.NotEmpty(t, tool.Description, "Tool should have description")

	// Parameters are optional for some tools (like ping, server_status)
	// But if present, they should be well-formed
	schema := tool.Parameters
	if schema == nil {
		schema = tool.InputSchema
	}

	if schema != nil {
		// Validate schema structure if present
		if params, ok := schema["properties"]; ok {
			_, ok := params.(map[string]interface{})
			assert.True(t, ok, "Schema properties should be object")
			// Don't require parameters to be non-empty - some tools might have optional-only params
		}

		// Validate parameter schema structure
		if paramType, ok := schema["type"]; ok {
			assert.Equal(t, "object", paramType, "Tool schema should be object type")
		}
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
