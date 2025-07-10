package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// Mock tool for testing
type mockTool struct {
	name        string
	description string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "test"},
	}, nil
}
func (m *mockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
	}
}

func TestUnifiedRegistry_Register(t *testing.T) {
	registry := NewUnified()

	t.Run("successful registration", func(t *testing.T) {
		err := RegisterTool(registry, "test-tool", func() string { return "test" })
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify tool is registered
		tools := registry.List()
		if len(tools) != 1 || tools[0] != "test-tool" {
			t.Fatalf("Expected [test-tool], got %v", tools)
		}
	})

	t.Run("duplicate registration", func(t *testing.T) {
		err := RegisterTool(registry, "test-tool", func() string { return "duplicate" })
		if err == nil {
			t.Fatal("Expected error for duplicate registration")
		}
	})

	t.Run("empty name registration", func(t *testing.T) {
		err := RegisterTool(registry, "", func() string { return "test" })
		if err == nil {
			t.Fatal("Expected error for empty name")
		}
	})

	t.Run("register different types", func(t *testing.T) {
		// Register int factory
		err := RegisterTool(registry, "int-tool", func() int { return 42 })
		if err != nil {
			t.Fatalf("Failed to register int tool: %v", err)
		}

		// Register struct factory
		err = RegisterTool(registry, "struct-tool", func() mockTool {
			return mockTool{name: "mock", description: "test"}
		})
		if err != nil {
			t.Fatalf("Failed to register struct tool: %v", err)
		}

		// Register interface factory
		err = RegisterTool(registry, "api-tool", func() api.Tool {
			return &mockTool{name: "api", description: "test"}
		})
		if err != nil {
			t.Fatalf("Failed to register api.Tool: %v", err)
		}
	})
}

func TestUnifiedRegistry_Discover(t *testing.T) {
	registry := NewUnified()

	// Register test tools
	RegisterTool(registry, "string-tool", func() string { return "test-result" })
	RegisterTool(registry, "int-tool", func() int { return 42 })
	RegisterTool(registry, "api-tool", func() api.Tool {
		return &mockTool{name: "test", description: "test tool"}
	})

	t.Run("discover string tool", func(t *testing.T) {
		result, err := DiscoverTool[string](registry, "string-tool")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result != "test-result" {
			t.Fatalf("Expected 'test-result', got %s", result)
		}
	})

	t.Run("discover int tool", func(t *testing.T) {
		result, err := DiscoverTool[int](registry, "int-tool")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result != 42 {
			t.Fatalf("Expected 42, got %d", result)
		}
	})

	t.Run("discover api.Tool", func(t *testing.T) {
		result, err := DiscoverTool[api.Tool](registry, "api-tool")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result.Name() != "test" {
			t.Fatalf("Expected 'test', got %s", result.Name())
		}
	})

	t.Run("discover missing tool", func(t *testing.T) {
		_, err := DiscoverTool[string](registry, "missing-tool")
		if err == nil {
			t.Fatal("Expected error for missing tool")
		}
	})

	t.Run("discover with wrong type", func(t *testing.T) {
		_, err := DiscoverTool[int](registry, "string-tool")
		if err == nil {
			t.Fatal("Expected error for type mismatch")
		}
	})
}

func TestUnifiedRegistry_ThreadSafety(t *testing.T) {
	registry := NewUnified()

	// Test concurrent registration
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := RegisterTool(registry, fmt.Sprintf("tool-%d", id), func() int { return id })
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Concurrent registration error: %v", err)
	}

	// Verify all tools registered
	tools := registry.List()
	if len(tools) != 100 {
		t.Fatalf("Expected 100 tools, got %d", len(tools))
	}

	// Test concurrent discovery
	wg = sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result, err := DiscoverTool[int](registry, fmt.Sprintf("tool-%d", id))
			if err != nil {
				t.Errorf("Discovery error for tool-%d: %v", id, err)
				return
			}
			if result != id {
				t.Errorf("Expected %d, got %d", id, result)
			}
		}(i)
	}

	wg.Wait()
}

func TestUnifiedRegistry_Metadata(t *testing.T) {
	registry := NewUnified()

	// Register a tool
	err := RegisterTool(registry, "test-tool", func() string { return "test" })
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	t.Run("get metadata", func(t *testing.T) {
		meta, err := registry.Metadata("test-tool")
		if err != nil {
			t.Fatalf("Failed to get metadata: %v", err)
		}

		if meta.Name != "test-tool" {
			t.Fatalf("Expected name 'test-tool', got %s", meta.Name)
		}

		if meta.Version != "1.0.0" {
			t.Fatalf("Expected version '1.0.0', got %s", meta.Version)
		}
	})

	t.Run("set metadata", func(t *testing.T) {
		newMeta := api.ToolMetadata{
			Name:        "test-tool",
			Description: "Updated description",
			Version:     "2.0.0",
			Tags:        []string{"test", "updated"},
		}

		err := registry.SetMetadata("test-tool", newMeta)
		if err != nil {
			t.Fatalf("Failed to set metadata: %v", err)
		}

		// Verify update
		meta, err := registry.Metadata("test-tool")
		if err != nil {
			t.Fatalf("Failed to get updated metadata: %v", err)
		}

		if meta.Description != "Updated description" {
			t.Fatalf("Expected 'Updated description', got %s", meta.Description)
		}

		if meta.Version != "2.0.0" {
			t.Fatalf("Expected version '2.0.0', got %s", meta.Version)
		}
	})

	t.Run("metadata for missing tool", func(t *testing.T) {
		_, err := registry.Metadata("missing-tool")
		if err == nil {
			t.Fatal("Expected error for missing tool")
		}
	})
}

func TestUnifiedRegistry_Execute(t *testing.T) {
	registry := NewUnified()

	// Register an api.Tool
	err := RegisterTool(registry, "exec-tool", func() api.Tool {
		return &mockTool{name: "exec", description: "executable tool"}
	})
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	t.Run("execute tool", func(t *testing.T) {
		ctx := context.Background()
		input := api.ToolInput{
			SessionID: "test-session",
			Data:      map[string]interface{}{"key": "value"},
		}

		output, err := registry.Execute(ctx, "exec-tool", input)
		if err != nil {
			t.Fatalf("Failed to execute tool: %v", err)
		}

		if !output.Success {
			t.Fatal("Expected successful execution")
		}

		if output.Data["result"] != "test" {
			t.Fatalf("Expected result 'test', got %v", output.Data["result"])
		}
	})

	t.Run("execute missing tool", func(t *testing.T) {
		ctx := context.Background()
		input := api.ToolInput{SessionID: "test"}

		output, err := registry.Execute(ctx, "missing-tool", input)
		if err == nil {
			t.Fatal("Expected error for missing tool")
		}

		if output.Success {
			t.Fatal("Expected failure for missing tool")
		}
	})
}

func TestUnifiedRegistry_Unregister(t *testing.T) {
	registry := NewUnified()

	// Register a tool
	err := RegisterTool(registry, "test-tool", func() string { return "test" })
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	t.Run("unregister existing tool", func(t *testing.T) {
		err := registry.Unregister("test-tool")
		if err != nil {
			t.Fatalf("Failed to unregister tool: %v", err)
		}

		// Verify tool is gone
		_, err = DiscoverTool[string](registry, "test-tool")
		if err == nil {
			t.Fatal("Expected error for unregistered tool")
		}
	})

	t.Run("unregister missing tool", func(t *testing.T) {
		err := registry.Unregister("missing-tool")
		if err == nil {
			t.Fatal("Expected error for missing tool")
		}
	})
}

func TestUnifiedRegistry_Close(t *testing.T) {
	registry := NewUnified()

	// Register some tools
	RegisterTool(registry, "tool1", func() string { return "1" })
	RegisterTool(registry, "tool2", func() string { return "2" })

	t.Run("close registry", func(t *testing.T) {
		err := registry.Close()
		if err != nil {
			t.Fatalf("Failed to close registry: %v", err)
		}

		// Verify tools are cleared
		tools := registry.List()
		if len(tools) != 0 {
			t.Fatalf("Expected empty registry, got %v", tools)
		}
	})

	t.Run("operations on closed registry", func(t *testing.T) {
		// Try to register after close
		err := RegisterTool(registry, "new-tool", func() string { return "test" })
		if err == nil {
			t.Fatal("Expected error for closed registry")
		}

		// Try to discover after close
		_, err = DiscoverTool[string](registry, "tool1")
		if err == nil {
			t.Fatal("Expected error for closed registry")
		}
	})
}

func TestUnifiedRegistry_Metrics(t *testing.T) {
	registry := NewUnified()

	// Perform some operations
	RegisterTool(registry, "metric-tool", func() string { return "test" })
	DiscoverTool[string](registry, "metric-tool")
	DiscoverTool[string](registry, "metric-tool")

	// Register api.Tool for execution
	RegisterTool(registry, "exec-tool", func() api.Tool {
		return &mockTool{name: "exec", description: "test"}
	})

	ctx := context.Background()
	input := api.ToolInput{SessionID: "test"}
	registry.Execute(ctx, "exec-tool", input)

	// Type assert to get access to GetMetrics
	if unifiedReg, ok := registry.(*UnifiedRegistry); ok {
		metrics := unifiedReg.GetMetrics()

		if metrics.TotalTools != 2 {
			t.Fatalf("Expected 2 tools, got %d", metrics.TotalTools)
		}

		if metrics.TotalExecutions != 1 {
			t.Fatalf("Expected 1 execution, got %d", metrics.TotalExecutions)
		}

		if metrics.AverageExecutionTime < 0 {
			t.Fatal("Expected non-negative average execution time")
		}
	} else {
		t.Skip("Registry is not UnifiedRegistry type, skipping metrics test")
	}
}

func BenchmarkRegistryOperations(b *testing.B) {
	registry := NewUnified()

	// Setup: Register 100 tools
	for i := 0; i < 100; i++ {
		RegisterTool(registry, fmt.Sprintf("tool-%d", i), func() string { return "result" })
	}

	b.Run("Discovery", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := DiscoverTool[string](registry, "tool-50")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Registration", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			name := fmt.Sprintf("bench-tool-%d", i)
			err := RegisterTool(registry, name, func() string { return "test" })
			if err != nil {
				b.Fatal(err)
			}
			// Clean up to avoid accumulation
			registry.Unregister(name)
		}
	})

	b.Run("List", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tools := registry.List()
			if len(tools) != 100 {
				b.Fatalf("Expected 100 tools, got %d", len(tools))
			}
		}
	})
}

func TestRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	registry := NewUnified()

	// Run multiple operations concurrently to detect races
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				name := fmt.Sprintf("race-tool-%d-%d", id, j)
				RegisterTool(registry, name, func() int { return id*100 + j })
				time.Sleep(time.Microsecond)
				registry.Unregister(name)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				registry.List()
				// Skip GetMetrics call as it's not in the interface
				time.Sleep(time.Microsecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
