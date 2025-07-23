// Package workflow provides unified progress tracking middleware for step execution
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// ProgressMode defines the verbosity level of progress tracking
type ProgressMode int

const (
	// SimpleProgress emits only start and completion events (2 events per step)
	SimpleProgress ProgressMode = iota
	// ComprehensiveProgress emits start, running, and completion events with detailed metadata (3 events per step)
	ComprehensiveProgress
	// RetryAwareProgress includes retry attempt information in progress updates
	RetryAwareProgress
)

// ProgressMiddleware provides configurable progress tracking for step execution.
// This unified middleware consolidates the functionality of ProgressMiddleware,
// SimpleProgressMiddleware, and ProgressWithRetryMiddleware into a single,
// configurable implementation.
func ProgressMiddleware(mode ProgressMode) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			// Skip progress tracking if no emitter is available
			if state.ProgressEmitter == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			stepIndex := state.CurrentStep + 1
			percentage := int(float64(stepIndex) / float64(state.TotalSteps) * 100)
			startTime := time.Now()

			// Extract retry attempt information from context
			attempt := getRetryAttempt(ctx)

			// Emit start event based on mode
			if err := emitStartEvent(ctx, state, stepName, stepIndex, percentage, attempt, step, mode); err != nil {
				// Log error but continue execution
				// TODO: Add structured logging here when logging middleware is consolidated
			}

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			// Handle completion based on mode
			if err != nil {
				handleStepFailure(ctx, state, stepName, stepIndex, percentage, duration, err, attempt, mode)
			} else {
				handleStepSuccess(ctx, state, stepName, stepIndex, percentage, duration, mode)
			}

			return err
		}
	}
}

// getRetryAttempt extracts retry attempt information from context
func getRetryAttempt(ctx context.Context) int {
	if attempt, ok := GetRetryAttempt(ctx); ok {
		return attempt
	}
	return 1 // Default to first attempt
}

// emitStartEvent emits the appropriate start event based on progress mode
func emitStartEvent(ctx context.Context, state *WorkflowState, stepName string, stepIndex, percentage, attempt int, step Step, mode ProgressMode) error {
	switch mode {
	case SimpleProgress:
		return state.ProgressEmitter.Emit(ctx, stepName, percentage,
			fmt.Sprintf("Starting %s", stepName))

	case ComprehensiveProgress:
		return state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       stepIndex,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    fmt.Sprintf("Starting %s", stepName),
			Percentage: percentage,
			Status:     "started",
			Metadata: map[string]interface{}{
				"step_name":   stepName,
				"can_abort":   true,
				"max_retries": step.MaxRetries(),
				"step_index":  stepIndex,
				"workflow_id": state.WorkflowID,
				"total_steps": state.TotalSteps,
			},
		})

	case RetryAwareProgress:
		message := fmt.Sprintf("Starting %s", stepName)
		if attempt > 1 {
			message = fmt.Sprintf("Retrying %s (attempt %d/%d)", stepName, attempt, step.MaxRetries())
		}

		return state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       stepIndex,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    message,
			Percentage: percentage,
			Status:     "running",
			Metadata: map[string]interface{}{
				"attempt":     attempt,
				"max_retries": step.MaxRetries(),
				"step_name":   stepName,
				"workflow_id": state.WorkflowID,
			},
		})
	}

	return nil
}

// emitRunningEvent emits a running event for comprehensive progress mode
func emitRunningEvent(ctx context.Context, state *WorkflowState, stepName string, stepIndex, percentage int) error {
	return state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
		Step:       stepIndex,
		Total:      state.TotalSteps,
		Stage:      stepName,
		Message:    fmt.Sprintf("Executing %s", stepName),
		Percentage: percentage,
		Status:     "running",
		Metadata: map[string]interface{}{
			"step_name":   stepName,
			"workflow_id": state.WorkflowID,
		},
	})
}

// handleStepFailure handles failure events based on progress mode
func handleStepFailure(ctx context.Context, state *WorkflowState, stepName string, stepIndex, percentage int, duration time.Duration, err error, attempt int, mode ProgressMode) {
	// Add structured step result for all modes
	state.AddStepResult(stepName, "failed", duration.String(),
		fmt.Sprintf("Step %s failed: %v", stepName, err), attempt-1, err)

	switch mode {
	case SimpleProgress:
		_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       stepIndex,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    err.Error(),
			Percentage: percentage,
			Status:     "failed",
			Metadata: map[string]interface{}{
				"duration_ms": duration.Milliseconds(),
				"error_type":  fmt.Sprintf("%T", err),
			},
		})

	case ComprehensiveProgress:
		_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       stepIndex,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    fmt.Sprintf("Failed %s", stepName),
			Percentage: percentage,
			Status:     "failed",
			Metadata: map[string]interface{}{
				"step_name":   stepName,
				"error":       err.Error(),
				"duration_ms": duration.Milliseconds(),
				"workflow_id": state.WorkflowID,
			},
		})

	case RetryAwareProgress:
		_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       stepIndex,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    fmt.Sprintf("Failed after %d attempts: %v", attempt, err),
			Percentage: percentage,
			Status:     "failed",
			Metadata: map[string]interface{}{
				"duration_ms": duration.Milliseconds(),
				"attempt":     attempt,
				"step_name":   stepName,
				"workflow_id": state.WorkflowID,
			},
		})
	}
}

// handleStepSuccess handles success events based on progress mode
func handleStepSuccess(ctx context.Context, state *WorkflowState, stepName string, stepIndex, percentage int, duration time.Duration, mode ProgressMode) {
	// Update workflow progress for all modes
	state.UpdateProgress()

	// Add successful step result for all modes
	state.AddStepResult(stepName, "completed", duration.String(),
		fmt.Sprintf("Step %s completed successfully", stepName), 0, nil)

	// Calculate completion percentage after progress update
	completionPercent := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)

	switch mode {
	case SimpleProgress:
		_ = state.ProgressEmitter.Emit(ctx, stepName, completionPercent,
			fmt.Sprintf("Completed %s in %s", stepName, duration.Round(time.Millisecond)))

	case ComprehensiveProgress:
		// Emit running event first, then completion
		_ = emitRunningEvent(ctx, state, stepName, stepIndex, percentage)

		_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
			Step:       state.CurrentStep,
			Total:      state.TotalSteps,
			Stage:      stepName,
			Message:    fmt.Sprintf("Completed %s", stepName),
			Percentage: completionPercent,
			Status:     "completed",
			Metadata: map[string]interface{}{
				"step_name":      stepName,
				"duration_ms":    duration.Milliseconds(),
				"result_summary": fmt.Sprintf("%s completed in %s", stepName, duration),
				"workflow_id":    state.WorkflowID,
			},
		})

	case RetryAwareProgress:
		_ = state.ProgressEmitter.Emit(ctx, stepName, completionPercent,
			fmt.Sprintf("Completed %s", stepName))
	}
}
