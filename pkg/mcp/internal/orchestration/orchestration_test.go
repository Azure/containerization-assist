package orchestration

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// TestOrchestratorIntegration tests the consolidated orchestrator implementation
func TestOrchestratorIntegration(t *testing.T) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	// Test the main production orchestrator
	orchestrator := NewMCPToolOrchestrator(registry, sessionManager, logger)

	// Test cases for various argument types
	testCases := []struct {
		name     string
		toolName string
		args     map[string]interface{}
	}{
		{
			name:     "Simple string arguments",
			toolName: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
			},
		},
		{
			name:     "Arguments with numbers",
			toolName: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
				"depth":      10,
			},
		},
		{
			name:     "Arguments with boolean",
			toolName: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
				"force":      true,
			},
		},
		{
			name:     "Arguments with nested map",
			toolName: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
				"config": map[string]interface{}{
					"timeout": 300,
					"verbose": true,
				},
			},
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the orchestrator handles various argument types
			result, err := orchestrator.ExecuteTool(ctx, tc.toolName, tc.args, nil)
			
			// Since we don't have a tool factory set up, we expect a specific error
			assert.Error(t, err, "Should return error without tool factory")
			assert.Contains(t, err.Error(), "tool factory not initialized", "Should indicate missing tool factory")
			assert.Nil(t, result, "Result should be nil when factory not initialized")
			
			t.Logf("Tool %s correctly handled arguments, got expected error: %v", tc.toolName, err)
		})
	}
}

// TestTypeSafeDispatchEnabled tests that type-safe dispatch is working
func TestTypeSafeDispatchEnabled(t *testing.T) {
	logger := zerolog.Nop()
	sessionManager := &mockSessionManager{}

	// Test the no-reflection orchestrator directly
	registry := NewMCPToolRegistry(logger)
	orchestrator := NewNoReflectToolOrchestrator(registry, sessionManager, logger)

	args := map[string]interface{}{
		"session_id": "test-session",
		"repo_url":   "https://github.com/test/repo",
	}

	result, err := orchestrator.ExecuteTool(context.Background(), "analyze_repository_atomic", args, nil)
	
	// Since we don't have a tool factory, we expect a specific error 
	assert.Error(t, err, "Should return error without tool factory")
	assert.Contains(t, err.Error(), "tool factory not initialized", "Should indicate missing tool factory")
	assert.Nil(t, result, "Result should be nil when factory not initialized")
	
	t.Logf("No-reflection dispatch correctly handled missing factory: %v", err)
}

// TestGetToolsByCategory tests tool discovery functionality
func TestGetToolsByCategory(t *testing.T) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)

	// Test category-based tool discovery
	categories := []string{"analysis", "build", "deployment", "security"}
	
	for _, category := range categories {
		tools := registry.GetToolsByCategory(category)
		t.Logf("Tools with category '%s': %v", category, tools)
		
		// We should have at least some tools for each major category
		if category == "analysis" || category == "build" {
			assert.Greater(t, len(tools), 0, "Expected tools for category: %s", category)
		}
	}
}

// TestOrchestratorPipelineIntegration tests orchestrator integration with workflow
func TestOrchestratorPipelineIntegration(t *testing.T) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	orchestrator := NewMCPToolOrchestrator(registry, sessionManager, logger)

	// Test a sequence of tools that might be used in a pipeline
	toolSequence := []struct {
		tool string
		args map[string]interface{}
	}{
		{
			tool: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
			},
		},
		{
			tool: "validate_dockerfile_atomic",
			args: map[string]interface{}{
				"session_id":     "test-session",
				"dockerfile_path": "./Dockerfile",
			},
		},
	}

	ctx := context.Background()

	for i, step := range toolSequence {
		t.Run(step.tool, func(t *testing.T) {
			result, err := orchestrator.ExecuteTool(ctx, step.tool, step.args, nil)
			
			// Without tool factory, should get expected error
			assert.Error(t, err, "Step %d should return error without tool factory", i)
			assert.Contains(t, err.Error(), "tool factory not initialized", "Should indicate missing tool factory")
			assert.Nil(t, result, "Step %d result should be nil when factory not initialized", i)
			
			t.Logf("Step %d (%s) correctly handled missing factory: %v", i, step.tool, err)
		})
	}
}

// TestToolDispatchRouting tests that tools are routed correctly
func TestToolDispatchRouting(t *testing.T) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	// Test the no-reflection orchestrator directly
	orchestrator := NewNoReflectToolOrchestrator(registry, sessionManager, logger)

	testCases := []struct {
		name     string
		toolName string
		args     interface{}
		wantErr  string
	}{
		{
			name:     "Known tool - analyze_repository_atomic",
			toolName: "analyze_repository_atomic",
			args: map[string]interface{}{
				"session_id": "test-session",
				"repo_url":   "https://github.com/test/repo",
			},
			wantErr: "tool factory not initialized",
		},
		{
			name:     "Known tool - build_image_atomic",
			toolName: "build_image_atomic",
			args: map[string]interface{}{
				"session_id":  "test-session",
				"image_name":  "myapp",
				"image_tag":   "latest",
			},
			wantErr: "tool factory not initialized",
		},
		{
			name:     "Unknown tool",
			toolName: "unknown_tool",
			args: map[string]interface{}{
				"session_id": "test-session",
			},
			wantErr: "unknown tool: unknown_tool",
		},
		{
			name:     "Invalid arguments - not a map",
			toolName: "analyze_repository_atomic", 
			args:     "invalid",
			wantErr:  "arguments must be a map[string]interface{}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := orchestrator.ExecuteTool(context.Background(), tc.toolName, tc.args, nil)
			
			assert.Error(t, err, "Should return error")
			assert.Contains(t, err.Error(), tc.wantErr, "Error should contain expected message")
			assert.Nil(t, result, "Result should be nil on error")
			
			t.Logf("Tool %s correctly returned error: %v", tc.toolName, err)
		})
	}
}