// Package workflow provides interfaces for workflow orchestration.
package workflow

import (
	"context"
	"time"

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

// AdaptiveOrchestrator extends workflow orchestration with adaptive capabilities
type AdaptiveOrchestrator interface {
	WorkflowOrchestrator

	// GetAdaptationStatistics returns statistics about workflow adaptations
	GetAdaptationStatistics() *AdaptationStatistics

	// UpdateAdaptationStrategy allows manual updates to adaptation strategies
	UpdateAdaptationStrategy(patternID string, strategy *AdaptationStrategy) error

	// ClearAdaptationHistory clears the adaptation history
	ClearAdaptationHistory() error
}

// StepOrchestrator defines the interface for individual step execution
type StepOrchestrator interface {
	// ExecuteStep runs a single workflow step
	ExecuteStep(ctx context.Context, step Step, state *WorkflowState) error

	// CanExecuteStep checks if a step can be executed given the current state
	CanExecuteStep(step Step, state *WorkflowState) bool
}

// BuildOptimizer defines the interface for AI-powered build optimization
type BuildOptimizer interface {
	AnalyzeBuildRequirements(ctx context.Context, dockerfilePath, repoPath string) (*BuildOptimization, error)
	PredictResourceUsage(ctx context.Context, optimization *BuildOptimization) error
}

// BuildOptimization represents build optimization recommendations
type BuildOptimization struct {
	RecommendedCPU    string
	RecommendedMemory string
	EstimatedDuration time.Duration
	CacheStrategy     string
	Parallelism       int
}

// StepProvider provides workflow steps from the infrastructure layer
type StepProvider interface {
	GetAnalyzeStep() Step
	GetDockerfileStep() Step
	GetBuildStep() Step
	GetScanStep() Step
	GetTagStep() Step
	GetPushStep() Step
	GetManifestStep() Step
	GetClusterStep() Step
	GetDeployStep() Step
	GetVerifyStep() Step
}

// MetricsCollector collects metrics for workflow steps
type MetricsCollector interface {
	RecordStepDuration(stepName string, duration time.Duration)
	RecordStepSuccess(stepName string)
	RecordStepFailure(stepName string)
}

// Tracer provides distributed tracing capabilities for workflows
type Tracer interface {
	// StartSpan creates a new span and returns the updated context and span
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

// Span represents a tracing span
type Span interface {
	// End completes the span
	End()
	// RecordError records an error on the span
	RecordError(err error)
	// SetAttribute sets a key-value attribute on the span
	SetAttribute(key string, value interface{})
}

// ContainerManager abstracts container operations
type ContainerManager interface {
	// RemoveImage removes a container image by reference
	RemoveImage(ctx context.Context, imageRef string) error
}

// DeploymentManager abstracts Kubernetes deployment operations
type DeploymentManager interface {
	// DeleteDeployment removes a deployment
	DeleteDeployment(ctx context.Context, namespace, name string) error
	// DeleteService removes a service
	DeleteService(ctx context.Context, namespace, name string) error
}

// StateStore abstracts workflow state persistence
type StateStore interface {
	// SaveCheckpoint persists a workflow checkpoint
	SaveCheckpoint(checkpoint *WorkflowCheckpoint) error
	// LoadLatestCheckpoint retrieves the most recent checkpoint for a workflow
	LoadLatestCheckpoint(workflowID string) (*WorkflowCheckpoint, error)
	// CleanupOldCheckpoints removes checkpoints older than the specified duration
	CleanupOldCheckpoints(maxAge time.Duration) error
}

// FileManager abstracts file system operations
type FileManager interface {
	// RemoveFile removes a file if it exists
	RemoveFile(ctx context.Context, path string) error
}
