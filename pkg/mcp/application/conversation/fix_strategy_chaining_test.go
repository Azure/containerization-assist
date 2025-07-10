package conversation

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// TestFixChainExecutor_BasicFunctionality tests basic chain executor functionality
func TestFixChainExecutor_BasicFunctionality(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	executor := NewFixChainExecutor(logger, helper)

	if executor == nil {
		t.Fatal("FixChainExecutor should not be nil")
	}

	if len(executor.chains) == 0 {
		t.Error("FixChainExecutor should have registered chains")
	}

	// Test getting available chains
	chains := executor.GetAvailableChains()
	if len(chains) == 0 {
		t.Error("Expected some available chains")
	}

	// Check for expected chains
	expectedChains := []string{
		"docker_build_complex",
		"network_connectivity_fix",
		"resource_conflict_resolution",
		"manifest_deployment_recovery",
	}

	for _, expectedChain := range expectedChains {
		if _, exists := chains[expectedChain]; !exists {
			t.Errorf("Expected chain '%s' to be available", expectedChain)
		}
	}
}

// TestFixChainExecutor_ChainMatching tests chain condition matching
func TestFixChainExecutor_ChainMatching(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	tests := []struct {
		toolName      string
		errorMsg      string
		expectedMatch string
		shouldMatch   bool
	}{
		{
			toolName:      "build_image",
			errorMsg:      "docker build failed with multiple errors",
			expectedMatch: "docker_build_complex",
			shouldMatch:   true,
		},
		{
			toolName:      "push_image",
			errorMsg:      "network connection timeout",
			expectedMatch: "network_connectivity_fix",
			shouldMatch:   true,
		},
		{
			toolName:      "deploy_container",
			errorMsg:      "port already in use and resource limit exceeded",
			expectedMatch: "resource_conflict_resolution",
			shouldMatch:   true,
		},
		{
			toolName:      "generate_manifests",
			errorMsg:      "manifest generation failed deployment error",
			expectedMatch: "manifest_deployment_recovery",
			shouldMatch:   true,
		},
		{
			toolName:    "unknown_tool",
			errorMsg:    "completely unrelated error",
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		tool := &MockTool{name: test.toolName}
		err := errors.NewError().Message(test.errorMsg).Build()

		hasApplicable := executor.HasApplicableChain(tool, err)

		if test.shouldMatch && !hasApplicable {
			t.Errorf("Expected chain to match for tool '%s' and error '%s'", test.toolName, test.errorMsg)
		}

		if !test.shouldMatch && hasApplicable {
			t.Errorf("Expected no chain to match for tool '%s' and error '%s'", test.toolName, test.errorMsg)
		}

		if test.shouldMatch {
			applicableChains := executor.findApplicableChains(tool, err)
			if len(applicableChains) == 0 {
				t.Errorf("Expected to find applicable chains for tool '%s' and error '%s'", test.toolName, test.errorMsg)
			} else {
				// Check if the expected chain is among the applicable ones
				found := false
				for _, chain := range applicableChains {
					if chain.Name == test.expectedMatch {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected chain '%s' to be applicable for tool '%s' and error '%s'", test.expectedMatch, test.toolName, test.errorMsg)
				}
			}
		}
	}
}

// TestFixChainExecutor_ExecuteChain tests executing a fix chain
func TestFixChainExecutor_ExecuteChain(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	ctx := context.Background()

	// Create a tool that succeeds on certain strategies
	executionLog := make([]string, 0)
	tool := &MockTool{
		name: "build_image",
		executeFunc: func(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
			// Log which strategy is being executed
			if strategy, ok := input.Data["strategy"].(string); ok {
				executionLog = append(executionLog, strategy)
			}

			// Simulate dockerfile_syntax_fix succeeding
			return api.ToolOutput{
				Success: true,
				Data:    map[string]interface{}{"fixed": true},
			}, nil
		},
	}

	args := map[string]interface{}{
		"session_id": "test-session",
	}

	err := errors.NewError().Message("docker build failed").Build()

	// Execute the chain
	result, chainErr := executor.ExecuteChain(ctx, tool, args, err)

	if chainErr != nil {
		t.Errorf("Expected no error executing chain, got: %v", chainErr)
	}

	if result == nil {
		t.Fatal("Expected chain result, got nil")
	}

	// Verify result structure
	if result.ChainName == "" {
		t.Error("Expected chain name in result")
	}

	if len(result.ExecutedSteps) == 0 {
		t.Error("Expected executed steps in result")
	}

	if result.TotalDuration <= 0 {
		t.Error("Expected positive total duration")
	}

	if len(result.Suggestions) == 0 {
		t.Error("Expected suggestions in result")
	}
}

// TestFixChainExecutor_ChainTimeout tests chain execution timeout
func TestFixChainExecutor_ChainTimeout(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	// Register a chain with very short timeout
	testChain := &FixChain{
		Name:        "timeout_test_chain",
		Description: "Chain for testing timeout",
		MaxRetries:  1,
		Timeout:     10 * time.Millisecond, // Very short timeout
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "timeout_test_unique"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "slow_strategy",
				Timeout:         100 * time.Millisecond, // Longer than chain timeout
				MaxRetries:      1,
				ContinueOnError: false,
			},
		},
	}

	executor.RegisterChain(testChain)

	ctx := context.Background()

	tool := &MockTool{
		name: "test_tool",
		executeFunc: func(execCtx context.Context, _ api.ToolInput) (api.ToolOutput, error) {
			// Simulate slow operation
			select {
			case <-time.After(200 * time.Millisecond):
				return api.ToolOutput{Success: true}, nil
			case <-execCtx.Done():
				return api.ToolOutput{}, execCtx.Err()
			}
		},
	}

	args := map[string]interface{}{"session_id": "test-session"}
	err := errors.NewError().Message("timeout_test_unique_error").Build()

	start := time.Now()
	result, chainErr := executor.ExecuteChain(ctx, tool, args, err)
	duration := time.Since(start)

	// Should complete quickly due to timeout
	if duration > 50*time.Millisecond {
		t.Errorf("Expected chain to timeout quickly, took: %v", duration)
	}

	if chainErr != nil {
		t.Errorf("Expected no chain error, got: %v", chainErr)
	}

	if result == nil {
		t.Fatal("Expected result even with timeout")
	}

	// Should not be successful due to timeout
	if result.Success {
		t.Error("Expected chain to not succeed due to timeout")
	}
}

// TestFixChainExecutor_ChainStepRetries tests retry logic for chain steps
func TestFixChainExecutor_ChainStepRetries(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	// Register a chain with retry logic
	testChain := &FixChain{
		Name:        "retry_test_chain",
		Description: "Chain for testing retries",
		MaxRetries:  1,
		Timeout:     5 * time.Second,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "retry_test"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "retry_strategy",
				Timeout:         1 * time.Second,
				MaxRetries:      3, // Should retry 3 times
				ContinueOnError: false,
			},
		},
	}

	executor.RegisterChain(testChain)

	ctx := context.Background()

	attemptCount := 0
	tool := &MockTool{
		name: "test_tool",
		executeFunc: func(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
			attemptCount++

			// Succeed on the 3rd attempt
			if attemptCount >= 3 {
				return api.ToolOutput{Success: true, Data: map[string]interface{}{"attempt": attemptCount}}, nil
			}

			return api.ToolOutput{}, errors.NewError().Message("retry needed").Build()
		},
	}

	args := map[string]interface{}{"session_id": "test-session"}
	err := errors.NewError().Message("retry_test error").Build()

	result, chainErr := executor.ExecuteChain(ctx, tool, args, err)

	if chainErr != nil {
		t.Errorf("Expected no chain error, got: %v", chainErr)
	}

	if result == nil {
		t.Fatal("Expected result")
	}

	// Should succeed after retries
	if !result.Success {
		t.Error("Expected chain to succeed after retries")
	}

	// Verify that retries happened
	if attemptCount < 3 {
		t.Errorf("Expected at least 3 attempts, got: %d", attemptCount)
	}

	// Check step result
	if len(result.ExecutedSteps) != 1 {
		t.Errorf("Expected 1 executed step, got: %d", len(result.ExecutedSteps))
	}

	step := result.ExecutedSteps[0]
	if step.RetryCount < 2 {
		t.Errorf("Expected retry count >= 2, got: %d", step.RetryCount)
	}
}

// TestFixChainExecutor_ContinueOnError tests the continue on error functionality
func TestFixChainExecutor_ContinueOnError(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	// Register a chain with continue on error
	testChain := &FixChain{
		Name:        "continue_on_error_test",
		Description: "Chain for testing continue on error",
		MaxRetries:  1,
		Timeout:     5 * time.Second,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "continue_test"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "failing_strategy",
				Timeout:         1 * time.Second,
				MaxRetries:      1,
				ContinueOnError: true, // Should continue even if this fails
			},
			{
				Name:            "succeeding_strategy",
				Timeout:         1 * time.Second,
				MaxRetries:      1,
				ContinueOnError: false,
			},
		},
	}

	executor.RegisterChain(testChain)

	ctx := context.Background()

	stepCount := 0
	tool := &MockTool{
		name: "test_tool",
		executeFunc: func(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
			stepCount++

			// Check which strategy is being executed
			if strategy, ok := input.Data["strategy"].(string); ok {
				if strategy == "failing_strategy" {
					return api.ToolOutput{}, errors.NewError().Message("first strategy fails").Build()
				}
				if strategy == "succeeding_strategy" {
					return api.ToolOutput{Success: true, Data: map[string]interface{}{"step": stepCount}}, nil
				}
			}

			// Fallback to step count for compatibility
			if stepCount == 1 {
				return api.ToolOutput{}, errors.NewError().Message("first strategy fails").Build()
			}

			return api.ToolOutput{Success: true, Data: map[string]interface{}{"step": stepCount}}, nil
		},
	}

	args := map[string]interface{}{"session_id": "test-session"}
	err := errors.NewError().Message("continue_test error").Build()

	result, chainErr := executor.ExecuteChain(ctx, tool, args, err)

	if chainErr != nil {
		t.Errorf("Expected no chain error, got: %v", chainErr)
	}

	if result == nil {
		t.Fatal("Expected result")
	}

	// Should succeed overall because second strategy succeeded
	if !result.Success {
		t.Error("Expected chain to succeed after first strategy failed but second succeeded")
	}

	// Verify both steps were executed
	if len(result.ExecutedSteps) != 2 {
		t.Errorf("Expected 2 executed steps, got: %d", len(result.ExecutedSteps))
	}

	// First step should have failed
	if result.ExecutedSteps[0].Success {
		t.Error("Expected first step to fail")
	}

	// Second step should have succeeded
	if !result.ExecutedSteps[1].Success {
		t.Error("Expected second step to succeed")
	}

	// Verify that both strategies were tried (with retries)
	// failing_strategy: 1 attempt + 1 retry = 2 calls
	// succeeding_strategy: 1 successful call = 1 call
	// Total = 3 calls
	if stepCount < 2 {
		t.Errorf("Expected at least 2 strategy attempts, got: %d", stepCount)
	}
}

// TestFixChainExecutor_GenerateChainSuggestions tests suggestion generation
func TestFixChainExecutor_GenerateChainSuggestions(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	chain := &FixChain{
		Name:        "suggestion_test",
		Description: "Test chain for suggestions",
	}

	// Test successful chain result
	successResult := &ChainResult{
		ChainName: "suggestion_test",
		Success:   true,
		ExecutedSteps: []ChainStepResult{
			{StepName: "step1", Success: true},
			{StepName: "step2", Success: true},
		},
	}

	suggestions := executor.generateChainSuggestions(successResult, chain)

	if len(suggestions) == 0 {
		t.Error("Expected suggestions for successful result")
	}

	// Should contain success message
	found := false
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, "completed successfully") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected success message in suggestions")
	}

	// Test failed chain result
	failedResult := &ChainResult{
		ChainName: "suggestion_test",
		Success:   false,
		ExecutedSteps: []ChainStepResult{
			{StepName: "dockerfile_syntax_fix", Success: false, Error: "syntax error"},
			{StepName: "image_base_fix", Success: false, Error: "image not found"},
		},
	}

	suggestions = executor.generateChainSuggestions(failedResult, chain)

	if len(suggestions) == 0 {
		t.Error("Expected suggestions for failed result")
	}

	// Should contain specific suggestions for failed steps
	foundDockerfileSuggestion := false
	foundImageSuggestion := false

	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, "Dockerfile syntax") {
			foundDockerfileSuggestion = true
		}
		if strings.Contains(suggestion, "base image") {
			foundImageSuggestion = true
		}
	}

	if !foundDockerfileSuggestion {
		t.Error("Expected Dockerfile-specific suggestion")
	}

	if !foundImageSuggestion {
		t.Error("Expected image-specific suggestion")
	}
}

// TestFixChainExecutor_RegisterChain tests custom chain registration
func TestFixChainExecutor_RegisterChain(t *testing.T) {
	helper := setupAutoFixHelper(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	executor := NewFixChainExecutor(logger, helper)

	customChain := &FixChain{
		Name:        "custom_test_chain",
		Description: "Custom chain for testing",
		MaxRetries:  2,
		Timeout:     30 * time.Second,
		Conditions: []ChainCondition{
			{Type: ConditionTypeErrorPattern, Pattern: "custom_error"},
		},
		Strategies: []ChainedFixStrategy{
			{
				Name:            "custom_strategy",
				Timeout:         10 * time.Second,
				MaxRetries:      1,
				ContinueOnError: false,
			},
		},
	}

	// Register the custom chain
	executor.RegisterChain(customChain)

	// Verify it's registered
	chains := executor.GetAvailableChains()
	if _, exists := chains["custom_test_chain"]; !exists {
		t.Error("Expected custom chain to be registered")
	}

	// Verify it can be found for matching errors
	tool := &MockTool{name: "test_tool"}
	err := errors.NewError().Message("custom_error occurred").Build()

	if !executor.HasApplicableChain(tool, err) {
		t.Error("Expected custom chain to be applicable")
	}

	applicableChains := executor.findApplicableChains(tool, err)
	found := false
	for _, chain := range applicableChains {
		if chain.Name == "custom_test_chain" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected custom chain to be found as applicable")
	}
}

// Benchmark tests for chain execution performance
func BenchmarkFixChainExecutor_ExecuteChain(b *testing.B) {
	helper := setupAutoFixHelper(b)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	executor := NewFixChainExecutor(logger, helper)

	ctx := context.Background()
	tool := &MockTool{
		name: "build_image",
		executeFunc: func(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
			return api.ToolOutput{Success: true}, nil
		},
	}

	args := map[string]interface{}{"session_id": "bench-session"}
	err := errors.NewError().Message("docker build failed").Build()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = executor.ExecuteChain(ctx, tool, args, err)
	}
}

func BenchmarkFixChainExecutor_HasApplicableChain(b *testing.B) {
	helper := setupAutoFixHelper(b)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	executor := NewFixChainExecutor(logger, helper)

	tool := &MockTool{name: "build_image"}
	err := errors.NewError().Message("docker build failed").Build()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		executor.HasApplicableChain(tool, err)
	}
}
