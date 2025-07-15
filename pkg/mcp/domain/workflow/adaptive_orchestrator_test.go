// Package workflow provides tests for adaptive workflow orchestration
package workflow

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWorkflowOrchestrator is a mock implementation of WorkflowOrchestrator for testing
type MockWorkflowOrchestrator struct {
	mock.Mock
}

func (m *MockWorkflowOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	mArgs := m.Called(ctx, req, args)
	return mArgs.Get(0).(*ContainerizeAndDeployResult), mArgs.Error(1)
}

// MockErrorPatternRecognizer is a mock implementation of ErrorPatternRecognizer for testing
type MockErrorPatternRecognizer struct {
	mock.Mock
}

func (m *MockErrorPatternRecognizer) RecognizePattern(ctx context.Context, err error, stepContext *WorkflowState) (*ErrorClassification, error) {
	mArgs := m.Called(ctx, err, stepContext)
	return mArgs.Get(0).(*ErrorClassification), mArgs.Error(1)
}

// MockAdaptiveStepEnhancer is a mock implementation of StepEnhancer for testing
type MockAdaptiveStepEnhancer struct {
	mock.Mock
}

func (m *MockAdaptiveStepEnhancer) EnhanceStep(ctx context.Context, step Step, state *WorkflowState) (Step, error) {
	mArgs := m.Called(ctx, step, state)
	return mArgs.Get(0).(Step), mArgs.Error(1)
}

func (m *MockAdaptiveStepEnhancer) OptimizeWorkflow(ctx context.Context, steps []Step) (*WorkflowOptimization, error) {
	mArgs := m.Called(ctx, steps)
	return mArgs.Get(0).(*WorkflowOptimization), mArgs.Error(1)
}

func TestNewAdaptiveWorkflowOrchestrator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	assert.NotNil(t, adaptive)
	assert.NotNil(t, adaptive.adaptationEngine)
	assert.Equal(t, mockOrchestrator, adaptive.baseOrchestrator)
	assert.Equal(t, mockPatternRecognizer, adaptive.patternRecognizer)
	assert.Equal(t, mockStepEnhancer, adaptive.stepEnhancer)
}

func TestAdaptiveWorkflowOrchestrator_Execute_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Setup test data
	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "containerize_and_deploy",
		},
	}

	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	expectedResult := &ContainerizeAndDeployResult{
		Success: true,
	}

	// Mock successful execution
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return(expectedResult, nil)

	// Execute
	result, err := adaptive.Execute(context.Background(), req, args)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResult, result)

	// Verify adaptation record was created
	stats := adaptive.GetAdaptationStatistics()
	assert.Equal(t, 1, stats.TotalExecutions)

	mockOrchestrator.AssertExpectations(t)
}

func TestAdaptiveWorkflowOrchestrator_Execute_WithAdaptation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Setup test data
	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "containerize_and_deploy",
		},
	}

	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	testError := errors.New("network timeout occurred")
	expectedResult := &ContainerizeAndDeployResult{
		Success: true,
	}

	// Mock first execution fails, second succeeds
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return((*ContainerizeAndDeployResult)(nil), testError).Once()
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return(expectedResult, nil).Once()

	// Mock pattern recognition
	errorClassification := &ErrorClassification{
		Category:    "network",
		Confidence:  0.9,
		Patterns:    []string{"timeout", "network"},
		Suggestions: []string{"retry with exponential backoff", "increase timeout"},
	}
	mockPatternRecognizer.On("RecognizePattern", mock.Anything, testError, mock.Anything).Return(errorClassification, nil)

	// Execute
	result, err := adaptive.Execute(context.Background(), req, args)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResult, result)

	// Verify adaptation was applied
	stats := adaptive.GetAdaptationStatistics()
	assert.Equal(t, 1, stats.TotalExecutions)
	assert.Greater(t, stats.TotalStrategies, 0)

	mockOrchestrator.AssertExpectations(t)
	mockPatternRecognizer.AssertExpectations(t)
}

func TestAdaptiveWorkflowOrchestrator_Execute_AllAdaptationsFail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Setup test data
	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "containerize_and_deploy",
		},
	}

	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	testError := errors.New("persistent error")

	// Mock all executions fail
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return((*ContainerizeAndDeployResult)(nil), testError)

	// Mock pattern recognition
	errorClassification := &ErrorClassification{
		Category:    "unknown",
		Confidence:  0.5,
		Patterns:    []string{"unknown"},
		Suggestions: []string{"check logs"},
	}
	mockPatternRecognizer.On("RecognizePattern", mock.Anything, testError, mock.Anything).Return(errorClassification, nil)

	// Execute
	result, err := adaptive.Execute(context.Background(), req, args)

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	// The original error is returned when adaptations fail
	assert.Contains(t, err.Error(), "persistent error")

	mockOrchestrator.AssertExpectations(t)
	mockPatternRecognizer.AssertExpectations(t)
}

func TestAdaptiveWorkflowOrchestrator_Execute_WithLearnedStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Pre-populate a successful strategy
	strategy := &AdaptationStrategy{
		PatternID:    "network_timeout_test",
		StepName:     "all",
		ErrorPattern: "timeout occurred",
		Adaptations: []AdaptationEvent{
			{
				StepName:       "all",
				AdaptationType: AdaptationRetryStrategy,
				Reason:         "Network timeout adaptation",
				OriginalConfig: map[string]interface{}{"max_retries": 3},
				AdaptedConfig:  map[string]interface{}{"max_retries": 10},
				Confidence:     0.9,
				Timestamp:      time.Now(),
			},
		},
		SuccessRate: 0.8,
		UsageCount:  5,
		LastUsed:    time.Now(),
		Confidence:  0.9,
	}

	adaptive.adaptationEngine.successfulAdaptations["network_timeout_test"] = strategy

	// Setup test data
	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "containerize_and_deploy",
		},
	}

	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	testError := errors.New("network timeout occurred")
	expectedResult := &ContainerizeAndDeployResult{
		Success: true,
	}

	// Mock first execution fails, second succeeds with learned strategy
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return((*ContainerizeAndDeployResult)(nil), testError).Once()
	mockOrchestrator.On("Execute", mock.Anything, req, args).Return(expectedResult, nil).Once()

	// Mock pattern recognition
	errorClassification := &ErrorClassification{
		Category:    "network",
		Confidence:  0.9,
		Patterns:    []string{"timeout", "network"},
		Suggestions: []string{"retry with exponential backoff"},
	}
	mockPatternRecognizer.On("RecognizePattern", mock.Anything, testError, mock.Anything).Return(errorClassification, nil)

	// Execute
	result, err := adaptive.Execute(context.Background(), req, args)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResult, result)

	// Verify strategy was used (usage count should increase from 5 to 7)
	// The strategy is updated both during application and during learning
	updatedStrategy := adaptive.adaptationEngine.successfulAdaptations["network_timeout_test"]
	assert.Equal(t, 7, updatedStrategy.UsageCount)

	mockOrchestrator.AssertExpectations(t)
	mockPatternRecognizer.AssertExpectations(t)
}

func TestAdaptationEngine_FindMatchingStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewAdaptationEngine(logger)

	// Add some strategies
	strategy1 := &AdaptationStrategy{
		PatternID:    "network_timeout_1",
		StepName:     "all",
		ErrorPattern: "network timeout error",
		SuccessRate:  0.9,
		UsageCount:   5,
		LastUsed:     time.Now(),
		Confidence:   0.9,
	}

	strategy2 := &AdaptationStrategy{
		PatternID:    "build_error_1",
		StepName:     "build",
		ErrorPattern: "build failed due to missing dependency",
		SuccessRate:  0.7,
		UsageCount:   3,
		LastUsed:     time.Now().Add(-2 * time.Hour),
		Confidence:   0.7,
	}

	engine.successfulAdaptations["network_timeout_1"] = strategy1
	engine.successfulAdaptations["build_error_1"] = strategy2

	// Test finding matching strategy
	matchingStrategy := engine.findMatchingStrategy("network", "network timeout occurred")
	assert.NotNil(t, matchingStrategy)
	assert.Equal(t, "network_timeout_1", matchingStrategy.PatternID)

	// Test no matching strategy
	noMatchStrategy := engine.findMatchingStrategy("unknown", "completely different error")
	assert.Nil(t, noMatchStrategy)
}

func TestAdaptationEngine_StoreAndRetrieveStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewAdaptationEngine(logger)

	// Store a strategy
	adaptations := []AdaptationEvent{
		{
			StepName:       "build",
			AdaptationType: AdaptationResourceAllocation,
			Reason:         "Test adaptation",
			OriginalConfig: map[string]interface{}{"memory": "1g"},
			AdaptedConfig:  map[string]interface{}{"memory": "4g"},
			Confidence:     0.8,
			Timestamp:      time.Now(),
		},
	}

	engine.storeSuccessfulStrategy("build", "build timeout error", adaptations)

	// Verify strategy was stored
	assert.Greater(t, len(engine.successfulAdaptations), 0)

	// Find the stored strategy
	matchingStrategy := engine.findMatchingStrategy("build", "build timeout error")
	assert.NotNil(t, matchingStrategy)
	assert.Equal(t, "build", matchingStrategy.StepName)
	assert.Equal(t, 1.0, matchingStrategy.SuccessRate)
	assert.Equal(t, 1, matchingStrategy.UsageCount)
}

func TestAdaptationEngine_GetStatistics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := NewAdaptationEngine(logger)

	// Add some strategies
	strategy1 := &AdaptationStrategy{
		PatternID:   "strategy1",
		SuccessRate: 0.8,
		Adaptations: []AdaptationEvent{
			{AdaptationType: AdaptationRetryStrategy},
			{AdaptationType: AdaptationTimeout},
		},
	}

	strategy2 := &AdaptationStrategy{
		PatternID:   "strategy2",
		SuccessRate: 0.9,
		Adaptations: []AdaptationEvent{
			{AdaptationType: AdaptationRetryStrategy},
		},
	}

	engine.successfulAdaptations["strategy1"] = strategy1
	engine.successfulAdaptations["strategy2"] = strategy2

	// Add some execution history
	engine.adaptationHistory["execution1"] = &AdaptationRecord{WorkflowID: "execution1"}
	engine.adaptationHistory["execution2"] = &AdaptationRecord{WorkflowID: "execution2"}

	// Get statistics
	stats := engine.GetAdaptationStatistics()

	assert.Equal(t, 2, stats.TotalStrategies)
	assert.Equal(t, 2, stats.TotalExecutions)
	assert.InDelta(t, 0.85, stats.AverageSuccessRate, 0.001)                // (0.8 + 0.9) / 2 with tolerance
	assert.Equal(t, 2, stats.StrategyDistribution[AdaptationRetryStrategy]) // Each strategy has 1 retry adaptation
	assert.Equal(t, 1, stats.StrategyDistribution[AdaptationTimeout])
}

func TestAdaptiveWorkflowOrchestrator_UpdateAdaptationStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Create a strategy
	strategy := &AdaptationStrategy{
		PatternID:   "test_strategy",
		StepName:    "build",
		SuccessRate: 0.8,
		UsageCount:  5,
	}

	// Update strategy
	err := adaptive.UpdateAdaptationStrategy("test_strategy", strategy)
	assert.NoError(t, err)

	// Verify strategy was updated
	stats := adaptive.GetAdaptationStatistics()
	assert.Equal(t, 1, stats.TotalStrategies)

	// Test with nil strategy
	err = adaptive.UpdateAdaptationStrategy("test_strategy", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "strategy cannot be nil")
}

func TestAdaptiveWorkflowOrchestrator_ClearAdaptationHistory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Add some data
	adaptive.adaptationEngine.adaptationHistory["test"] = &AdaptationRecord{WorkflowID: "test"}
	adaptive.adaptationEngine.successfulAdaptations["test"] = &AdaptationStrategy{PatternID: "test"}

	// Clear history
	err := adaptive.ClearAdaptationHistory()
	assert.NoError(t, err)

	// Verify history was cleared
	stats := adaptive.GetAdaptationStatistics()
	assert.Equal(t, 0, stats.TotalStrategies)
	assert.Equal(t, 0, stats.TotalExecutions)
}

func TestGenerateAdaptations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockOrchestrator := new(MockWorkflowOrchestrator)
	mockPatternRecognizer := new(MockErrorPatternRecognizer)
	mockStepEnhancer := new(MockAdaptiveStepEnhancer)

	adaptive := NewAdaptiveWorkflowOrchestrator(
		mockOrchestrator,
		mockPatternRecognizer,
		mockStepEnhancer,
		logger,
	)

	// Test network error adaptations
	networkClassification := &ErrorClassification{
		Category:    "network",
		Confidence:  0.9,
		Patterns:    []string{"timeout", "network"},
		Suggestions: []string{"retry with backoff"},
	}

	adaptations := adaptive.generateAdaptations(networkClassification)
	assert.Greater(t, len(adaptations), 0)

	// Check that we have retry and timeout adaptations
	hasRetry := false
	hasTimeout := false
	for _, adaptation := range adaptations {
		if adaptation.AdaptationType == AdaptationRetryStrategy {
			hasRetry = true
		}
		if adaptation.AdaptationType == AdaptationTimeout {
			hasTimeout = true
		}
	}
	assert.True(t, hasRetry, "Should have retry adaptation for network errors")
	assert.True(t, hasTimeout, "Should have timeout adaptation for network errors")
}
