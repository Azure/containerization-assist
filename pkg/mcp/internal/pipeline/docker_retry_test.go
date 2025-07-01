package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewDockerRetryManager(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	config := RetryManagerConfig{
		MaxHistorySize:   1000,
		LearningEnabled:  true,
		DefaultPolicies:  true,
		DefaultFallbacks: true,
	}

	manager := NewDockerRetryManager(config, logger)

	assert.NotNil(t, manager)
	assert.Equal(t, config.MaxHistorySize, manager.maxHistorySize)
	assert.Equal(t, config.LearningEnabled, manager.learningEnabled)
	assert.NotNil(t, manager.retryPolicies)
	assert.NotNil(t, manager.fallbackChains)
	assert.NotNil(t, manager.operationHistory)
	assert.NotNil(t, manager.circuitBreakers)
	assert.NotNil(t, manager.adaptiveSettings)
	assert.NotNil(t, manager.retryMetrics)

	// Check default policies were created
	assert.NotEmpty(t, manager.retryPolicies)
	assert.NotEmpty(t, manager.fallbackChains)
}

func TestDockerRetryManager_AddRetryPolicy(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	policy := &RetryPolicy{
		Name:                 "test_policy",
		MaxAttempts:          3,
		BaseDelay:            time.Second,
		MaxDelay:             10 * time.Second,
		BackoffStrategy:      BackoffExponential,
		BackoffMultiplier:    2.0,
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          time.Minute,
		},
	}

	manager.AddRetryPolicy(policy)

	// Verify policy was added
	storedPolicy, exists := manager.retryPolicies["test_policy"]
	assert.True(t, exists)
	assert.Equal(t, policy.Name, storedPolicy.Name)
	assert.Equal(t, policy.MaxAttempts, storedPolicy.MaxAttempts)

	// Verify circuit breaker was created
	breaker, exists := manager.circuitBreakers["test_policy"]
	assert.True(t, exists)
	assert.Equal(t, CircuitBreakerClosed, breaker.State)
}

func TestDockerRetryManager_AddFallbackChain(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	chain := &FallbackChain{
		Name:         "test_chain",
		Description:  "Test fallback chain",
		Enabled:      true,
		MaxFallbacks: 2,
		Strategies: []FallbackStrategy{
			{
				Type:        FallbackRegistrySwitch,
				Name:        "registry_switch",
				Description: "Switch registry",
				Priority:    1,
				Parameters: map[string]interface{}{
					"alternative_registry": "ghcr.io",
				},
			},
		},
	}

	manager.AddFallbackChain(chain)

	// Verify chain was added
	storedChain, exists := manager.fallbackChains["test_chain"]
	assert.True(t, exists)
	assert.Equal(t, chain.Name, storedChain.Name)
	assert.Equal(t, chain.Enabled, storedChain.Enabled)
	assert.Len(t, storedChain.Strategies, 1)
}

func TestDockerRetryManager_ExecuteWithRetry_Success(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{
		DefaultPolicies: true,
	}
	manager := NewDockerRetryManager(config, logger)

	ctx := context.Background()
	expectedResult := "success"
	callCount := 0

	// Operation that succeeds on first try
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		callCount++
		return expectedResult, nil
	}

	params := map[string]interface{}{
		"image": "nginx:latest",
	}

	result, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "standard")

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	assert.Equal(t, 1, callCount) // Should only be called once

	// Check metrics
	metrics := manager.GetRetryMetrics("pull")
	assert.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.TotalAttempts)
	assert.Equal(t, 0, metrics.SuccessfulRetries) // No retries needed
	assert.Equal(t, 0, metrics.FailedRetries)
}

func TestDockerRetryManager_ExecuteWithRetry_SuccessAfterRetries(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add a test policy with short delays
	policy := &RetryPolicy{
		Name:             "test_policy",
		MaxAttempts:      3,
		BaseDelay:        10 * time.Millisecond,
		MaxDelay:         100 * time.Millisecond,
		BackoffStrategy:  BackoffConstant,
		RetryableErrors:  []string{"network"},
		OperationTimeout: time.Second,
		TotalTimeout:     5 * time.Second,
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	expectedResult := "success"
	callCount := 0

	// Operation that fails twice, then succeeds
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("network timeout")
		}
		return expectedResult, nil
	}

	params := map[string]interface{}{
		"image": "nginx:latest",
	}

	result, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "test_policy")

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	assert.Equal(t, 3, callCount) // Should be called 3 times

	// Check metrics
	metrics := manager.GetRetryMetrics("pull")
	assert.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.TotalAttempts)
	assert.Equal(t, 1, metrics.SuccessfulRetries) // One successful retry operation
	assert.Equal(t, 0, metrics.FailedRetries)

	// Check operation history
	history := manager.GetOperationHistory("pull", 10)
	assert.Len(t, history, 3)
	assert.False(t, history[0].Success) // First attempt failed
	assert.False(t, history[1].Success) // Second attempt failed
	assert.True(t, history[2].Success)  // Third attempt succeeded
}

func TestDockerRetryManager_ExecuteWithRetry_AllAttemptsFail(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add a test policy with short delays
	policy := &RetryPolicy{
		Name:             "test_policy",
		MaxAttempts:      2,
		BaseDelay:        10 * time.Millisecond,
		MaxDelay:         100 * time.Millisecond,
		BackoffStrategy:  BackoffConstant,
		RetryableErrors:  []string{"network"},
		OperationTimeout: time.Second,
		TotalTimeout:     5 * time.Second,
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	callCount := 0

	// Operation that always fails
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		callCount++
		return nil, errors.New("network timeout")
	}

	params := map[string]interface{}{
		"image": "nginx:latest",
	}

	result, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "test_policy")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 2, callCount) // Should be called 2 times (max attempts)
	assert.Contains(t, err.Error(), "operation failed after 2 attempts")

	// Check metrics
	metrics := manager.GetRetryMetrics("pull")
	assert.NotNil(t, metrics)
	assert.Equal(t, 2, metrics.TotalAttempts)
	assert.Equal(t, 0, metrics.SuccessfulRetries)
	assert.Equal(t, 1, metrics.FailedRetries)
}

func TestDockerRetryManager_ExecuteWithRetry_NonRetryableError(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add a test policy
	policy := &RetryPolicy{
		Name:               "test_policy",
		MaxAttempts:        3,
		BaseDelay:          10 * time.Millisecond,
		NonRetryableErrors: []string{"not found"},
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	callCount := 0

	// Operation that fails with non-retryable error
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		callCount++
		return nil, errors.New("image not found")
	}

	params := map[string]interface{}{
		"image": "nonexistent:latest",
	}

	result, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "test_policy")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, callCount) // Should only be called once (non-retryable)
}

func TestDockerRetryManager_BackoffStrategies(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	tests := []struct {
		name      string
		strategy  BackoffStrategy
		baseDelay time.Duration
		attempt   int
		expected  time.Duration
	}{
		{
			name:      "constant_backoff",
			strategy:  BackoffConstant,
			baseDelay: 100 * time.Millisecond,
			attempt:   3,
			expected:  100 * time.Millisecond,
		},
		{
			name:      "linear_backoff",
			strategy:  BackoffLinear,
			baseDelay: 100 * time.Millisecond,
			attempt:   3,
			expected:  300 * time.Millisecond,
		},
		{
			name:      "exponential_backoff",
			strategy:  BackoffExponential,
			baseDelay: 100 * time.Millisecond,
			attempt:   3,
			expected:  400 * time.Millisecond, // 100 * 2^(3-1) = 100 * 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &RetryPolicy{
				BackoffStrategy:   tt.strategy,
				BaseDelay:         tt.baseDelay,
				MaxDelay:          time.Hour, // High max to not interfere
				BackoffMultiplier: 2.0,
				Jitter:            false, // Disable jitter for predictable testing
			}

			delay := manager.calculateDelay(policy, tt.attempt, "test")
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestDockerRetryManager_FallbackExecution(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add fallback chain
	chain := &FallbackChain{
		Name:         "pull",
		Description:  "Pull fallback chain",
		Enabled:      true,
		MaxFallbacks: 2,
		Strategies: []FallbackStrategy{
			{
				Type:        FallbackRegistrySwitch,
				Name:        "registry_switch",
				Description: "Switch to alternative registry",
				Priority:    1,
				Parameters: map[string]interface{}{
					"alternative_registry": "ghcr.io",
				},
			},
			{
				Type:        FallbackDegradedMode,
				Name:        "degraded_mode",
				Description: "Degraded mode fallback",
				Priority:    2,
			},
		},
	}
	manager.AddFallbackChain(chain)

	// Add retry policy that exhausts attempts quickly
	policy := &RetryPolicy{
		Name:            "test_policy",
		MaxAttempts:     1, // Only 1 attempt, then fallback
		BaseDelay:       10 * time.Millisecond,
		RetryableErrors: []string{"network"},
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	callCount := 0

	// Operation that always fails
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		callCount++
		return nil, errors.New("network timeout")
	}

	params := map[string]interface{}{
		"image": "nginx:latest",
	}

	result, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "test_policy")

	// Should succeed due to fallback
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, callCount) // Original operation called once

	// Check that fallback was used
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "registry_switch", resultMap["fallback_type"])

	// Check metrics
	metrics := manager.GetRetryMetrics("pull")
	assert.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.FallbacksUsed)
}

func TestDockerRetryManager_CircuitBreaker(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add policy with circuit breaker
	policy := &RetryPolicy{
		Name:                 "test_policy",
		MaxAttempts:          1,
		BaseDelay:            10 * time.Millisecond,
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 2, // Open after 2 failures
			SuccessThreshold: 1,
			Timeout:          100 * time.Millisecond,
		},
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()

	// Failing operation
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return nil, errors.New("always fails")
	}

	params := map[string]interface{}{"image": "test:latest"}

	// First failure
	_, err := manager.ExecuteWithRetry(ctx, "test_op", operationFunc, params, "test_policy")
	assert.Error(t, err)

	// Check circuit breaker is still closed
	breaker := manager.GetCircuitBreakerStatus("test_policy")
	assert.NotNil(t, breaker)
	assert.Equal(t, CircuitBreakerClosed, breaker.State)

	// Second failure - should open circuit
	_, err = manager.ExecuteWithRetry(ctx, "test_op", operationFunc, params, "test_policy")
	assert.Error(t, err)

	// Check circuit breaker is now open
	breaker = manager.GetCircuitBreakerStatus("test_policy")
	assert.Equal(t, CircuitBreakerOpen, breaker.State)

	// Third attempt should fail immediately due to open circuit
	_, err = manager.ExecuteWithRetry(ctx, "test_op", operationFunc, params, "test_policy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestDockerRetryManager_CircuitBreakerReset(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Add policy with circuit breaker
	policy := &RetryPolicy{
		Name:                 "test_policy",
		MaxAttempts:          1,
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 1, // Open after 1 failure
			SuccessThreshold: 1,
			Timeout:          10 * time.Millisecond,
		},
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	params := map[string]interface{}{"image": "test:latest"}

	// Cause circuit to open
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return nil, errors.New("failure")
	}

	_, err := manager.ExecuteWithRetry(ctx, "test_op", operationFunc, params, "test_policy")
	assert.Error(t, err)

	// Verify circuit is open
	breaker := manager.GetCircuitBreakerStatus("test_policy")
	assert.Equal(t, CircuitBreakerOpen, breaker.State)

	// Reset circuit breaker manually
	err = manager.ResetCircuitBreaker("test_policy")
	assert.NoError(t, err)

	// Verify circuit is closed
	breaker = manager.GetCircuitBreakerStatus("test_policy")
	assert.Equal(t, CircuitBreakerClosed, breaker.State)

	// Should be able to execute operations again
	successFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return "success", nil
	}

	result, err := manager.ExecuteWithRetry(ctx, "test_op", successFunc, params, "test_policy")
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestDockerRetryManager_AdaptiveLearning(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{
		LearningEnabled: true,
	}
	manager := NewDockerRetryManager(config, logger)

	// Add policy with adaptive backoff
	policy := &RetryPolicy{
		Name:            "test_policy",
		MaxAttempts:     1,
		BaseDelay:       100 * time.Millisecond,
		BackoffStrategy: BackoffAdaptive,
		EnableAdaptive:  true,
	}
	manager.AddRetryPolicy(policy)

	ctx := context.Background()
	params := map[string]interface{}{"image": "test:latest"}

	// Execute some operations to build learning data
	for i := 0; i < 5; i++ {
		operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			time.Sleep(50 * time.Millisecond) // Simulate work
			return "success", nil
		}

		_, err := manager.ExecuteWithRetry(ctx, "test_op", operationFunc, params, "test_policy")
		assert.NoError(t, err)
	}

	// Check that adaptive settings were created and updated
	manager.mutex.RLock()
	settings, exists := manager.adaptiveSettings["test_op"]
	manager.mutex.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, settings)
	assert.Equal(t, "test_op", settings.OperationType)
	assert.Equal(t, 5, settings.SampleCount)
	assert.Greater(t, settings.AverageLatency, time.Duration(0))
	assert.Greater(t, settings.SuccessRate, 0.0)
	assert.Greater(t, settings.OptimalRetryDelay, time.Duration(0))
}

func TestDockerRetryManager_ErrorCategorization(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	tests := []struct {
		error    string
		expected string
	}{
		{"connection timeout", "timeout"},
		{"network unreachable", "network"},
		{"permission denied", "permission"},
		{"image not found", "not_found"},
		{"unauthorized access", "auth"},
		{"internal server error", "server_error"},
		{"rate limit exceeded", "rate_limit"},
		{"unknown error", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.error, func(t *testing.T) {
			err := errors.New(tt.error)
			category := manager.categorizeError(err)
			assert.Equal(t, tt.expected, category)
		})
	}
}

func TestDockerRetryManager_FallbackStrategies(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	ctx := context.Background()
	originalError := errors.New("network timeout")

	tests := []struct {
		name          string
		strategy      FallbackStrategy
		params        map[string]interface{}
		expectSuccess bool
	}{
		{
			name: "registry_switch",
			strategy: FallbackStrategy{
				Type: FallbackRegistrySwitch,
				Parameters: map[string]interface{}{
					"alternative_registry": "ghcr.io",
				},
			},
			params: map[string]interface{}{
				"image": "nginx:latest",
			},
			expectSuccess: true,
		},
		{
			name: "image_variant",
			strategy: FallbackStrategy{
				Type: FallbackImageVariant,
				Parameters: map[string]interface{}{
					"variant_mapping": map[string]string{
						"nginx": "alpine",
					},
				},
			},
			params: map[string]interface{}{
				"image": "nginx:latest",
			},
			expectSuccess: true,
		},
		{
			name: "cached_image",
			strategy: FallbackStrategy{
				Type: FallbackCachedImage,
			},
			params: map[string]interface{}{
				"image": "nginx:latest",
			},
			expectSuccess: true,
		},
		{
			name: "degraded_mode",
			strategy: FallbackStrategy{
				Type: FallbackDegradedMode,
			},
			params: map[string]interface{}{
				"image": "any:latest",
			},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.executeFallbackStrategy(ctx, tt.strategy, tt.params, originalError)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				resultMap, ok := result.(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, string(tt.strategy.Type), resultMap["fallback_type"])
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestDockerRetryManager_Metrics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	// Simulate some operation metrics
	manager.updateSuccessMetrics("pull", 2, 500*time.Millisecond)
	manager.updateSuccessMetrics("pull", 1, 300*time.Millisecond)
	manager.updateFailureMetrics("pull", 3, 1*time.Second)
	manager.updateFallbackMetrics("pull")

	// Get metrics
	metrics := manager.GetRetryMetrics("pull")
	assert.NotNil(t, metrics)
	assert.Equal(t, "pull", metrics.OperationType)
	assert.Equal(t, 6, metrics.TotalAttempts) // 2 + 1 + 3
	assert.Equal(t, 1, metrics.SuccessfulRetries)
	assert.Equal(t, 1, metrics.FailedRetries)
	assert.Equal(t, 1, metrics.FallbacksUsed)
	assert.Equal(t, 3, metrics.MaxRetries)
	assert.Greater(t, metrics.AverageRetries, 0.0)
	assert.Greater(t, metrics.TotalRetryTime, time.Duration(0))

	// Get all metrics
	allMetrics := manager.GetAllRetryMetrics()
	assert.Contains(t, allMetrics, "pull")
	assert.Equal(t, metrics.TotalAttempts, allMetrics["pull"].TotalAttempts)
}

func TestDockerRetryManager_OperationHistory(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	config := RetryManagerConfig{
		MaxHistorySize: 5,
	}
	manager := NewDockerRetryManager(config, logger)

	// Add some operation attempts
	for i := 0; i < 7; i++ {
		attempt := OperationAttempt{
			ID:        fmt.Sprintf("attempt_%d", i),
			Operation: "pull",
			Attempt:   i + 1,
			Timestamp: time.Now(),
			Success:   i%2 == 0, // Alternate success/failure
		}
		manager.recordAttempt("pull", attempt)
	}

	// Get history
	history := manager.GetOperationHistory("pull", 10)

	// Should be limited to max history size
	assert.Len(t, history, 5)

	// Should contain the most recent attempts
	assert.Equal(t, "attempt_6", history[4].ID) // Most recent
	assert.Equal(t, "attempt_2", history[0].ID) // Oldest remaining

	// Test limited retrieval
	limitedHistory := manager.GetOperationHistory("pull", 3)
	assert.Len(t, limitedHistory, 3)
	assert.Equal(t, "attempt_6", limitedHistory[2].ID)
}

// Benchmark tests
func BenchmarkDockerRetryManager_ExecuteWithRetry_Success(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))
	config := RetryManagerConfig{
		DefaultPolicies: true,
	}
	manager := NewDockerRetryManager(config, logger)

	ctx := context.Background()
	operationFunc := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return "success", nil
	}
	params := map[string]interface{}{"image": "nginx:latest"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.ExecuteWithRetry(ctx, "pull", operationFunc, params, "standard")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDockerRetryManager_CalculateDelay(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	policy := &RetryPolicy{
		BackoffStrategy:   BackoffExponential,
		BaseDelay:         100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		JitterRange:       0.1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.calculateDelay(policy, 3, "test_operation")
	}
}

func BenchmarkDockerRetryManager_UpdateMetrics(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))
	config := RetryManagerConfig{}
	manager := NewDockerRetryManager(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.updateSuccessMetrics("pull", 2, 100*time.Millisecond)
	}
}
