package registry

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// MockRegistry implements api.Registry for testing
type MockRegistry struct {
	tools     map[string]api.Tool
	metadata  map[string]api.ToolMetadata
	metrics   api.RegistryMetrics
	callbacks map[api.RegistryEventType][]api.RegistryEventCallback
}

// NewMockRegistry creates a new mock registry
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		tools:     make(map[string]api.Tool),
		metadata:  make(map[string]api.ToolMetadata),
		callbacks: make(map[api.RegistryEventType][]api.RegistryEventCallback),
	}
}

// MockTool implements api.Tool for testing
type MockTool struct {
	name        string
	description string
}

func (t *MockTool) Name() string {
	if t.name != "" {
		return t.name
	}
	return "mock_tool"
}

func (t *MockTool) Description() string {
	if t.description != "" {
		return t.description
	}
	return "Mock tool for testing"
}

func (t *MockTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"executed": true,
			"tool":     t.Name(),
		},
	}, nil
}

func (t *MockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
	}
}

// Register implements api.Registry
func (m *MockRegistry) Register(tool api.Tool, _ ...api.RegistryOption) error {
	m.tools[tool.Name()] = tool
	m.metadata[tool.Name()] = api.ToolMetadata{
		Name:         tool.Name(),
		Description:  tool.Description(),
		RegisteredAt: time.Now(),
	}
	return nil
}

// Unregister implements api.Registry
func (m *MockRegistry) Unregister(name string) error {
	delete(m.tools, name)
	delete(m.metadata, name)
	return nil
}

// Get implements api.Registry
func (m *MockRegistry) Get(name string) (api.Tool, error) {
	if tool, exists := m.tools[name]; exists {
		return tool, nil
	}
	return nil, api.ErrorInvalidInput
}

// List implements api.Registry
func (m *MockRegistry) List() []string {
	names := make([]string, 0, len(m.tools))
	for name := range m.tools {
		names = append(names, name)
	}
	return names
}

// ListByCategory implements api.Registry
func (m *MockRegistry) ListByCategory(category api.ToolCategory) []string {
	var names []string
	for name, tool := range m.tools {
		if tool.Schema().Category == category {
			names = append(names, name)
		}
	}
	return names
}

// ListByTags implements api.Registry
func (m *MockRegistry) ListByTags(tags ...string) []string {
	var names []string
	for name, tool := range m.tools {
		schema := tool.Schema()
		for _, tag := range tags {
			for _, schemaTag := range schema.Tags {
				if schemaTag == tag {
					names = append(names, name)
					break
				}
			}
		}
	}
	return names
}

// Execute implements api.Registry
func (m *MockRegistry) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	tool, err := m.Get(name)
	if err != nil {
		return api.ToolOutput{}, err
	}
	return tool.Execute(ctx, input)
}

// ExecuteWithRetry implements api.Registry
func (m *MockRegistry) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, _ api.RetryPolicy) (api.ToolOutput, error) {
	// Simple implementation without retry logic for testing
	return m.Execute(ctx, name, input)
}

// GetMetadata implements api.Registry
func (m *MockRegistry) GetMetadata(name string) (api.ToolMetadata, error) {
	if metadata, exists := m.metadata[name]; exists {
		return metadata, nil
	}
	return api.ToolMetadata{}, api.ErrorInvalidInput
}

// GetStatus implements api.Registry
func (m *MockRegistry) GetStatus(name string) (api.ToolStatus, error) {
	if _, exists := m.tools[name]; exists {
		return "active", nil
	}
	return "not_found", api.ErrorInvalidInput
}

// SetStatus implements api.Registry
func (m *MockRegistry) SetStatus(name string, _ api.ToolStatus) error {
	if _, exists := m.tools[name]; exists {
		return nil
	}
	return api.ErrorInvalidInput
}

// Close implements api.Registry
func (m *MockRegistry) Close() error {
	return nil
}

// GetMetrics implements api.Registry
func (m *MockRegistry) GetMetrics() api.RegistryMetrics {
	return m.metrics
}

// Subscribe implements api.Registry
func (m *MockRegistry) Subscribe(event api.RegistryEventType, callback api.RegistryEventCallback) error {
	m.callbacks[event] = append(m.callbacks[event], callback)
	return nil
}

// Unsubscribe implements api.Registry (placeholder)
func (m *MockRegistry) Unsubscribe(_ api.RegistryEventType, _ api.RegistryEventCallback) error {
	// Simple implementation for testing
	return nil
}

// Test functions

func TestAutoRegistrar_RegisterCategory(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Create test tools
	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
		"build": func() (api.Tool, error) {
			return &MockTool{name: "containerization_build"}, nil
		},
	}

	// Register category
	err := autoReg.RegisterCategory("containerization", testTools)
	if err != nil {
		t.Fatalf("Failed to register category: %v", err)
	}

	// Verify tools are registered
	registeredTools := autoReg.GetRegisteredToolNames()
	expectedTools := []string{"containerization_analyze", "containerization_build"}

	if len(registeredTools) != len(expectedTools) {
		t.Fatalf("Expected %d tools, got %d", len(expectedTools), len(registeredTools))
	}

	for _, expected := range expectedTools {
		found := false
		for _, registered := range registeredTools {
			if registered == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found in registered tools", expected)
		}
	}
}

func TestAutoRegistrar_MigrateAllTools(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Register test tools
	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
	}

	err := autoReg.RegisterCategory("containerization", testTools)
	if err != nil {
		t.Fatalf("Failed to register category: %v", err)
	}

	// Migrate tools
	ctx := context.Background()
	err = autoReg.MigrateAllTools(ctx)
	if err != nil {
		t.Fatalf("Failed to migrate tools: %v", err)
	}

	// Verify tools are in the registry
	registryTools := mockRegistry.List()
	if len(registryTools) != 1 {
		t.Fatalf("Expected 1 tool in registry, got %d", len(registryTools))
	}

	if registryTools[0] != "containerization_analyze" {
		t.Errorf("Expected tool containerization_analyze, got %s", registryTools[0])
	}
}

func TestAutoRegistrar_CreateTool(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Register test tool
	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
	}

	err := autoReg.RegisterCategory("containerization", testTools)
	if err != nil {
		t.Fatalf("Failed to register category: %v", err)
	}

	// Create tool
	tool, err := autoReg.CreateTool("containerization_analyze")
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	if tool.Name() != "containerization_analyze" {
		t.Errorf("Expected tool name containerization_analyze, got %s", tool.Name())
	}
}

func TestAutoRegistrar_GetToolSchema(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Register test tool
	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
	}

	err := autoReg.RegisterCategory("containerization", testTools)
	if err != nil {
		t.Fatalf("Failed to register category: %v", err)
	}

	// Get schema
	schema, err := autoReg.GetToolSchema("containerization_analyze")
	if err != nil {
		t.Fatalf("Failed to get tool schema: %v", err)
	}

	if schema.Name != "containerization_analyze" {
		t.Errorf("Expected schema name containerization_analyze, got %s", schema.Name)
	}

	if schema.Category != "containerization" {
		t.Errorf("Expected schema category containerization, got %s", schema.Category)
	}
}

func TestAutoRegistrar_ListCategories(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Register multiple categories
	containerTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
	}

	sessionTools := map[string]api.ToolCreator{
		"create": func() (api.Tool, error) {
			return &MockTool{name: "session_create"}, nil
		},
	}

	err := autoReg.RegisterCategory("containerization", containerTools)
	if err != nil {
		t.Fatalf("Failed to register containerization category: %v", err)
	}

	err = autoReg.RegisterCategory("session", sessionTools)
	if err != nil {
		t.Fatalf("Failed to register session category: %v", err)
	}

	// List categories
	categories := autoReg.ListCategories()
	expectedCategories := []string{"containerization", "session"}

	if len(categories) != len(expectedCategories) {
		t.Fatalf("Expected %d categories, got %d", len(expectedCategories), len(categories))
	}

	for _, expected := range expectedCategories {
		found := false
		for _, category := range categories {
			if category == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected category %s not found", expected)
		}
	}
}

func TestAutoRegistrar_GetToolsInCategory(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Register category with multiple tools
	containerTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
		"build": func() (api.Tool, error) {
			return &MockTool{name: "containerization_build"}, nil
		},
	}

	err := autoReg.RegisterCategory("containerization", containerTools)
	if err != nil {
		t.Fatalf("Failed to register category: %v", err)
	}

	// Get tools in category
	tools := autoReg.GetToolsInCategory("containerization")
	expectedTools := []string{"analyze", "build"}

	if len(tools) != len(expectedTools) {
		t.Fatalf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	for _, expected := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found in category", expected)
		}
	}
}

func TestAutoRegistrar_ConcurrentAccess(t *testing.T) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	// Test concurrent registration
	done := make(chan bool, 2)

	go func() {
		containerTools := map[string]api.ToolCreator{
			"analyze": func() (api.Tool, error) {
				return &MockTool{name: "containerization_analyze"}, nil
			},
		}
		if err := autoReg.RegisterCategory("containerization", containerTools); err != nil {
			t.Errorf("Failed to register containerization tools: %v", err)
		}
		done <- true
	}()

	go func() {
		sessionTools := map[string]api.ToolCreator{
			"create": func() (api.Tool, error) {
				return &MockTool{name: "session_create"}, nil
			},
		}
		if err := autoReg.RegisterCategory("session", sessionTools); err != nil {
			t.Errorf("Failed to register session tools: %v", err)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify both categories were registered
	tools := autoReg.GetRegisteredToolNames()
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools after concurrent registration, got %d", len(tools))
	}
}

// Benchmark tests

func BenchmarkAutoRegistrar_RegisterCategory(b *testing.B) {
	mockRegistry := NewMockRegistry()
	autoReg := NewAutoRegistrar(mockRegistry)

	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		autoReg.ClearRegistrations()
		_ = autoReg.RegisterCategory("containerization", testTools)
	}
}

func BenchmarkAutoRegistrar_MigrateAllTools(b *testing.B) {
	testTools := map[string]api.ToolCreator{
		"analyze": func() (api.Tool, error) {
			return &MockTool{name: "containerization_analyze"}, nil
		},
		"build": func() (api.Tool, error) {
			return &MockTool{name: "containerization_build"}, nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockRegistry := NewMockRegistry()
		autoReg := NewAutoRegistrar(mockRegistry)
		_ = autoReg.RegisterCategory("containerization", testTools)

		ctx := context.Background()
		_ = autoReg.MigrateAllTools(ctx)
	}
}
