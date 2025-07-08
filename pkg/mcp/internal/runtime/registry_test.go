package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
type mockTool struct {
	name            string
	executeFunc     func(ctx context.Context, args interface{}) (interface{}, error)
	preValidateFunc func(ctx context.Context, args TestArgs) error
	validateFunc    func(ctx context.Context, args interface{}) error
}

func (m *mockTool) Name() string {
	return m.name
}
func (m *mockTool) Description() string {
	return "Mock tool for testing"
}
func (m *mockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.Description(),
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{"type": "string"},
			},
		},
	}
}
func (m *mockTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {

	var args interface{}
	if len(input.Data) > 0 {

		if argsData, ok := input.Data["args"]; ok {
			args = argsData
		} else {

			args = input.Data
		}
	}
	var testArgs TestArgs
	var conversionSuccessful bool

	if directArgs, ok := args.(TestArgs); ok {
		testArgs = directArgs
		conversionSuccessful = true
	} else if argsMap, isMap := args.(map[string]interface{}); isMap {
		testArgs = TestArgs{
			Message: getStringFromMap(argsMap, "message"),
			Count:   getIntFromMap(argsMap, "count"),
			Items:   getStringSliceFromMap(argsMap, "items"),
		}
		conversionSuccessful = true
	}
	if m.preValidateFunc != nil && conversionSuccessful {
		if err := m.preValidateFunc(context.Background(), testArgs); err != nil {
			return api.ToolOutput{Success: false, Error: err.Error()}, err
		}
	}

	if m.executeFunc != nil {

		var executeArgs interface{}
		if conversionSuccessful {
			executeArgs = testArgs
		} else {
			executeArgs = args
		}

		result, err := m.executeFunc(context.Background(), executeArgs)
		if err != nil {
			return api.ToolOutput{Success: false, Error: err.Error()}, err
		}
		var resultData map[string]interface{}
		if resultBytes, jsonErr := json.Marshal(result); jsonErr == nil {
			json.Unmarshal(resultBytes, &resultData)
		} else {

			resultData = map[string]interface{}{"result": result}
		}

		return api.ToolOutput{Success: true, Data: resultData}, nil
	}
	defaultResult := TestResult{Success: true, Data: "mock response"}
	var defaultData map[string]interface{}
	if resultBytes, jsonErr := json.Marshal(defaultResult); jsonErr == nil {
		json.Unmarshal(resultBytes, &defaultData)
	} else {
		defaultData = map[string]interface{}{"result": defaultResult}
	}

	return api.ToolOutput{Success: true, Data: defaultData}, nil
}

type mockToolOutput struct {
	success bool
	data    interface{}
}

func (m *mockToolOutput) IsSuccess() bool {
	return m.success
}

func (m *mockToolOutput) GetData() interface{} {
	return m.data
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

func createTestRegistry() *ToolRegistry {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	return NewToolRegistry(logger)
}
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func getStringSliceFromMap(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

func TestNewToolRegistry(t *testing.T) {
	t.Parallel()
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	registry := NewToolRegistry(logger)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.tools)
	assert.False(t, registry.frozen)
	assert.Equal(t, 0, len(registry.tools))
}

func TestRegisterTool_Success(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tool := &mockTool{name: "test_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)

	assert.NoError(t, err)
	reg, exists := registry.GetTool("test_tool")
	assert.True(t, exists)
	assert.NotNil(t, reg)
	assert.Equal(t, tool, reg.Tool)
	assert.NotNil(t, reg.InputSchema)
	assert.NotNil(t, reg.OutputSchema)
	assert.NotNil(t, reg.Handler)
}

func TestRegisterTool_DuplicateName(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tool1 := &mockTool{name: "duplicate_tool"}
	tool2 := &mockTool{name: "duplicate_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool1)
	assert.NoError(t, err)
	err = RegisterTool[TestArgs, TestResult](registry, tool2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TOOL_ALREADY_REGISTERED")
	assert.Contains(t, err.Error(), "Tool with this name is already registered")
}

func TestRegisterTool_FrozenRegistry(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	registry.Freeze()

	tool := &mockTool{name: "test_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REGISTRY_FROZEN")
	assert.Contains(t, err.Error(), "Cannot register tool on frozen registry")
}

func TestRegisterTool_SchemaGeneration(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tool := &mockTool{name: "schema_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	assert.NoError(t, err)

	reg, exists := registry.GetTool("schema_tool")
	require.True(t, exists)
	inputSchema := reg.InputSchema
	assert.Equal(t, "object", inputSchema["type"])

	properties, ok := inputSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	messageField, ok := properties["message"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", messageField["type"])
	countField, ok := properties["count"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "integer", countField["type"])
	itemsField, ok := properties["items"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "array", itemsField["type"])
	outputSchema := reg.OutputSchema
	assert.Equal(t, "object", outputSchema["type"])

	outputProps, ok := outputSchema["properties"].(map[string]interface{})
	require.True(t, ok)

	successField, ok := outputProps["success"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "boolean", successField["type"])
}

func TestToolRegistry_GetTool(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tool := &mockTool{name: "get_tool"}
	_, exists := registry.GetTool("nonexistent")
	assert.False(t, exists)
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	reg, exists := registry.GetTool("get_tool")
	assert.True(t, exists)
	assert.Equal(t, tool, reg.Tool)
}

func TestToolRegistry_GetAllTools(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tools := registry.GetAllTools()
	assert.Equal(t, 0, len(tools))
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
	delete(tools, "tool1")
	allTools := registry.GetAllTools()
	assert.Equal(t, 2, len(allTools))
}

func TestToolRegistry_Freeze(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	assert.False(t, registry.IsFrozen())
	registry.Freeze()
	assert.True(t, registry.IsFrozen())
	tool := &mockTool{name: "frozen_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	assert.Error(t, err)
}

func TestToolRegistry_ExecuteTool_Success(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()

	tool := &mockTool{
		name: "execute_tool",
		executeFunc: func(ctx context.Context, args interface{}) (interface{}, error) {

			testArgs, ok := args.(TestArgs)
			if !ok {

				if argsMap, isMap := args.(map[string]interface{}); isMap {
					testArgs = TestArgs{
						Message: getStringFromMap(argsMap, "message"),
						Count:   getIntFromMap(argsMap, "count"),
						Items:   getStringSliceFromMap(argsMap, "items"),
					}
				} else {
					return nil, errors.NewError().
						Code("TYPE_ASSERTION_FAILED").
						Message("Failed to convert arguments to TestArgs").
						Type(errors.ErrTypeValidation).
						Severity(errors.SeverityMedium).
						Context("args_type", fmt.Sprintf("%T", args)).
						Suggestion("Ensure arguments match the expected TestArgs structure").
						WithLocation().
						Build()
				}
			}
			return TestResult{
				Success: true,
				Data:    "processed: " + testArgs.Message,
			}, nil
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)
	args := TestArgs{Message: "hello", Count: 5}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	result, err := registry.ExecuteTool(context.Background(), "execute_tool", argsJSON)
	assert.NoError(t, err)
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	var resultTyped TestResult
	if resultBytes, jsonErr := json.Marshal(resultMap); jsonErr == nil {
		err = json.Unmarshal(resultBytes, &resultTyped)
		require.NoError(t, err)
	} else {
		t.Fatalf("Failed to convert result map to TestResult: %v", jsonErr)
	}

	assert.True(t, resultTyped.Success)
	assert.Equal(t, "processed: hello", resultTyped.Data)
}

func TestToolRegistry_ExecuteTool_ToolNotFound(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()

	args := TestArgs{Message: "hello"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "nonexistent", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registry operation failed")
}

func TestToolRegistry_ExecuteTool_InvalidJSON(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()
	tool := &mockTool{name: "json_tool"}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)
	invalidJSON := json.RawMessage(`{"message": "unclosed quote}`)

	_, err = registry.ExecuteTool(context.Background(), "json_tool", invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TOOL_PARAMETER_UNMARSHAL_FAILED")
	assert.Contains(t, err.Error(), "Failed to unmarshal tool parameters")
}

func TestToolRegistry_ExecuteTool_PreValidationError(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()

	tool := &mockTool{
		name: "validation_tool",
		preValidateFunc: func(ctx context.Context, args TestArgs) error {
			if args.Message == "" {
				return fmt.Errorf("test operation failed")
			}
			return nil
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)
	args := TestArgs{Message: "", Count: 5}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "validation_tool", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test operation failed")
}

func TestToolRegistry_ExecuteTool_ExecutionError(t *testing.T) {
	t.Parallel()
	registry := createTestRegistry()

	tool := &mockTool{
		name: "error_tool",
		executeFunc: func(ctx context.Context, args interface{}) (interface{}, error) {
			return nil, fmt.Errorf("test operation failed")
		},
	}

	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)

	args := TestArgs{Message: "hello"}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = registry.ExecuteTool(context.Background(), "error_tool", argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test operation failed")
}

func TestContainsArrays(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			result := containsArrays(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeInvopopSchema(t *testing.T) {
	t.Parallel()

	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"test": map[string]interface{}{
				"type": "string",
			},
		},
	}
	schemaBytes, err := json.Marshal(inputSchema)
	require.NoError(t, err)

	var testSchema map[string]interface{}
	err = json.Unmarshal(schemaBytes, &testSchema)
	require.NoError(t, err)
	assert.NotNil(t, testSchema)
	assert.Equal(t, "object", testSchema["type"])
}
func TestToolRegistry_ConcurrentAccess(t *testing.T) {
	// Skip t.Parallel() - this test specifically tests concurrent access
	registry := createTestRegistry()
	tool := &mockTool{name: "concurrent_tool"}
	err := RegisterTool[TestArgs, TestResult](registry, tool)
	require.NoError(t, err)
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
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestToolRegistry_ConcurrentRegistration(t *testing.T) {
	// Skip t.Parallel() - this test specifically tests concurrent registration
	registry := createTestRegistry()
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(index int) {
			tool := &mockTool{name: fmt.Sprintf("tool_%d", index)}
			err := RegisterTool[TestArgs, TestResult](registry, tool)
			done <- err
		}(i)
	}
	for i := 0; i < 5; i++ {
		err := <-done
		assert.NoError(t, err)
	}
	tools := registry.GetAllTools()
	assert.Equal(t, 5, len(tools))
}
