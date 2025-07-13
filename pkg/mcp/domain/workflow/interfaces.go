// Package workflow provides interfaces for workflow orchestration.
package workflow

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowOrchestrator defines the contract for workflow execution
type WorkflowOrchestrator interface {
	// Execute runs the complete containerization workflow
	Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error)
}

// EventAwareOrchestrator extends workflow orchestration with event publishing capabilities
type EventAwareOrchestrator interface {
	WorkflowOrchestrator

	// PublishWorkflowEvent publishes workflow-related events
	PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error
}

// SagaAwareOrchestrator extends workflow orchestration with saga transaction support
type SagaAwareOrchestrator interface {
	EventAwareOrchestrator

	// ExecuteWithSaga runs the workflow with saga transaction support
	ExecuteWithSaga(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error)

	// CompensateSaga triggers compensation for a failed saga
	CompensateSaga(ctx context.Context, sagaID string) error
}

// StepOrchestrator defines the interface for individual step execution
type StepOrchestrator interface {
	// ExecuteStep runs a single workflow step
	ExecuteStep(ctx context.Context, step Step, state *WorkflowState) error

	// CanExecuteStep checks if a step can be executed given the current state
	CanExecuteStep(step Step, state *WorkflowState) bool
}
