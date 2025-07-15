// Package workflow provides tests for AI-powered step enhancement middleware
package workflow

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStepEnhancer is a mock implementation of the StepEnhancer interface
type MockStepEnhancer struct {
	mock.Mock
}

func (m *MockStepEnhancer) EnhanceStep(ctx context.Context, step Step, state *WorkflowState) (Step, error) {
	args := m.Called(ctx, step, state)
	return args.Get(0).(Step), args.Error(1)
}

func (m *MockStepEnhancer) OptimizeWorkflow(ctx context.Context, steps []Step) (*WorkflowOptimization, error) {
	args := m.Called(ctx, steps)
	return args.Get(0).(*WorkflowOptimization), args.Error(1)
}

// MockEnhancedStep is a mock implementation of the Step interface for enhancement tests
type MockEnhancedStep struct {
	mock.Mock
	name       string
	maxRetries int
}

func (m *MockEnhancedStep) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock_enhanced_step"
}

func (m *MockEnhancedStep) MaxRetries() int {
	if m.maxRetries > 0 {
		return m.maxRetries
	}
	return 3
}

func (m *MockEnhancedStep) Execute(ctx context.Context, state *WorkflowState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func TestStepEnhancementMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mocks
	mockEnhancer := new(MockStepEnhancer)
	mockStep := &MockEnhancedStep{name: "test_step", maxRetries: 3}
	enhancedStep := &MockEnhancedStep{name: "enhanced_test_step", maxRetries: 5}

	// Setup expectations
	mockEnhancer.On("EnhanceStep", mock.Anything, mockStep, mock.Anything).Return(enhancedStep, nil)
	enhancedStep.On("Execute", mock.Anything, mock.Anything).Return(nil)

	// Create middleware
	middleware := StepEnhancementMiddleware(mockEnhancer, logger)

	// Create a simple handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	enhancedHandler := middleware(handler)

	// Create test state
	state := &WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 1,
		TotalSteps:  3,
		Logger:      logger,
	}

	// Execute
	err := enhancedHandler(context.Background(), mockStep, state)

	// Verify
	assert.NoError(t, err)
	mockEnhancer.AssertExpectations(t)
	enhancedStep.AssertExpectations(t)
}

func TestStepEnhancementMiddleware_WithNilEnhancer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock step
	mockStep := &MockEnhancedStep{name: "test_step", maxRetries: 3}
	mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)

	// Create middleware with nil enhancer
	middleware := StepEnhancementMiddleware(nil, logger)

	// Create a simple handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	enhancedHandler := middleware(handler)

	// Create test state
	state := &WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 1,
		TotalSteps:  3,
		Logger:      logger,
	}

	// Execute
	err := enhancedHandler(context.Background(), mockStep, state)

	// Verify
	assert.NoError(t, err)
	mockStep.AssertExpectations(t)
}

func TestWorkflowOptimizationMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mocks
	mockEnhancer := new(MockStepEnhancer)
	mockStep := &MockEnhancedStep{name: "test_step", maxRetries: 3}

	// Create optimization result
	optimization := &WorkflowOptimization{
		Suggestions: []OptimizationSuggestion{
			{
				StepName:    "test_step",
				Type:        "caching",
				Description: "Enable caching for better performance",
				Impact:      0.25,
			},
		},
		EstimatedImprovement: 0.25,
		Metadata: map[string]interface{}{
			"analysis_type": "rule_based",
		},
	}

	// Setup expectations
	mockEnhancer.On("OptimizeWorkflow", mock.Anything, mock.MatchedBy(func(steps []Step) bool {
		return len(steps) > 0
	})).Return(optimization, nil)
	mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)

	// Create middleware
	middleware := WorkflowOptimizationMiddleware(mockEnhancer, logger)

	// Create a simple handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	enhancedHandler := middleware(handler)

	// Create test state with steps
	steps := []Step{mockStep}
	state := &WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 1, // First step - should trigger optimization
		TotalSteps:  1,
		Logger:      logger,
	}
	state.SetAllSteps(steps)

	// Execute
	err := enhancedHandler(context.Background(), mockStep, state)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, state.GetOptimization())
	assert.Equal(t, 1, len(state.GetOptimization().Suggestions))
	assert.Equal(t, float64(0.25), state.GetOptimization().EstimatedImprovement)

	mockEnhancer.AssertExpectations(t)
	mockStep.AssertExpectations(t)
}

func TestWorkflowOptimizationMiddleware_NotFirstStep(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mocks
	mockEnhancer := new(MockStepEnhancer)
	mockStep := &MockEnhancedStep{name: "test_step", maxRetries: 3}

	// Setup expectations - OptimizeWorkflow should NOT be called
	mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)

	// Create middleware
	middleware := WorkflowOptimizationMiddleware(mockEnhancer, logger)

	// Create a simple handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	enhancedHandler := middleware(handler)

	// Create test state with steps
	steps := []Step{mockStep}
	state := &WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 2, // Not first step - should NOT trigger optimization
		TotalSteps:  3,
		Logger:      logger,
	}
	state.SetAllSteps(steps)

	// Execute
	err := enhancedHandler(context.Background(), mockStep, state)

	// Verify
	assert.NoError(t, err)
	assert.Nil(t, state.GetOptimization())

	// OptimizeWorkflow should NOT have been called
	mockEnhancer.AssertNotCalled(t, "OptimizeWorkflow", mock.Anything, mock.Anything)
	mockStep.AssertExpectations(t)
}

func TestCombinedEnhancementMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mocks
	mockEnhancer := new(MockStepEnhancer)
	mockStep := &MockEnhancedStep{name: "test_step", maxRetries: 3}
	enhancedStep := &MockEnhancedStep{name: "enhanced_test_step", maxRetries: 5}

	// Create optimization result
	optimization := &WorkflowOptimization{
		Suggestions: []OptimizationSuggestion{
			{
				StepName:    "test_step",
				Type:        "optimization",
				Description: "Sample optimization",
				Impact:      0.1,
			},
		},
		EstimatedImprovement: 0.1,
		Metadata: map[string]interface{}{
			"analysis_type": "combined",
		},
	}

	// Setup expectations
	mockEnhancer.On("OptimizeWorkflow", mock.Anything, mock.MatchedBy(func(steps []Step) bool {
		return len(steps) > 0
	})).Return(optimization, nil)
	// First call with original step, second call with enhanced step
	mockEnhancer.On("EnhanceStep", mock.Anything, mockStep, mock.Anything).Return(enhancedStep, nil).Once()
	mockEnhancer.On("EnhanceStep", mock.Anything, mock.AnythingOfType("*workflow.MockEnhancedStep"), mock.Anything).Return(enhancedStep, nil).Once()
	enhancedStep.On("Execute", mock.Anything, mock.Anything).Return(nil)

	// Create combined middleware
	middleware := CombinedEnhancementMiddleware(mockEnhancer, logger)

	// Create a simple handler that executes the step
	handler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Apply middleware
	enhancedHandler := middleware(handler)

	// Create test state
	steps := []Step{mockStep}
	state := &WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 1, // First step - should trigger optimization
		TotalSteps:  1,
		Logger:      logger,
	}
	state.SetAllSteps(steps)

	// Execute
	err := enhancedHandler(context.Background(), mockStep, state)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, state.GetOptimization())

	mockEnhancer.AssertExpectations(t)
	enhancedStep.AssertExpectations(t)
}

func TestWorkflowStateEnhancementMethods(t *testing.T) {
	// Test step management
	mockStep := &MockEnhancedStep{name: "test_step"}
	steps := []Step{mockStep}

	state := &WorkflowState{
		WorkflowID: "test-workflow",
		Logger:     slog.Default(),
	}

	// Test SetAllSteps and GetAllSteps
	state.SetAllSteps(steps)
	retrievedSteps := state.GetAllSteps()
	assert.Equal(t, 1, len(retrievedSteps))
	assert.Equal(t, "test_step", retrievedSteps[0].Name())

	// Test optimization management
	optimization := &WorkflowOptimization{
		EstimatedImprovement: 0.15,
		Suggestions: []OptimizationSuggestion{
			{
				StepName:    "test_step",
				Type:        "test",
				Description: "Test optimization",
				Impact:      0.15,
			},
		},
	}

	// Test SetOptimization and GetOptimization
	assert.Nil(t, state.GetOptimization())
	state.SetOptimization(optimization)
	retrievedOptimization := state.GetOptimization()
	assert.NotNil(t, retrievedOptimization)
	assert.Equal(t, float64(0.15), retrievedOptimization.EstimatedImprovement)
	assert.Equal(t, 1, len(retrievedOptimization.Suggestions))
}
