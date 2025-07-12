// Package workflow provides enhanced step execution with progressive error context
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/utilities"
)

// executeStepWithRetryEnhanced runs a workflow step with enhanced AI retry and error context
func executeStepWithRetryEnhanced(
	ctx context.Context,
	result *ContainerizeAndDeployResult,
	stepName string,
	maxRetries int,
	stepFunc func() error,
	logger *slog.Logger,
	progressFunc func() (int, string),
	message string,
	progressTracker *progress.Tracker,
	workflowProgress *WorkflowProgress,
	errorContext *ProgressiveErrorContext,
	stateManager *WorkflowStateManager,
) error {
	startTime := time.Now()
	percentage, progressStr := progressFunc()

	// Create step info
	stepInfo := NewStepInfo(stepName, message, progressTracker.GetCurrent(), progressTracker.GetTotal())
	workflowProgress.AddStep(stepInfo)

	step := WorkflowStep{
		Name:     stepName,
		Status:   "running",
		Progress: progressStr,
		Message:  fmt.Sprintf("[%d%%] %s", percentage, message),
	}

	// Update progress through unified manager
	metadata := map[string]interface{}{
		"step_name":   stepName,
		"status":      "running",
		"max_retries": maxRetries,
	}
	progressTracker.Update(progressTracker.GetCurrent(), message, metadata)

	// Track retry attempts
	var retryCount int

	// Create wrapped function that tracks errors
	wrappedFunc := func() error {
		retryCount++
		err := stepFunc()
		if err != nil {
			// Add error to progressive context
			errorContext.AddError(stepName, err, retryCount, metadata)

			// Add to state manager for persistence
			stateManager.AddError(fmt.Sprintf("[%s] Attempt %d: %v", stepName, retryCount, err))
		}
		return err
	}

	// Build retry context from error history
	retryCtx := &utilities.RetryContext{
		ErrorHistory:   errorContext.GetAIContext(),
		StepContext:    metadata,
		FixesAttempted: []string{},
	}

	// Check if we should escalate based on error patterns
	if errorContext.ShouldEscalate(stepName) {
		logger.Warn("Error patterns suggest escalation needed",
			"step", stepName,
			"previous_errors", len(errorContext.GetStepErrors(stepName)))
		// Reduce retries for problematic steps
		if maxRetries > 1 {
			maxRetries = 1
		}
	}

	// Execute the step with enhanced AI retry
	err := utilities.WithAIRetryEnhanced(ctx, stepName, maxRetries, wrappedFunc, retryCtx, logger)
	step.Duration = time.Since(startTime).String()
	step.Retries = retryCount - 1 // Subtract 1 for the initial attempt

	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		result.Steps = append(result.Steps, step)

		// Include error summary in result
		errorSummary := errorContext.GetSummary()
		result.Error = fmt.Sprintf("Step %s failed after %d attempts.\n\n%s",
			stepName, retryCount, errorSummary)

		// Update progress with failure
		metadata["status"] = "failed"
		metadata["error"] = err.Error()
		metadata["duration"] = step.Duration
		metadata["retries"] = step.Retries
		progressTracker.Update(progressTracker.GetCurrent(), fmt.Sprintf("Failed: %s", message), metadata)

		stepInfo.Fail(err)

		// Save failure state
		if stateManager != nil {
			stateManager.SetState("lastFailedStep", stepName)
			stateManager.SetState("errorSummary", errorSummary)
			stateManager.SaveState(stepName + "_failed")
		}

		return err
	}

	step.Status = "completed"
	result.Steps = append(result.Steps, step)

	// Update progress with completion
	metadata["status"] = "completed"
	metadata["duration"] = step.Duration
	metadata["retries"] = step.Retries
	progressTracker.Update(progressTracker.GetCurrent(), fmt.Sprintf("Completed: %s", message), metadata)

	stepInfo.Complete()

	// Mark step as completed in state manager
	if stateManager != nil {
		stateManager.SetStepCompleted(stepName)
	}

	logger.Info("Step completed successfully",
		"step", stepName,
		"duration", step.Duration,
		"retries", step.Retries)

	return nil
}
