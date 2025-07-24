// Package workflow provides tracing middleware for step execution
package workflow

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware adds distributed tracing to step execution
// This middleware creates spans for each step execution, recording timing,
// attributes, and error information for observability.
func TracingMiddleware(tracer trace.Tracer) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if tracer == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			ctx, span := tracer.Start(ctx, fmt.Sprintf("workflow.step.%s", stepName))
			defer span.End()

			// Add step attributes for observability
			span.SetAttributes(
				attribute.String("step.name", stepName),
				attribute.Int("step.max_retries", step.MaxRetries()),
				attribute.String("workflow.id", state.WorkflowID),
				attribute.String("component", "workflow_step"),
			)

			// Execute the step
			err := next(ctx, step, state)

			// Record execution result
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.String("step.status", "failed"))
			} else {
				span.SetAttributes(attribute.String("step.status", "completed"))
			}

			return err
		}
	}
}
