package errors

import (
	"time"

	"github.com/rs/zerolog"
)

// RetryManager handles retry logic and delay calculations
type RetryManager struct {
	logger        zerolog.Logger
	retryPolicies map[string]*RetryPolicy
}

// NewRetryManager creates a new retry manager
func NewRetryManager(logger zerolog.Logger) *RetryManager {
	return &RetryManager{
		logger:        logger.With().Str("component", "retry_manager").Logger(),
		retryPolicies: make(map[string]*RetryPolicy),
	}
}

// SetRetryPolicy sets a retry policy for a specific stage
func (rm *RetryManager) SetRetryPolicy(stageName string, policy *RetryPolicy) {
	rm.retryPolicies[stageName] = policy

	rm.logger.Info().
		Str("stage_name", stageName).
		Int("max_attempts", policy.MaxAttempts).
		Str("backoff_mode", policy.BackoffMode).
		Msg("Set retry policy for stage")
}

// GetRetryPolicy returns the retry policy for a stage
func (rm *RetryManager) GetRetryPolicy(stageName string) *RetryPolicy {
	if policy, exists := rm.retryPolicies[stageName]; exists {
		return policy
	}

	// Return default policy
	return &RetryPolicy{
		MaxAttempts:  3,
		BackoffMode:  "exponential",
		InitialDelay: 5 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
	}
}

// CalculateRetryDelay calculates the delay before next retry attempt
func (rm *RetryManager) CalculateRetryDelay(policy *RetryPolicy, retryCount int) time.Duration {
	if retryCount >= policy.MaxAttempts {
		return 0 // No more retries
	}

	var delay time.Duration

	switch policy.BackoffMode {
	case "fixed":
		delay = policy.InitialDelay
	case "linear":
		delay = time.Duration(retryCount+1) * policy.InitialDelay
	case "exponential":
		multiplier := policy.Multiplier
		if multiplier <= 0 {
			multiplier = 2.0
		}
		// Fixed exponential calculation
		base := float64(policy.InitialDelay)
		for i := 0; i < retryCount; i++ {
			base *= multiplier
		}
		delay = time.Duration(base)
	default:
		delay = policy.InitialDelay
	}

	// Apply max delay limit
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	rm.logger.Debug().
		Int("retry_count", retryCount).
		Dur("delay", delay).
		Str("backoff_mode", policy.BackoffMode).
		Msg("Calculated retry delay")

	return delay
}

// ShouldRetry determines if a retry should be attempted
func (rm *RetryManager) ShouldRetry(policy *RetryPolicy, retryCount int) bool {
	return retryCount < policy.MaxAttempts
}

// InitializeDefaultPolicies sets up default retry policies
func (rm *RetryManager) InitializeDefaultPolicies() {
	// Network errors - retry with exponential backoff
	rm.retryPolicies["network_error"] = &RetryPolicy{
		MaxAttempts:  3,
		BackoffMode:  "exponential",
		InitialDelay: 5 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
	}

	// Timeout errors - retry with longer timeout
	rm.retryPolicies["timeout_error"] = &RetryPolicy{
		MaxAttempts:  2,
		BackoffMode:  "fixed",
		InitialDelay: 10 * time.Second,
	}

	// Resource unavailable - wait and retry
	rm.retryPolicies["resource_unavailable"] = &RetryPolicy{
		MaxAttempts:  5,
		BackoffMode:  "linear",
		InitialDelay: 30 * time.Second,
		MaxDelay:     300 * time.Second,
		Multiplier:   1.5,
	}
}
