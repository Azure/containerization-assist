// Package application_test provides unit tests for the application layer dependency providers.
// These tests verify the simplified dependency injection approach.
package application

import (
	"context"
	"testing"

	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestEventOrchestratorAdapter_PublishWorkflowEvent(t *testing.T) {
	// Arrange
	mockOrchestrator := &MockWorkflowOrchestrator{}
	mockEventPublisher := &MockEventPublisher{}

	// Set up mock expectations
	mockEventPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	adapter := &eventOrchestratorAdapter{
		base:      mockOrchestrator,
		publisher: mockEventPublisher,
		logger:    nil,
	}

	// Act
	ctx := context.Background()
	err := adapter.PublishWorkflowEvent(ctx, "test-workflow", "test-event", map[string]string{"key": "value"})

	// Assert
	assert.NoError(t, err, "PublishWorkflowEvent should not error")
	mockEventPublisher.AssertCalled(t, "Publish", ctx, mock.Anything)
}
