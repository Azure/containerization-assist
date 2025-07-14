// Package workflow provides middleware for step execution
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// StepHandler is a function that executes a workflow step
type StepHandler func(ctx context.Context, step Step, state *WorkflowState) error

// StepMiddleware is a function that wraps a StepHandler to add functionality
type StepMiddleware func(next StepHandler) StepHandler

// Chain creates a single StepHandler from a list of middlewares
func Chain(middlewares ...StepMiddleware) StepMiddleware {
	return func(next StepHandler) StepHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// TracingMiddleware adds distributed tracing to step execution
func TracingMiddleware(tracer Tracer) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if tracer == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			ctx, span := tracer.StartSpan(ctx, fmt.Sprintf("workflow.step.%s", stepName))
			defer span.End()

			// Add step attributes
			span.SetAttribute("step.name", stepName)
			span.SetAttribute("step.max_retries", step.MaxRetries())
			span.SetAttribute("component", "workflow_step")

			// Execute the step
			err := next(ctx, step, state)

			// Record result
			if err != nil {
				span.RecordError(err)
				span.SetAttribute("step.status", "failed")
			} else {
				span.SetAttribute("step.status", "completed")
			}

			return err
		}
	}
}

// RetryMiddleware adds retry logic to step execution
func RetryMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepName := step.Name()
			maxRetries := step.MaxRetries()

			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					state.Logger.Info("Retrying step", "step", stepName, "attempt", attempt, "max_retries", maxRetries)

					// Wait before retry with exponential backoff
					backoffDelay := time.Duration(attempt+1) * time.Second
					time.Sleep(backoffDelay)
				}

				// Execute the step
				err := next(ctx, step, state)

				if err == nil {
					// Success - record retry count if needed
					if attempt > 0 {
						state.Logger.Info("Step succeeded after retry", "step", stepName, "attempts", attempt+1)
					}
					return nil
				}

				lastErr = err
				state.Logger.Warn("Step failed", "step", stepName, "attempt", attempt+1, "error", err)
			}

			// All retries exhausted
			return errors.New(errors.CodeOperationFailed, "workflow",
				fmt.Sprintf("step %s failed after %d retries", stepName, maxRetries), lastErr)
		}
	}
}

// ProgressMiddleware adds progress tracking to step execution
func ProgressMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepName := step.Name()
			stepIndex := state.CurrentStep + 1
			startTime := time.Now()

			// Emit step start event
			state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Starting %s", stepName), map[string]interface{}{
				"step_name":   stepName,
				"status":      "started",
				"can_abort":   true,
				"max_retries": step.MaxRetries(),
				"step_index":  stepIndex,
			})

			// Update to running status
			state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Executing %s", stepName), map[string]interface{}{
				"step_name": stepName,
				"status":    "running",
			})

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			if err != nil {
				// Record error
				state.ProgressTracker.RecordError(err)

				// Emit failure event
				state.ProgressTracker.Update(stepIndex, fmt.Sprintf("Failed %s", stepName), map[string]interface{}{
					"step_name":   stepName,
					"status":      "failed",
					"error":       err.Error(),
					"duration_ms": duration.Milliseconds(),
				})

				// Add step result
				state.AddStepResult(stepName, "failed", duration.String(),
					fmt.Sprintf("Step %s failed: %v", stepName, err), 0, err)
			} else {
				// Update progress
				state.UpdateProgress()

				// Emit completion event
				state.ProgressTracker.Update(state.CurrentStep, fmt.Sprintf("Completed %s", stepName), map[string]interface{}{
					"step_name":      stepName,
					"status":         "completed",
					"duration_ms":    duration.Milliseconds(),
					"result_summary": fmt.Sprintf("%s completed in %s", stepName, duration),
				})

				// Add step result
				state.AddStepResult(stepName, "completed", duration.String(),
					fmt.Sprintf("Step %s completed successfully", stepName), 0, nil)
			}

			return err
		}
	}
}

// MetricsMiddleware adds metrics collection to step execution
func MetricsMiddleware(collector MetricsCollector) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if collector == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			startTime := time.Now()

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(startTime)

			// Record metrics
			collector.RecordStepDuration(stepName, duration)
			if err != nil {
				collector.RecordStepFailure(stepName)
			} else {
				collector.RecordStepSuccess(stepName)
			}

			return err
		}
	}
}
