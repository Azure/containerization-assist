package conversation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

type SimpleRetryManager struct {
	logger           *slog.Logger
	retryCoordinator services.RetryCoordinator
}

func NewSimpleRetryManager(logger *slog.Logger, retryCoordinator services.RetryCoordinator) *SimpleRetryManager {
	retryCoordinator.SetPolicy("conversation", &services.RetryPolicy{
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

	return &SimpleRetryManager{
		logger:           logger.With("component", "retry_manager"),
		retryCoordinator: retryCoordinator,
	}
}
func (rm *SimpleRetryManager) ExecuteWithRetry(ctx context.Context, operationType string, fn func() error) error {
	return rm.retryCoordinator.ExecuteWithRetry(ctx, operationType, fn)
}
