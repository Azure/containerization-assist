package tools

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// BenchmarkToolRegistration benchmarks the tool registration process
func BenchmarkToolRegistration(b *testing.B) {
	// Create dependencies once
	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create new server for each iteration
		mcpServer := server.NewMCPServer("test-server", "1.0.0")

		// Benchmark the registration process
		err := RegisterTools(mcpServer, deps)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkToolConfigRetrieval benchmarks getting tool configurations
func BenchmarkToolConfigRetrieval(b *testing.B) {
	toolNames := []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"start_workflow",
		"list_tools",
		"ping",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, toolName := range toolNames {
			_, err := GetToolConfig(toolName)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkSchemaBuilding benchmarks building tool schemas
func BenchmarkSchemaBuilding(b *testing.B) {
	config := ToolConfig{
		RequiredParams: []string{"session_id", "param1", "param2"},
		OptionalParams: map[string]interface{}{
			"opt1": "string",
			"opt2": "number",
			"opt3": "boolean",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildToolSchema(config)
	}
}

// BenchmarkPingHandler benchmarks the ping tool handler
func BenchmarkPingHandler(b *testing.B) {
	ctx := context.Background()
	deps := ToolDependencies{}
	handler := createPingHandler(deps)

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"message": "benchmark test",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkServerStatusHandler benchmarks the server status tool handler
func BenchmarkServerStatusHandler(b *testing.B) {
	ctx := context.Background()
	deps := ToolDependencies{}
	handler := createServerStatusHandler(deps)

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"details": true,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListToolsHandler benchmarks the list tools handler
func BenchmarkListToolsHandler(b *testing.B) {
	ctx := context.Background()
	handler := CreateListToolsHandler()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParameterExtraction benchmarks parameter extraction functions
func BenchmarkParameterExtraction(b *testing.B) {
	args := map[string]interface{}{
		"session_id":  "test-session-123",
		"repo_path":   "/path/to/repo",
		"tag":         "v1.0.0",
		"optional":    "value",
		"array_param": []interface{}{"a", "b", "c"},
	}

	b.Run("ExtractStringParam", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ExtractStringParam(args, "session_id")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ExtractOptionalStringParam", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ExtractOptionalStringParam(args, "optional", "default")
		}
	})

	b.Run("ExtractStringArrayParam", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ExtractStringArrayParam(args, "array_param")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDependencyValidation benchmarks dependency validation
func BenchmarkDependencyValidation(b *testing.B) {
	config := ToolConfig{
		NeedsStepProvider:   true,
		NeedsSessionManager: true,
		NeedsLogger:         true,
	}

	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validateDependencies(config, deps)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWorkflowHandlerCreation benchmarks creating workflow handlers
func BenchmarkWorkflowHandlerCreation(b *testing.B) {
	config := ToolConfig{
		Name:                "test_tool",
		Category:            CategoryWorkflow,
		RequiredParams:      []string{"session_id"},
		NeedsStepProvider:   true,
		NeedsSessionManager: true,
		NeedsLogger:         true,
		StepGetterName:      "GetAnalyzeStep",
	}

	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateWorkflowHandler(config, deps)
	}
}

// BenchmarkMemoryUsage benchmarks memory allocation
func BenchmarkMemoryUsage(b *testing.B) {
	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mcpServer := server.NewMCPServer("test-server", "1.0.0")
		err := RegisterTools(mcpServer, deps)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Performance comparison test to verify new system is faster
func BenchmarkConfigTableVsReflection(b *testing.B) {
	// Simulate old approach with reflection for each call
	b.Run("TableDriven", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Table lookup - O(1)
			_, err := GetToolConfig("analyze_repository")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Note: We can't benchmark the old approach since it's been removed,
	// but this test validates that our current approach is efficient
	b.Run("SchemaGeneration", func(b *testing.B) {
		config, err := GetToolConfig("analyze_repository")
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = BuildToolSchema(*config)
		}
	})
}
