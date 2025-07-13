// Package workflow provides event-driven orchestration for containerization workflows.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/mark3labs/mcp-go/mcp"
)

// EventOrchestrator extends the basic orchestrator with event publishing capabilities
type EventOrchestrator struct {
	*Orchestrator
	eventPublisher *events.Publisher
	eventUtils     events.EventUtils
}

// Ensure EventOrchestrator implements EventAwareOrchestrator
var _ EventAwareOrchestrator = (*EventOrchestrator)(nil)

// NewEventOrchestrator creates a new event-driven workflow orchestrator
func NewEventOrchestrator(logger *slog.Logger, eventPublisher *events.Publisher) *EventOrchestrator {
	return &EventOrchestrator{
		Orchestrator:   NewOrchestrator(logger),
		eventPublisher: eventPublisher,
		eventUtils:     events.EventUtils{},
	}
}

// Execute runs the complete workflow with event publishing
func (o *EventOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	workflowID := o.generateWorkflowID(args)
	startTime := time.Now()

	o.logger.Info("Starting event-driven containerize_and_deploy workflow",
		"workflow_id", workflowID,
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"scan", args.Scan)

	// Publish workflow started event
	startEvent := events.WorkflowStartedEvent{
		ID:        o.eventUtils.GenerateEventID(),
		Timestamp: startTime,
		Workflow:  workflowID,
		RepoURL:   args.RepoURL,
		Branch:    args.Branch,
		UserID:    o.extractUserID(ctx),
	}

	if err := o.eventPublisher.Publish(ctx, startEvent); err != nil {
		o.logger.Error("Failed to publish workflow started event", "error", err)
		// Continue execution despite event publishing failure
	}

	// Initialize workflow state with workflow ID
	state := NewWorkflowState(ctx, req, args, o.logger)
	state.WorkflowID = workflowID // Add workflow ID to state
	defer state.ProgressTracker.Finish()

	// Begin progress tracking
	state.ProgressTracker.Begin("Starting containerization and deployment workflow")

	// Execute each step with event publishing
	for i, step := range o.steps {
		stepStartTime := time.Now()
		stepNumber := i + 1

		o.logger.Info("Executing step",
			"workflow_id", workflowID,
			"step", stepNumber,
			"step_name", step.Name(),
			"total_steps", len(o.steps))

		if err := o.executeStepWithRetry(ctx, step, state); err != nil {
			// Publish step failed event
			progress := float64(state.ProgressTracker.GetCurrent()) / float64(state.ProgressTracker.GetTotal()) * 100
			o.publishStepCompletedEvent(ctx, workflowID, step, stepNumber, len(o.steps),
				time.Since(stepStartTime), false, err.Error(), progress)

			// Publish workflow failed event
			o.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), false, state.Result, err.Error())

			state.Result.Success = false
			state.Result.Error = err.Error()
			return state.Result, nil
		}

		// Publish step completed event
		progress := float64(state.ProgressTracker.GetCurrent()) / float64(state.ProgressTracker.GetTotal()) * 100
		o.publishStepCompletedEvent(ctx, workflowID, step, stepNumber, len(o.steps),
			time.Since(stepStartTime), true, "", progress)
	}

	// Mark workflow as successful and publish completion event
	state.Result.Success = true
	state.ProgressTracker.Complete("Containerization and deployment completed successfully")

	o.publishWorkflowCompletedEvent(ctx, workflowID, time.Since(startTime), true, state.Result, "")

	o.logger.Info("Event-driven workflow completed successfully",
		"workflow_id", workflowID,
		"duration", time.Since(startTime))

	return state.Result, nil
}

// publishStepCompletedEvent publishes a step completion event
func (o *EventOrchestrator) publishStepCompletedEvent(ctx context.Context, workflowID string, step Step, stepNumber, totalSteps int, duration time.Duration, success bool, errorMsg string, progress float64) {
	event := events.WorkflowStepCompletedEvent{
		ID:         o.eventUtils.GenerateEventID(),
		Timestamp:  time.Now(),
		Workflow:   workflowID,
		StepName:   step.Name(),
		Duration:   duration,
		Success:    success,
		ErrorMsg:   errorMsg,
		Progress:   progress,
		StepNumber: stepNumber,
		TotalSteps: totalSteps,
	}

	// Publish asynchronously to not block workflow execution
	o.eventPublisher.PublishAsync(ctx, event)
}

// publishWorkflowCompletedEvent publishes a workflow completion event
func (o *EventOrchestrator) publishWorkflowCompletedEvent(ctx context.Context, workflowID string, duration time.Duration, success bool, result *ContainerizeAndDeployResult, errorMsg string) {
	event := events.WorkflowCompletedEvent{
		ID:            o.eventUtils.GenerateEventID(),
		Timestamp:     time.Now(),
		Workflow:      workflowID,
		Success:       success,
		TotalDuration: duration,
		ImageRef:      result.ImageRef,
		Namespace:     result.Namespace,
		Endpoint:      result.Endpoint,
		ErrorMsg:      errorMsg,
	}

	// Publish synchronously for workflow completion to ensure it's recorded
	if err := o.eventPublisher.Publish(ctx, event); err != nil {
		o.logger.Error("Failed to publish workflow completed event", "error", err, "workflow_id", workflowID)
	}
}

// PublishWorkflowEvent publishes workflow-related events (implements EventAwareOrchestrator)
func (o *EventOrchestrator) PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error {
	// Create a generic workflow event
	event := &genericWorkflowEvent{
		id:         o.eventUtils.GenerateEventID(),
		timestamp:  time.Now(),
		workflowID: workflowID,
		eventType:  eventType,
		payload:    payload,
	}

	return o.eventPublisher.Publish(ctx, event)
}

// genericWorkflowEvent implements DomainEvent for arbitrary workflow events
type genericWorkflowEvent struct {
	id         string
	timestamp  time.Time
	workflowID string
	eventType  string
	payload    interface{}
}

func (e *genericWorkflowEvent) EventID() string       { return e.id }
func (e *genericWorkflowEvent) OccurredAt() time.Time { return e.timestamp }
func (e *genericWorkflowEvent) WorkflowID() string    { return e.workflowID }
func (e *genericWorkflowEvent) EventType() string     { return e.eventType }

// generateWorkflowID creates a unique workflow identifier
func (o *EventOrchestrator) generateWorkflowID(args *ContainerizeAndDeployArgs) string {
	timestamp := time.Now().Format("20060102-150405")
	repoName := extractWorkflowRepoName(args.RepoURL)
	return fmt.Sprintf("workflow-%s-%s-%s", repoName, timestamp, o.eventUtils.GenerateEventID()[0:6])
}

// extractWorkflowRepoName extracts repository name from URL for workflow ID
func extractWorkflowRepoName(repoURL string) string {
	// Simple extraction: get last part of URL without .git
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if strings.HasSuffix(name, ".git") {
			name = name[:len(name)-4]
		}
		return name
	}
	return "unknown"
}

// extractUserID extracts user ID from context (placeholder for actual implementation)
func (o *EventOrchestrator) extractUserID(ctx context.Context) string {
	// Extract user ID from authentication context.
	// In a real implementation, this would parse JWT tokens,
	// API keys, or other authentication mechanisms.
	// For now, we use "system" as the default user.
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		return userID
	}
	return "system"
}
