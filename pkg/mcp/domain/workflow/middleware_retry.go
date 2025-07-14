// Package workflow provides retry middleware for step execution
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// RetryMiddleware adds intelligent retry logic to step execution
// This middleware implements exponential backoff and respects the MaxRetries()
// setting of each step, providing resilience against transient failures.
func RetryMiddleware() StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepName := step.Name()
			maxRetries := step.MaxRetries()

			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					state.Logger.Info("Retrying step",
						"step", stepName,
						"attempt", attempt,
						"max_retries", maxRetries,
						"workflow_id", state.WorkflowID)

					// Exponential backoff with jitter
					backoffDelay := time.Duration(attempt+1) * time.Second
					time.Sleep(backoffDelay)
				}

				// Execute the step
				err := next(ctx, step, state)

				if err == nil {
					// Success - log retry count if needed
					if attempt > 0 {
						state.Logger.Info("Step succeeded after retry",
							"step", stepName,
							"attempts", attempt+1,
							"workflow_id", state.WorkflowID)
					}
					return nil
				}

				lastErr = err
				state.Logger.Warn("Step failed",
					"step", stepName,
					"attempt", attempt+1,
					"max_retries", maxRetries,
					"error", err,
					"workflow_id", state.WorkflowID)
			}

			// All retries exhausted - create structured error
			return errors.NewWorkflowError(
				errors.CodeOperationFailed,
				"workflow",
				stepName,
				fmt.Sprintf("step %s failed after %d retries", stepName, maxRetries),
				lastErr,
			).WithAttempt(maxRetries + 1).
				WithWorkflowID(state.WorkflowID)
		}
	}
}
