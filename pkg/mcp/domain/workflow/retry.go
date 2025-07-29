package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryableError indicates an error that should be retried
type RetryableError interface {
	IsRetryable() bool
}

// NonRetryableError indicates an error that should not be retried
type NonRetryableError struct {
	Err error
}

func (e NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable error: %v", e.Err)
}

func (e NonRetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable returns false for NonRetryableError
func (e NonRetryableError) IsRetryable() bool {
	return false
}

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements RetryableError interface
	if retryable, ok := err.(RetryableError); ok {
		return retryable.IsRetryable()
	}

	// Check for common non-retryable errors
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Default to retryable for most errors
	return true
}

// ExponentialBackoff calculates backoff duration with jitter
func ExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	// Exponential backoff: baseDelay * 2^(attempt-1)
	multiplier := 1 << (attempt - 1)
	backoff := time.Duration(float64(baseDelay) * float64(multiplier))

	// Cap at 30 seconds
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	return backoff
}
