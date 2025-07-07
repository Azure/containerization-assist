package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// BackoffStrategy defines different retry backoff strategies
type BackoffStrategy string

const (
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
)

// Policy defines configuration for retry behavior
type Policy struct {
	MaxAttempts     int             `json:"max_attempts"`
	InitialDelay    time.Duration   `json:"initial_delay"`
	MaxDelay        time.Duration   `json:"max_delay"`
	BackoffStrategy BackoffStrategy `json:"backoff_strategy"`
	Multiplier      float64         `json:"multiplier"`
	Jitter          bool            `json:"jitter"`
	ErrorPatterns   []string        `json:"error_patterns"`
}

// FixStrategy represents a fix operation strategy
type FixStrategy struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`
	Parameters  map[string]interface{} `json:"parameters"`
	Automated   bool                   `json:"automated"`
}

// AttemptResult contains the result of a single retry/fix attempt
type AttemptResult struct {
	Attempt   int                    `json:"attempt"`
	Success   bool                   `json:"success"`
	Error     error                  `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Strategy  *FixStrategy           `json:"strategy,omitempty"`
	Applied   bool                   `json:"applied"`
	Timestamp time.Time              `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Context holds context for retry operations
type Context struct {
	OperationID    string                 `json:"operation_id"`
	SessionID      string                 `json:"session_id,omitempty"`
	Policy         *Policy                `json:"policy"`
	AttemptHistory []AttemptResult        `json:"attempt_history"`
	FixStrategies  []FixStrategy          `json:"fix_strategies"`
	MaxFixAttempts int                    `json:"max_fix_attempts"`
	Context        map[string]interface{} `json:"context"`
	CircuitBreaker *CircuitBreakerState   `json:"circuit_breaker,omitempty"`
}

// CircuitBreakerState tracks circuit breaker status
type CircuitBreakerState struct {
	State        string    `json:"state"` // "closed", "open", "half-open"
	FailureCount int       `json:"failure_count"`
	LastFailure  time.Time `json:"last_failure"`
	NextAttempt  time.Time `json:"next_attempt"`
	SuccessCount int       `json:"success_count"`
	Threshold    int       `json:"threshold"`
}

// Coordinator provides unified retry and fix coordination
type Coordinator struct {
	defaultPolicy   *Policy
	policies        map[string]*Policy
	fixProviders    map[string]FixProvider
	errorClassifier *ErrorClassifier
	circuitBreakers map[string]*CircuitBreakerState
	// Performance optimizations
	attemptPool     sync.Pool                // Pool for AttemptResult objects
	delayCache      map[string]time.Duration // Cache for common delay calculations
	delayCacheMutex sync.RWMutex
	errorCache      map[string]string // Cache error classifications
	rng             *rand.Rand
	rngMutex        sync.Mutex
}

// FixProvider interface for implementing fix strategies
type FixProvider interface {
	GetFixStrategies(ctx context.Context, err error, context map[string]interface{}) ([]FixStrategy, error)
	ApplyFix(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error
	Name() string
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func(ctx context.Context) error

// FixableFunc represents a function that can be fixed and retried
type FixableFunc func(ctx context.Context, retryCtx *Context) error

// New creates a new unified retry coordinator
func New() *Coordinator {
	return &Coordinator{
		defaultPolicy: &Policy{
			MaxAttempts:     3,
			InitialDelay:    time.Second,
			MaxDelay:        10 * time.Second,
			BackoffStrategy: BackoffExponential,
			Multiplier:      2.0,
			Jitter:          true,
			ErrorPatterns: []string{
				"timeout", "deadline exceeded", "connection refused",
				"temporary failure", "rate limit", "throttled",
				"service unavailable", "504", "503", "502",
			},
		},
		policies:        make(map[string]*Policy),
		fixProviders:    make(map[string]FixProvider),
		errorClassifier: NewErrorClassifier(),
		circuitBreakers: make(map[string]*CircuitBreakerState),
		// Performance optimizations
		attemptPool: sync.Pool{
			New: func() interface{} {
				return &AttemptResult{}
			},
		},
		delayCache: make(map[string]time.Duration),
		errorCache: make(map[string]string),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetPolicy sets a retry policy for a specific operation
func (rc *Coordinator) SetPolicy(operationType string, policy *Policy) {
	rc.policies[operationType] = policy
}

// RegisterFixProvider registers a fix provider for a specific error type
func (rc *Coordinator) RegisterFixProvider(errorType string, provider FixProvider) {
	rc.fixProviders[errorType] = provider
}

// Execute executes a function with retry coordination
func (rc *Coordinator) Execute(ctx context.Context, operationType string, fn RetryableFunc) error {
	policy := rc.getPolicy(operationType)

	retryCtx := &Context{
		OperationID:    fmt.Sprintf("%s_%d", operationType, time.Now().Unix()),
		Policy:         policy,
		AttemptHistory: make([]AttemptResult, 0),
		Context:        make(map[string]interface{}),
	}

	return rc.executeWithContext(ctx, retryCtx, func(ctx context.Context, _ *Context) error {
		return fn(ctx)
	})
}

// ExecuteWithFix executes a function with both retry and fix coordination
func (rc *Coordinator) ExecuteWithFix(ctx context.Context, operationType string, fn FixableFunc) error {
	policy := rc.getPolicy(operationType)

	retryCtx := &Context{
		OperationID:    fmt.Sprintf("%s_%d", operationType, time.Now().Unix()),
		Policy:         policy,
		AttemptHistory: make([]AttemptResult, 0),
		FixStrategies:  make([]FixStrategy, 0),
		MaxFixAttempts: 5,
		Context:        make(map[string]interface{}),
		CircuitBreaker: rc.getCircuitBreaker(operationType),
	}

	return rc.executeWithContext(ctx, retryCtx, fn)
}

// executeWithContext handles the core retry/fix logic
func (rc *Coordinator) executeWithContext(ctx context.Context, retryCtx *Context, fn FixableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= retryCtx.Policy.MaxAttempts; attempt++ {
		// Check circuit breaker
		if retryCtx.CircuitBreaker != nil && rc.isCircuitOpen(retryCtx.CircuitBreaker) {
			return errors.Network("retry/coordinator", "circuit breaker is open")
		}

		// Apply delay for retry attempts using optimized calculation
		if attempt > 1 {
			delay := rc.getCachedDelay(retryCtx.Policy, attempt-1)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Record attempt start using pooled object
		startTime := time.Now()
		result := rc.getOptimizedAttemptResult()
		result.Attempt = attempt
		result.Timestamp = startTime

		// Execute the function
		err := fn(ctx, retryCtx)
		result.Duration = time.Since(startTime)
		result.Error = err

		if err == nil {
			result.Success = true
			retryCtx.AttemptHistory = append(retryCtx.AttemptHistory, *result)
			rc.putOptimizedAttemptResult(result) // Return to pool
			rc.recordCircuitSuccess(retryCtx.CircuitBreaker)
			return nil
		}

		lastErr = err
		result.Success = false
		retryCtx.AttemptHistory = append(retryCtx.AttemptHistory, *result)
		rc.putOptimizedAttemptResult(result) // Return to pool
		rc.recordCircuitFailure(retryCtx.CircuitBreaker)

		// Check if error is retryable
		if !rc.shouldRetry(err, attempt, retryCtx.Policy) {
			break
		}

		// Attempt to apply fixes before next retry
		if attempt < retryCtx.Policy.MaxAttempts {
			if err := rc.attemptFixes(ctx, retryCtx, err); err != nil {
				// Fix failed, but continue with retry
				continue
			}
		}
	}

	if lastErr != nil {
		return errors.Wrapf(lastErr, "retry/coordinator", "operation failed after %d attempts", retryCtx.Policy.MaxAttempts)
	}

	return errors.Internal("retry/coordinator", "unexpected execution path")
}

// attemptFixes tries to apply available fix strategies
func (rc *Coordinator) attemptFixes(ctx context.Context, retryCtx *Context, err error) error {
	errorType := rc.errorClassifier.ClassifyError(err)

	// Get fix strategies from registered providers
	provider, exists := rc.fixProviders[errorType]
	if !exists {
		return errors.Resourcef("retry/coordinator", "no fix provider for error type: %s", errorType)
	}

	strategies, err := provider.GetFixStrategies(ctx, err, retryCtx.Context)
	if err != nil {
		return errors.Wrap(err, "retry/coordinator", "failed to get fix strategies")
	}

	// Try to apply the highest priority strategy
	for i, strategy := range strategies {
		if strategy.Automated && len(retryCtx.AttemptHistory) <= retryCtx.MaxFixAttempts {
			if err := provider.ApplyFix(ctx, strategy, retryCtx.Context); err == nil {
				// Fix applied successfully
				if len(retryCtx.AttemptHistory) > 0 {
					// Create a copy to avoid memory aliasing
					strategyCopy := strategies[i]
					retryCtx.AttemptHistory[len(retryCtx.AttemptHistory)-1].Strategy = &strategyCopy
					retryCtx.AttemptHistory[len(retryCtx.AttemptHistory)-1].Applied = true
				}
				return nil
			}
		}
	}

	return errors.Internal("retry/coordinator", "no applicable automated fixes found")
}

// getPolicy returns the policy for an operation type
func (rc *Coordinator) getPolicy(operationType string) *Policy {
	if policy, exists := rc.policies[operationType]; exists {
		return policy
	}
	return rc.defaultPolicy
}

// shouldRetry determines if an error should trigger a retry
func (rc *Coordinator) shouldRetry(err error, attempt int, policy *Policy) bool {
	if attempt >= policy.MaxAttempts {
		return false
	}

	// Check if error matches retry patterns
	errStr := strings.ToLower(err.Error())
	for _, pattern := range policy.ErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check for specific error types
	if mcpErr, ok := err.(*errors.MCPError); ok {
		return mcpErr.Retryable
	}

	return false
}

// calculateDelay calculates the delay for a retry attempt
func (rc *Coordinator) calculateDelay(policy *Policy, attempt int) time.Duration {
	var delay time.Duration

	switch policy.BackoffStrategy {
	case BackoffFixed:
		delay = policy.InitialDelay
	case BackoffLinear:
		delay = time.Duration(attempt+1) * policy.InitialDelay
	case BackoffExponential:
		delay = time.Duration(math.Pow(policy.Multiplier, float64(attempt))) * policy.InitialDelay
	default:
		delay = policy.InitialDelay
	}

	// Apply maximum delay limit
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Apply jitter if enabled using optimized random generator
	if policy.Jitter {
		jitter := rc.getOptimizedJitter(delay)
		delay += jitter
	}

	return delay
}

// getCircuitBreaker gets or creates a circuit breaker for an operation
func (rc *Coordinator) getCircuitBreaker(operationType string) *CircuitBreakerState {
	if cb, exists := rc.circuitBreakers[operationType]; exists {
		return cb
	}

	cb := &CircuitBreakerState{
		State:     "closed",
		Threshold: 5,
	}
	rc.circuitBreakers[operationType] = cb
	return cb
}

// IsRetryable checks if an error should be retried using the error classifier
func (rc *Coordinator) IsRetryable(err error) bool {
	return rc.errorClassifier.IsRetryable(err)
}

// ClassifyError categorizes an error using the error classifier
func (rc *Coordinator) ClassifyError(err error) string {
	return rc.errorClassifier.ClassifyError(err)
}

// CalculateDelay calculates delay for a given policy and attempt (exposed for backward compatibility)
func (rc *Coordinator) CalculateDelay(policy *Policy, attempt int) time.Duration {
	return rc.calculateDelay(policy, attempt)
}

// isCircuitOpen checks if the circuit breaker is open
func (rc *Coordinator) isCircuitOpen(cb *CircuitBreakerState) bool {
	if cb.State == "open" {
		if time.Now().After(cb.NextAttempt) {
			cb.State = "half-open"
			cb.SuccessCount = 0
			return false
		}
		return true
	}
	return false
}

// recordCircuitSuccess records a successful operation for circuit breaker
func (rc *Coordinator) recordCircuitSuccess(cb *CircuitBreakerState) {
	if cb == nil {
		return
	}

	if cb.State == "half-open" {
		cb.SuccessCount++
		if cb.SuccessCount >= 2 {
			cb.State = "closed"
			cb.FailureCount = 0
		}
	} else if cb.State == "closed" {
		cb.FailureCount = 0
	}
}

// recordCircuitFailure records a failed operation for circuit breaker
func (rc *Coordinator) recordCircuitFailure(cb *CircuitBreakerState) {
	if cb == nil {
		return
	}

	cb.FailureCount++
	cb.LastFailure = time.Now()

	if cb.State == "closed" && cb.FailureCount >= cb.Threshold {
		cb.State = "open"
		cb.NextAttempt = time.Now().Add(30 * time.Second) // 30 second recovery window
	} else if cb.State == "half-open" {
		cb.State = "open"
		cb.NextAttempt = time.Now().Add(30 * time.Second)
	}
}

// Performance optimization methods

// getOptimizedAttemptResult gets an AttemptResult from the pool
func (rc *Coordinator) getOptimizedAttemptResult() *AttemptResult {
	result := rc.attemptPool.Get().(*AttemptResult)
	// Reset fields to ensure clean state
	*result = AttemptResult{}
	return result
}

// putOptimizedAttemptResult returns an AttemptResult to the pool
func (rc *Coordinator) putOptimizedAttemptResult(result *AttemptResult) {
	rc.attemptPool.Put(result)
}

// getCachedDelay gets delay from cache or calculates and caches it
func (rc *Coordinator) getCachedDelay(policy *Policy, attempt int) time.Duration {
	// Create cache key based on policy parameters and attempt
	cacheKey := fmt.Sprintf("%s_%d_%v_%v_%f_%d",
		policy.BackoffStrategy, attempt, policy.InitialDelay, policy.MaxDelay, policy.Multiplier, policy.MaxAttempts)

	// Check cache first (read lock)
	rc.delayCacheMutex.RLock()
	if cached, exists := rc.delayCache[cacheKey]; exists {
		rc.delayCacheMutex.RUnlock()
		return cached
	}
	rc.delayCacheMutex.RUnlock()

	// Calculate delay
	delay := rc.calculateDelay(policy, attempt)

	// Cache the result (write lock)
	rc.delayCacheMutex.Lock()
	// Prevent cache from growing too large
	if len(rc.delayCache) > 1000 {
		// Clear cache when it gets too large
		rc.delayCache = make(map[string]time.Duration)
	}
	rc.delayCache[cacheKey] = delay
	rc.delayCacheMutex.Unlock()

	return delay
}

// getOptimizedJitter generates jitter using a thread-safe random number generator
func (rc *Coordinator) getOptimizedJitter(delay time.Duration) time.Duration {
	rc.rngMutex.Lock()
	jitter := time.Duration(rc.rng.Float64() * float64(delay) * 0.1) // 10% jitter
	rc.rngMutex.Unlock()
	return jitter
}
