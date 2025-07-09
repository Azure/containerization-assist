package services

import (
	"context"
	"time"
)

// RetryPolicy defines the retry behavior for operations
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffStrategy string
	Multiplier      float64
	Jitter          bool
	ErrorPatterns   []string
}

// ServiceRetryCoordinator - Use api.RetryCoordinator for the canonical interface
// This is a simplified interface for basic retry operations
// Deprecated: Use api.RetryCoordinator for new code
type ServiceRetryCoordinator interface {
	// SetPolicy sets the retry policy for a given operation type
	SetPolicy(operationType string, policy *RetryPolicy)

	// GetPolicy retrieves the retry policy for an operation type
	GetPolicy(operationType string) *RetryPolicy

	// ExecuteWithRetry executes an operation with retry logic
	ExecuteWithRetry(ctx context.Context, operationType string, operation func() error) error

	// ShouldRetry determines if an error is retryable
	ShouldRetry(err error, operationType string) bool
}
