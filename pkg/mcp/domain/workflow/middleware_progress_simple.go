// Package workflow provides a simplified progress tracking middleware
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// SimpleProgressMiddleware provides streamlined progress tracking with less verbose updates
// This is an alternative to ProgressMiddleware that reduces notification spam
func SimpleProgressMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if state.ProgressEmitter == nil {
				// No progress emitter, just execute the step
				return next(ctx, step, state)
			}

			stepName := step.Name()
			stepIndex := state.CurrentStep
			percentage := int(float64(stepIndex) / float64(state.TotalSteps) * 100)
			startTime := time.Now()

			// Single notification for step start
			_ = state.ProgressEmitter.Emit(ctx, stepName, percentage,
				fmt.Sprintf("Starting %s", stepName))

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			// Single notification for step result
			if err != nil {
				// Failed step
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

				// Update step result
				state.AddStepResult(stepName, "failed", duration.String(),
					fmt.Sprintf("Failed: %v", err), 0, err)
			} else {
				// Successful step
				state.UpdateProgress()
				newPercentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)

				_ = state.ProgressEmitter.Emit(ctx, stepName, newPercentage,
					fmt.Sprintf("Completed %s in %s", stepName, duration.Round(time.Millisecond)))

				// Update step result
				state.AddStepResult(stepName, "completed", duration.String(),
					"Success", 0, nil)
			}

			return err
		}
	}
}

// ProgressWithRetryMiddleware combines progress tracking with retry information
// This middleware is useful when using retry middleware to show retry attempts
func ProgressWithRetryMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if state.ProgressEmitter == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			stepIndex := state.CurrentStep
			percentage := int(float64(stepIndex) / float64(state.TotalSteps) * 100)

			// Track retry attempts
			attempt := 1
			if attemptVal := ctx.Value("retry_attempt"); attemptVal != nil {
				if a, ok := attemptVal.(int); ok {
					attempt = a
				}
			}

			message := fmt.Sprintf("Starting %s", stepName)
			if attempt > 1 {
				message = fmt.Sprintf("Retrying %s (attempt %d/%d)", stepName, attempt, step.MaxRetries())
			}

			// Emit progress with retry info
			_ = state.ProgressEmitter.EmitDetailed(ctx, api.ProgressUpdate{
				Step:       stepIndex,
				Total:      state.TotalSteps,
				Stage:      stepName,
				Message:    message,
				Percentage: percentage,
				Status:     "running",
				Metadata: map[string]interface{}{
					"attempt":     attempt,
					"max_retries": step.MaxRetries(),
				},
			})

			// Execute with timing
			startTime := time.Now()
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			// Report result
			if err != nil {
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
					},
				})
			} else {
				state.UpdateProgress()
				newPercentage := int(float64(state.CurrentStep) / float64(state.TotalSteps) * 100)

				_ = state.ProgressEmitter.Emit(ctx, stepName, newPercentage,
					fmt.Sprintf("Completed %s", stepName))
			}

			return err
		}
	}
}
