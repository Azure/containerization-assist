package adapters

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/infra/retry"
)

// RetryCoordinatorAdapter adapts the infra retry coordinator to the application interface
type RetryCoordinatorAdapter struct {
	coordinator *retry.Coordinator
}

// NewRetryCoordinator creates a new retry coordinator adapter
func NewRetryCoordinator() services.RetryCoordinator {
	return &RetryCoordinatorAdapter{
		coordinator: retry.New(),
	}
}

// SetPolicy sets the retry policy for a given operation type
func (r *RetryCoordinatorAdapter) SetPolicy(operationType string, policy *services.RetryPolicy) {
	r.coordinator.SetPolicy(operationType, &retry.Policy{
		MaxAttempts:     policy.MaxAttempts,
		InitialDelay:    policy.InitialDelay,
		MaxDelay:        policy.MaxDelay,
		BackoffStrategy: retry.BackoffStrategy(policy.BackoffStrategy),
		Multiplier:      policy.Multiplier,
		Jitter:          policy.Jitter,
		ErrorPatterns:   policy.ErrorPatterns,
	})
}

// GetPolicy retrieves the retry policy for an operation type
func (r *RetryCoordinatorAdapter) GetPolicy(operationType string) *services.RetryPolicy {
	p := r.coordinator.GetPolicy(operationType)
	if p == nil {
		return nil
	}

	return &services.RetryPolicy{
		MaxAttempts:     p.MaxAttempts,
		InitialDelay:    p.InitialDelay,
		MaxDelay:        p.MaxDelay,
		BackoffStrategy: string(p.BackoffStrategy),
		Multiplier:      p.Multiplier,
		Jitter:          p.Jitter,
		ErrorPatterns:   p.ErrorPatterns,
	}
}

// ExecuteWithRetry executes an operation with retry logic
func (r *RetryCoordinatorAdapter) ExecuteWithRetry(ctx context.Context, operationType string, operation func() error) error {
	return r.coordinator.ExecuteWithRetry(ctx, operationType, operation)
}

// ShouldRetry determines if an error is retryable
func (r *RetryCoordinatorAdapter) ShouldRetry(err error, operationType string) bool {
	return r.coordinator.ShouldRetry(err, operationType)
}
