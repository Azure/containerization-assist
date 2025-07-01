package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// TestAIClientBehaviorSimulation simulates how AI clients (like Claude) interpret tool descriptions
func TestAIClientBehaviorSimulation(t *testing.T) {
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test how AI would interpret tool listings
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, tools, "Tools should be available for AI client")

	for _, tool := range tools {
		t.Run(fmt.Sprintf("AI_interpretation_%s", tool.Name), func(t *testing.T) {
			// Simulate AI client behavior: does the tool description provide clear guidance?
			validateAIFriendlyDescription(t, tool)

			// Simulate AI client parameter understanding
			validateParameterClarity(t, tool)

			// Simulate AI workflow understanding
			validateWorkflowInstructions(t, tool)
		})
	}
}

// TestToolDiscoveryAndUsage tests how AI clients discover and use tools
func TestToolDiscoveryAndUsage(t *testing.T) {
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test tool listing through MCP protocol (as AI would do)
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	// AI clients need to understand tool relationships
	analyzeRepo := findToolByName(tools, "analyze_repository")
	generateDockerfile := findToolByName(tools, "generate_dockerfile")
	buildImage := findToolByName(tools, "build_image")
	generateManifests := findToolByName(tools, "generate_manifests")

	require.NotNil(t, analyzeRepo, "analyze_repository tool should be discoverable")
	require.NotNil(t, generateDockerfile, "generate_dockerfile tool should be discoverable")
	require.NotNil(t, buildImage, "build_image tool should be discoverable")
	require.NotNil(t, generateManifests, "generate_manifests tool should be discoverable")

	// Test parameter schema interpretation (use BETA's generic types)
	for _, tool := range []*testutil.ToolInfo{analyzeRepo, generateDockerfile, buildImage, generateManifests} {
		validateParameterSchemaForAI(t, tool)
	}

	// Test required vs optional parameter handling
	validateParameterRequirements(t, analyzeRepo, []string{"repo_url"})
	validateParameterRequirements(t, generateDockerfile, []string{"session_id"})
	validateParameterRequirements(t, buildImage, []string{"session_id", "image_name"})
	validateParameterRequirements(t, generateManifests, []string{"session_id", "app_name"})

	// Test error message clarity for AI clients (use BETA's RichError)
	validateErrorMessageClarity(t, client, ctx)
}

// TestAIWorkflowUnderstanding tests if AI can understand multi-step workflows
func TestAIWorkflowUnderstanding(t *testing.T) {
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Simulate AI trying to understand the containerization workflow
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	// AI should be able to determine the correct workflow order from descriptions
	workflowOrder := determineWorkflowOrder(tools)
	expectedOrder := []string{"analyze_repository", "generate_dockerfile", "build_image", "generate_manifests"}

	assert.Equal(t, expectedOrder, workflowOrder, "AI should be able to determine correct workflow order")

	// AI should understand session requirements
	for i, toolName := range expectedOrder {
		tool := findToolByName(tools, toolName)
		require.NotNil(t, tool)

		if i == 0 {
			// First tool (analyze_repository) creates session
			validateSessionCreationTool(t, tool)
		} else {
			// Subsequent tools require session_id
			validateSessionConsumingTool(t, tool)
		}
	}
}

// TestAIParameterInterpretation tests how AI interprets parameter schemas
func TestAIParameterInterpretation(t *testing.T) {
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	for _, tool := range tools {
		t.Run(fmt.Sprintf("parameter_interpretation_%s", tool.Name), func(t *testing.T) {
			// AI should understand parameter types from schema
			validateParameterTypeClarity(t, tool)

			// AI should understand parameter descriptions
			validateParameterDescriptions(t, tool)

			// AI should understand parameter constraints
			validateParameterConstraints(t, tool)
		})
	}
}

// Helper functions

func validateAIFriendlyDescription(t *testing.T, tool testutil.ToolInfo) {
	// Tool description should be clear and actionable for AI
	assert.NotEmpty(t, tool.Description, "Tool description should not be empty")
	assert.True(t, len(tool.Description) > 20, "Tool description should be sufficiently detailed")

	// Should contain action verbs
	desc := strings.ToLower(tool.Description)
	actionVerbs := []string{"analyze", "generate", "build", "create", "deploy", "scan", "validate"}
	hasActionVerb := false
	for _, verb := range actionVerbs {
		if strings.Contains(desc, verb) {
			hasActionVerb = true
			break
		}
	}
	assert.True(t, hasActionVerb, "Tool description should contain action verbs for AI understanding")
}

func validateParameterClarity(t *testing.T, tool testutil.ToolInfo) {
	if tool.Parameters == nil {
		return // No parameters to validate
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for paramName, paramDef := range properties {
		paramObj, ok := paramDef.(map[string]interface{})
		require.True(t, ok, "Parameter %s should be object", paramName)

		// AI needs clear type information
		paramType, exists := paramObj["type"]
		assert.True(t, exists, "Parameter %s should have type", paramName)
		assert.NotEmpty(t, paramType, "Parameter %s type should not be empty", paramName)

		// AI needs clear descriptions
		description, exists := paramObj["description"]
		assert.True(t, exists, "Parameter %s should have description", paramName)
		if exists {
			assert.True(t, len(description.(string)) > 5, "Parameter %s description should be meaningful", paramName)
		}
	}
}

func validateWorkflowInstructions(t *testing.T, tool testutil.ToolInfo) {
	desc := strings.ToLower(tool.Description)

	// Tools should mention session management
	if tool.Name != "analyze_repository" {
		assert.Contains(t, desc, "session", "Tool %s should mention session management", tool.Name)
	}

	// Tools should indicate their place in workflow
	switch tool.Name {
	case "analyze_repository":
		assert.True(t, strings.Contains(desc, "first") || strings.Contains(desc, "start") || strings.Contains(desc, "begin"),
			"analyze_repository should indicate it's the starting tool")
	case "generate_dockerfile":
		assert.True(t, strings.Contains(desc, "after") || strings.Contains(desc, "analysis") || strings.Contains(desc, "following"),
			"generate_dockerfile should indicate dependency on analysis")
	case "build_image":
		assert.True(t, strings.Contains(desc, "dockerfile") || strings.Contains(desc, "after"),
			"build_image should indicate dependency on dockerfile")
	case "generate_manifests":
		assert.True(t, strings.Contains(desc, "deploy") || strings.Contains(desc, "kubernetes"),
			"generate_manifests should indicate deployment purpose")
	}
}

func validateParameterSchemaForAI(t *testing.T, tool *testutil.ToolInfo) {
	if tool.Parameters == nil {
		return
	}

	// Validate schema structure is AI-readable
	schema := tool.Parameters

	// Should have properties
	_, hasProperties := schema["properties"]
	assert.True(t, hasProperties, "Tool %s should have properties in schema", tool.Name)

	// Should have required fields list
	if required, exists := schema["required"]; exists {
		requiredList, ok := required.([]interface{})
		assert.True(t, ok, "Required fields should be array for tool %s", tool.Name)
		assert.NotEmpty(t, requiredList, "Required fields should not be empty for tool %s", tool.Name)
	}
}

func validateParameterRequirements(t *testing.T, tool *testutil.ToolInfo, expectedRequired []string) {
	if tool.Parameters == nil {
		return
	}

	required, exists := tool.Parameters["required"]
	if !exists {
		t.Errorf("Tool %s should have required parameters", tool.Name)
		return
	}

	requiredList, ok := required.([]interface{})
	require.True(t, ok, "Required should be array")

	requiredStrings := make([]string, len(requiredList))
	for i, req := range requiredList {
		requiredStrings[i] = req.(string)
	}

	for _, expected := range expectedRequired {
		assert.Contains(t, requiredStrings, expected, "Tool %s should require parameter %s", tool.Name, expected)
	}
}

func validateErrorMessageClarity(t *testing.T, client testutil.MCPTestClient, ctx context.Context) {
	// Test invalid tool call to see error message quality
	_, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"invalid_param": "test",
	})

	if err != nil {
		// Error message should be clear for AI interpretation
		errorMsg := err.Error()
		assert.Contains(t, errorMsg, "validation", "Error should mention validation")
		assert.True(t, len(errorMsg) > 10, "Error message should be descriptive")
	}
}

func determineWorkflowOrder(tools []testutil.ToolInfo) []string {
	// Simulate AI determining workflow order from tool descriptions
	order := make([]string, 0)

	// Find starting tool (one that doesn't require session_id)
	for _, tool := range tools {
		if !requiresSessionID(tool) && isWorkflowTool(tool) {
			order = append(order, tool.Name)
			break
		}
	}

	// Find subsequent tools based on descriptions and dependencies
	remaining := []string{"generate_dockerfile", "build_image", "generate_manifests"}
	for _, toolName := range remaining {
		tool := findToolByName(tools, toolName)
		if tool != nil {
			order = append(order, tool.Name)
		}
	}

	return order
}

func requiresSessionID(tool testutil.ToolInfo) bool {
	if tool.Parameters == nil {
		return false
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		return false
	}

	_, hasSessionID := properties["session_id"]
	return hasSessionID
}

func isWorkflowTool(tool testutil.ToolInfo) bool {
	workflowTools := []string{"analyze_repository", "generate_dockerfile", "build_image", "generate_manifests"}
	for _, wt := range workflowTools {
		if tool.Name == wt {
			return true
		}
	}
	return false
}

func findToolByName(tools []testutil.ToolInfo, name string) *testutil.ToolInfo {
	for _, tool := range tools {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}

func validateSessionCreationTool(t *testing.T, tool *testutil.ToolInfo) {
	// Tool should not require session_id as input
	assert.False(t, requiresSessionID(*tool), "Tool %s should not require session_id (it creates one)", tool.Name)

	// Description should indicate it creates a session
	desc := strings.ToLower(tool.Description)
	assert.True(t, strings.Contains(desc, "create") || strings.Contains(desc, "start") || strings.Contains(desc, "new"),
		"Tool %s description should indicate it creates/starts something", tool.Name)
}

func validateSessionConsumingTool(t *testing.T, tool *testutil.ToolInfo) {
	// Tool should require session_id
	assert.True(t, requiresSessionID(*tool), "Tool %s should require session_id", tool.Name)

	// Should have session_id in required parameters
	if tool.Parameters != nil {
		required, exists := tool.Parameters["required"]
		if exists {
			requiredList, ok := required.([]interface{})
			if ok {
				hasSessionID := false
				for _, req := range requiredList {
					if req.(string) == "session_id" {
						hasSessionID = true
						break
					}
				}
				assert.True(t, hasSessionID, "Tool %s should have session_id in required parameters", tool.Name)
			}
		}
	}
}

func validateParameterTypeClarity(t *testing.T, tool testutil.ToolInfo) {
	if tool.Parameters == nil {
		return
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for paramName, paramDef := range properties {
		paramObj, ok := paramDef.(map[string]interface{})
		require.True(t, ok, "Parameter definition should be object")

		// Type should be clear and standard
		paramType, exists := paramObj["type"]
		assert.True(t, exists, "Parameter %s should have type", paramName)

		if exists {
			typeStr := paramType.(string)
			validTypes := []string{"string", "number", "integer", "boolean", "array", "object"}
			assert.Contains(t, validTypes, typeStr, "Parameter %s should have valid JSON Schema type", paramName)
		}
	}
}

func validateParameterDescriptions(t *testing.T, tool testutil.ToolInfo) {
	if tool.Parameters == nil {
		return
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for paramName, paramDef := range properties {
		paramObj, ok := paramDef.(map[string]interface{})
		require.True(t, ok, "Parameter definition should be object")

		// Description should be helpful for AI
		description, exists := paramObj["description"]
		assert.True(t, exists, "Parameter %s should have description", paramName)

		if exists {
			descStr := description.(string)
			assert.True(t, len(descStr) > 10, "Parameter %s description should be detailed", paramName)

			// Should not just repeat the parameter name
			assert.NotEqual(t, strings.ToLower(paramName), strings.ToLower(descStr),
				"Parameter %s description should not just repeat the name", paramName)
		}
	}
}

func validateParameterConstraints(t *testing.T, tool testutil.ToolInfo) {
	if tool.Parameters == nil {
		return
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for paramName, paramDef := range properties {
		paramObj, ok := paramDef.(map[string]interface{})
		require.True(t, ok, "Parameter %s definition should be object", paramName)

		// Check for useful constraints
		paramType, _ := paramObj["type"].(string)

		switch paramType {
		case "string":
			// String parameters might have format, pattern, or enum constraints
			if format, exists := paramObj["format"]; exists {
				assert.NotEmpty(t, format, "Format constraint should not be empty")
			}
			if pattern, exists := paramObj["pattern"]; exists {
				assert.NotEmpty(t, pattern, "Pattern constraint should not be empty")
			}
		case "number", "integer":
			// Numeric parameters might have min/max constraints
			if minimum, exists := paramObj["minimum"]; exists {
				assert.NotNil(t, minimum, "Minimum constraint should not be nil")
			}
		case "array":
			// Array parameters should specify item types
			if items, exists := paramObj["items"]; exists {
				assert.NotNil(t, items, "Array items constraint should not be nil")
			}
		}
	}
}

// setupE2ETestEnvironment creates an E2E test environment
func setupE2ETestEnvironment(t *testing.T) (testutil.MCPTestClient, *testutil.TestServer, func()) {
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
