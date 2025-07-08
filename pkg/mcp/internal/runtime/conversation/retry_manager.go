package conversation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/retry"
)

type SimpleRetryManager struct {
	logger           *slog.Logger
	retryCoordinator *retry.Coordinator
}

func NewSimpleRetryManager(logger *slog.Logger) *SimpleRetryManager {
	coordinator := retry.New()
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
		logger:           logger.With("component", "retry_manager"),
		retryCoordinator: coordinator,
	}
}
func (rm *SimpleRetryManager) ExecuteWithRetry(ctx context.Context, operationType string, fn retry.RetryableFunc) error {
	return rm.retryCoordinator.Execute(ctx, operationType, fn)
}
