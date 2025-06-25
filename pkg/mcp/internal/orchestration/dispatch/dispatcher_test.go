package dispatch

import (
	"context"
	"fmt"
	"testing"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// testToolImpl is a simple test tool implementation
type testToolImpl struct {
	name string
}

func (t *testToolImpl) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	return map[string]interface{}{
		"success": true,
		"message": "test tool executed",
	}, nil
}

func (t *testToolImpl) GetMetadata() interface{} {
	return map[string]interface{}{
		"name":        t.name,
		"description": "Test tool for unit tests",
	}
}

// testToolWrapper wraps testToolImpl to implement mcptypes.InternalTool
type testToolWrapper struct {
	impl *testToolImpl
}

func (w *testToolWrapper) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	return w.impl.Execute(ctx, args)
}

func (w *testToolWrapper) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        w.impl.name,
		Description: "Test tool for unit tests",
		Version:     "1.0.0",
		Category:    "test",
	}
}

func (w *testToolWrapper) Validate(ctx context.Context, args interface{}) error {
	return nil
}

// testToolArgs implements mcptypes.InternalToolArgs
type testToolArgs struct {
	data map[string]interface{}
}

func (a *testToolArgs) Validate() error {
	return nil
}

func (a *testToolArgs) GetSessionID() string {
	if sessionID, ok := a.data["session_id"].(string); ok {
		return sessionID
	}
	return "test-session"
}

// testToolResult implements mcptypes.InternalToolResult
type testToolResult struct {
	success bool
	result  interface{}
	error   string
}

func (r *testToolResult) GetResult() interface{} {
	return r.result
}

func (r *testToolResult) IsSuccess() bool {
	return r.success
}

func (r *testToolResult) GetSuccess() bool {
	return r.success
}

func (r *testToolResult) GetError() error {
	if r.error != "" {
		return fmt.Errorf("%s", r.error)
	}
	return nil
}

func TestToolDispatcher(t *testing.T) {
	// Create a new dispatcher
	dispatcher := NewToolDispatcher()

	// Create a simple test tool directly without adapter
	factory := func() interface{} {
		return &testToolWrapper{impl: &testToolImpl{name: "test_tool"}}
	}
	converter := func(args map[string]interface{}) (interface{}, error) {
		// Return a simple test args implementation
		return &testToolArgs{data: args}, nil
	}
	err := dispatcher.RegisterTool("test_tool", factory, converter)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Test 1: Tool registration
	t.Run("ToolRegistration", func(t *testing.T) {
		t.Parallel()
		tools := dispatcher.ListTools()
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(tools))
		}

		if tools[0] != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0])
		}
	})

	// Test 2: Get tool factory
	t.Run("GetToolFactory", func(t *testing.T) {
		t.Parallel()
		factory, exists := dispatcher.GetToolFactory("test_tool")
		if !exists {
			t.Error("Tool factory not found")
		}

		tool := factory()
		if tool == nil {
			t.Error("Factory returned nil tool")
		}
	})

	// Test 3: Tool execution via dispatcher
	t.Run("ToolExecution", func(t *testing.T) {
		factory, _ := dispatcher.GetToolFactory("test_tool")
		toolInstance := factory()
		tool, ok := toolInstance.(mcptypes.InternalTool)
		if !ok {
			t.Fatalf("Tool factory did not return a valid Tool instance")
		}

		args := map[string]interface{}{
			"test": "data",
		}

		result, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		// Type assert result
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Result has wrong type")
		}

		if success, ok := resultMap["success"].(bool); !ok || !success {
			t.Error("Expected successful execution")
		}
	})

	// Test 4: Get tools by category (simplified)
	t.Run("GetToolsByCategory", func(t *testing.T) {
		// Our test tool has category "test"
		tools := dispatcher.GetToolsByCategory("test")
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool in 'test' category, got %d", len(tools))
		}

		// Test non-existent category
		tools = dispatcher.GetToolsByCategory("non-existent")
		if len(tools) != 0 {
			t.Errorf("Expected 0 tools in 'non-existent' category, got %d", len(tools))
		}
	})
}

func TestDispatcherConcurrency(t *testing.T) {
	dispatcher := NewToolDispatcher()

	// Register test tool
	factory := func() interface{} {
		return &testToolWrapper{impl: &testToolImpl{name: "concurrent_test_tool"}}
	}
	converter := func(args map[string]interface{}) (interface{}, error) {
		// Return a simple test args implementation
		return &testToolArgs{data: args}, nil
	}
	dispatcher.RegisterTool("concurrent_test_tool", factory, converter)

	// Test concurrent access
	done := make(chan bool, 10)

	// Multiple goroutines accessing dispatcher
	for i := 0; i < 10; i++ {
		go func() {
			// List tools
			_ = dispatcher.ListTools()

			// Get factory
			factory, _ := dispatcher.GetToolFactory("concurrent_test_tool")
			if factory != nil {
				toolInstance := factory()
				if tool, ok := toolInstance.(mcptypes.InternalTool); ok {
					// Execute tool
					_, _ = tool.Execute(context.Background(), nil)
				}
			}

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
