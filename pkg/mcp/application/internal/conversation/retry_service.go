package conversation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// RetryService handles retry logic for operations
type RetryService interface {
	// ExecuteWithRetry executes an operation with retry logic
	ExecuteWithRetry(ctx context.Context, operationType string, fn func() error) error

	// SetPolicy sets the retry policy for a specific operation type
	SetPolicy(operationType string, policy *services.RetryPolicy)
}

// retryService implements RetryService
type retryService struct {
	logger           *slog.Logger
	retryCoordinator services.RetryCoordinator
}

// NewRetryService creates a new RetryService
func NewRetryService(logger *slog.Logger, retryCoordinator services.RetryCoordinator) RetryService {
	service := &retryService{
		logger:           logger.With("component", "retry_service"),
		retryCoordinator: retryCoordinator,
	}

	// Set default policy for conversation operations
	if retryCoordinator != nil {
		service.SetPolicy("conversation", &services.RetryPolicy{
			MaxAttempts:     3,
			InitialDelay:    1 * time.Second,
			MaxDelay:        10 * time.Second,
			BackoffStrategy: "exponential",
			Multiplier:      2.0,
			Jitter:          true,
			ErrorPatterns: []string{
				"timeout", "deadline exceeded", "connection refused",
				"temporary failure", "rate limit", "throttled",
				"service unavailable", "504", "503", "502",
			},
		})
	}

	return service
}

func (r *retryService) ExecuteWithRetry(ctx context.Context, operationType string, fn func() error) error {
	if r.retryCoordinator == nil {
		// If no retry coordinator, just execute once
		return fn()
	}
	return r.retryCoordinator.ExecuteWithRetry(ctx, operationType, fn)
}

func (r *retryService) SetPolicy(operationType string, policy *services.RetryPolicy) {
	if r.retryCoordinator != nil {
		r.retryCoordinator.SetPolicy(operationType, policy)
	}
}

// Backward compatibility note:
// SimpleRetryManager and NewSimpleRetryManager have been moved to retry_manager.go
// Use RetryService and NewRetryService for new code
