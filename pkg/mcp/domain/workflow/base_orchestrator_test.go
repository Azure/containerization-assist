package workflow

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaseOrchestrator tests the base orchestrator functionality
func TestBaseOrchestrator(t *testing.T) {
	logger := slog.Default()

	// Create a simple test step
	testStep := &MockStep{
		name: "test-step",
		executeFunc: func(ctx context.Context, state *WorkflowState) error {
			state.Result.Success = true
			return nil
		},
	}

	// Create mock step provider
	mockProvider := &MockStepProvider{
		steps: []Step{testStep},
	}

	// Create step factory with mock provider
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)

	// Create base orchestrator
	baseOrch := NewBaseOrchestrator(stepFactory, nil, logger)

	// Execute with test args
	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	result, err := baseOrch.Execute(context.Background(), nil, args)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

// TestBaseOrchestratorWithMiddleware tests middleware composition
func TestBaseOrchestratorWithMiddleware(t *testing.T) {
	logger := slog.Default()

	// Track middleware execution order
	var executionOrder []string

	// Create test middleware
	testMiddleware1 := func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			executionOrder = append(executionOrder, "middleware1-before")
			err := next(ctx, step, state)
			executionOrder = append(executionOrder, "middleware1-after")
			return err
		}
	}

	testMiddleware2 := func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			executionOrder = append(executionOrder, "middleware2-before")
			err := next(ctx, step, state)
			executionOrder = append(executionOrder, "middleware2-after")
			return err
		}
	}

	// Create test step
	testStep := &MockStep{
		name: "test-step",
		executeFunc: func(ctx context.Context, state *WorkflowState) error {
			executionOrder = append(executionOrder, "step-execute")
			return nil
		},
	}

	// Create a custom step provider that only returns one step
	singleStepProvider := &singleStepMockProvider{step: testStep}

	stepFactory := NewStepFactory(singleStepProvider, nil, nil, logger)
	baseOrch := NewBaseOrchestrator(stepFactory, nil, logger, WithMiddleware(testMiddleware1, testMiddleware2))

	// Execute
	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
	}

	_, err := baseOrch.Execute(context.Background(), nil, args)
	require.NoError(t, err)

	// Verify execution order - middleware applied in reverse order
	// Since we only have one step, we should see the middleware called once
	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"step-execute",
		"middleware2-after",
		"middleware1-after",
	}
	assert.Equal(t, expected, executionOrder)
}

// TestEventDecorator tests the event publishing decorator
func TestEventDecorator(t *testing.T) {
	logger := slog.Default()

	// Create event publisher
	publisher := events.NewPublisher(logger)

	// Note: In a real test, we would subscribe to events through the publisher's actual API
	// For this test, we'll verify the decorator is applied correctly

	// Create base orchestrator
	testStep := &MockStep{
		name: "test-step",
		executeFunc: func(ctx context.Context, state *WorkflowState) error {
			return nil
		},
	}
	mockProvider := &MockStepProvider{steps: []Step{testStep}}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	baseOrch := NewBaseOrchestrator(stepFactory, nil, logger)

	// Wrap with event decorator
	eventOrch := WithEvents(baseOrch, publisher)

	// Execute
	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
	}

	result, err := eventOrch.Execute(context.Background(), nil, args)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// The event decorator wraps the base orchestrator
	// In a full integration test, we would verify events were published
}

// TestSagaDecorator tests the saga transaction decorator
func TestSagaDecorator(t *testing.T) {
	logger := slog.Default()

	// Create dependencies
	publisher := events.NewPublisher(logger)
	coordinator := saga.NewSagaCoordinator(logger, publisher)

	// Create base orchestrator
	testStep := &MockStep{
		name: "test-step",
		executeFunc: func(ctx context.Context, state *WorkflowState) error {
			return nil
		},
	}
	mockProvider := &MockStepProvider{steps: []Step{testStep}}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	baseOrch := NewBaseOrchestrator(stepFactory, nil, logger)

	// Apply decorators in order
	eventOrch := WithEvents(baseOrch, publisher)
	sagaOrch := WithSaga(eventOrch, coordinator, logger)

	// Test ExecuteWithSaga method
	args := &ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
	}

	result, err := sagaOrch.ExecuteWithSaga(context.Background(), nil, args)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// MockStep implements the Step interface for testing
type MockStep struct {
	name        string
	executeFunc func(ctx context.Context, state *WorkflowState) error
	maxRetries  int
}

func (m *MockStep) Name() string {
	return m.name
}

func (m *MockStep) Execute(ctx context.Context, state *WorkflowState) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, state)
	}
	return nil
}

func (m *MockStep) MaxRetries() int {
	return m.maxRetries
}

// MockProgressSink for testing
type MockProgressSink struct {
	updates []progress.Update
}

func (m *MockProgressSink) Publish(ctx context.Context, u progress.Update) error {
	m.updates = append(m.updates, u)
	return nil
}

func (m *MockProgressSink) Close() error {
	return nil
}

// MockStepProvider for testing
type MockStepProvider struct {
	steps []Step
}

func (m *MockStepProvider) GetAnalyzeStep() Step {
	if len(m.steps) > 0 {
		return m.steps[0]
	}
	return &MockStep{name: "analyze"}
}
func (m *MockStepProvider) GetDockerfileStep() Step { return &MockStep{name: "dockerfile"} }
func (m *MockStepProvider) GetBuildStep() Step      { return &MockStep{name: "build"} }
func (m *MockStepProvider) GetScanStep() Step       { return &MockStep{name: "scan"} }
func (m *MockStepProvider) GetTagStep() Step        { return &MockStep{name: "tag"} }
func (m *MockStepProvider) GetPushStep() Step       { return &MockStep{name: "push"} }
func (m *MockStepProvider) GetManifestStep() Step   { return &MockStep{name: "manifest"} }
func (m *MockStepProvider) GetClusterStep() Step    { return &MockStep{name: "cluster"} }
func (m *MockStepProvider) GetDeployStep() Step     { return &MockStep{name: "deploy"} }
func (m *MockStepProvider) GetVerifyStep() Step     { return &MockStep{name: "verify"} }

// singleStepMockProvider returns only one step for the first call
type singleStepMockProvider struct {
	step   Step
	called bool
}

func (s *singleStepMockProvider) GetAnalyzeStep() Step {
	if !s.called {
		s.called = true
		return s.step
	}
	return nil
}
func (s *singleStepMockProvider) GetDockerfileStep() Step { return nil }
func (s *singleStepMockProvider) GetBuildStep() Step      { return nil }
func (s *singleStepMockProvider) GetScanStep() Step       { return nil }
func (s *singleStepMockProvider) GetTagStep() Step        { return nil }
func (s *singleStepMockProvider) GetPushStep() Step       { return nil }
func (s *singleStepMockProvider) GetManifestStep() Step   { return nil }
func (s *singleStepMockProvider) GetClusterStep() Step    { return nil }
func (s *singleStepMockProvider) GetDeployStep() Step     { return nil }
func (s *singleStepMockProvider) GetVerifyStep() Step     { return nil }
