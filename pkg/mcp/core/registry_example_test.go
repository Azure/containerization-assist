package core

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// MockTool implements api.Tool for testing
type MockTool struct {
	name        string
	description string
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return m.description
}

func (m *MockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
		Version:     "1.0.0",
	}
}

func (m *MockTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "test result from " + m.name},
	}, nil
}

func TestUnifiedRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewUnifiedRegistry(logger)

	mockTool := &MockTool{
		name:        "test_tool",
		description: "A test tool",
	}

	err := registry.Register(mockTool,
		WithNamespace("test"),
		WithMetrics(true),
		WithCaching(false, 0),
	)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	retrievedTool, err := registry.Get("test_tool")
	if err != nil {
		t.Fatalf("Failed to get tool: %v", err)
	}

	if retrievedTool.Name() != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", retrievedTool.Name())
	}

	input := api.ToolInput{
		SessionID: "test_session",
		Data:      map[string]interface{}{"test": "value"},
	}

	result, err := registry.Execute(context.Background(), "test_tool", input)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if !result.Success {
		t.Error("Expected execution to be successful")
	}

	if result.Data == nil {
		t.Error("Expected result data to be present")
	}

	tools := registry.List()
	if len(tools) != 1 || tools[0] != "test_tool" {
		t.Errorf("Expected list to contain 'test_tool', got %v", tools)
	}

	count := registry.Count()
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}

	metadata, err := registry.GetRegistryMetadata("test_tool")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if metadata.Name != "test_tool" {
		t.Errorf("Expected metadata name 'test_tool', got '%s'", metadata.Name)
	}

	if !metadata.Enabled {
		t.Error("Expected tool to be enabled")
	}

	stats := registry.GetStats()
	if stats.TotalTools != 1 || stats.EnabledTools != 1 {
		t.Errorf("Unexpected stats: %+v", stats)
	}

	err = registry.Unregister("test_tool")
	if err != nil {
		t.Fatalf("Failed to unregister tool: %v", err)
	}

	_, err = registry.Get("test_tool")
	if err == nil {
		t.Error("Expected error when getting unregistered tool")
	}
}

func TestRegistryOptions(t *testing.T) {
	config := &RegistryConfig{}

	WithNamespace("test-ns")(config)
	if config.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", config.Namespace)
	}

	WithCaching(true, 300)(config)
	if !config.EnableCaching || config.CacheTTL != 300 {
		t.Errorf("Caching option not applied correctly: %+v", config)
	}

	WithMetrics(true)(config)
	if !config.EnableMetrics {
		t.Error("Metrics option not applied correctly")
	}

	WithConcurrency(10)(config)
	if config.MaxConcurrency != 10 {
		t.Errorf("Expected max concurrency 10, got %d", config.MaxConcurrency)
	}

	WithTags("tag1", "tag2")(config)
	if len(config.Tags) != 2 || config.Tags[0] != "tag1" || config.Tags[1] != "tag2" {
		t.Errorf("Tags option not applied correctly: %v", config.Tags)
	}

	WithPriority(5)(config)
	if config.Priority != 5 {
		t.Errorf("Expected priority 5, got %d", config.Priority)
	}

	WithPersistence(true)(config)
	if !config.EnablePersistence {
		t.Error("Persistence option not applied correctly")
	}

	WithEvents(true)(config)
	if !config.EnableEvents {
		t.Error("Events option not applied correctly")
	}
}
