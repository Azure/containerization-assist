// Package workflow provides metrics collection middleware for step execution
package workflow

import (
	"context"
	"time"
)

// MetricsMiddleware adds comprehensive metrics collection to step execution
// This middleware records timing, success/failure rates, and other operational
// metrics for workflow monitoring and performance analysis.
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

			// Record comprehensive metrics
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
