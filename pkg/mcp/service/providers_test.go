// Package service_test provides unit tests for the service layer dependency providers.
// These tests verify the simplified dependency injection approach.
package service

import (
	"context"

	domainevents "github.com/Azure/containerization-assist/pkg/mcp/domain/events"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
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

// TestEventOrchestratorAdapter_PublishWorkflowEvent was removed
// The eventOrchestratorAdapter was part of over-engineered patterns that were simplified
// Event publishing is now handled directly through the simplified architecture
