package dispatch

import (
	"context"
	"testing"
)

func TestToolDispatcher(t *testing.T) {
	// Create a new dispatcher
	dispatcher := NewToolDispatcher()

	// Register the example tool
	err := RegisterAnalyzeRepositoryTool(dispatcher)
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

		if tools[0] != "analyze_repository_atomic" {
			t.Errorf("Expected tool name 'analyze_repository_atomic', got '%s'", tools[0])
		}
	})

	// Test 2: Get tool factory
	t.Run("GetToolFactory", func(t *testing.T) {
		t.Parallel()
		factory, exists := dispatcher.GetToolFactory("analyze_repository_atomic")
		if !exists {
			t.Error("Tool factory not found")
		}

		tool := factory()
		if tool == nil {
			t.Error("Factory returned nil tool")
		}
	})

	// Test 3: Argument conversion
	t.Run("ArgumentConversion", func(t *testing.T) {
		t.Parallel()
		args := map[string]interface{}{
			"session_id": "test-session",
			"repo_url":   "https://github.com/test/repo",
			"branch":     "main",
			"depth":      10,
		}

		toolArgs, err := dispatcher.ConvertArgs("analyze_repository_atomic", args)
		if err != nil {
			t.Fatalf("Failed to convert args: %v", err)
		}

		// Type assert to verify correct type
		analyzeArgs, ok := toolArgs.(*AnalyzeRepositoryArgs)
		if !ok {
			t.Fatal("Converted args have wrong type")
		}

		if analyzeArgs.SessionID != "test-session" {
			t.Errorf("Expected SessionID 'test-session', got '%s'", analyzeArgs.SessionID)
		}

		if analyzeArgs.RepoURL != "https://github.com/test/repo" {
			t.Errorf("Expected RepoURL 'https://github.com/test/repo', got '%s'", analyzeArgs.RepoURL)
		}

		if analyzeArgs.Branch != "main" {
			t.Errorf("Expected Branch 'main', got '%s'", analyzeArgs.Branch)
		}

		if analyzeArgs.Depth != 10 {
			t.Errorf("Expected Depth 10, got %d", analyzeArgs.Depth)
		}
	})

	// Test 4: Argument validation
	t.Run("ArgumentValidation", func(t *testing.T) {
		// Missing required field
		args := map[string]interface{}{
			"session_id": "test-session",
			// repo_url is missing
		}

		_, err := dispatcher.ConvertArgs("analyze_repository_atomic", args)
		if err == nil {
			t.Error("Expected validation error for missing repo_url")
		}
	})

	// Test 5: Tool execution
	t.Run("ToolExecution", func(t *testing.T) {
		factory, _ := dispatcher.GetToolFactory("analyze_repository_atomic")
		tool := factory()

		args := &AnalyzeRepositoryArgs{
			SessionID: "test-session",
			RepoURL:   "https://github.com/test/repo",
		}

		result, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		// Type assert result
		analyzeResult, ok := result.(*AnalyzeRepositoryResult)
		if !ok {
			t.Fatal("Result has wrong type")
		}

		if !analyzeResult.IsSuccess() {
			t.Error("Expected successful execution")
		}

		if analyzeResult.Language != "go" {
			t.Errorf("Expected language 'go', got '%s'", analyzeResult.Language)
		}
	})

	// Test 6: Metadata
	t.Run("ToolMetadata", func(t *testing.T) {
		metadata, exists := dispatcher.GetToolMetadata("analyze_repository_atomic")
		if !exists {
			t.Error("Tool metadata not found")
		}

		if metadata.Name != "analyze_repository_atomic" {
			t.Errorf("Expected name 'analyze_repository_atomic', got '%s'", metadata.Name)
		}

		if metadata.Category != "analysis" {
			t.Errorf("Expected category 'analysis', got '%s'", metadata.Category)
		}

		if len(metadata.Capabilities) != 3 {
			t.Errorf("Expected 3 capabilities, got %d", len(metadata.Capabilities))
		}
	})

	// Test 7: Get tools by category
	t.Run("GetToolsByCategory", func(t *testing.T) {
		tools := dispatcher.GetToolsByCategory("analysis")
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool in 'analysis' category, got %d", len(tools))
		}
	})

	// Test 8: Get tools by capability
	t.Run("GetToolsByCapability", func(t *testing.T) {
		tools := dispatcher.GetToolsByCapability("language_detection")
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool with 'language_detection' capability, got %d", len(tools))
		}
	})
}

func TestDispatcherConcurrency(t *testing.T) {
	dispatcher := NewToolDispatcher()

	// Register tool
	RegisterAnalyzeRepositoryTool(dispatcher)

	// Test concurrent access
	done := make(chan bool, 10)

	// Multiple goroutines accessing dispatcher
	for i := 0; i < 10; i++ {
		go func() {
			// List tools
			_ = dispatcher.ListTools()

			// Get factory
			factory, _ := dispatcher.GetToolFactory("analyze_repository_atomic")
			if factory != nil {
				tool := factory()
				_ = tool.GetMetadata()
			}

			// Convert args
			args := map[string]interface{}{
				"session_id": "test",
				"repo_url":   "test",
			}
			_, _ = dispatcher.ConvertArgs("analyze_repository_atomic", args)

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
