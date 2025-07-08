package registry

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// mockTool implements api.Tool for testing
type mockTool struct {
	name        string
	description string
	schema      api.ToolSchema
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "mock executed"},
	}, nil
}

func (m *mockTool) Schema() api.ToolSchema {
	return m.schema
}

func newMockTool(name, description string) *mockTool {
	return &mockTool{
		name:        name,
		description: description,
		schema: api.ToolSchema{
			Version:  "1.0.0",
			Category: api.ToolCategory("test"),
			Tags:     []string{"test", "mock"},
		},
	}
}

func TestRegistryBasicOperations(t *testing.T) {
	r := New()

	// Test empty registry
	if tools := r.List(); len(tools) != 0 {
		t.Errorf("Expected empty registry, got %d tools", len(tools))
	}

	// Test registering a tool
	tool := newMockTool("test-tool", "A test tool")
	err := r.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Test tool exists
	tools := r.List()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
	if tools[0] != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got '%s'", tools[0])
	}

	// Test getting tool
	retrievedTool, err := r.Get("test-tool")
	if err != nil {
		t.Fatalf("Failed to get tool: %v", err)
	}
	if retrievedTool.Name() != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got '%s'", retrievedTool.Name())
	}

	// Test unregistering tool
	err = r.Unregister("test-tool")
	if err != nil {
		t.Fatalf("Failed to unregister tool: %v", err)
	}

	// Test tool is gone
	tools = r.List()
	if len(tools) != 0 {
		t.Errorf("Expected empty registry after unregister, got %d tools", len(tools))
	}
}

func TestRegistryOptions(t *testing.T) {
	r := New(WithMaxTools(5), WithMetrics(false), WithNamespace("test"))

	stats := r.Stats()
	if stats.MaxTools != 5 {
		t.Errorf("Expected max tools 5, got %d", stats.MaxTools)
	}
	if stats.Namespace != "test" {
		t.Errorf("Expected namespace 'test', got '%s'", stats.Namespace)
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := New()
	tool := newMockTool("test-tool", "A test tool")

	// Register tool
	err := r.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Try to register same tool again
	err = r.Register(tool)
	if err == nil {
		t.Error("Expected error when registering duplicate tool")
	}
}

func TestRegistryMaxToolsLimit(t *testing.T) {
	r := New(WithMaxTools(2))

	// Register 2 tools successfully
	tool1 := newMockTool("tool1", "Tool 1")
	tool2 := newMockTool("tool2", "Tool 2")

	err := r.Register(tool1)
	if err != nil {
		t.Fatalf("Failed to register tool1: %v", err)
	}

	err = r.Register(tool2)
	if err != nil {
		t.Fatalf("Failed to register tool2: %v", err)
	}

	// Try to register a third tool - should fail
	tool3 := newMockTool("tool3", "Tool 3")
	err = r.Register(tool3)
	if err == nil {
		t.Error("Expected error when exceeding max tools limit")
	}
}

func TestRegistryExecute(t *testing.T) {
	r := New()
	tool := newMockTool("test-tool", "A test tool")

	err := r.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Execute tool
	input := api.ToolInput{
		SessionID: "test-session",
		Data:      map[string]interface{}{"param": "value"},
	}

	result, err := r.Execute(context.Background(), "test-tool", input)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful execution")
	}

	expectedResult := "mock executed"
	if result.Data["result"] != expectedResult {
		t.Errorf("Expected result '%s', got '%v'", expectedResult, result.Data["result"])
	}
}

func TestRegistryExecuteWithRetry(t *testing.T) {
	r := New()
	tool := newMockTool("test-tool", "A test tool")

	err := r.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Execute tool with retry policy
	input := api.ToolInput{
		SessionID: "test-session",
		Data:      map[string]interface{}{"param": "value"},
	}

	policy := api.RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	result, err := r.ExecuteWithRetry(context.Background(), "test-tool", input, policy)
	if err != nil {
		t.Fatalf("Failed to execute tool with retry: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful execution")
	}
}

func TestRegistryGetMetadata(t *testing.T) {
	r := New()
	tool := newMockTool("test-tool", "A test tool")

	err := r.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	metadata, err := r.GetMetadata("test-tool")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if metadata.Name != "test-tool" {
		t.Errorf("Expected name 'test-tool', got '%s'", metadata.Name)
	}
	if metadata.Description != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", metadata.Description)
	}
	if metadata.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", metadata.Version)
	}
}

func TestRegistryListByCategory(t *testing.T) {
	r := New()

	tool1 := newMockTool("tool1", "Tool 1")
	tool1.schema.Category = api.ToolCategory("category1")

	tool2 := newMockTool("tool2", "Tool 2")
	tool2.schema.Category = api.ToolCategory("category2")

	tool3 := newMockTool("tool3", "Tool 3")
	tool3.schema.Category = api.ToolCategory("category1")

	r.Register(tool1)
	r.Register(tool2)
	r.Register(tool3)

	// Test listing by category
	category1Tools := r.ListByCategory(api.ToolCategory("category1"))
	if len(category1Tools) != 2 {
		t.Errorf("Expected 2 tools in category1, got %d", len(category1Tools))
	}

	category2Tools := r.ListByCategory(api.ToolCategory("category2"))
	if len(category2Tools) != 1 {
		t.Errorf("Expected 1 tool in category2, got %d", len(category2Tools))
	}
}

func TestRegistryListByTags(t *testing.T) {
	r := New()

	tool1 := newMockTool("tool1", "Tool 1")
	tool1.schema.Tags = []string{"tag1", "tag2"}

	tool2 := newMockTool("tool2", "Tool 2")
	tool2.schema.Tags = []string{"tag2", "tag3"}

	tool3 := newMockTool("tool3", "Tool 3")
	tool3.schema.Tags = []string{"tag4"}

	r.Register(tool1)
	r.Register(tool2)
	r.Register(tool3)

	// Test listing by tags
	tag1Tools := r.ListByTags("tag1")
	if len(tag1Tools) != 1 {
		t.Errorf("Expected 1 tool with tag1, got %d", len(tag1Tools))
	}

	tag2Tools := r.ListByTags("tag2")
	if len(tag2Tools) != 2 {
		t.Errorf("Expected 2 tools with tag2, got %d", len(tag2Tools))
	}

	multiTagTools := r.ListByTags("tag1", "tag3")
	if len(multiTagTools) != 2 {
		t.Errorf("Expected 2 tools with tag1 or tag3, got %d", len(multiTagTools))
	}
}

func TestRegistryCompatibilityAliases(t *testing.T) {
	// Test that type aliases work
	var _ *Registry = NewTypedToolRegistry()
	var _ *Registry = NewFederatedRegistry()
	var _ *Registry = NewToolRegistry()
	var _ *Registry = NewMemoryRegistry()
	var _ *Registry = NewMemoryToolRegistry()

	// Test services.ToolRegistry interface compatibility
	r := New()
	tool := newMockTool("test-tool", "A test tool")

	// Test RegisterTool method
	err := r.RegisterTool(tool)
	if err != nil {
		t.Fatalf("Failed to register tool via RegisterTool: %v", err)
	}

	// Test GetTool method
	retrievedTool, err := r.GetTool("test-tool")
	if err != nil {
		t.Fatalf("Failed to get tool via GetTool: %v", err)
	}
	if retrievedTool.Name() != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got '%s'", retrievedTool.Name())
	}

	// Test ListTools method
	tools := r.ListTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool via ListTools, got %d", len(tools))
	}

	// Test UnregisterTool method
	err = r.UnregisterTool("test-tool")
	if err != nil {
		t.Fatalf("Failed to unregister tool via UnregisterTool: %v", err)
	}

	// Verify tool is gone
	tools = r.ListTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty registry after unregister, got %d tools", len(tools))
	}
}