// Package workflow provides orchestration for the containerization workflow.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
)

// noOpSink is a no-operation sink for fallback cases
type noOpSink struct{}

func (n *noOpSink) Publish(ctx context.Context, u progress.Update) error { return nil }
func (n *noOpSink) Close() error                                         { return nil }

// Step defines the interface for individual workflow steps
type Step interface {
	Name() string
	Execute(ctx context.Context, state *WorkflowState) error
	MaxRetries() int
}

// WorkflowState holds all the state that flows between workflow steps
type WorkflowState struct {
	// Workflow identification
	WorkflowID string

	// Input arguments
	Args *ContainerizeAndDeployArgs

	// Result object that accumulates information
	Result *ContainerizeAndDeployResult

	// Step outputs
	AnalyzeResult    *AnalyzeResult
	DockerfileResult *DockerfileResult
	BuildResult      *BuildResult
	K8sResult        *K8sResult
	ScanReport       map[string]interface{}

	// Progress tracking
	ProgressTracker  *progress.Tracker
	WorkflowProgress *WorkflowProgress
	CurrentStep      int
	TotalSteps       int

	// Utilities
	Logger *slog.Logger
}

// NewWorkflowState creates a new workflow state
func NewWorkflowState(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs, progressTracker *progress.Tracker, logger *slog.Logger) *WorkflowState {
	totalSteps := 10

	result := &ContainerizeAndDeployResult{
		Steps: make([]WorkflowStep, 0, totalSteps),
	}

	workflowID := fmt.Sprintf("workflow-%d", time.Now().Unix())
	workflowProgress := NewWorkflowProgress(workflowID, "containerize_and_deploy", totalSteps)

	return &WorkflowState{
		Args:             args,
		Result:           result,
		ProgressTracker:  progressTracker,
		WorkflowProgress: workflowProgress,
		CurrentStep:      0,
		TotalSteps:       totalSteps,
		Logger:           logger,
	}
}

// UpdateProgress advances the progress tracker and returns progress info
func (ws *WorkflowState) UpdateProgress() (int, string) {
	ws.CurrentStep++
	progress := fmt.Sprintf("%d/%d", ws.CurrentStep, ws.TotalSteps)
	percentage := int((float64(ws.CurrentStep) / float64(ws.TotalSteps)) * 100)
	ws.ProgressTracker.SetCurrent(ws.CurrentStep)
	return percentage, progress
}

// AddStepResult adds a step result to the workflow result
func (ws *WorkflowState) AddStepResult(name, status, duration, message string, retries int, err error) {
	step := WorkflowStep{
		Name:     name,
		Status:   status,
		Duration: duration,
		Progress: fmt.Sprintf("%d/%d", ws.CurrentStep, ws.TotalSteps),
		Message:  message,
		Retries:  retries,
	}

	if err != nil {
		step.Error = err.Error()
	}

	ws.Result.Steps = append(ws.Result.Steps, step)
}

// ProgressTrackerFactory creates progress trackers
type ProgressTrackerFactory interface {
	CreateTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) *progress.Tracker
}

// Orchestrator executes the complete containerization workflow
type Orchestrator struct {
	steps           []Step
	logger          *slog.Logger
	progressFactory ProgressTrackerFactory
	tracer          Tracer
	stepExecutor    StepHandler
}

// Ensure Orchestrator implements WorkflowOrchestrator
var _ WorkflowOrchestrator = (*Orchestrator)(nil)

// NewOrchestrator creates a new workflow orchestrator with all steps
func NewOrchestrator(logger *slog.Logger) *Orchestrator {
	// Create default orchestrator without optimizations
	return NewOrchestratorWithFactory(nil, nil, nil, logger)
}

// NewOrchestratorWithFactory creates a new workflow orchestrator with custom step factory
func NewOrchestratorWithFactory(factory *StepFactory, progressFactory ProgressTrackerFactory, tracer Tracer, logger *slog.Logger) *Orchestrator {
	// If no factory provided, create a default one
	if factory == nil {
		factory = NewStepFactory(nil, nil, nil, logger)
	}

	o := &Orchestrator{
		steps:           factory.CreateAllSteps(),
		logger:          logger,
		progressFactory: progressFactory,
		tracer:          tracer,
	}

	// Build the middleware chain
	var middlewares []StepMiddleware

	// Add tracing middleware if tracer is available
	if tracer != nil {
		middlewares = append(middlewares, TracingMiddleware(tracer))
	}

	// Always add retry and progress middleware
	middlewares = append(middlewares, RetryMiddleware())
	middlewares = append(middlewares, ProgressMiddleware())

	// Create the base handler that executes the step
	baseHandler := func(ctx context.Context, step Step, state *WorkflowState) error {
		return step.Execute(ctx, state)
	}

	// Build the complete handler chain
	o.stepExecutor = Chain(middlewares...)(baseHandler)

	return o
}

// Execute runs the complete workflow with error handling and retry logic
func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	// Start workflow tracing
	var span Span
	if o.tracer != nil {
		ctx, span = o.tracer.StartSpan(ctx, "workflow.containerize_and_deploy")
		defer span.End()

		// Add workflow attributes
		span.SetAttribute("workflow.type", "containerize_and_deploy")
		span.SetAttribute("workflow.repo_url", args.RepoURL)
		span.SetAttribute("workflow.branch", args.Branch)
		span.SetAttribute("workflow.scan_enabled", args.Scan)
		span.SetAttribute("component", "workflow")
	}

	o.logger.Info("Starting containerize_and_deploy workflow",
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"scan", args.Scan)

	// Create progress tracker
	var progressTracker *progress.Tracker
	if o.progressFactory != nil {
		progressTracker = o.progressFactory.CreateTracker(ctx, req, 10) // 10 steps
	} else {
		// Fallback: create a minimal tracker if no factory provided
		progressTracker = progress.NewTracker(ctx, 10, &noOpSink{})
	}

	// Initialize workflow state
	state := NewWorkflowState(ctx, req, args, progressTracker, o.logger)
	defer state.ProgressTracker.Finish()

	// Begin progress tracking
	state.ProgressTracker.Begin("Starting containerization and deployment workflow")

	// Execute each step using the middleware chain
	for _, step := range o.steps {
		if err := o.stepExecutor(ctx, step, state); err != nil {
			state.Result.Success = false
			state.Result.Error = err.Error()
			return state.Result, nil // Return result with error info, don't propagate error
		}
	}

	// Mark workflow as successful
	state.Result.Success = true
	state.ProgressTracker.Complete("Containerization and deployment completed successfully")

	o.logger.Info("Containerize and deploy workflow completed successfully",
		"repo_url", args.RepoURL,
		"image_ref", state.Result.ImageRef,
		"endpoint", state.Result.Endpoint)

	return state.Result, nil
}
