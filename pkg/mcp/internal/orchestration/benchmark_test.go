package orchestration

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

// Mock session manager for benchmarks
type mockSessionManager struct{}

func (m *mockSessionManager) GetSession(sessionID string) (interface{}, error) {
	return &struct{ ID string }{ID: sessionID}, nil
}

func (m *mockSessionManager) UpdateSession(session interface{}) error {
	return nil
}

// BenchmarkReflectionDispatch benchmarks the reflection-based dispatch
func BenchmarkReflectionDispatch(b *testing.B) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	orchestrator := NewMCPToolOrchestrator(registry, sessionManager, logger)

	args := map[string]interface{}{
		"session_id": "test-session",
		"repo_url":   "https://github.com/test/repo",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test the dispatch overhead, method changed to type-safe dispatch
		_, _ = orchestrator.ExecuteTool(context.Background(), "analyze_repository_atomic", args, nil)
	}
}

// BenchmarkNoReflectDispatch benchmarks the no-reflection dispatch
func BenchmarkNoReflectDispatch(b *testing.B) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	orchestrator := NewNoReflectToolOrchestrator(registry, sessionManager, logger)

	args := map[string]interface{}{
		"session_id": "test-session",
		"repo_url":   "https://github.com/test/repo",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Just test the dispatch overhead - expect error due to missing factory
		_, err := orchestrator.ExecuteTool(context.Background(), "analyze_repository_atomic", args, nil)
		// We expect "tool factory not initialized" error
		_ = err
	}
}

// BenchmarkFullExecutionReflection benchmarks full execution with reflection
func BenchmarkFullExecutionReflection(b *testing.B) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	// Register a mock tool
	registry.RegisterTool("test_tool", &mockTool{})

	orchestrator := NewMCPToolOrchestrator(registry, sessionManager, logger)

	args := map[string]interface{}{
		"session_id": "test-session",
		"data":       "test-data",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = orchestrator.ExecuteTool(ctx, "test_tool", args, nil)
	}
}

// BenchmarkFullExecutionNoReflect benchmarks full execution with no-reflection dispatch
func BenchmarkFullExecutionNoReflect(b *testing.B) {
	logger := zerolog.Nop()
	registry := NewMCPToolRegistry(logger)
	sessionManager := &mockSessionManager{}

	orchestrator := NewNoReflectToolOrchestrator(registry, sessionManager, logger)

	args := map[string]interface{}{
		"session_id": "test-session",
		"repo_url":   "https://github.com/test/repo",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test actual tool execution without reflection - expect error due to missing factory
		_, err := orchestrator.ExecuteTool(ctx, "analyze_repository_atomic", args, nil)
		// We expect "tool factory not initialized" error
		_ = err
	}
}

// Mock tool for testing
type mockTool struct{}

type mockToolArgs struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type mockToolResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (t *mockTool) Execute(ctx context.Context, args *mockToolArgs) (*mockToolResult, error) {
	return &mockToolResult{
		Success: true,
		Message: "Mock execution successful",
	}, nil
}
