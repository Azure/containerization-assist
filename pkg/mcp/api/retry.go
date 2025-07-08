package api

import (
	"context"
	"time"
)

// RetryService consolidates all retry-related functionality
// Replaces: RetryCoordinator, RetryContext, FixProvider, CircuitBreaker
type RetryService interface {
	// Core retry functionality (was RetryCoordinator)
	Execute(ctx context.Context, name string, fn RetryableFunc) error
	ExecuteWithPolicy(ctx context.Context, name string, policy RetryPolicy, fn RetryableFunc) error
	RegisterPolicy(name string, policy RetryPolicy) error

	// Fix provider functionality (was FixProvider)
	RegisterFixProvider(name string, provider FixHandler) error
	ExecuteWithFix(ctx context.Context, name string, fn FixableFunc) error

	// Circuit breaker functionality (was CircuitBreaker)
	ExecuteWithCircuitBreaker(ctx context.Context, name string, fn RetryableFunc) error
	GetCircuitBreakerState(name string) CircuitState
}

// Supporting types for retry functionality
type RetryableFunc func() error
type FixableFunc func() error
type FixHandler func(error) error

type CircuitState struct {
	State        string // "closed", "open", "half-open"
	FailureCount int
	LastFailure  time.Time
	NextRetry    time.Time
}

// Utility functions to create retry services
func NewRetryService(config RetryConfig) RetryService {
	// Implementation would be provided by concrete types
	return nil
}

type RetryConfig struct {
	DefaultPolicy           RetryPolicy
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration
	MaxConcurrentRetries    int
}
