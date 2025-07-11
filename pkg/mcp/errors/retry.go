package errors

import (
	"context"
	"time"
)

// RetryPolicy defines how an error should be retried
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's our Rich error type
	if rich, ok := err.(*Rich); ok {
		return rich.Retryable
	}

	// For non-Rich errors, use generated metadata if we can extract a code
	// Otherwise, conservative default is false
	return false
}


// CalculateDelay calculates the delay for a retry attempt
func CalculateDelay(attempt int, policy RetryPolicy) time.Duration {
	if attempt <= 0 {
		return policy.BaseDelay
	}

	delay := policy.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * policy.Multiplier)
	}

	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	return delay
}

// ShouldRetry checks if we should retry based on attempt count and error
func ShouldRetry(ctx context.Context, err error, attempt int, policy RetryPolicy) bool {
	if ctx.Err() != nil {
		return false // Context cancelled/expired
	}

	if attempt >= policy.MaxAttempts {
		return false // Exceeded max attempts
	}

	return IsRetryable(err)
}