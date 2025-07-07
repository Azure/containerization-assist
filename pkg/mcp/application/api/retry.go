package api

import (
	"context"
	"time"
)

// RetryCoordinator is the canonical interface for retry coordination
// This consolidates retry logic from internal packages to break import cycles
type RetryCoordinator interface {
	// Execute runs a function with retry logic
	Execute(ctx context.Context, name string, fn RetryableFunc) error

	// ExecuteWithPolicy runs a function with a specific retry policy
	ExecuteWithPolicy(ctx context.Context, name string, policy RetryPolicy, fn RetryableFunc) error

	// RegisterPolicy registers a named retry policy
	RegisterPolicy(name string, policy RetryPolicy) error

	// GetPolicy retrieves a named retry policy
	GetPolicy(name string) (RetryPolicy, error)

	// RegisterFixProvider registers a fix provider for automated recovery
	RegisterFixProvider(name string, provider FixProvider) error

	// ExecuteWithFix runs a function with retry and fix capabilities
	ExecuteWithFix(ctx context.Context, name string, fn FixableFunc) error
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func(ctx context.Context) error

// FixableFunc represents a function that can be fixed and retried
type FixableFunc func(ctx context.Context, retryCtx RetryContext) error

// RetryContext provides context for retry operations
type RetryContext interface {
	// GetAttempt returns the current attempt number
	GetAttempt() int

	// GetLastError returns the error from the last attempt
	GetLastError() error

	// GetMetadata returns retry metadata
	GetMetadata() map[string]interface{}

	// SetMetadata sets retry metadata
	SetMetadata(key string, value interface{})
}

// FixProvider defines the interface for implementing fix strategies
type FixProvider interface {
	// Name returns the provider name
	Name() string

	// GetFixStrategies returns available fix strategies for an error
	GetFixStrategies(ctx context.Context, err error, metadata map[string]interface{}) ([]FixStrategy, error)

	// ApplyFix applies a specific fix strategy
	ApplyFix(ctx context.Context, strategy FixStrategy, metadata map[string]interface{}) error
}

// FixStrategy represents a strategy for fixing errors
type FixStrategy struct {
	// Type identifies the fix type
	Type string `json:"type"`

	// Name is the strategy name
	Name string `json:"name"`

	// Description explains what the fix does
	Description string `json:"description"`

	// Priority determines fix order (higher = first)
	Priority int `json:"priority"`

	// Parameters contains fix-specific parameters
	Parameters map[string]interface{} `json:"parameters"`

	// Automated indicates if the fix can be applied automatically
	Automated bool `json:"automated"`
}

// BackoffStrategy defines different retry backoff strategies
type BackoffStrategy string

const (
	// BackoffFixed uses a fixed delay between retries
	BackoffFixed BackoffStrategy = "fixed"

	// BackoffLinear increases delay linearly
	BackoffLinear BackoffStrategy = "linear"

	// BackoffExponential increases delay exponentially
	BackoffExponential BackoffStrategy = "exponential"
)

// RetryMetrics provides metrics about retry operations
type RetryMetrics struct {
	// TotalAttempts is the total number of attempts made
	TotalAttempts int64

	// SuccessfulAttempts is the number of successful attempts
	SuccessfulAttempts int64

	// FailedAttempts is the number of failed attempts
	FailedAttempts int64

	// TotalRetries is the total number of retries (attempts - 1)
	TotalRetries int64

	// AverageDelay is the average delay between retries
	AverageDelay time.Duration

	// LastAttemptTime is when the last attempt was made
	LastAttemptTime time.Time
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	// CircuitClosed allows requests through
	CircuitClosed CircuitBreakerState = "closed"

	// CircuitOpen blocks all requests
	CircuitOpen CircuitBreakerState = "open"

	// CircuitHalfOpen allows limited requests for testing
	CircuitHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreaker provides circuit breaker functionality
type CircuitBreaker interface {
	// GetState returns the current circuit state
	GetState() CircuitBreakerState

	// RecordSuccess records a successful operation
	RecordSuccess()

	// RecordFailure records a failed operation
	RecordFailure(err error)

	// CanExecute checks if execution is allowed
	CanExecute() bool

	// Reset resets the circuit breaker
	Reset()
}
