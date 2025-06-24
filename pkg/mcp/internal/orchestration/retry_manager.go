package orchestration

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// RetryManager handles retry logic with configurable backoff strategies
type RetryManager struct {
	logger zerolog.Logger
}

// NewRetryManager creates a new retry manager
func NewRetryManager(logger zerolog.Logger) *RetryManager {
	return &RetryManager{
		logger: logger.With().Str("component", "retry_manager").Logger(),
	}
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts int
	Delays      []time.Duration              // Explicit delays for each retry attempt
	OnRetry     func(attempt int, err error) // Callback before each retry
}

// DefaultKubernetesRetryConfig returns the default retry configuration for Kubernetes operations
func DefaultKubernetesRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Delays:      []time.Duration{1 * time.Second, 4 * time.Second, 10 * time.Second},
	}
}

// ExponentialBackoffRetryConfig returns a retry configuration with exponential backoff and jitter
func ExponentialBackoffRetryConfig(maxAttempts int, baseDelay time.Duration, maxDelay time.Duration) RetryConfig {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	delays := make([]time.Duration, maxAttempts)
	for i := 0; i < maxAttempts; i++ {
		// Exponential backoff: baseDelay * 2^attempt with jitter
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(i)))

		// Cap at maxDelay
		if delay > maxDelay {
			delay = maxDelay
		}

		// Add jitter (±25% randomization to prevent thundering herd)
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.5) // ±25% jitter
		if rand.Intn(2) == 0 {
			delay += jitter
		} else {
			delay -= jitter
		}

		// Ensure minimum delay
		if delay < baseDelay/2 {
			delay = baseDelay / 2
		}

		delays[i] = delay
	}

	return RetryConfig{
		MaxAttempts: maxAttempts,
		Delays:      delays,
	}
}

// RetryOperation represents an operation that can be retried
type RetryOperation func(ctx context.Context, attempt int) error

// ExecuteWithRetry executes an operation with retry logic
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, operation string, config RetryConfig, fn RetryOperation) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Log attempt
		if attempt > 0 {
			rm.logger.Info().
				Str("operation", operation).
				Int("attempt", attempt+1).
				Int("max_attempts", config.MaxAttempts).
				Msg("Retrying operation")
		}

		// Execute the operation
		err := fn(ctx, attempt)

		if err == nil {
			// Success
			if attempt > 0 {
				rm.logger.Info().
					Str("operation", operation).
					Int("attempts", attempt+1).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if attempt >= config.MaxAttempts-1 {
			// No more retries
			break
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Call retry callback if provided
		if config.OnRetry != nil {
			config.OnRetry(attempt+1, err)
		}

		// Determine delay
		var delay time.Duration
		if attempt < len(config.Delays) {
			delay = config.Delays[attempt]
		} else if len(config.Delays) > 0 {
			// Use the last delay if we've exhausted the list
			delay = config.Delays[len(config.Delays)-1]
		} else {
			// Default to 1 second if no delays specified
			delay = 1 * time.Second
		}

		rm.logger.Warn().
			Str("operation", operation).
			Err(err).
			Dur("delay", delay).
			Int("next_attempt", attempt+2).
			Msg("Operation failed, waiting before retry")

		// Wait before retry
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	rm.logger.Error().
		Str("operation", operation).
		Err(lastErr).
		Int("attempts", config.MaxAttempts).
		Msg("Operation failed after all retries")

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// RetryState tracks retry state across conversation turns
type RetryState struct {
	Operation   string
	Attempts    int
	LastError   error
	LastAttempt time.Time
	NextDelay   time.Duration
}

// ShouldRetry determines if an operation should be retried based on error type
func (rm *RetryManager) ShouldRetry(err error, operation string) bool {
	if err == nil {
		return false
	}

	// For Kubernetes operations, check specific error conditions
	errorStr := err.Error()

	// Temporary/transient errors that should be retried
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
		"etcdserver: request timed out",
		"the server is currently unable to handle the request",
	}

	for _, pattern := range retryablePatterns {
		if containsIgnoreCase(errorStr, pattern) {
			rm.logger.Debug().
				Str("operation", operation).
				Str("pattern", pattern).
				Msg("Error matches retryable pattern")
			return true
		}
	}

	// Permanent errors that should NOT be retried
	permanentPatterns := []string{
		"unauthorized",
		"forbidden",
		"invalid",
		"already exists",
		"not found",
		"permission denied",
		"authentication failed",
	}

	for _, pattern := range permanentPatterns {
		if containsIgnoreCase(errorStr, pattern) {
			rm.logger.Debug().
				Str("operation", operation).
				Str("pattern", pattern).
				Msg("Error matches permanent pattern, should not retry")
			return false
		}
	}

	// Default to retrying for unknown errors
	return true
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// CalculateExponentialBackoffDelay calculates delay with exponential backoff and jitter
func (rm *RetryManager) CalculateExponentialBackoffDelay(
	attempt int,
	baseDelay time.Duration,
	maxDelay time.Duration,
	multiplier float64,
) time.Duration {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	// Calculate exponential delay: baseDelay * multiplier^attempt
	delay := time.Duration(float64(baseDelay) * math.Pow(multiplier, float64(attempt)))

	// Cap at maximum delay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter to prevent thundering herd (±25% randomization)
	jitterRange := float64(delay) * 0.25
	jitter := time.Duration((rand.Float64() - 0.5) * 2 * jitterRange)

	finalDelay := delay + jitter

	// Ensure minimum delay (half of base delay)
	minDelay := baseDelay / 2
	if finalDelay < minDelay {
		finalDelay = minDelay
	}

	rm.logger.Debug().
		Int("attempt", attempt).
		Dur("base_delay", baseDelay).
		Dur("calculated_delay", delay).
		Dur("jitter", jitter).
		Dur("final_delay", finalDelay).
		Msg("Calculated exponential backoff delay")

	return finalDelay
}

// ExecuteWithExponentialBackoff executes an operation with exponential backoff retry
func (rm *RetryManager) ExecuteWithExponentialBackoff(
	ctx context.Context,
	operation string,
	maxAttempts int,
	baseDelay time.Duration,
	maxDelay time.Duration,
	fn RetryOperation,
) error {
	multiplier := 2.0
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Log attempt
		if attempt > 0 {
			rm.logger.Info().
				Str("operation", operation).
				Int("attempt", attempt+1).
				Int("max_attempts", maxAttempts).
				Msg("Retrying operation with exponential backoff")
		}

		// Execute the operation
		err := fn(ctx, attempt)
		if err == nil {
			// Success
			if attempt > 0 {
				rm.logger.Info().
					Str("operation", operation).
					Int("attempts", attempt+1).
					Msg("Operation succeeded after exponential backoff retry")
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if attempt >= maxAttempts-1 {
			break
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Calculate exponential backoff delay
		delay := rm.CalculateExponentialBackoffDelay(attempt, baseDelay, maxDelay, multiplier)

		rm.logger.Warn().
			Str("operation", operation).
			Err(err).
			Dur("delay", delay).
			Int("next_attempt", attempt+2).
			Msg("Operation failed, waiting with exponential backoff before retry")

		// Wait before retry
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	rm.logger.Error().
		Str("operation", operation).
		Err(lastErr).
		Int("attempts", maxAttempts).
		Msg("Operation failed after all exponential backoff retries")

	return fmt.Errorf("operation failed after %d attempts with exponential backoff: %w", maxAttempts, lastErr)
}

// GetRetryMessage provides user-friendly retry messaging
func (rm *RetryManager) GetRetryMessage(state RetryState) string {
	if state.Attempts == 0 {
		return fmt.Sprintf("The %s operation failed. Would you like to retry?", state.Operation)
	}

	return fmt.Sprintf(
		"The %s operation failed after %d attempt(s). Error: %v\n\n"+
			"I'll wait %v before the next retry to avoid overwhelming the system.\n"+
			"Would you like to continue retrying?",
		state.Operation,
		state.Attempts,
		state.LastError,
		state.NextDelay,
	)
}
