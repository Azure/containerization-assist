package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// Coordinator implements api.RetryCoordinator
// This is moved from internal/retry to break import cycles
type Coordinator struct {
	defaultPolicy   api.RetryPolicy
	policies        map[string]api.RetryPolicy
	fixProviders    map[string]api.FixProvider
	circuitBreakers map[string]*circuitBreaker
	mu              sync.RWMutex
	rng             *rand.Rand
}

// New creates a new retry coordinator
func New() api.RetryCoordinator {
	return &Coordinator{
		defaultPolicy:   api.DefaultRetryPolicy(),
		policies:        make(map[string]api.RetryPolicy),
		fixProviders:    make(map[string]api.FixProvider),
		circuitBreakers: make(map[string]*circuitBreaker),
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute implements api.RetryCoordinator
func (c *Coordinator) Execute(ctx context.Context, name string, fn api.RetryableFunc) error {
	return c.ExecuteWithPolicy(ctx, name, c.defaultPolicy, fn)
}

// ExecuteWithPolicy implements api.RetryCoordinator
func (c *Coordinator) ExecuteWithPolicy(ctx context.Context, name string, policy api.RetryPolicy, fn api.RetryableFunc) error {
	// Check circuit breaker
	if cb := c.getCircuitBreaker(name); cb != nil && !cb.CanExecute() {
		return mcperrors.NewError().Messagef("circuit breaker open for %s", name).WithLocation().Build()
	}

	var lastErr error
	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		// Check context
		if err := ctx.Err(); err != nil {
			return mcperrors.NewError().Messagef("context cancelled: %w", err).WithLocation(

			// Execute function
			).Build()
		}

		err := fn(ctx)
		if err == nil {
			// Success - record in circuit breaker
			if cb := c.getCircuitBreaker(name); cb != nil {
				cb.RecordSuccess()
			}
			return nil
		}

		lastErr = err

		// Record failure in circuit breaker
		if cb := c.getCircuitBreaker(name); cb != nil {
			cb.RecordFailure(err)
		}

		// Check if we should retry
		if attempt >= policy.MaxAttempts-1 {
			break
		}

		// Calculate delay
		delay := c.calculateDelay(attempt, policy)

		// Wait before retry
		select {
		case <-ctx.Done():
			return mcperrors.NewError().Messagef("context cancelled during retry: %w", ctx.Err()).WithLocation().Build()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return mcperrors.NewError().Messagef("all retry attempts failed: %w", lastErr).WithLocation(

	// RegisterPolicy implements api.RetryCoordinator
	).Build()
}

func (c *Coordinator) RegisterPolicy(name string, policy api.RetryPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.policies[name] = policy
	return nil
}

// GetPolicy implements api.RetryCoordinator
func (c *Coordinator) GetPolicy(name string) (api.RetryPolicy, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policy, ok := c.policies[name]
	if !ok {
		return api.RetryPolicy{}, mcperrors.NewError().Messagef("policy not found: %s", name).WithLocation().Build()
	}

	return policy, nil
}

// RegisterFixProvider implements api.RetryCoordinator
func (c *Coordinator) RegisterFixProvider(name string, provider api.FixProvider) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fixProviders[name] = provider
	return nil
}

// ExecuteWithFix implements api.RetryCoordinator
func (c *Coordinator) ExecuteWithFix(ctx context.Context, name string, fn api.FixableFunc) error {
	retryCtx := &retryContext{
		metadata: make(map[string]interface{}),
	}

	return c.ExecuteWithPolicy(ctx, name, c.defaultPolicy, func(ctx context.Context) error {
		retryCtx.attempt++
		err := fn(ctx, retryCtx)
		if err != nil {
			retryCtx.lastError = err

			// Try to apply fixes
			if err := c.applyFixes(ctx, err, retryCtx); err == nil {
				// Fix applied successfully, retry the operation
				return fn(ctx, retryCtx)
			}
		}
		return err
	})
}

// calculateDelay calculates the delay for the next retry attempt
func (c *Coordinator) calculateDelay(attempt int, policy api.RetryPolicy) time.Duration {
	var delay time.Duration

	switch policy.BackoffMultiplier {
	case 0:
		// Fixed backoff
		delay = policy.InitialDelay
	case 1:
		// Linear backoff
		delay = policy.InitialDelay * time.Duration(attempt+1)
	default:
		// Exponential backoff
		delay = time.Duration(float64(policy.InitialDelay) * math.Pow(policy.BackoffMultiplier, float64(attempt)))
	}

	// Apply max delay
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Apply jitter if enabled (not in api.RetryPolicy but could be added)
	// For now, add 10% jitter
	if delay > 0 {
		c.mu.Lock()
		jitter := time.Duration(c.rng.Int63n(int64(delay / 10)))
		c.mu.Unlock()
		delay += jitter
	}

	return delay
}

// getCircuitBreaker gets or creates a circuit breaker for the given name
func (c *Coordinator) getCircuitBreaker(name string) *circuitBreaker {
	c.mu.RLock()
	cb, ok := c.circuitBreakers[name]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		// Double-check after acquiring write lock
		if cb, ok = c.circuitBreakers[name]; !ok {
			cb = newCircuitBreaker()
			c.circuitBreakers[name] = cb
		}
		c.mu.Unlock()
	}

	return cb
}

// applyFixes attempts to apply fixes for the given error
func (c *Coordinator) applyFixes(ctx context.Context, err error, retryCtx *retryContext) error {
	c.mu.RLock()
	providers := make([]api.FixProvider, 0, len(c.fixProviders))
	for _, p := range c.fixProviders {
		providers = append(providers, p)
	}
	c.mu.RUnlock()

	for _, provider := range providers {
		strategies, err := provider.GetFixStrategies(ctx, err, retryCtx.metadata)
		if err != nil {
			continue
		}

		for _, strategy := range strategies {
			if strategy.Automated {
				if err := provider.ApplyFix(ctx, strategy, retryCtx.metadata); err == nil {
					return nil // Fix applied successfully
				}
			}
		}
	}

	return fmt.Errorf("no fixes available")
}

// retryContext implements api.RetryContext
type retryContext struct {
	attempt   int
	lastError error
	metadata  map[string]interface{}
	mu        sync.RWMutex
}

func (r *retryContext) GetAttempt() int {
	return r.attempt
}

func (r *retryContext) GetLastError() error {
	return r.lastError
}

func (r *retryContext) GetMetadata() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy
	result := make(map[string]interface{}, len(r.metadata))
	for k, v := range r.metadata {
		result[k] = v
	}
	return result
}

func (r *retryContext) SetMetadata(key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metadata[key] = value
}

// circuitBreaker implements a simple circuit breaker
type circuitBreaker struct {
	state        api.CircuitBreakerState
	failures     int
	lastFailure  time.Time
	successCount int
	mu           sync.RWMutex

	// Configuration
	failureThreshold int
	recoveryTimeout  time.Duration
	successThreshold int
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		state:            api.CircuitClosed,
		failureThreshold: 5,
		recoveryTimeout:  30 * time.Second,
		successThreshold: 2,
	}
}

func (cb *circuitBreaker) GetState() api.CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++

	if cb.state == api.CircuitHalfOpen && cb.successCount >= cb.successThreshold {
		cb.state = api.CircuitClosed
		cb.failures = 0
		cb.successCount = 0
	}
}

func (cb *circuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	cb.successCount = 0

	if cb.failures >= cb.failureThreshold {
		cb.state = api.CircuitOpen
	}
}

func (cb *circuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case api.CircuitClosed:
		return true
	case api.CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailure) > cb.recoveryTimeout {
			cb.state = api.CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case api.CircuitHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *circuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = api.CircuitClosed
	cb.failures = 0
	cb.successCount = 0
	cb.lastFailure = time.Time{}
}
