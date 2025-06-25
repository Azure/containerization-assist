package conversation

import (
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// SimpleRetryManager implements RetryManager with basic retry logic
type SimpleRetryManager struct {
	logger zerolog.Logger
}

// NewSimpleRetryManager creates a new simple retry manager
func NewSimpleRetryManager(logger zerolog.Logger) *SimpleRetryManager {
	return &SimpleRetryManager{
		logger: logger.With().Str("component", "retry_manager").Logger(),
	}
}

// ShouldRetry determines if an operation should be retried based on the error
func (rm *SimpleRetryManager) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}

	// Max 3 retries
	if attempt >= 3 {
		return false
	}

	// Check if error is retryable
	errStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"temporary failure",
		"rate limit",
		"throttled",
		"service unavailable",
		"504",
		"503",
		"502",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			rm.logger.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("Error is retryable")
			return true
		}
	}

	return false
}

// GetBackoff returns the backoff duration for a given attempt
func (rm *SimpleRetryManager) GetBackoff(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}
	return backoff
}
