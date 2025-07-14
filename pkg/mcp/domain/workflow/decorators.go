// Package workflow provides decorator patterns for orchestrators
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/mark3labs/mcp-go/mcp"
)

// WithEvents wraps an orchestrator with event publishing capabilities
func WithEvents(base WorkflowOrchestrator, publisher *events.Publisher) EventAwareOrchestrator {
	return &eventDecorator{
		base:           base,
		eventPublisher: publisher,
		eventUtils:     events.EventUtils{},
		logger:         slog.Default().With("component", "event_orchestrator"),
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
	// Generate workflow ID
	workflowID := generateWorkflowID(args.RepoURL)

	// Publish workflow started event
	d.publishWorkflowStartedEvent(ctx, workflowID, args)

	startTime := time.Now()

	// Create a wrapper that intercepts the progress tracker to add event publishing
	wrappedReq := req // For now, use the same request

	// Execute the base workflow
	result, err := d.base.Execute(ctx, wrappedReq, args)

	// Publish workflow completed event
	if result != nil && result.Success {
		d.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), true, result, "")
	} else {
		errorMsg := ""
		if err != nil {
			errorMsg = err.Error()
		} else if result != nil && result.Error != "" {
			errorMsg = result.Error
		}
		d.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), false, result, errorMsg)
	}

	return result, err
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

func (d *eventDecorator) publishWorkflowStartedEvent(ctx context.Context, workflowID string, args *ContainerizeAndDeployArgs) {
	event := events.WorkflowStartedEvent{
		ID:        d.eventUtils.GenerateEventID(),
		Timestamp: time.Now(),
		Workflow:  workflowID,
		RepoURL:   args.RepoURL,
		Branch:    args.Branch,
	}

	d.eventPublisher.PublishAsync(ctx, event)
}

func (d *eventDecorator) publishWorkflowCompletedEvent(ctx context.Context, workflowID string, duration time.Duration, success bool, result *ContainerizeAndDeployResult, errorMsg string) {
	event := events.WorkflowCompletedEvent{
		ID:            d.eventUtils.GenerateEventID(),
		Timestamp:     time.Now(),
		Workflow:      workflowID,
		Success:       success,
		TotalDuration: duration,
		ErrorMsg:      errorMsg,
	}

	if result != nil {
		event.ImageRef = result.ImageRef
		event.Namespace = result.Namespace
		event.Endpoint = result.Endpoint
	}

	if err := d.eventPublisher.Publish(ctx, event); err != nil {
		d.logger.Error("Failed to publish workflow completed event", "error", err, "workflow_id", workflowID)
	}
}

// WithSaga wraps an orchestrator with saga transaction support
func WithSaga(base EventAwareOrchestrator, coordinator *saga.SagaCoordinator, logger *slog.Logger) SagaAwareOrchestrator {
	return &sagaDecorator{
		base:        base,
		coordinator: coordinator,
		logger:      logger.With("component", "saga_orchestrator"),
	}
}

// sagaDecorator adds saga transaction support to any orchestrator
type sagaDecorator struct {
	base        EventAwareOrchestrator
	coordinator *saga.SagaCoordinator
	logger      *slog.Logger
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
	// Generate workflow and saga IDs
	workflowID := generateWorkflowID(args.RepoURL)
	sagaID := fmt.Sprintf("saga-%s-%d", workflowID, time.Now().Unix())

	d.logger.Info("Starting saga-enabled workflow",
		"workflow_id", workflowID,
		"saga_id", sagaID,
		"repo_url", args.RepoURL)

	// Create saga with compensating actions
	sagaSteps := d.createWorkflowSagaSteps(args)

	// Start saga execution
	sagaExec, err := d.coordinator.StartSaga(ctx, sagaID, workflowID, sagaSteps)
	if err != nil {
		d.logger.Error("Failed to start saga", "error", err, "saga_id", sagaID)
		// Fall back to regular execution
		return d.base.Execute(ctx, req, args)
	}

	// Store saga context for potential cancellation
	ctx = context.WithValue(ctx, "saga_id", sagaID)
	ctx = context.WithValue(ctx, "saga_execution", sagaExec)

	// Execute workflow with saga context
	result, execErr := d.base.Execute(ctx, req, args)

	// Handle saga completion
	if execErr != nil || (result != nil && !result.Success) {
		// Workflow failed, cancel saga to trigger compensation
		d.logger.Info("Workflow failed, cancelling saga to trigger compensation", "saga_id", sagaID)
		if cancelErr := d.coordinator.CancelSaga(ctx, sagaID); cancelErr != nil {
			d.logger.Error("Failed to cancel saga", "error", cancelErr, "saga_id", sagaID)
		}
	}
	// Note: Saga coordinator doesn't have a CompleteSaga method - it auto-completes

	return result, execErr
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

// createWorkflowSagaSteps creates saga steps for the workflow
func (d *sagaDecorator) createWorkflowSagaSteps(args *ContainerizeAndDeployArgs) []saga.SagaStep {
	return []saga.SagaStep{
		&workflowSagaStep{
			name:   "build_image",
			logger: d.logger,
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				// Actual work is done by workflow steps
				return nil
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				d.logger.Info("Compensating: Removing built Docker image")
				// Implementation would remove the Docker image
				return nil
			},
			canCompensate: true,
		},
		&workflowSagaStep{
			name:   "push_image",
			logger: d.logger,
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				// Actual work is done by workflow steps
				return nil
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				d.logger.Info("Compensating: Removing image from registry")
				// Implementation would remove image from registry
				return nil
			},
			canCompensate: true,
		},
		&workflowSagaStep{
			name:   "deploy_application",
			logger: d.logger,
			executeFunc: func(ctx context.Context, data map[string]interface{}) error {
				// Actual work is done by workflow steps
				return nil
			},
			compensateFunc: func(ctx context.Context, data map[string]interface{}) error {
				d.logger.Info("Compensating: Removing deployed application")
				// Implementation would delete Kubernetes resources
				return nil
			},
			canCompensate: true,
		},
	}
}

// Helper function to generate workflow ID
func generateWorkflowID(repoURL string) string {
	// Extract repo name from URL
	parts := strings.Split(repoURL, "/")
	repoName := "unknown"
	if len(parts) > 0 {
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
	}

	// Generate unique workflow ID
	timestamp := time.Now().Unix()
	return fmt.Sprintf("workflow-%s-%d", repoName, timestamp)
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
