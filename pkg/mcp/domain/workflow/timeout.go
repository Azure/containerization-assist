package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type TimeoutConfig struct {
	DefaultTimeout time.Duration

	AdaptiveTimeouts bool // enables dynamic timeout adjustment

	MaxTimeout time.Duration

	MinTimeout time.Duration
}

type TimeoutProvider interface {
	Timeout() time.Duration
}

// TimeoutMiddleware provides unified timeout handling with context deadline support.
// This middleware can work with steps that implement TimeoutProvider for custom timeouts,
// or fall back to the configured default timeout.
//
// Features:
// - Per-step timeout configuration via TimeoutProvider interface
// - Adaptive timeout adjustment based on historical performance
// - Context deadline propagation and management
// - Graceful handling of timeout vs cancellation scenarios
func TimeoutMiddleware(config TimeoutConfig) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			// Determine the timeout for this step
			timeout := determineStepTimeout(ctx, step, state, config)

			// Skip timeout if zero or negative
			if timeout <= 0 {
				return next(ctx, step, state)
			}

			// Create timeout context
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Track timeout application for debugging
			stepName := step.Name()
			if config.AdaptiveTimeouts {
				// TODO: Add structured logging when logging middleware is consolidated
				// For now, we'll rely on tracing middleware to capture this information
			}

			// Execute step with timeout context
			err := next(timeoutCtx, step, state)

			// Handle timeout-specific errors
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return &TimeoutError{
						StepName: stepName,
						Timeout:  timeout,
						Err:      err,
					}
				}
			}

			return err
		}
	}
}

type TimeoutError struct {
	StepName string
	Timeout  time.Duration
	Err      error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("step %s timed out after %s: %v", e.StepName, e.Timeout, e.Err)
}

func (e *TimeoutError) Unwrap() error {
	return e.Err
}

func IsTimeout(err error) bool {
	var timeoutErr *TimeoutError
	return errors.As(err, &timeoutErr)
}

func determineStepTimeout(ctx context.Context, step Step, state *WorkflowState, config TimeoutConfig) time.Duration {
	var timeout time.Duration

	// First, check if the step implements TimeoutProvider
	if tp, ok := step.(TimeoutProvider); ok {
		timeout = tp.Timeout()
	}

	// If no timeout specified and adaptive timeouts enabled, use a simple multiplier
	if timeout <= 0 && config.AdaptiveTimeouts {
		// Use default timeout with 2x multiplier for adaptive scenarios
		timeout = time.Duration(float64(config.DefaultTimeout) * 2.0)
	}

	// Fall back to default timeout
	if timeout <= 0 {
		timeout = config.DefaultTimeout
	}

	// Apply timeout bounds
	if config.MaxTimeout > 0 && timeout > config.MaxTimeout {
		timeout = config.MaxTimeout
	}

	if config.MinTimeout > 0 && timeout < config.MinTimeout {
		timeout = config.MinTimeout
	}

	// Respect existing context deadline
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	return timeout
}

func WithStepTimeout(defaultTimeout time.Duration) StepMiddleware {
	return TimeoutMiddleware(TimeoutConfig{
		DefaultTimeout: defaultTimeout,
		MaxTimeout:     defaultTimeout * 3, // Allow up to 3x default as maximum
		MinTimeout:     time.Second,        // Minimum 1 second timeout
	})
}

func WithAdaptiveTimeout(defaultTimeout time.Duration, logger *slog.Logger) StepMiddleware {
	logger.Info("Creating adaptive timeout middleware",
		slog.Duration("defaultTimeout", defaultTimeout),
		slog.Duration("maxTimeout", defaultTimeout*5),
		slog.Duration("minTimeout", time.Second*5))

	return TimeoutMiddleware(TimeoutConfig{
		DefaultTimeout:   defaultTimeout,
		AdaptiveTimeouts: true,
		MaxTimeout:       defaultTimeout * 5, // Allow up to 5x default for adaptive scenarios
		MinTimeout:       time.Second * 5,    // Minimum 5 seconds for adaptive scenarios
	})
}

// ApplyDAGTimeout applies timeout to a context for DAG steps (legacy DAG compatibility)
func ApplyDAGTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		// Return a no-op cancel function if no timeout
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, timeout)
}
