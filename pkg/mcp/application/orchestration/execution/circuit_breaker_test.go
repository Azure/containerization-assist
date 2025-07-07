package execution

import (
	"log/slog"
	"testing"
	"time"
)

// Test CircuitState constants and String method
func TestCircuitState(t *testing.T) {
	// Test constants
	if CircuitClosed != 0 {
		t.Errorf("Expected CircuitClosed to be 0, got %d", CircuitClosed)
	}
	if CircuitOpen != 1 {
		t.Errorf("Expected CircuitOpen to be 1, got %d", CircuitOpen)
	}
	if CircuitHalfOpen != 2 {
		t.Errorf("Expected CircuitHalfOpen to be 2, got %d", CircuitHalfOpen)
	}
}

// Test CircuitState String method
func TestCircuitStateString(t *testing.T) {
	testCases := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"}, // Invalid state
	}

	for _, tc := range testCases {
		result := tc.state.String()
		if result != tc.expected {
			t.Errorf("Expected %v.String() to be '%s', got '%s'", tc.state, tc.expected, result)
		}
	}
}

// Test CircuitBreakerConfig type
func TestCircuitBreakerConfig(t *testing.T) {
	timeout := time.Second * 30
	logger := slog.Default()

	config := CircuitBreakerConfig{
		Name:             "test-circuit",
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          timeout,
		Logger:           logger,
	}

	if config.Name != "test-circuit" {
		t.Errorf("Expected Name to be 'test-circuit', got '%s'", config.Name)
	}
	if config.FailureThreshold != 5 {
		t.Errorf("Expected FailureThreshold to be 5, got %d", config.FailureThreshold)
	}
	if config.SuccessThreshold != 3 {
		t.Errorf("Expected SuccessThreshold to be 3, got %d", config.SuccessThreshold)
	}
	if config.Timeout != timeout {
		t.Errorf("Expected Timeout to be %v, got %v", timeout, config.Timeout)
	}
}

// Test NewCircuitBreaker constructor
func TestNewCircuitBreaker(t *testing.T) {
	logger := slog.Default()
	config := CircuitBreakerConfig{
		Name:             "api-circuit",
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          time.Minute,
		Logger:           logger,
	}

	cb := NewCircuitBreaker(config)

	if cb == nil {
		t.Error("NewCircuitBreaker should not return nil")
	}
	if cb.name != "api-circuit" {
		t.Errorf("Expected name to be 'api-circuit', got '%s'", cb.name)
	}
	if cb.failureThreshold != 10 {
		t.Errorf("Expected failureThreshold to be 10, got %d", cb.failureThreshold)
	}
	if cb.successThreshold != 5 {
		t.Errorf("Expected successThreshold to be 5, got %d", cb.successThreshold)
	}
	if cb.timeout != time.Minute {
		t.Errorf("Expected timeout to be %v, got %v", time.Minute, cb.timeout)
	}
	if cb.state != CircuitClosed {
		t.Errorf("Expected initial state to be CircuitClosed, got %v", cb.state)
	}
	if cb.failureCount != 0 {
		t.Errorf("Expected initial failureCount to be 0, got %d", cb.failureCount)
	}
	if cb.successCount != 0 {
		t.Errorf("Expected initial successCount to be 0, got %d", cb.successCount)
	}
}

// Test CircuitBreaker struct fields
func TestCircuitBreakerStruct(t *testing.T) {
	logger := slog.Default()
	now := time.Now()

	cb := CircuitBreaker{
		name:             "test-breaker",
		failureThreshold: 3,
		successThreshold: 2,
		timeout:          time.Second * 10,
		state:            CircuitOpen,
		failureCount:     5,
		successCount:     1,
		lastFailure:      now,
		lastStateChange:  now.Add(-time.Minute),
		logger:           logger,
	}

	if cb.name != "test-breaker" {
		t.Errorf("Expected name to be 'test-breaker', got '%s'", cb.name)
	}
	if cb.failureThreshold != 3 {
		t.Errorf("Expected failureThreshold to be 3, got %d", cb.failureThreshold)
	}
	if cb.successThreshold != 2 {
		t.Errorf("Expected successThreshold to be 2, got %d", cb.successThreshold)
	}
	if cb.timeout != time.Second*10 {
		t.Errorf("Expected timeout to be 10s, got %v", cb.timeout)
	}
	if cb.state != CircuitOpen {
		t.Errorf("Expected state to be CircuitOpen, got %v", cb.state)
	}
	if cb.failureCount != 5 {
		t.Errorf("Expected failureCount to be 5, got %d", cb.failureCount)
	}
	if cb.successCount != 1 {
		t.Errorf("Expected successCount to be 1, got %d", cb.successCount)
	}
	if cb.lastFailure != now {
		t.Errorf("Expected lastFailure to match, got %v", cb.lastFailure)
	}
}

// Test different CircuitBreakerConfig variations
func TestCircuitBreakerConfigVariations(t *testing.T) {
	logger := slog.Default()

	// Test minimal config
	minimalConfig := CircuitBreakerConfig{
		Name:             "minimal",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Second,
		Logger:           logger,
	}

	cb1 := NewCircuitBreaker(minimalConfig)
	if cb1.failureThreshold != 1 {
		t.Errorf("Expected minimal failureThreshold to be 1, got %d", cb1.failureThreshold)
	}

	// Test high threshold config
	highConfig := CircuitBreakerConfig{
		Name:             "high-threshold",
		FailureThreshold: 100,
		SuccessThreshold: 50,
		Timeout:          time.Hour,
		Logger:           logger,
	}

	cb2 := NewCircuitBreaker(highConfig)
	if cb2.failureThreshold != 100 {
		t.Errorf("Expected high failureThreshold to be 100, got %d", cb2.failureThreshold)
	}
	if cb2.timeout != time.Hour {
		t.Errorf("Expected high timeout to be 1 hour, got %v", cb2.timeout)
	}
}
