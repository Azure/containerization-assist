package conversation

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
	"github.com/rs/zerolog"
)

// SimpleRetryManager implements RetryManager with unified retry coordinator
type SimpleRetryManager struct {
	logger           zerolog.Logger
	retryCoordinator *retry.Coordinator
}

// NewSimpleRetryManager creates a new simple retry manager with unified coordinator
func NewSimpleRetryManager(logger zerolog.Logger) *SimpleRetryManager {
	coordinator := retry.New()

	// Set up conversation-specific retry policy
	coordinator.SetPolicy("conversation", &retry.Policy{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffStrategy: retry.BackoffExponential,
		Multiplier:      2.0,
		Jitter:          true,
		ErrorPatterns: []string{
			"timeout", "deadline exceeded", "connection refused",
			"temporary failure", "rate limit", "throttled",
			"service unavailable", "504", "503", "502",
		},
	})

	return &SimpleRetryManager{
		logger:           logger.With().Str("component", "retry_manager").Logger(),
		retryCoordinator: coordinator,
	}
}

// ExecuteWithRetry executes a function with unified retry coordination
func (rm *SimpleRetryManager) ExecuteWithRetry(ctx context.Context, operationType string, fn retry.RetryableFunc) error {
	return rm.retryCoordinator.Execute(ctx, operationType, fn)
}
