package retry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryCoordinator_Execute_Success(t *testing.T) {
	t.Parallel()
	coordinator := New()

	callCount := 0
	fn := func(_ context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.Network("test", "temporary network error")
		}
		return nil
	}

	err := coordinator.Execute(context.Background(), "test", fn)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryCoordinator_Execute_MaxAttemptsReached(t *testing.T) {
	t.Parallel()
	coordinator := New()

	// Set a custom policy with max 2 attempts
	coordinator.SetPolicy("test", &Policy{
		MaxAttempts:     2,
		InitialDelay:    time.Millisecond,
		MaxDelay:        time.Millisecond * 10,
		BackoffStrategy: BackoffFixed,
		ErrorPatterns:   []string{"network error"},
	})

	callCount := 0
	fn := func(_ context.Context) error {
		callCount++
		return errors.Network("test", "persistent network error")
	}

	err := coordinator.Execute(context.Background(), "test", fn)
	assert.Error(t, err)
	assert.Equal(t, 2, callCount)
	assert.Contains(t, err.Error(), "operation failed after 2 attempts")
}

func TestRetryCoordinator_Execute_NonRetryableError(t *testing.T) {
	t.Parallel()
	coordinator := New()

	callCount := 0
	fn := func(_ context.Context) error {
		callCount++
		return errors.Validation("test", "invalid input")
	}

	err := coordinator.Execute(context.Background(), "test", fn)
	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // Should not retry validation errors
}

func TestRetryCoordinator_ExecuteWithFix_Success(t *testing.T) {
	t.Parallel()
	coordinator := New()

	// Register a mock fix provider
	mockFixProvider := &MockFixProvider{}
	coordinator.RegisterFixProvider("unknown", mockFixProvider) // Default category for unmatched errors

	// Set policy that retries on config errors
	coordinator.SetPolicy("test", &Policy{
		MaxAttempts:     3,
		InitialDelay:    time.Millisecond,
		MaxDelay:        time.Millisecond * 10,
		BackoffStrategy: BackoffFixed,
		ErrorPatterns:   []string{"config not found"},
	})

	callCount := 0
	fn := func(_ context.Context, _ *Context) error {
		callCount++
		if callCount < 2 {
			return fmt.Errorf("config not found")
		}
		return nil
	}

	err := coordinator.ExecuteWithFix(context.Background(), "test", fn)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 2) // Should be called at least twice
}

func TestRetryCoordinator_BackoffStrategies(t *testing.T) {
	t.Parallel()
	coordinator := New()

	tests := []struct {
		name     string
		strategy BackoffStrategy
		attempt  int
		initial  time.Duration
		expected time.Duration
	}{
		{
			name:     "Fixed backoff",
			strategy: BackoffFixed,
			attempt:  2,
			initial:  time.Second,
			expected: time.Second,
		},
		{
			name:     "Linear backoff",
			strategy: BackoffLinear,
			attempt:  2,
			initial:  time.Second,
			expected: time.Second * 3, // (attempt + 1) * initial
		},
		{
			name:     "Exponential backoff",
			strategy: BackoffExponential,
			attempt:  2,
			initial:  time.Second,
			expected: time.Second * 4, // 2^2 * initial
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policy := &Policy{
				BackoffStrategy: tt.strategy,
				InitialDelay:    tt.initial,
				MaxDelay:        time.Minute,
				Multiplier:      2.0,
				Jitter:          false,
			}

			delay := coordinator.calculateDelay(policy, tt.attempt)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestRetryCoordinator_CircuitBreaker(t *testing.T) {
	t.Parallel()
	coordinator := New()

	// Set policy that ensures retries happen for network errors
	coordinator.SetPolicy("test", &Policy{
		MaxAttempts:     2, // Small number to trigger circuit breaker faster
		InitialDelay:    time.Millisecond,
		MaxDelay:        time.Millisecond * 10,
		BackoffStrategy: BackoffFixed,
		ErrorPatterns:   []string{"service down"},
	})

	// Use ExecuteWithFix since circuit breaker is only active there
	callCount := 0
	fn := func(_ context.Context, _ *Context) error {
		callCount++
		return fmt.Errorf("service down")
	}

	// Make several failed calls to trip the circuit breaker
	for i := 0; i < 6; i++ {
		_ = coordinator.ExecuteWithFix(context.Background(), "test", fn)
	}

	cb := coordinator.getCircuitBreaker("test")
	assert.Equal(t, "open", cb.State)

	// Next call should fail immediately due to open circuit
	err := coordinator.ExecuteWithFix(context.Background(), "test", fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestErrorClassifier_ClassifyError(t *testing.T) {
	t.Parallel()
	classifier := NewErrorClassifier()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "Network error",
			err:      fmt.Errorf("connection refused"),
			expected: "network",
		},
		{
			name:     "Docker error",
			err:      fmt.Errorf("docker daemon not running"),
			expected: "docker",
		},
		{
			name:     "Config error",
			err:      fmt.Errorf("configuration error: missing required field"),
			expected: "config",
		},
		{
			name:     "MCP Network error",
			err:      errors.Network("test", "network issue"),
			expected: "network",
		},
		{
			name:     "Unknown error",
			err:      fmt.Errorf("something completely unexpected"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			category := classifier.ClassifyError(tt.err)
			assert.Equal(t, tt.expected, category)
		})
	}
}

func TestErrorClassifier_IsRetryable(t *testing.T) {
	t.Parallel()
	classifier := NewErrorClassifier()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Network error is retryable",
			err:      fmt.Errorf("connection timeout"),
			expected: true,
		},
		{
			name:     "Validation error is not retryable",
			err:      fmt.Errorf("validation failed"),
			expected: false,
		},
		{
			name:     "MCP retryable error",
			err:      &errors.MCPError{Retryable: true},
			expected: true,
		},
		{
			name:     "MCP non-retryable error",
			err:      &errors.MCPError{Retryable: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			retryable := classifier.IsRetryable(tt.err)
			assert.Equal(t, tt.expected, retryable)
		})
	}
}

func TestFixProviders_Docker(t *testing.T) {
	t.Parallel()
	provider := NewDockerFixProvider()

	err := fmt.Errorf("dockerfile syntax error on line 5")
	strategies, stratErr := provider.GetFixStrategies(context.Background(), err, map[string]interface{}{
		"dockerfile_path": "/tmp/Dockerfile",
	})

	require.NoError(t, stratErr)
	require.Len(t, strategies, 1)

	strategy := strategies[0]
	assert.Equal(t, "dockerfile", strategy.Type)
	assert.Equal(t, "Fix Dockerfile Syntax", strategy.Name)
	assert.True(t, strategy.Automated)
	assert.Equal(t, 1, strategy.Priority)
}

func TestFixProviders_Config(t *testing.T) {
	t.Parallel()
	provider := NewConfigFixProvider()

	err := fmt.Errorf("config.json not found")
	strategies, stratErr := provider.GetFixStrategies(context.Background(), err, map[string]interface{}{})

	require.NoError(t, stratErr)
	require.Len(t, strategies, 1)

	strategy := strategies[0]
	assert.Equal(t, "config", strategy.Type)
	assert.Equal(t, "Create Default Config", strategy.Name)
	assert.True(t, strategy.Automated)
}

func TestFixProviders_Dependency(t *testing.T) {
	t.Parallel()
	provider := NewDependencyFixProvider()

	err := fmt.Errorf("command not found: git")
	strategies, stratErr := provider.GetFixStrategies(context.Background(), err, map[string]interface{}{})

	require.NoError(t, stratErr)
	require.Len(t, strategies, 1)

	strategy := strategies[0]
	assert.Equal(t, "dependency", strategy.Type)
	assert.Equal(t, "Install Missing Command", strategy.Name)
	assert.Equal(t, "git", strategy.Parameters["command"])
}

// MockFixProvider for testing
type MockFixProvider struct {
	GetStrategiesCalled bool
	ApplyFixCalled      bool
}

func (m *MockFixProvider) Name() string {
	return "mock"
}

func (m *MockFixProvider) GetFixStrategies(_ context.Context, _ error, _ map[string]interface{}) ([]FixStrategy, error) {
	m.GetStrategiesCalled = true
	return []FixStrategy{
		{
			Type:        "config",
			Name:        "Mock Fix",
			Description: "Mock fix for testing",
			Priority:    1,
			Automated:   true,
		},
	}, nil
}

func (m *MockFixProvider) ApplyFix(_ context.Context, _ FixStrategy, _ map[string]interface{}) error {
	m.ApplyFixCalled = true
	return nil
}
