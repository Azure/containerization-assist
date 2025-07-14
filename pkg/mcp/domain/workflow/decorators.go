// Package workflow provides decorator patterns for orchestrators
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/mark3labs/mcp-go/mcp"
)

// WithEvents wraps a base orchestrator with event publishing capabilities
func WithEvents(base *BaseOrchestrator, publisher *events.Publisher) EventAwareOrchestrator {
	// Add event middleware to the base orchestrator
	base.AddMiddleware(EventMiddleware(publisher, base.logger))

	return &eventDecorator{
		base:           base,
		eventPublisher: publisher,
		eventUtils:     events.EventUtils{},
		logger:         base.logger.With("component", "event_decorator"),
	}
}

// eventDecorator adds event publishing to any orchestrator
type eventDecorator struct {
	base           WorkflowOrchestrator
	eventPublisher *events.Publisher
	eventUtils     events.EventUtils
	logger         *slog.Logger
}

// Execute runs the workflow with event publishing
func (d *eventDecorator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Use workflow event middleware to wrap the entire execution
	handler := WorkflowEventMiddleware(d.eventPublisher, d.logger)(d.base.Execute)
	return handler(ctx, req, args)
}

// PublishWorkflowEvent publishes arbitrary workflow events
func (d *eventDecorator) PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error {
	event := &decoratorWorkflowEvent{
		id:         d.eventUtils.GenerateEventID(),
		timestamp:  time.Now(),
		workflowID: workflowID,
		eventType:  eventType,
		payload:    payload,
	}

	return d.eventPublisher.Publish(ctx, event)
}

// WithSaga wraps an orchestrator with saga transaction support
func WithSaga(base EventAwareOrchestrator, coordinator *saga.SagaCoordinator, logger *slog.Logger) SagaAwareOrchestrator {
	// If base is an eventDecorator that wraps BaseOrchestrator, add saga middleware
	if decorator, ok := base.(*eventDecorator); ok {
		if baseOrch, ok := decorator.base.(*BaseOrchestrator); ok {
			baseOrch.AddMiddleware(SagaMiddleware(coordinator, logger))
		}
	}

	return &sagaDecorator{
		base:        base,
		coordinator: coordinator,
		logger:      logger.With("component", "saga_decorator"),
	}
}

// WithSagaAndDependencies wraps an orchestrator with saga transaction support and infrastructure dependencies
func WithSagaAndDependencies(base EventAwareOrchestrator, coordinator *saga.SagaCoordinator, containerManager ContainerManager, deploymentManager DeploymentManager, logger *slog.Logger) SagaAwareOrchestrator {
	// If base is an eventDecorator that wraps BaseOrchestrator, add saga middleware
	if decorator, ok := base.(*eventDecorator); ok {
		if baseOrch, ok := decorator.base.(*BaseOrchestrator); ok {
			baseOrch.AddMiddleware(SagaMiddleware(coordinator, logger))
		}
	}

	return &sagaDecorator{
		base:              base,
		coordinator:       coordinator,
		containerManager:  containerManager,
		deploymentManager: deploymentManager,
		logger:            logger.With("component", "saga_decorator"),
	}
}

// sagaDecorator adds saga transaction support to any orchestrator
type sagaDecorator struct {
	base              EventAwareOrchestrator
	coordinator       *saga.SagaCoordinator
	containerManager  ContainerManager
	deploymentManager DeploymentManager
	logger            *slog.Logger
}

// Execute runs the workflow (delegates to base)
func (d *sagaDecorator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// For backward compatibility, Execute without saga just delegates to base
	return d.base.Execute(ctx, req, args)
}

// PublishWorkflowEvent delegates to base
func (d *sagaDecorator) PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error {
	return d.base.PublishWorkflowEvent(ctx, workflowID, eventType, payload)
}

// ExecuteWithSaga runs the workflow with saga transaction support
func (d *sagaDecorator) ExecuteWithSaga(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Use workflow saga middleware to wrap the entire execution
	handler := WorkflowSagaMiddleware(d.coordinator, d.logger)(d.base.Execute)
	return handler(ctx, req, args)
}

// CancelWorkflow cancels a running workflow and triggers compensation
func (d *sagaDecorator) CancelWorkflow(ctx context.Context, workflowID string) error {
	d.logger.Info("Cancelling workflow", "workflow_id", workflowID)

	// Extract saga ID from context or lookup
	sagaID, ok := ctx.Value("saga_id").(string)
	if !ok {
		return fmt.Errorf("no saga associated with workflow %s", workflowID)
	}

	// Cancel saga to trigger compensation
	return d.coordinator.CancelSaga(ctx, sagaID)
}

// CompensateSaga manually triggers saga compensation
func (d *sagaDecorator) CompensateSaga(ctx context.Context, sagaID string) error {
	d.logger.Info("Manually compensating saga", "saga_id", sagaID)
	return d.coordinator.CancelSaga(ctx, sagaID)
}

// decoratorWorkflowEvent is a generic event for custom workflow events
type decoratorWorkflowEvent struct {
	id         string
	timestamp  time.Time
	workflowID string
	eventType  string
	payload    interface{}
}

func (e *decoratorWorkflowEvent) EventID() string       { return e.id }
func (e *decoratorWorkflowEvent) EventType() string     { return e.eventType }
func (e *decoratorWorkflowEvent) Timestamp() time.Time  { return e.timestamp }
func (e *decoratorWorkflowEvent) OccurredAt() time.Time { return e.timestamp }
func (e *decoratorWorkflowEvent) WorkflowID() string    { return e.workflowID }
func (e *decoratorWorkflowEvent) Serialize() ([]byte, error) {
	// Implementation would serialize the event
	return []byte(fmt.Sprintf(`{"id":"%s","type":"%s","workflow":"%s"}`, e.id, e.eventType, e.workflowID)), nil
}
