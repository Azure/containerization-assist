// Package workflow provides tracing middleware for step execution
package workflow

import (
	"context"
	"fmt"
)

// TracingMiddleware adds distributed tracing to step execution
// This middleware creates spans for each step execution, recording timing,
// attributes, and error information for observability.
func TracingMiddleware(tracer Tracer) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if tracer == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			ctx, span := tracer.StartSpan(ctx, fmt.Sprintf("workflow.step.%s", stepName))
			defer span.End()

			// Add step attributes for observability
			span.SetAttribute("step.name", stepName)
			span.SetAttribute("step.max_retries", step.MaxRetries())
			span.SetAttribute("workflow.id", state.WorkflowID)
			span.SetAttribute("component", "workflow_step")

			// Execute the step
			err := next(ctx, step, state)

			// Record execution result
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
