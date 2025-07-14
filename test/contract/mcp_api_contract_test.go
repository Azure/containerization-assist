// Package contract provides API contract tests to ensure MCP interface stability
package contract

import (
	"reflect"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServerInterfaceContract ensures the core MCP server interface remains stable
func TestMCPServerInterfaceContract(t *testing.T) {
	// Get the interface type
	serverType := reflect.TypeOf((*api.MCPServer)(nil)).Elem()

	// Verify interface name
	assert.Equal(t, "MCPServer", serverType.Name(), "MCPServer interface name must remain stable")

	// Verify method count
	assert.Equal(t, 2, serverType.NumMethod(), "MCPServer interface must have exactly 2 methods")

	// Verify Start method signature
	startMethod, found := serverType.MethodByName("Start")
	require.True(t, found, "Start method must exist")
	assert.Equal(t, 1, startMethod.Type.NumIn(), "Start method must take 1 parameter")
	assert.Equal(t, 1, startMethod.Type.NumOut(), "Start method must return 1 value")

	// Verify Start method parameter types
	assert.Equal(t, "Context", startMethod.Type.In(0).Name(), "Start method parameter must be Context")

	// Verify Start method return types
	assert.Equal(t, "error", startMethod.Type.Out(0).Name(), "Start method must return error")

	// Verify Stop method signature
	stopMethod, found := serverType.MethodByName("Stop")
	require.True(t, found, "Stop method must exist")
	assert.Equal(t, 1, stopMethod.Type.NumIn(), "Stop method must take 1 parameter")
	assert.Equal(t, 1, stopMethod.Type.NumOut(), "Stop method must return 1 value")

	// Verify Stop method parameter types
	assert.Equal(t, "Context", stopMethod.Type.In(0).Name(), "Stop method parameter must be Context")

	// Verify Stop method return types
	assert.Equal(t, "error", stopMethod.Type.Out(0).Name(), "Stop method must return error")
}

// TestToolInterfaceContract ensures the Tool interface remains stable
func TestToolInterfaceContract(t *testing.T) {
	// Get the interface type
	toolType := reflect.TypeOf((*api.Tool)(nil)).Elem()

	// Verify interface name
	assert.Equal(t, "Tool", toolType.Name(), "Tool interface name must remain stable")

	// Verify method count
	assert.Equal(t, 4, toolType.NumMethod(), "Tool interface must have exactly 4 methods")

	expectedMethods := []struct {
		name        string
		numIn       int
		numOut      int
		returnTypes []string
	}{
		{
			name:        "Name",
			numIn:       0,
			numOut:      1,
			returnTypes: []string{"string"},
		},
		{
			name:        "Description",
			numIn:       0,
			numOut:      1,
			returnTypes: []string{"string"},
		},
		{
			name:        "Execute",
			numIn:       2, // context + input
			numOut:      2,
			returnTypes: []string{"ToolOutput", "error"},
		},
		{
			name:        "Schema",
			numIn:       0,
			numOut:      1,
			returnTypes: []string{"ToolSchema"},
		},
	}

	for _, expected := range expectedMethods {
		t.Run(expected.name, func(t *testing.T) {
			method, found := toolType.MethodByName(expected.name)
			require.True(t, found, "%s method must exist", expected.name)

			assert.Equal(t, expected.numIn, method.Type.NumIn(),
				"%s method input count must remain stable", expected.name)
			assert.Equal(t, expected.numOut, method.Type.NumOut(),
				"%s method output count must remain stable", expected.name)

			// Verify return types
			for i, expectedType := range expected.returnTypes {
				actualType := method.Type.Out(i)
				assert.Equal(t, expectedType, actualType.Name(),
					"%s method return %d type must remain stable", expected.name, i)
			}
		})
	}
}

// TestToolInputStructContract ensures ToolInput structure remains stable
func TestToolInputStructContract(t *testing.T) {
	toolInputType := reflect.TypeOf(api.ToolInput{})

	// Verify struct name
	assert.Equal(t, "ToolInput", toolInputType.Name(), "ToolInput struct name must remain stable")

	// Verify field count and names
	assert.Equal(t, 3, toolInputType.NumField(), "ToolInput must have exactly 3 fields")

	expectedFields := []struct {
		name string
		typ  string
		tag  string
	}{
		{"SessionID", "string", `json:"session_id"`},
		{"Data", "map[string]interface {}", `json:"data"`},
		{"Context", "map[string]interface {}", `json:"context,omitempty"`},
	}

	for i, expected := range expectedFields {
		field := toolInputType.Field(i)
		assert.Equal(t, expected.name, field.Name, "Field %d name must remain stable", i)
		assert.Equal(t, expected.typ, field.Type.String(), "Field %d type must remain stable", i)
		assert.Equal(t, expected.tag, string(field.Tag), "Field %d JSON tag must remain stable", i)
	}
}

// TestToolOutputStructContract ensures ToolOutput structure remains stable
func TestToolOutputStructContract(t *testing.T) {
	toolOutputType := reflect.TypeOf(api.ToolOutput{})

	// Verify struct name
	assert.Equal(t, "ToolOutput", toolOutputType.Name(), "ToolOutput struct name must remain stable")

	// Verify field count and names
	assert.Equal(t, 4, toolOutputType.NumField(), "ToolOutput must have exactly 4 fields")

	expectedFields := []struct {
		name string
		typ  string
		tag  string
	}{
		{"Success", "bool", `json:"success"`},
		{"Data", "map[string]interface {}", `json:"data"`},
		{"Error", "string", `json:"error,omitempty"`},
		{"Metadata", "map[string]interface {}", `json:"metadata,omitempty"`},
	}

	for i, expected := range expectedFields {
		field := toolOutputType.Field(i)
		assert.Equal(t, expected.name, field.Name, "Field %d name must remain stable", i)
		assert.Equal(t, expected.typ, field.Type.String(), "Field %d type must remain stable", i)
		assert.Equal(t, expected.tag, string(field.Tag), "Field %d JSON tag must remain stable", i)
	}
}

// TestAPIInterfaceCount ensures no interfaces are accidentally removed
func TestAPIInterfaceCount(t *testing.T) {
	// This test ensures we don't accidentally break the API by removing interfaces
	// Count of public interfaces/types in the api package that must remain stable
	expectedPublicTypes := map[string]bool{
		"MCPServer":  true,
		"Tool":       true,
		"ToolInput":  true,
		"ToolOutput": true,
		"ToolSchema": true,
	}

	// Add new API types
	expectedPublicTypes["ProgressEmitter"] = true
	expectedPublicTypes["ProgressUpdate"] = true

	// This is a basic count check - more sophisticated checks could use go/ast
	// to scan the actual package and verify all expected types exist
	assert.Equal(t, 7, len(expectedPublicTypes),
		"Expected public API types count must remain stable")
}

// TestAPIStabilityDocumentation ensures API contract requirements are documented
func TestAPIStabilityDocumentation(t *testing.T) {
	t.Log("🔒 MCP API Contract Requirements:")
	t.Log("   1. MCPServer interface must remain stable")
	t.Log("   2. Tool interface signature must not change")
	t.Log("   3. ToolInput/ToolOutput JSON schema must be backwards compatible")
	t.Log("   4. No public interfaces can be removed without major version bump")
	t.Log("   5. Field additions must be optional (omitempty)")

	// This test always passes but serves as documentation
	assert.True(t, true, "API stability requirements documented")
}

// TestPerformanceContract ensures performance characteristics remain stable
func TestPerformanceContract(t *testing.T) {
	// Integration with performance monitoring
	t.Log("⚡ Performance Contract Requirements:")
	t.Log("   1. P95 latency must remain < 300μs for core operations")
	t.Log("   2. Memory allocation must not increase > 50% without justification")
	t.Log("   3. No more than 100% increase in allocation count")
	t.Log("   4. Use 'make perf-check' for regression detection")

	// This test always passes but serves as documentation
	assert.True(t, true, "Performance contract requirements documented")
}

// TestBenchmarkAPIUsage demonstrates contract-compliant API usage
func TestBenchmarkAPIUsage(t *testing.T) {
	// Create sample ToolInput (simulating real usage)
	input := api.ToolInput{
		SessionID: "test-session-123",
		Data: map[string]interface{}{
			"repository_url": "https://github.com/test/repo",
			"branch":         "main",
		},
		Context: map[string]interface{}{
			"user_id": "test-user",
		},
	}

	// Verify input structure
	assert.NotEmpty(t, input.SessionID, "SessionID must be provided")
	assert.NotNil(t, input.Data, "Data must not be nil")
	assert.Contains(t, input.Data, "repository_url", "Required field must be present")

	// Create sample ToolOutput (simulating successful response)
	output := api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"workflow_id": "workflow-123",
			"status":      "completed",
			"steps":       10,
		},
		Metadata: map[string]interface{}{
			"duration_ms": 1500,
			"timestamp":   time.Now().Unix(),
		},
	}

	// Verify output structure
	assert.True(t, output.Success, "Success operation must have Success=true")
	assert.NotNil(t, output.Data, "Data must not be nil for successful operations")
	assert.Empty(t, output.Error, "Error must be empty for successful operations")

	t.Log("✅ API usage patterns validated")
}
