package workflow

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type WorkflowOrchestrator interface {
	Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error)
}

type EventAwareOrchestrator interface {
	WorkflowOrchestrator

	PublishWorkflowEvent(ctx context.Context, workflowID string, eventType string, payload interface{}) error
}

type StepProvider interface {
	GetStep(name string) (Step, error)

	ListSteps() []string
}

// Placeholder types for unused but referenced functionality

type WorkflowCheckpoint struct {
	WorkflowID  string                 `json:"workflow_id"`
	StepIndex   int                    `json:"step_index"`
	CurrentStep string                 `json:"current_step"`
	Timestamp   time.Time              `json:"timestamp"`
	State       map[string]interface{} `json:"state"`
}

type AdaptationStrategy struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type AdaptationStatistics struct {
	TotalAdaptations int `json:"total_adaptations"`
}

type OrchestratorConfig struct {
	MaxRetries int `json:"max_retries"`
}
