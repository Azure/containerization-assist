// Package workflow provides orchestration for the containerization workflow.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
)

// Step defines the interface for individual workflow steps
type Step interface {
	Name() string
	Execute(ctx context.Context, state *WorkflowState) error
	MaxRetries() int
}

// WorkflowState holds all the state that flows between workflow steps
type WorkflowState struct {
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
func NewWorkflowState(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs, logger *slog.Logger) *WorkflowState {
	totalSteps := 10

	result := &ContainerizeAndDeployResult{
		Steps: make([]WorkflowStep, 0, totalSteps),
	}

	progressTracker := progress.NewProgressTracker(ctx, req, totalSteps, logger)
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

// Orchestrator executes the complete containerization workflow
type Orchestrator struct {
	steps  []Step
	logger *slog.Logger
}

// NewOrchestrator creates a new workflow orchestrator with all steps
func NewOrchestrator(logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		steps: []Step{
			NewAnalyzeStep(),
			NewDockerfileStep(),
			NewBuildStep(),
			NewScanStep(),
			NewTagStep(),
			NewPushStep(),
			NewManifestStep(),
			NewClusterStep(),
			NewDeployStep(),
			NewVerifyStep(),
		},
		logger: logger,
	}
}

// Execute runs the complete workflow with error handling and retry logic
func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	o.logger.Info("Starting containerize_and_deploy workflow",
		"repo_url", args.RepoURL,
		"branch", args.Branch,
		"scan", args.Scan)

	// Initialize workflow state
	state := NewWorkflowState(ctx, req, args, o.logger)
	defer state.ProgressTracker.Finish()

	// Begin progress tracking
	state.ProgressTracker.Begin("Starting containerization and deployment workflow")

	// Execute each step with retry logic
	for _, step := range o.steps {
		if err := o.executeStepWithRetry(ctx, step, state); err != nil {
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

// executeStepWithRetry executes a single step with retry logic
func (o *Orchestrator) executeStepWithRetry(ctx context.Context, step Step, state *WorkflowState) error {
	stepName := step.Name()
	maxRetries := step.MaxRetries()

	startTime := time.Now()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			o.logger.Info("Retrying step", "step", stepName, "attempt", attempt, "max_retries", maxRetries)
		}

		// Execute the step
		err := step.Execute(ctx, state)
		duration := time.Since(startTime).String()

		if err == nil {
			// Step succeeded
			state.UpdateProgress()
			state.AddStepResult(stepName, "completed", duration, fmt.Sprintf("Step %s completed successfully", stepName), attempt, nil)

			state.ProgressTracker.Update(state.CurrentStep, fmt.Sprintf("Completed %s", stepName), map[string]interface{}{
				"step":    stepName,
				"retries": attempt,
			})

			return nil
		}

		lastErr = err
		o.logger.Warn("Step failed", "step", stepName, "attempt", attempt, "error", err)

		// Record error in progress tracker
		state.ProgressTracker.RecordError(err)

		// Check if we should retry
		if attempt < maxRetries {
			// Wait before retry with exponential backoff
			backoffDelay := time.Duration(attempt+1) * time.Second
			time.Sleep(backoffDelay)
		}
	}

	// All retries exhausted
	duration := time.Since(startTime).String()
	state.AddStepResult(stepName, "failed", duration, fmt.Sprintf("Step %s failed after %d retries", stepName, maxRetries), maxRetries, lastErr)

	return errors.New(errors.CodeOperationFailed, "workflow",
		fmt.Sprintf("step %s failed after %d retries", stepName, maxRetries), lastErr)
}
