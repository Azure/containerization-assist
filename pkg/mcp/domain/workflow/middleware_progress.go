// Package workflow provides progress tracking middleware for step execution
package workflow

import (
	"context"
	"fmt"
	"time"
)

// ProgressMiddleware adds comprehensive progress tracking to step execution
// This middleware provides real-time progress updates, timing information,
// and detailed step result recording for workflow monitoring.
func ProgressMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepName := step.Name()
			stepIndex := state.CurrentStep + 1
			startTime := time.Now()

			// Emit step start event with metadata
			state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Starting %s", stepName), map[string]interface{}{
				"step_name":   stepName,
				"status":      "started",
				"can_abort":   true,
				"max_retries": step.MaxRetries(),
				"step_index":  stepIndex,
				"workflow_id": state.WorkflowID,
				"total_steps": state.TotalSteps,
			})

			// Update to running status
			state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Executing %s", stepName), map[string]interface{}{
				"step_name":   stepName,
				"status":      "running",
				"workflow_id": state.WorkflowID,
			})

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			if err != nil {
				// Record error with structured context
				state.ProgressTracker.RecordError(err)

				// Emit failure event with detailed information
				state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Failed %s", stepName), map[string]interface{}{
					"step_name":   stepName,
					"status":      "failed",
					"error":       err.Error(),
					"duration_ms": duration.Milliseconds(),
					"workflow_id": state.WorkflowID,
				})

				// Add structured step result
				state.AddStepResult(stepName, "failed", duration.String(),
					fmt.Sprintf("Step %s failed: %v", stepName, err), 0, err)
			} else {
				// Update workflow progress
				state.UpdateProgress()

				// Emit successful completion event
				state.ProgressTracker.Update(state.CurrentStep, fmt.Sprintf("Completed %s", stepName), map[string]interface{}{
					"step_name":      stepName,
					"status":         "completed",
					"duration_ms":    duration.Milliseconds(),
					"result_summary": fmt.Sprintf("%s completed in %s", stepName, duration),
					"workflow_id":    state.WorkflowID,
				})

				// Add successful step result
				state.AddStepResult(stepName, "completed", duration.String(),
					fmt.Sprintf("Step %s completed successfully", stepName), 0, nil)
			}

			return err
		}
	}
}
