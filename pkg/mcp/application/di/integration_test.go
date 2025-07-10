package di

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

func TestDIIntegration(t *testing.T) {
	// Test that the Wire container can be successfully initialized
	container, err := InitializeContainer()
	if err != nil {
		t.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Verify all services are properly injected
	if container.ToolRegistry == nil {
		t.Fatal("ToolRegistry not injected")
	}

	if container.SessionStore == nil {
		t.Fatal("SessionStore not injected")
	}

	if container.SessionState == nil {
		t.Fatal("SessionState not injected")
	}

	if container.BuildExecutor == nil {
		t.Fatal("BuildExecutor not injected")
	}

	if container.WorkflowExecutor == nil {
		t.Fatal("WorkflowExecutor not injected")
	}

	if container.Scanner == nil {
		t.Fatal("Scanner not injected")
	}

	if container.ConfigValidator == nil {
		t.Fatal("ConfigValidator not injected")
	}

	if container.ErrorReporter == nil {
		t.Fatal("ErrorReporter not injected")
	}
}

func TestRegistryIntegration(t *testing.T) {
	container, err := InitializeContainer()
	if err != nil {
		t.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Create a mock tool
	tool := &mockTool{
		name:   "test-tool",
		result: "test-result",
	}

	// Test that we can register tools through the registry
	// Register expects a factory function that returns the tool
	toolFactory := registry.ToolFactory(func() (api.Tool, error) {
		return tool, nil
	})
	err = container.ToolRegistry.Register("test-tool", toolFactory)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Test discovery
	discoveredToolInterface, err := container.ToolRegistry.Discover("test-tool")
	if err != nil {
		t.Fatalf("Failed to discover tool: %v", err)
	}

	// Cast the discovered interface to api.Tool
	discoveredTool, ok := discoveredToolInterface.(api.Tool)
	if !ok {
		t.Fatalf("Discovered tool is not an api.Tool")
	}

	if discoveredTool.Name() != "test-tool" {
		t.Fatalf("Expected 'test-tool', got %s", discoveredTool.Name())
	}

	// Test that the tool registry lists tools correctly
	tools := container.ToolRegistry.List()
	if len(tools) != 1 || tools[0] != "test-tool" {
		t.Fatalf("Expected [test-tool], got %v", tools)
	}
}

// mockTool implements api.Tool for testing purposes
type mockTool struct {
	name   string
	result string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return "Mock tool for testing"
}

func (m *mockTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"result": m.result,
		},
	}, nil
}

func (m *mockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: "Mock tool for testing",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}
}

func TestServiceInteractions(t *testing.T) {
	container, err := InitializeContainer()
	if err != nil {
		t.Fatalf("Failed to initialize DI container: %v", err)
	}

	ctx := context.Background()

	// Test that services can interact through the injected dependencies
	// For example, the WorkflowExecutor should be able to use the ToolRegistry

	// Register a mock tool that implements api.Tool
	mockToolInstance := &mockTool{name: "workflow-tool", result: "workflow-result"}
	toolFactory := registry.ToolFactory(func() (api.Tool, error) {
		return mockToolInstance, nil
	})
	err = container.ToolRegistry.Register("workflow-tool", toolFactory)
	if err != nil {
		t.Fatalf("Failed to register workflow tool: %v", err)
	}

	// Verify it can be discovered through the registry
	toolInterface, err := container.ToolRegistry.Discover("workflow-tool")
	if err != nil {
		t.Fatalf("Failed to discover tool through registry: %v", err)
	}

	// Cast to api.Tool
	tool, ok := toolInterface.(api.Tool)
	if !ok {
		t.Fatalf("Discovered tool is not an api.Tool")
	}

	// Verify the tool is what we expect
	if tool.Name() != "workflow-tool" {
		t.Fatalf("Expected tool name 'workflow-tool', got %s", tool.Name())
	}

	// Test error reporter (create a dummy error for testing)
	testErr := errors.NewError().Message("test error").Build()
	container.ErrorReporter.ReportError(ctx, testErr, map[string]interface{}{
		"test": "integration",
	})

	// Test config validator (should not fail even with stub implementation)
	_, err = container.ConfigValidator.ValidateConfig(ctx, map[string]interface{}{
		"test": "config",
	})
	// Note: This will return a "not implemented" error, which is expected for stubs
	if err == nil {
		t.Log("Config validator returned nil (unexpected for stub)")
	}
}

func TestContainerLifecycle(t *testing.T) {
	// Test multiple container initializations
	for i := 0; i < 3; i++ {
		container, err := InitializeContainer()
		if err != nil {
			t.Fatalf("Failed to initialize DI container on iteration %d: %v", i, err)
		}

		// Each container should have its own registry instance
		// Create a tool with a unique name for each iteration
		toolName := fmt.Sprintf("lifecycle-tool-%d", i)
		lifecycleTool := &mockTool{
			name:   toolName,
			result: fmt.Sprintf("result-%d", i),
		}

		toolFactory := registry.ToolFactory(func() (api.Tool, error) {
			return lifecycleTool, nil
		})
		err = container.ToolRegistry.Register(toolName, toolFactory)
		if err != nil {
			t.Fatalf("Failed to register tool on iteration %d: %v", i, err)
		}

		toolInterface, err := container.ToolRegistry.Discover(toolName)
		if err != nil {
			t.Fatalf("Failed to discover tool on iteration %d: %v", i, err)
		}

		// Cast to api.Tool
		tool, ok := toolInterface.(api.Tool)
		if !ok {
			t.Fatalf("Failed to cast tool to api.Tool on iteration %d", i)
		}

		if tool.Name() != toolName {
			t.Fatalf("Expected %s, got %s on iteration %d", toolName, tool.Name(), i)
		}

		// Clean up
		err = container.ToolRegistry.Close()
		if err != nil {
			t.Fatalf("Failed to close registry on iteration %d: %v", i, err)
		}
	}
}

func BenchmarkContainerInitialization(b *testing.B) {
	for i := 0; i < b.N; i++ {
		container, err := InitializeContainer()
		if err != nil {
			b.Fatal(err)
		}

		// Ensure the container is valid
		if container.ToolRegistry == nil {
			b.Fatal("ToolRegistry not initialized")
		}
	}
}

func BenchmarkServiceInteraction(b *testing.B) {
	container, err := InitializeContainer()
	if err != nil {
		b.Fatal(err)
	}

	// Register a tool for benchmarking using proper api.Tool interface
	benchTool := &mockTool{name: "bench-tool", result: "bench-result"}
	toolFactory := registry.ToolFactory(func() (api.Tool, error) {
		return benchTool, nil
	})
	err = container.ToolRegistry.Register("bench-tool", toolFactory)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Discovery
		toolInterface, err := container.ToolRegistry.Discover("bench-tool")
		if err != nil {
			b.Fatal(err)
		}

		// Cast to api.Tool
		tool, ok := toolInterface.(api.Tool)
		if !ok {
			b.Fatal("Failed to cast tool to api.Tool")
		}

		// Execution
		input := api.ToolInput{
			SessionID: "benchmark-session",
			Data:      map[string]interface{}{"iteration": i},
		}
		output, err := tool.Execute(ctx, input)
		if err != nil {
			b.Fatal(err)
		}

		if !output.Success {
			b.Fatal("Tool execution failed")
		}
	}
}
