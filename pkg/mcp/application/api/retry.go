package api

import (
	"context"
	"time"
)

// RetryCoordinator provides retry coordination functionality
type RetryCoordinator interface {
	Execute(ctx context.Context, name string, fn RetryableFunc) error
	ExecuteWithPolicy(ctx context.Context, name string, policy RetryPolicy, fn RetryableFunc) error
	RegisterPolicy(name string, policy RetryPolicy) error
	GetPolicy(name string) (RetryPolicy, error)
	RegisterFixProvider(name string, provider FixProvider) error
	ExecuteWithFix(ctx context.Context, name string, fn FixableFunc) error
}

// FixProvider provides fix strategies for errors
type FixProvider interface {
	GetFixStrategies(ctx context.Context, err error, metadata map[string]interface{}) ([]FixStrategy, error)
	ApplyFix(ctx context.Context, strategy FixStrategy, metadata map[string]interface{}) error
}

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
type RetryableFunc func(context.Context) error
type FixableFunc func(context.Context, RetryContext) error
type FixHandler func(context.Context, error) error

// RetryContext provides context information during retry operations
type RetryContext interface {
	GetAttempt() int
	GetLastError() error
	GetMetadata() map[string]interface{}
	SetMetadata(key string, value interface{})
}

// FixStrategy represents a fix operation strategy
type FixStrategy struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`
	Parameters  map[string]interface{} `json:"parameters"`
	Automated   bool                   `json:"automated"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

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
