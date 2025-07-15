package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxAttempts)
	assert.Equal(t, 1*time.Second, policy.InitialDelay)
	assert.Equal(t, 30*time.Second, policy.MaxDelay)
	assert.Equal(t, 2.0, policy.BackoffFactor)
	assert.Equal(t, 0.1, policy.JitterFactor)
}

func TestRetryPolicy_Execute_Success(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()

	attempts := 0
	err := policy.Execute(ctx, func() error {
		attempts++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetryPolicy_Execute_RetryableError(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 10 * time.Millisecond // Speed up test

	attempts := 0
	err := policy.Execute(ctx, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("connection_refused")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryPolicy_Execute_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()

	attempts := 0
	err := policy.Execute(ctx, func() error {
		attempts++
		return errors.New("permanent_error")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetryPolicy_Execute_MaxAttempts(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := policy.Execute(ctx, func() error {
		attempts++
		return errors.New("timeout")
	})

	assert.Error(t, err)
	assert.Equal(t, policy.MaxAttempts, attempts)
}

func TestRetryPolicy_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 100 * time.Millisecond

	attempts := 0

	// Cancel context after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := policy.Execute(ctx, func() error {
		attempts++
		return errors.New("timeout")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Equal(t, 1, attempts)
}

func TestRetryPolicy_ExecuteWithResult(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 10 * time.Millisecond

	attempts := 0
	result, err := policy.ExecuteWithResult(ctx, func() (interface{}, error) {
		attempts++
		if attempts < 2 {
			return nil, errors.New("timeout")
		}
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 2, attempts)
}

func TestRetryPolicy_Callbacks(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 10 * time.Millisecond

	var retryCount int
	var successAttempt int
	var failureErr error

	policy.OnRetry = func(attempt int, err error) {
		retryCount++
	}
	policy.OnSuccess = func(attempt int) {
		successAttempt = attempt
	}
	policy.OnFailure = func(err error) {
		failureErr = err
	}

	// Test success case
	err := policy.Execute(ctx, func() error {
		if retryCount < 2 {
			return errors.New("timeout")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, retryCount)
	assert.Equal(t, 3, successAttempt)
	assert.Nil(t, failureErr)

	// Test failure case
	retryCount = 0
	successAttempt = 0
	failureErr = nil

	err = policy.Execute(ctx, func() error {
		return errors.New("permanent_error")
	})

	assert.Error(t, err)
	assert.Equal(t, 0, retryCount)
	assert.Equal(t, 0, successAttempt)
	assert.NotNil(t, failureErr)
}

func TestRetryableError(t *testing.T) {
	err := NewRetryableError("test error", true, nil)
	assert.True(t, err.IsRetryable())
	assert.Equal(t, "test error", err.Error())

	underlyingErr := errors.New("underlying")
	err = NewRetryableError("wrapper", false, underlyingErr)
	assert.False(t, err.IsRetryable())
	assert.Contains(t, err.Error(), "wrapper")
	assert.Contains(t, err.Error(), "underlying")
	assert.Equal(t, underlyingErr, err.Unwrap())
}

func TestCircuitBreaker_Closed(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.MaxAttempts = 1
	cb := NewCircuitBreaker(policy, 3, 1*time.Second)

	// Circuit should be closed initially
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.MaxAttempts = 1
	cb := NewCircuitBreaker(policy, 3, 100*time.Millisecond)

	// Cause failures to open circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func() error {
			return errors.New("failure")
		})
		assert.Error(t, err)
	}

	// Circuit should be open
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.MaxAttempts = 1
	cb := NewCircuitBreaker(policy, 2, 50*time.Millisecond)

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("failure")
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should be half-open, allowing one request
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)

	// Circuit should be closed after success
	err = cb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestAdaptiveRetryPolicy(t *testing.T) {
	ctx := context.Background()
	basePolicy := DefaultRetryPolicy()
	basePolicy.InitialDelay = 10 * time.Millisecond
	arp := NewAdaptiveRetryPolicy(basePolicy)
	arp.adjustInterval = 10 * time.Millisecond

	// Record mostly successes
	for i := 0; i < 20; i++ {
		err := arp.Execute(ctx, func() error {
			if i%10 == 0 {
				return errors.New("timeout")
			}
			return nil
		})
		if err != nil {
			// Expected for some iterations
		}
	}

	// Wait for adjustment
	time.Sleep(20 * time.Millisecond)

	// Backoff should have decreased
	assert.Less(t, arp.currentBackoff, basePolicy.BackoffFactor)

	// Record mostly failures
	arp.successCount = 0
	arp.failureCount = 0

	for i := 0; i < 20; i++ {
		err := arp.Execute(ctx, func() error {
			if i%5 == 0 {
				return nil
			}
			return errors.New("timeout")
		})
		if err != nil {
			// Expected for most iterations
		}
	}

	// Wait for adjustment
	time.Sleep(20 * time.Millisecond)

	// Backoff should have increased
	assert.Greater(t, arp.currentBackoff, 1.5)
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := DefaultRetryPolicy()

	// Test exponential backoff
	delay1 := policy.calculateDelay(1)
	delay2 := policy.calculateDelay(2)
	delay3 := policy.calculateDelay(3)

	// Each delay should be roughly double the previous (minus jitter)
	assert.Greater(t, delay2, delay1)
	assert.Greater(t, delay3, delay2)

	// Test max delay cap
	delayMax := policy.calculateDelay(10)
	assert.LessOrEqual(t, delayMax, policy.MaxDelay)
}

func BenchmarkRetryPolicy_Execute(b *testing.B) {
	ctx := context.Background()
	policy := DefaultRetryPolicy()
	policy.InitialDelay = 1 * time.Microsecond

	attempts := 0

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		attempts = 0
		policy.Execute(ctx, func() error {
			attempts++
			if attempts < 2 {
				return errors.New("timeout")
			}
			return nil
		})
	}
}

// Mock workflow orchestrator for testing
type mockOrchestrator struct {
	attempts  int
	failUntil int
}

func (m *mockOrchestrator) ExecuteWorkflow(ctx context.Context, args interface{}) (interface{}, error) {
	m.attempts++
	if m.attempts <= m.failUntil {
		return nil, fmt.Errorf("timeout")
	}
	return "success", nil
}
