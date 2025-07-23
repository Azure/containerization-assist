// Package workflow provides enhanced retry middleware for step execution
package workflow

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// StepRetryPolicy defines the retry behavior for step execution
type StepRetryPolicy struct {
	// BaseBackoff is the initial backoff duration
	BaseBackoff time.Duration

	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff (default: 2.0)
	BackoffMultiplier float64

	// Jitter adds randomness to backoff to prevent thundering herd
	Jitter bool

	// MaxJitter is the maximum jitter percentage (0.0 to 1.0)
	MaxJitter float64

	// RetryableErrors are error types that should trigger retries
	RetryableErrors []error

	// NonRetryableErrors are error types that should never be retried
	NonRetryableErrors []error

	// ErrorPatternRecognition enables intelligent retry based on error patterns
	ErrorPatternRecognition bool
}

// RetryableChecker is an interface for steps that can determine if an error is retryable
type RetryableChecker interface {
	IsRetryable(error) bool
}

// ErrorPatternProvider provides access to error pattern recognition
type ErrorPatternProvider interface {
	ShouldRetry(stepName string, err error, attempt int, maxRetries int) bool
	RecordAttempt(stepName string, err error, attempt int)
}

// DefaultStepRetryPolicy returns a sensible default retry policy
func DefaultStepRetryPolicy() StepRetryPolicy {
	return StepRetryPolicy{
		BaseBackoff:       time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		MaxJitter:         0.1, // 10% jitter
	}
}

// AggressiveStepRetryPolicy returns a retry policy for critical operations
func AggressiveStepRetryPolicy() StepRetryPolicy {
	return StepRetryPolicy{
		BaseBackoff:             500 * time.Millisecond,
		MaxBackoff:              60 * time.Second,
		BackoffMultiplier:       1.5,
		Jitter:                  true,
		MaxJitter:               0.2, // 20% jitter
		ErrorPatternRecognition: true,
	}
}

// RetryMiddleware provides enhanced retry logic with exponential backoff and pattern recognition.
// This middleware consolidates and enhances the original retry middleware with configurable
// policies and intelligent error pattern recognition.
//
// Features:
// - Configurable exponential backoff with jitter
// - Error type filtering (retryable vs non-retryable errors)
// - Integration with error pattern recognition
// - Context deadline awareness
// - Structured error reporting with attempt counts
func RetryMiddleware(policy StepRetryPolicy, errorContext ErrorPatternProvider) StepMiddleware {
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
				retryCtx := context.WithValue(ctx, "retry_attempt", attempt)

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
					if attempt > 1 && errorContext != nil {
						// Record successful retry pattern
						errorContext.RecordAttempt(stepName, nil, attempt)
					}
					return nil
				}

				lastErr = err

				// Record this attempt if we have error context
				if errorContext != nil {
					errorContext.RecordAttempt(stepName, err, attempt)
				}

				// Check if this is the last attempt
				if attempt > maxRetries {
					break
				}

				// Check if error is retryable
				if !isRetryableError(err, step, policy, errorContext, stepName, attempt, maxRetries) {
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

// calculateBackoff calculates the backoff duration for a retry attempt
func calculateBackoff(attempt int, policy StepRetryPolicy) time.Duration {
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

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error, step Step, policy StepRetryPolicy, errorContext ErrorPatternProvider, stepName string, attempt, maxRetries int) bool {
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

	// Use error pattern recognition if available and enabled
	if policy.ErrorPatternRecognition && errorContext != nil {
		return errorContext.ShouldRetry(stepName, err, attempt, maxRetries)
	}

	// Default: retry most errors, but not context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return true
}

// Error types for better error handling

// RetryExhaustedError indicates that all retry attempts have been exhausted
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

// NonRetryableError indicates that an error is not retryable
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

// Convenience constructors

// SimpleRetryMiddleware creates a retry middleware with default settings
func SimpleRetryMiddleware() StepMiddleware {
	return RetryMiddleware(DefaultStepRetryPolicy(), nil)
}

// PatternAwareRetryMiddleware creates a retry middleware with error pattern recognition
func PatternAwareRetryMiddleware(errorContext ErrorPatternProvider) StepMiddleware {
	policy := DefaultStepRetryPolicy()
	policy.ErrorPatternRecognition = true
	return RetryMiddleware(policy, errorContext)
}
