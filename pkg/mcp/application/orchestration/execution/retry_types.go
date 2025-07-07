package execution

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// RetryStrategy defines retry behavior for operations
type RetryStrategy interface {
	// ShouldRetry determines if an operation should be retried based on the error
	ShouldRetry(err error, attempt int) bool

	// NextDelay returns the delay before the next retry attempt
	NextDelay(attempt int) time.Duration

	// MaxAttempts returns the maximum number of retry attempts
	MaxAttempts() int
}

// RetryCoordinator manages retry operations
type RetryCoordinator interface {
	// Execute runs an operation with retry logic
	Execute(ctx context.Context, operation func() error, strategy RetryStrategy) error

	// ExecuteWithResult runs an operation that returns a result with retry logic
	ExecuteWithResult(ctx context.Context, operation func() (interface{}, error), strategy RetryStrategy) (interface{}, error)
}

// DefaultRetryStrategy provides a simple exponential backoff retry strategy
type DefaultRetryStrategy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// NewDefaultRetryStrategy creates a new default retry strategy
func NewDefaultRetryStrategy(maxAttempts int, baseDelay, maxDelay time.Duration) *DefaultRetryStrategy {
	return &DefaultRetryStrategy{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
	}
}

// ShouldRetry implements RetryStrategy
func (s *DefaultRetryStrategy) ShouldRetry(err error, attempt int) bool {
	return err != nil && attempt < s.maxAttempts
}

// NextDelay implements RetryStrategy
func (s *DefaultRetryStrategy) NextDelay(attempt int) time.Duration {
	delay := s.baseDelay * time.Duration(1<<uint(attempt-1))
	if delay > s.maxDelay {
		return s.maxDelay
	}
	return delay
}

// MaxAttempts implements RetryStrategy
func (s *DefaultRetryStrategy) MaxAttempts() int {
	return s.maxAttempts
}

// Note: RetryPolicy is already defined in error_types.go

// SimpleRetryCoordinator provides a basic retry coordinator implementation
type SimpleRetryCoordinator struct {
	policies map[string]*api.RetryPolicy
}

// NewSimpleRetryCoordinator creates a new simple retry coordinator
func NewSimpleRetryCoordinator() *SimpleRetryCoordinator {
	return &SimpleRetryCoordinator{
		policies: make(map[string]*api.RetryPolicy),
	}
}

// SetPolicy sets a retry policy for a stage
func (c *SimpleRetryCoordinator) SetPolicy(stageName string, policy *api.RetryPolicy) {
	c.policies[stageName] = policy
}

// CalculateDelay calculates the delay for a retry attempt
func (c *SimpleRetryCoordinator) CalculateDelay(stageName string, attempt int) time.Duration {
	policy, exists := c.policies[stageName]
	if !exists {
		// Default delay
		return time.Second * time.Duration(attempt)
	}

	delay := float64(policy.InitialDelay) * Pow(policy.BackoffMultiplier, float64(attempt-1))
	if time.Duration(delay) > policy.MaxDelay {
		return policy.MaxDelay
	}
	return time.Duration(delay)
}

// Pow is a simple power function
func Pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// Execute implements RetryCoordinator
func (c *SimpleRetryCoordinator) Execute(ctx context.Context, operation func() error, strategy RetryStrategy) error {
	var err error
	for attempt := 1; attempt <= strategy.MaxAttempts(); attempt++ {
		err = operation()
		if err == nil {
			return nil
		}

		if !strategy.ShouldRetry(err, attempt) {
			return err
		}

		if attempt < strategy.MaxAttempts() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(strategy.NextDelay(attempt)):
				// Continue to next attempt
			}
		}
	}
	return err
}

// ExecuteWithResult implements RetryCoordinator
func (c *SimpleRetryCoordinator) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error), strategy RetryStrategy) (interface{}, error) {
	var result interface{}
	var err error

	for attempt := 1; attempt <= strategy.MaxAttempts(); attempt++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		if !strategy.ShouldRetry(err, attempt) {
			return nil, err
		}

		if attempt < strategy.MaxAttempts() {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(strategy.NextDelay(attempt)):
				// Continue to next attempt
			}
		}
	}
	return nil, err
}
