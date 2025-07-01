package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerStates tests circuit breaker state transitions
func TestCircuitBreakerStates(t *testing.T) {
	coordinator := New()

	tests := []struct {
		name                 string
		initialState         string
		failureCount         int
		expectedNewState     string
		shouldAllowExecution bool
	}{
		{
			name:                 "closed circuit allows execution",
			initialState:         "closed",
			failureCount:         0,
			expectedNewState:     "closed",
			shouldAllowExecution: true,
		},
		{
			name:                 "circuit opens after threshold failures",
			initialState:         "closed",
			failureCount:         5,
			expectedNewState:     "open",
			shouldAllowExecution: true, // Should allow execution initially, then test state change
		},
		{
			name:                 "open circuit blocks execution",
			initialState:         "open",
			failureCount:         0,
			expectedNewState:     "open",
			shouldAllowExecution: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get circuit breaker for test operation
			cb := coordinator.getCircuitBreaker("test_operation")
			cb.State = tt.initialState
			cb.FailureCount = tt.failureCount
			cb.Threshold = 5

			// If circuit is open, set next attempt in the future
			if tt.initialState == "open" {
				cb.NextAttempt = time.Now().Add(30 * time.Second)
			}

			// Test if execution is allowed
			allowed := !coordinator.isCircuitOpen(cb)
			assert.Equal(t, tt.shouldAllowExecution, allowed)

			// Simulate failure and check state transition
			if tt.failureCount >= cb.Threshold {
				coordinator.recordCircuitFailure(cb)
				assert.Equal(t, tt.expectedNewState, cb.State)
			}
		})
	}
}

// TestRetryPolicies tests different retry policy configurations
func TestRetryPolicies(t *testing.T) {
	coordinator := New()

	tests := []struct {
		name     string
		policy   *Policy
		errorMsg string
		expected bool
	}{
		{
			name: "retryable error matches pattern",
			policy: &Policy{
				MaxAttempts:   3,
				ErrorPatterns: []string{"timeout", "connection refused"},
			},
			errorMsg: "operation timeout",
			expected: true,
		},
		{
			name: "non-retryable error",
			policy: &Policy{
				MaxAttempts:   3,
				ErrorPatterns: []string{"timeout", "connection refused"},
			},
			errorMsg: "invalid argument",
			expected: false,
		},
		{
			name: "max attempts reached",
			policy: &Policy{
				MaxAttempts:   1,
				ErrorPatterns: []string{"timeout"},
			},
			errorMsg: "timeout",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			result := coordinator.shouldRetry(err, 1, tt.policy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBackoffStrategies tests different backoff calculation strategies
func TestBackoffStrategies(t *testing.T) {
	coordinator := New()

	tests := []struct {
		name     string
		strategy BackoffStrategy
		attempt  int
		policy   *Policy
		expected time.Duration
	}{
		{
			name:     "fixed backoff",
			strategy: BackoffFixed,
			attempt:  1,
			policy: &Policy{
				InitialDelay:    time.Second,
				BackoffStrategy: BackoffFixed,
				MaxDelay:        10 * time.Second,
			},
			expected: time.Second,
		},
		{
			name:     "linear backoff",
			strategy: BackoffLinear,
			attempt:  2,
			policy: &Policy{
				InitialDelay:    time.Second,
				BackoffStrategy: BackoffLinear,
				MaxDelay:        10 * time.Second,
			},
			expected: 3 * time.Second,
		},
		{
			name:     "exponential backoff",
			strategy: BackoffExponential,
			attempt:  2,
			policy: &Policy{
				InitialDelay:    time.Second,
				BackoffStrategy: BackoffExponential,
				Multiplier:      2.0,
				Jitter:          false,
				MaxDelay:        10 * time.Second,
			},
			expected: 4 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := coordinator.calculateDelay(tt.policy, tt.attempt)

			// For non-jitter tests, check exact value
			if !tt.policy.Jitter {
				assert.Equal(t, tt.expected, delay)
			} else {
				// For jitter tests, check if delay is within reasonable range
				assert.GreaterOrEqual(t, delay, tt.expected/2)
				assert.LessOrEqual(t, delay, tt.expected*2)
			}
		})
	}
}

// TestRetryCoordinator tests the basic retry functionality
func TestRetryCoordinator(t *testing.T) {
	coordinator := New()

	// Configure fast test policies to avoid long delays in tests
	fastTestPolicy := &Policy{
		MaxAttempts:     3,
		InitialDelay:    time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffStrategy: BackoffFixed,
		ErrorPatterns:   []string{"timeout"},
	}

	coordinator.SetPolicy("success_test", fastTestPolicy)
	coordinator.SetPolicy("retry_success_test", fastTestPolicy)
	coordinator.SetPolicy("always_fail_test", fastTestPolicy)

	tests := []struct {
		name        string
		operation   string
		shouldFail  bool
		attempts    int
		expectError bool
	}{
		{
			name:        "successful operation",
			operation:   "success_test",
			shouldFail:  false,
			attempts:    1,
			expectError: false,
		},
		{
			name:        "retryable operation that eventually succeeds",
			operation:   "retry_success_test",
			shouldFail:  true,
			attempts:    2,
			expectError: false,
		},
		{
			name:        "operation that always fails",
			operation:   "always_fail_test",
			shouldFail:  true,
			attempts:    3,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0

			fn := func(ctx context.Context) error {
				attemptCount++
				if tt.shouldFail && (tt.expectError || attemptCount < tt.attempts) {
					return errors.New("timeout")
				}
				return nil
			}

			err := coordinator.Execute(context.Background(), tt.operation, fn)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.attempts, attemptCount)
		})
	}
}

// TestFixProviderIntegration tests fix provider integration
func TestFixProviderIntegration(t *testing.T) {
	coordinator := New()

	// Register a test fix provider for temporary errors
	testProvider := &TestFixProvider{}
	coordinator.RegisterFixProvider("temporary", testProvider)

	ctx := context.Background()
	attemptCount := 0
	fixApplied := false

	fn := func(ctx context.Context, retryCtx *Context) error {
		attemptCount++
		if attemptCount == 1 {
			return &TestError{message: "temporary failure"} // Use a retryable error pattern
		}
		// On subsequent attempts, succeed if fix was applied
		if fixApplied {
			return nil
		}
		return &TestError{message: "temporary failure"} // Keep it retryable
	}

	// Mock the fix application
	testProvider.OnApplyFix = func(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error {
		fixApplied = true
		return nil
	}

	err := coordinator.ExecuteWithFix(ctx, "fix_test", fn)

	// The test should succeed after fix is applied
	assert.NoError(t, err)
	assert.True(t, fixApplied)
	assert.Equal(t, 2, attemptCount)
}

// TestError is a test error type
type TestError struct {
	message string
}

func (e *TestError) Error() string {
	return e.message
}

// TestFixProvider is a test implementation of FixProvider
type TestFixProvider struct {
	OnApplyFix func(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error
}

func (p *TestFixProvider) Name() string {
	return "test_fix_provider"
}

func (p *TestFixProvider) GetFixStrategies(ctx context.Context, err error, context map[string]interface{}) ([]FixStrategy, error) {
	return []FixStrategy{
		{
			Type:        "test",
			Name:        "test_fix",
			Description: "Test fix strategy",
			Priority:    1,
			Automated:   true,
		},
	}, nil
}

func (p *TestFixProvider) ApplyFix(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error {
	if p.OnApplyFix != nil {
		return p.OnApplyFix(ctx, strategy, context)
	}
	return nil
}

// TestPerformance tests retry coordinator performance
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	coordinator := New()

	// Test with minimal policy for speed
	coordinator.SetPolicy("perf_test", &Policy{
		MaxAttempts:     2,
		InitialDelay:    time.Millisecond,
		BackoffStrategy: BackoffFixed,
	})

	start := time.Now()
	iterations := 100

	for i := 0; i < iterations; i++ {
		err := coordinator.Execute(context.Background(), "perf_test", func(ctx context.Context) error {
			return nil // Always succeed
		})
		require.NoError(t, err)
	}

	duration := time.Since(start)
	avgPerOp := duration / time.Duration(iterations)

	t.Logf("Performance test: %d operations in %v (avg: %v per operation)", iterations, duration, avgPerOp)

	// Should complete reasonably quickly
	assert.Less(t, avgPerOp, 10*time.Millisecond, "Operations should be fast")
}

// TestConcurrentRetries tests concurrent retry operations
func TestConcurrentRetries(t *testing.T) {
	coordinator := New()

	// Use a simple policy for concurrent testing
	coordinator.SetPolicy("concurrent_test", &Policy{
		MaxAttempts:     2,
		InitialDelay:    time.Millisecond,
		BackoffStrategy: BackoffFixed,
	})

	const goroutines = 10
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			err := coordinator.Execute(context.Background(), "concurrent_test", func(ctx context.Context) error {
				// Simulate some work
				time.Sleep(time.Millisecond)
				return nil
			})
			errors <- err
		}(i)
	}

	// Collect all results
	for i := 0; i < goroutines; i++ {
		err := <-errors
		assert.NoError(t, err, "Concurrent operation %d should succeed", i)
	}
}

// TestContextCancellation tests context cancellation during retries
func TestContextCancellation(t *testing.T) {
	coordinator := New()

	coordinator.SetPolicy("cancel_test", &Policy{
		MaxAttempts:     5,
		InitialDelay:    100 * time.Millisecond,
		BackoffStrategy: BackoffFixed,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := coordinator.Execute(ctx, "cancel_test", func(ctx context.Context) error {
		return errors.New("timeout") // Always fail to trigger retries
	})
	duration := time.Since(start)

	// Should fail due to context cancellation
	assert.Error(t, err)
	// Should not take longer than context timeout + some buffer
	assert.Less(t, duration, 200*time.Millisecond)
}
