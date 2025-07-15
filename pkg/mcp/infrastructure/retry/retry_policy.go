// Package retry provides advanced retry policies for Container Kit
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	RetryableErrors map[string]bool
	OnRetry         func(attempt int, err error)
	OnSuccess       func(attempt int)
	OnFailure       func(err error)
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
		RetryableErrors: map[string]bool{
			"timeout":            true,
			"connection_refused": true,
			"network_error":      true,
			"rate_limit":         true,
			"temporary_failure":  true,
		},
		OnRetry:   func(attempt int, err error) {},
		OnSuccess: func(attempt int) {},
		OnFailure: func(err error) {},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   5,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 1.5,
		JitterFactor:  0.2,
		RetryableErrors: map[string]bool{
			"timeout":            true,
			"connection_refused": true,
			"network_error":      true,
			"rate_limit":         true,
			"temporary_failure":  true,
			"docker_error":       true,
			"registry_error":     true,
		},
		OnRetry:   func(attempt int, err error) {},
		OnSuccess: func(attempt int) {},
		OnFailure: func(err error) {},
	}
}

// ConservativeRetryPolicy returns a conservative retry policy for less critical operations
func ConservativeRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   2,
		InitialDelay:  2 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.05,
		RetryableErrors: map[string]bool{
			"timeout":            true,
			"connection_refused": true,
		},
		OnRetry:   func(attempt int, err error) {},
		OnSuccess: func(attempt int) {},
		OnFailure: func(err error) {},
	}
}

// Execute runs a function with retry logic
func (p *RetryPolicy) Execute(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("retry cancelled: %w", err)
		}

		// Execute the operation
		err := operation()

		// Success
		if err == nil {
			p.OnSuccess(attempt)
			return nil
		}

		lastErr = err

		// Check if we should retry
		if attempt >= p.MaxAttempts || !p.shouldRetry(err) {
			p.OnFailure(err)
			return err
		}

		// Calculate delay with exponential backoff and jitter
		delay := p.calculateDelay(attempt)

		// Notify about retry
		p.OnRetry(attempt, err)

		// Wait before next attempt
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}
	}

	p.OnFailure(lastErr)
	return lastErr
}

// ExecuteWithResult runs a function that returns a result with retry logic
func (p *RetryPolicy) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	var result interface{}

	err := p.Execute(ctx, func() error {
		var err error
		result, err = operation()
		return err
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// shouldRetry determines if an error is retryable
func (p *RetryPolicy) shouldRetry(err error) bool {
	// Check specific error types
	if retryable, ok := err.(RetryableError); ok {
		return retryable.IsRetryable()
	}

	// Check error message patterns
	errMsg := err.Error()
	for pattern, retryable := range p.RetryableErrors {
		if contains(errMsg, pattern) && retryable {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay for the next retry attempt
func (p *RetryPolicy) calculateDelay(attempt int) time.Duration {
	// Exponential backoff
	delay := float64(p.InitialDelay) * math.Pow(p.BackoffFactor, float64(attempt-1))

	// Apply jitter
	jitter := delay * p.JitterFactor * (rand.Float64()*2 - 1) // -jitter to +jitter
	delay += jitter

	// Ensure delay is within bounds
	if delay < float64(p.InitialDelay) {
		delay = float64(p.InitialDelay)
	}
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	return time.Duration(delay)
}

// RetryableError interface for errors that can indicate if they're retryable
type RetryableError interface {
	error
	IsRetryable() bool
}

// RetryableErrorImpl is a simple implementation of RetryableError
type RetryableErrorImpl struct {
	Message    string
	Retryable  bool
	Underlying error
}

func (e *RetryableErrorImpl) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Underlying)
	}
	return e.Message
}

func (e *RetryableErrorImpl) IsRetryable() bool {
	return e.Retryable
}

func (e *RetryableErrorImpl) Unwrap() error {
	return e.Underlying
}

// NewRetryableError creates a new retryable error
func NewRetryableError(message string, retryable bool, underlying error) *RetryableErrorImpl {
	return &RetryableErrorImpl{
		Message:    message,
		Retryable:  retryable,
		Underlying: underlying,
	}
}

// CircuitBreaker implements circuit breaker pattern with retry
type CircuitBreaker struct {
	policy           *RetryPolicy
	failureThreshold int
	resetTimeout     time.Duration
	halfOpenRequests int

	failures     int
	lastFailTime time.Time
	state        CircuitState
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(policy *RetryPolicy, failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		policy:           policy,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		halfOpenRequests: 1,
		state:            StateClosed,
	}
}

// Execute runs an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check circuit state
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is open")
	}

	// Execute with retry policy
	err := cb.policy.Execute(ctx, operation)

	// Update circuit breaker state
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() bool {
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenRequests = 1
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests > 0 {
			cb.halfOpenRequests--
			return true
		}
		return false
	}
	return false
}

// recordFailure records a failure and updates state
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.failureThreshold {
		cb.state = StateOpen
	}
}

// recordSuccess records a success and updates state
func (cb *CircuitBreaker) recordSuccess() {
	if cb.state == StateHalfOpen {
		// Success in half-open state closes the circuit
		cb.state = StateClosed
		cb.failures = 0
	}
}

// AdaptiveRetryPolicy adjusts retry behavior based on success rate
type AdaptiveRetryPolicy struct {
	basePolicy     *RetryPolicy
	successWindow  time.Duration
	adjustInterval time.Duration

	successCount   int
	failureCount   int
	lastAdjustTime time.Time
	currentBackoff float64
}

// NewAdaptiveRetryPolicy creates a new adaptive retry policy
func NewAdaptiveRetryPolicy(basePolicy *RetryPolicy) *AdaptiveRetryPolicy {
	return &AdaptiveRetryPolicy{
		basePolicy:     basePolicy,
		successWindow:  5 * time.Minute,
		adjustInterval: 1 * time.Minute,
		currentBackoff: basePolicy.BackoffFactor,
		lastAdjustTime: time.Now(),
	}
}

// Execute runs an operation with adaptive retry
func (arp *AdaptiveRetryPolicy) Execute(ctx context.Context, operation func() error) error {
	// Adjust policy if needed
	arp.adjustPolicy()

	// Create adjusted policy
	adjustedPolicy := *arp.basePolicy
	adjustedPolicy.BackoffFactor = arp.currentBackoff

	// Execute with adjusted policy
	err := adjustedPolicy.Execute(ctx, operation)

	// Record result
	if err != nil {
		arp.recordFailure()
	} else {
		arp.recordSuccess()
	}

	return err
}

// adjustPolicy adjusts the retry policy based on recent performance
func (arp *AdaptiveRetryPolicy) adjustPolicy() {
	if time.Since(arp.lastAdjustTime) < arp.adjustInterval {
		return
	}

	arp.lastAdjustTime = time.Now()

	total := arp.successCount + arp.failureCount
	if total == 0 {
		return
	}

	successRate := float64(arp.successCount) / float64(total)

	// Adjust backoff based on success rate
	if successRate > 0.8 {
		// High success rate - reduce backoff
		arp.currentBackoff = math.Max(1.2, arp.currentBackoff*0.8)
	} else if successRate < 0.7 {
		// Low success rate - increase backoff
		arp.currentBackoff = math.Min(4.0, arp.currentBackoff*1.2)
	}

	// Reset counters
	arp.successCount = 0
	arp.failureCount = 0
}

func (arp *AdaptiveRetryPolicy) recordSuccess() {
	arp.successCount++
}

func (arp *AdaptiveRetryPolicy) recordFailure() {
	arp.failureCount++
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}

// RetryDecorator wraps a workflow orchestrator with retry logic
type RetryDecorator struct {
	base   workflow.WorkflowOrchestrator
	policy *RetryPolicy
}

// NewRetryDecorator creates a new retry decorator
func NewRetryDecorator(base workflow.WorkflowOrchestrator, policy *RetryPolicy) *RetryDecorator {
	return &RetryDecorator{
		base:   base,
		policy: policy,
	}
}

// Execute runs the complete containerization workflow with retry logic
func (d *RetryDecorator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	var result *workflow.ContainerizeAndDeployResult

	err := d.policy.Execute(ctx, func() error {
		var err error
		result, err = d.base.Execute(ctx, req, args)
		return err
	})

	return result, err
}
