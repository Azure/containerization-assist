// Package application_test provides unit tests for the application layer dependency providers.
// These tests verify that the ProvideWorkflowDeps function correctly creates EventAwareOrchestrator
// instances when given valid dependencies.
package application

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockWorkflowOrchestrator implements workflow.WorkflowOrchestrator for testing
type MockWorkflowOrchestrator struct {
	mock.Mock
}

func (m *MockWorkflowOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	callArgs := m.Called(ctx, req, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(*workflow.ContainerizeAndDeployResult), callArgs.Error(1)
}

// MockEventPublisher implements domainevents.Publisher for testing
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, event domainevents.DomainEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishAsync(ctx context.Context, event domainevents.DomainEvent) {
	m.Called(ctx, event)
}

// MockProgressEmitterFactory implements workflow.ProgressEmitterFactory for testing
type MockProgressEmitterFactory struct {
	mock.Mock
}

func (m *MockProgressEmitterFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	args := m.Called(ctx, req, totalSteps)
	if args.Get(0) == nil {
		return &MockProgressEmitter{}
	}
	return args.Get(0).(api.ProgressEmitter)
}

// MockProgressEmitter implements api.ProgressEmitter for testing
type MockProgressEmitter struct {
	mock.Mock
}

func (m *MockProgressEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	args := m.Called(ctx, stage, percent, message)
	return args.Error(0)
}

func (m *MockProgressEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	args := m.Called(ctx, update)
	return args.Error(0)
}

func (m *MockProgressEmitter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestProvideWorkflowDeps_ValidDependencies(t *testing.T) {
	// Arrange
	logger := slog.Default()

	// Create dependencies
	mockOrchestrator := &MockWorkflowOrchestrator{}
	mockEventPublisher := &MockEventPublisher{}
	mockProgressEmitterFactory := &MockProgressEmitterFactory{}

	// Act
	workflowDeps := ProvideWorkflowDeps(
		mockOrchestrator,
		mockEventPublisher,
		mockProgressEmitterFactory,
		logger,
	)

	// Assert
	require.NotNil(t, workflowDeps, "WorkflowDeps should not be nil")

	// Verify all fields are set
	assert.NotNil(t, workflowDeps.Orchestrator, "Orchestrator should not be nil")
	assert.NotNil(t, workflowDeps.EventAwareOrchestrator, "EventAwareOrchestrator should not be nil")
	assert.NotNil(t, workflowDeps.EventPublisher, "EventPublisher should not be nil")
	assert.NotNil(t, workflowDeps.ProgressEmitterFactory, "ProgressEmitterFactory should not be nil")

	// Verify that orchestrators implement expected interfaces
	var _ workflow.WorkflowOrchestrator = workflowDeps.EventAwareOrchestrator
	var _ workflow.EventAwareOrchestrator = workflowDeps.EventAwareOrchestrator

	// Verify the dependencies are the same instances we passed in
	assert.Same(t, mockOrchestrator, workflowDeps.Orchestrator, "Should preserve original orchestrator")
	assert.Same(t, mockEventPublisher, workflowDeps.EventPublisher, "Should preserve original event publisher")
	assert.Same(t, mockProgressEmitterFactory, workflowDeps.ProgressEmitterFactory, "Should preserve original progress emitter factory")
}

func TestProvideWorkflowDeps_NonBaseOrchestrator(t *testing.T) {
	// Arrange
	logger := slog.Default()

	// Use a mock orchestrator that is NOT a BaseOrchestrator
	mockOrchestrator := &MockWorkflowOrchestrator{}
	mockEventPublisher := &MockEventPublisher{}
	mockProgressEmitterFactory := &MockProgressEmitterFactory{}

	// Act
	workflowDeps := ProvideWorkflowDeps(
		mockOrchestrator,
		mockEventPublisher,
		mockProgressEmitterFactory,
		logger,
	)

	// Assert
	require.NotNil(t, workflowDeps, "WorkflowDeps should not be nil")

	// Verify all fields are set
	assert.NotNil(t, workflowDeps.Orchestrator, "Orchestrator should not be nil")
	assert.NotNil(t, workflowDeps.EventAwareOrchestrator, "EventAwareOrchestrator should not be nil")

	// Verify that orchestrators implement expected interfaces even with adapter
	var _ workflow.WorkflowOrchestrator = workflowDeps.EventAwareOrchestrator
	var _ workflow.EventAwareOrchestrator = workflowDeps.EventAwareOrchestrator

	// Verify the base orchestrator is preserved
	assert.Same(t, mockOrchestrator, workflowDeps.Orchestrator, "Should preserve original orchestrator")

	// When using a non-BaseOrchestrator, the EventAwareOrchestrator should be an adapter
	assert.IsType(t, &eventOrchestratorAdapter{}, workflowDeps.EventAwareOrchestrator, "Should create adapter for non-BaseOrchestrator")
}

func TestProvideWorkflowDeps_EventAwareOrchestratorFunctionality(t *testing.T) {
	// Arrange
	logger := slog.Default()
	mockOrchestrator := &MockWorkflowOrchestrator{}
	mockEventPublisher := &MockEventPublisher{}
	mockProgressEmitterFactory := &MockProgressEmitterFactory{}

	// Set up mock expectations
	mockEventPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	// Act
	workflowDeps := ProvideWorkflowDeps(
		mockOrchestrator,
		mockEventPublisher,
		mockProgressEmitterFactory,
		logger,
	)

	// Test EventAwareOrchestrator functionality
	ctx := context.Background()
	err := workflowDeps.EventAwareOrchestrator.PublishWorkflowEvent(ctx, "test-workflow", "test-event", map[string]string{"key": "value"})

	// Assert
	assert.NoError(t, err, "PublishWorkflowEvent should not error")
	mockEventPublisher.AssertCalled(t, "Publish", ctx, mock.Anything)
}

func TestProvideWorkflowDeps_AdapterEventPublishing(t *testing.T) {
	// Arrange
	logger := slog.Default()

	// Use a mock orchestrator to trigger adapter creation
	mockOrchestrator := &MockWorkflowOrchestrator{}
	mockEventPublisher := &MockEventPublisher{}
	mockProgressEmitterFactory := &MockProgressEmitterFactory{}

	// Set up mock expectations
	mockEventPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	// Act
	workflowDeps := ProvideWorkflowDeps(
		mockOrchestrator,
		mockEventPublisher,
		mockProgressEmitterFactory,
		logger,
	)

	// Test adapter event publishing
	ctx := context.Background()
	err := workflowDeps.EventAwareOrchestrator.PublishWorkflowEvent(ctx, "test-workflow", "test-event", map[string]string{"key": "value"})

	// Assert
	assert.NoError(t, err, "Adapter PublishWorkflowEvent should not error")
	mockEventPublisher.AssertCalled(t, "Publish", ctx, mock.Anything)
}
