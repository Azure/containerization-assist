// Package workflow provides shared utilities for workflow orchestration
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// NoOpSink is a no-operation progress sink for fallback cases
type NoOpSink struct{}

func (n *NoOpSink) Publish(ctx context.Context, u progress.Update) error { return nil }

func (n *NoOpSink) Close() error { return nil }

// NoOpEmitter is a no-operation progress emitter for fallback cases
type NoOpEmitter struct{}

func (n *NoOpEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return nil
}

func (n *NoOpEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	return nil
}

func (n *NoOpEmitter) Close() error { return nil }

// GenerateWorkflowID creates a unique workflow identifier based on repository URL or path
func GenerateWorkflowID(repoInput string) string {
	// Extract repo name from URL or path
	parts := strings.Split(repoInput, "/")
	repoName := "unknown"
	if len(parts) > 0 {
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
		// Handle empty names or special cases
		if repoName == "" || repoName == "." {
			if len(parts) > 1 {
				repoName = parts[len(parts)-2]
			}
		}
	}

	// Generate unique workflow ID
	timestamp := time.Now().Unix()
	return fmt.Sprintf("workflow-%s-%d", repoName, timestamp)
}

// CreateWorkflowStartedEvent creates a standardized workflow started event
func CreateWorkflowStartedEvent(workflowID string, repoURL string, branch string, userID string) events.WorkflowStartedEvent {
	return events.WorkflowStartedEvent{
		ID:        GenerateEventID(),
		Timestamp: time.Now(),
		Workflow:  workflowID,
		RepoURL:   repoURL,
		Branch:    branch,
		UserID:    userID,
	}
}

// CreateWorkflowCompletedEvent creates a standardized workflow completed event
func CreateWorkflowCompletedEvent(workflowID string, duration time.Duration, success bool, imageRef string, namespace string, endpoint string, errorMsg string) events.WorkflowCompletedEvent {
	event := events.WorkflowCompletedEvent{
		ID:            GenerateEventID(),
		Timestamp:     time.Now(),
		Workflow:      workflowID,
		Success:       success,
		TotalDuration: duration,
		ErrorMsg:      errorMsg,
		ImageRef:      imageRef,
		Namespace:     namespace,
		Endpoint:      endpoint,
	}

	return event
}

// CreateStepCompletedEvent creates a step completed event (success or failure)
func CreateStepCompletedEvent(stepName string, workflowID string, stepNumber int, totalSteps int, duration time.Duration, err error) events.WorkflowStepCompletedEvent {
	event := events.WorkflowStepCompletedEvent{
		ID:         GenerateEventID(),
		Timestamp:  time.Now(),
		Workflow:   workflowID,
		StepName:   stepName,
		Duration:   duration,
		Success:    err == nil,
		StepNumber: stepNumber,
		TotalSteps: totalSteps,
	}

	if err != nil {
		event.ErrorMsg = err.Error()
	}

	// Calculate progress
	if totalSteps > 0 {
		event.Progress = float64(stepNumber) / float64(totalSteps)
	}

	return event
}

// GenerateEventID creates a unique event identifier
func GenerateEventID() string {
	// Use the EventUtils implementation for consistency
	return events.EventUtils{}.GenerateEventID()
}

// ExtractUserID extracts user ID from context
func ExtractUserID(ctx context.Context) string {
	// For now, return empty string - can be enhanced later
	return ""
}

// GetRepositoryIdentifier returns the repository identifier from workflow args
func GetRepositoryIdentifier(args *ContainerizeAndDeployArgs) string {
	if args.RepoPath != "" {
		return args.RepoPath
	}
	return args.RepoURL
}
