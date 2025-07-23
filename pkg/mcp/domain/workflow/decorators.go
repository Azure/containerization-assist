// Package workflow provides decorator patterns for orchestrators
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/mark3labs/mcp-go/mcp"
)

// WithEvents wraps a base orchestrator with event publishing capabilities
func WithEvents(base *BaseOrchestrator, publisher events.Publisher) EventAwareOrchestrator {
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
	eventPublisher events.Publisher
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
