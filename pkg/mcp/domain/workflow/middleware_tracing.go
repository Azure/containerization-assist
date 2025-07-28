// Package workflow provides middleware for step execution
package workflow

import (
	"context"
)

// TracingMiddleware is a no-op middleware as tracing has been removed
// It's kept for API compatibility
func TracingMiddleware(tracer interface{}) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			// Simply pass through to the next handler without tracing
			return next(ctx, step, state)
		}
	}
}
