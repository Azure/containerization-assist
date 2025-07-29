package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
	MinTimeout     time.Duration
}

// TimeoutProvider interface for steps that want custom timeouts
type TimeoutProvider interface {
	Timeout() time.Duration
}

// TimeoutError represents a timeout error
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

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	var timeoutErr *TimeoutError
	return errors.As(err, &timeoutErr)
}

// DetermineStepTimeout calculates the timeout for a step
func DetermineStepTimeout(ctx context.Context, step Step, defaultTimeout time.Duration) time.Duration {
	var timeout time.Duration

	// Check if the step implements TimeoutProvider
	if tp, ok := step.(TimeoutProvider); ok {
		timeout = tp.Timeout()
	}

	// Fall back to default timeout
	if timeout <= 0 {
		timeout = defaultTimeout
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

// ApplyTimeout creates a timeout context for step execution
func ApplyTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		// Return a no-op cancel function if no timeout
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, timeout)
}
