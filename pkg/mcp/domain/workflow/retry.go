package workflow

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

type RetryPolicy struct {
	BaseBackoff time.Duration

	MaxBackoff time.Duration

	BackoffMultiplier float64 // default: 2.0

	Jitter bool // prevents thundering herd

	MaxJitter float64 // 0.0 to 1.0

	RetryableErrors []error

	NonRetryableErrors []error
}

type RetryableChecker interface {
	IsRetryable(error) bool
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseBackoff:       time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		MaxJitter:         0.1, // 10% jitter
	}
}

func AggressiveRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseBackoff:       500 * time.Millisecond,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 1.5,
		Jitter:            true,
		MaxJitter:         0.2, // 20% jitter
	}
}

// RetryMiddleware provides enhanced retry logic with exponential backoff.
// This middleware consolidates and enhances the original retry middleware with configurable
// policies and simple error type checking.
//
// Features:
// - Configurable exponential backoff with jitter
// - Error type filtering (retryable vs non-retryable errors)
// - Context deadline awareness
// - Structured error reporting with attempt counts
func RetryMiddleware(policy RetryPolicy) StepMiddleware {
	// Apply defaults if needed
	if policy.BaseBackoff <= 0 {
		policy.BaseBackoff = time.Second
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 30 * time.Second
	}
	if policy.BackoffMultiplier <= 0 {
		policy.BackoffMultiplier = 2.0
	}
	if policy.MaxJitter < 0 || policy.MaxJitter > 1.0 {
		policy.MaxJitter = 0.1
	}

	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			stepName := step.Name()
			maxRetries := step.MaxRetries()

			// Skip retry logic if maxRetries is 0
			if maxRetries == 0 {
				return next(ctx, step, state)
			}

			var lastErr error
			for attempt := 1; attempt <= maxRetries+1; attempt++ {
				// Add retry attempt to context for downstream middleware
				retryCtx := WithRetryAttempt(ctx, attempt)

				// Handle backoff for retry attempts (skip for first attempt)
				if attempt > 1 {
					backoffDuration := calculateBackoff(attempt-1, policy)

					// Check if we have enough time left in context
					if deadline, ok := ctx.Deadline(); ok {
						if time.Until(deadline) < backoffDuration {
							return &RetryExhaustedError{
								StepName:   stepName,
								Attempts:   attempt - 1,
								MaxRetries: maxRetries,
								LastError:  lastErr,
								Reason:     "context deadline would be exceeded during backoff",
							}
						}
					}

					// Perform backoff
					select {
					case <-time.After(backoffDuration):
						// Continue with retry
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				// Execute the step
				err := next(retryCtx, step, state)

				if err == nil {
					// Success
					return nil
				}

				lastErr = err

				// Record this attempt for debugging
				lastErr = err

				// Check if this is the last attempt
				if attempt > maxRetries {
					break
				}

				// Check if error is retryable
				if !isRetryableError(err, step, policy) {
					return &NonRetryableError{
						StepName:  stepName,
						Attempts:  attempt,
						LastError: err,
						Reason:    "error type is not retryable",
					}
				}

				// Check context before continuing
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}

			// All retries exhausted - create structured error
			return &RetryExhaustedError{
				StepName:   stepName,
				Attempts:   maxRetries + 1,
				MaxRetries: maxRetries,
				LastError:  lastErr,
				Reason:     "maximum retry attempts exceeded",
			}
		}
	}
}

func calculateBackoff(attempt int, policy RetryPolicy) time.Duration {
	// Calculate exponential backoff
	backoff := float64(policy.BaseBackoff) * math.Pow(policy.BackoffMultiplier, float64(attempt-1))

	// Apply maximum backoff limit
	if backoff > float64(policy.MaxBackoff) {
		backoff = float64(policy.MaxBackoff)
	}

	// Apply jitter if enabled
	if policy.Jitter && policy.MaxJitter > 0 {
		jitterRange := backoff * policy.MaxJitter
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange // Range: -jitterRange to +jitterRange
		backoff += jitter

		// Ensure we don't go below a reasonable minimum
		if backoff < float64(policy.BaseBackoff)/2 {
			backoff = float64(policy.BaseBackoff) / 2
		}
	}

	return time.Duration(backoff)
}

func isRetryableError(err error, step Step, policy RetryPolicy) bool {
	// Check step-specific retry logic first
	if checker, ok := step.(RetryableChecker); ok {
		if !checker.IsRetryable(err) {
			return false
		}
	}

	// Check non-retryable errors
	for _, nonRetryable := range policy.NonRetryableErrors {
		if errors.Is(err, nonRetryable) {
			return false
		}
	}

	// Check retryable errors (if specified, only these errors are retryable)
	if len(policy.RetryableErrors) > 0 {
		isRetryable := false
		for _, retryable := range policy.RetryableErrors {
			if errors.Is(err, retryable) {
				isRetryable = true
				break
			}
		}
		if !isRetryable {
			return false
		}
	}

	// Default: retry most errors, but not context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return true
}

type RetryExhaustedError struct {
	StepName   string
	Attempts   int
	MaxRetries int
	LastError  error
	Reason     string
}

func (e *RetryExhaustedError) Error() string {
	return fmt.Sprintf("step %s failed after %d attempts (max %d): %s - %v",
		e.StepName, e.Attempts, e.MaxRetries, e.Reason, e.LastError)
}

func (e *RetryExhaustedError) Unwrap() error {
	return e.LastError
}

type NonRetryableError struct {
	StepName  string
	Attempts  int
	LastError error
	Reason    string
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("step %s failed after %d attempts: %s - %v",
		e.StepName, e.Attempts, e.Reason, e.LastError)
}

func (e *NonRetryableError) Unwrap() error {
	return e.LastError
}

func DefaultRetryMiddleware() StepMiddleware {
	return RetryMiddleware(DefaultRetryPolicy())
}

func AggressiveRetryMiddleware() StepMiddleware {
	return RetryMiddleware(AggressiveRetryPolicy())
}
