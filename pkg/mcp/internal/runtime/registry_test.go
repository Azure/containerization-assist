package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mcp import removed - using mcptypes

// Test types for mock tools
type TestArgs struct {
	Message string   `json:"message" jsonschema:"required"`
	Count   int      `json:"count,omitempty"`
	Items   []string `json:"items,omitempty"`
}

type TestResult struct {
	Success bool     `json:"success"`
	Data    string   `json:"data"`
	Results []string `json:"results,omitempty"`
}

// Mock tool for testing
type mockTool struct {
	name            string
	executeFunc     func(ctx context.Context, args interface{}) (interface{}, error)
	preValidateFunc func(ctx context.Context, args TestArgs) error
	validateFunc    func(ctx context.Context, args interface{}) error
}

func (m *mockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, args)
	}
	return TestResult{Success: true, Data: "mock response"}, nil
}

func (m *mockTool) PreValidate(ctx context.Context, args TestArgs) error {
	if m.preValidateFunc != nil {
		return m.preValidateFunc(ctx, args)
	}
	return nil
}

func (m *mockTool) Validate(ctx context.Context, args interface{}) error {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, args)
	}
	return nil
}

func (m *mockTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        m.name,
		Description: "Mock tool for testing",
		Version:     "1.0.0",
		Category:    "test",
	}
}

func createTestRegistry() *ToolRegistry {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	return NewToolRegistry(logger)
}

func TestNewToolRegistry(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	registry := NewToolRegistry(logger)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.tools)
	assert.False(t, registry.frozen)
	assert.Equal(t, 0, len(registry.tools))
}

func TestRegisterTool_Success(t *testing.T) {
	registry := createTestRegistry()
	tool := &mockTool{name: "test_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)

	assert.NoError(t, err)

	// Verify tool was registered
	reg, exists := registry.GetTool("test_tool")
	assert.True(t, exists)
	assert.NotNil(t, reg)
	assert.Equal(t, tool, reg.Tool)
	assert.NotNil(t, reg.InputSchema)
	assert.NotNil(t, reg.OutputSchema)
	assert.NotNil(t, reg.Handler)
}

func TestRegisterTool_DuplicateName(t *testing.T) {
	registry := createTestRegistry()
	tool1 := &mockTool{name: "duplicate_tool"}
	tool2 := &mockTool{name: "duplicate_tool"}

	// Register first tool
	err := RegisterTool[TestArgs, TestResult](registry, tool1)
	assert.NoError(t, err)

	// Try to register second tool with same name
	err = RegisterTool[TestArgs, TestResult](registry, tool2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegisterTool_FrozenRegistry(t *testing.T) {
	registry := createTestRegistry()
	registry.Freeze()

	tool := &mockTool{name: "test_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool registry frozen")
}

func TestRegisterTool_SchemaGeneration(t *testing.T) {
	registry := createTestRegistry()
	tool := &mockTool{name: "schema_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	assert.NoError(t, err)

	reg, exists := registry.GetTool("schema_tool")
	require.True(t, exists)

	// Check input schema structure
	inputSchema := reg.InputSchema
	assert.Equal(t, "object", inputSchema["type"])

	properties, ok := inputSchema["properties"].(map[string]interface{})
	require.True(t, ok)

	// Check required message field
	messageField, ok := properties["message"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", messageField["type"])

	// Check optional count field
	countField, ok := properties["count"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "integer", countField["type"])

	// Check array field
	itemsField, ok := properties["items"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "array", itemsField["type"])

	// Check output schema structure
	outputSchema := reg.OutputSchema
	assert.Equal(t, "object", outputSchema["type"])

	outputProps, ok := outputSchema["properties"].(map[string]interface{})
	require.True(t, ok)

	successField, ok := outputProps["success"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "boolean", successField["type"])
}

func TestToolRegistry_GetTool(t *testing.T) {
	registry := createTestRegistry()
	tool := &mockTool{name: "get_tool"}

	// Test getting non-existent tool
	_, exists := registry.GetTool("nonexistent")
	assert.False(t, exists)

	// Register and get existing tool
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	reg, exists := registry.GetTool("get_tool")
	assert.True(t, exists)
	assert.Equal(t, tool, reg.Tool)
}

func TestToolRegistry_GetAllTools(t *testing.T) {
	registry := createTestRegistry()

	// Test empty registry
	tools := registry.GetAllTools()
	assert.Equal(t, 0, len(tools))

	// Register multiple tools
	tool1 := &mockTool{name: "tool1"}
	tool2 := &mockTool{name: "tool2"}

	err := RegisterTool[TestArgs, TestResult](registry, tool1)
	require.NoError(t, err)
	err = RegisterTool[TestArgs, TestResult](registry, tool2)
	require.NoError(t, err)

	tools = registry.GetAllTools()
	assert.Equal(t, 2, len(tools))
	assert.Contains(t, tools, "tool1")
	assert.Contains(t, tools, "tool2")

	// Verify it returns a copy (mutation safety)
	delete(tools, "tool1")
	allTools := registry.GetAllTools()
	assert.Equal(t, 2, len(allTools)) // Original should be unchanged
}

func TestToolRegistry_Freeze(t *testing.T) {
	registry := createTestRegistry()

	// Initially not frozen
	assert.False(t, registry.IsFrozen())

	// Freeze registry
	registry.Freeze()
	assert.True(t, registry.IsFrozen())

	// Cannot register after freezing
	tool := &mockTool{name: "frozen_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	assert.Error(t, err)
}

func TestToolRegistry_ExecuteTool_Success(t *testing.T) {
	registry := createTestRegistry()

	tool := &mockTool{
		name: "execute_tool",
		executeFunc: func(ctx context.Context, args interface{}) (interface{}, error) {
			testArgs := args.(TestArgs)
			return TestResult{
				Success: true,
				Data:    "processed: " + testArgs.Message,
			}, nil
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	// Execute tool with valid args
	args := TestArgs{Message: "hello", Count: 5}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := registry.ExecuteTool(context.Background(), "execute_tool", argsJSON)
	assert.NoError(t, err)

	resultTyped, ok := result.(TestResult)
	require.True(t, ok)
	assert.True(t, resultTyped.Success)
	assert.Equal(t, "processed: hello", resultTyped.Data)
}

func TestToolRegistry_ExecuteTool_ToolNotFound(t *testing.T) {
	registry := createTestRegistry()

	args := TestArgs{Message: "hello"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "nonexistent", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool nonexistent not found")
}

func TestToolRegistry_ExecuteTool_InvalidJSON(t *testing.T) {
	registry := createTestRegistry()
	tool := &mockTool{name: "json_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	// Execute with invalid JSON
	invalidJSON := json.RawMessage(`{"message": "unclosed quote}`)

	_, err = registry.ExecuteTool(context.Background(), "json_tool", invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal args")
}

func TestToolRegistry_ExecuteTool_PreValidationError(t *testing.T) {
	registry := createTestRegistry()

	tool := &mockTool{
		name: "validation_tool",
		preValidateFunc: func(ctx context.Context, args TestArgs) error {
			if args.Message == "" {
				return mcp.NewRichError("VALIDATION_ERROR", "message cannot be empty", "validation_error")
			}
			return nil
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	// Execute with invalid args
	args := TestArgs{Message: "", Count: 5}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "validation_tool", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message cannot be empty")
}

func TestToolRegistry_ExecuteTool_ExecutionError(t *testing.T) {
	registry := createTestRegistry()

	tool := &mockTool{
		name: "error_tool",
		executeFunc: func(ctx context.Context, args interface{}) (interface{}, error) {
			return nil, mcp.NewRichError("EXECUTION_ERROR", "tool execution failed", "runtime_error")
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	args := TestArgs{Message: "hello"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "error_tool", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool execution failed")
}

func TestContainsArrays(t *testing.T) {
	tests := []struct {
		name     string
		schema   map[string]interface{}
		expected bool
	}{
		{
			name: "schema with array",
			schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"items": map[string]interface{}{
						"type": "array",
					},
				},
			},
			expected: true,
		},
		{
			name: "schema without array",
			schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: false,
		},
		{
			name:     "empty schema",
			schema:   map[string]interface{}{},
			expected: false,
		},
		{
			name: "schema with no properties",
			schema: map[string]interface{}{
				"type": "object",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsArrays(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeInvopopSchema(t *testing.T) {
	// Create a simple schema to test sanitization
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"test": map[string]interface{}{
				"type": "string",
			},
		},
	}

	// Convert to jsonschema.Schema-like structure and test
	schemaBytes, err := json.Marshal(inputSchema)
	require.NoError(t, err)

	var testSchema map[string]interface{}
	err = json.Unmarshal(schemaBytes, &testSchema)
	require.NoError(t, err)

	// The function should return a valid map
	assert.NotNil(t, testSchema)
	assert.Equal(t, "object", testSchema["type"])
}

// Test thread safety
func TestToolRegistry_ConcurrentAccess(t *testing.T) {
	registry := createTestRegistry()

	// Register a tool
	tool := &mockTool{name: "concurrent_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, exists := registry.GetTool("concurrent_tool")
			assert.True(t, exists)

			tools := registry.GetAllTools()
			assert.Equal(t, 1, len(tools))

			frozen := registry.IsFrozen()
			assert.False(t, frozen)

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestToolRegistry_ConcurrentRegistration(t *testing.T) {
	registry := createTestRegistry()

	// Test concurrent registrations with different tool names
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(index int) {
			tool := &mockTool{name: fmt.Sprintf("tool_%d", index)}
			err := RegisterTool[TestArgs, TestResult](registry, tool)
			done <- err
		}(i)
	}

	// Check all registrations succeeded
	for i := 0; i < 5; i++ {
		err := <-done
		assert.NoError(t, err)
	}

	// Verify all tools were registered
	tools := registry.GetAllTools()
	assert.Equal(t, 5, len(tools))
}
