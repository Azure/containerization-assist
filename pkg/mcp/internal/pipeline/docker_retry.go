package pipeline

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// DockerRetryManager provides advanced retry and fallback mechanisms for Docker operations
type DockerRetryManager struct {
	logger zerolog.Logger
	mutex  sync.RWMutex

	// Retry configuration
	retryPolicies  map[string]*RetryPolicy
	fallbackChains map[string]*FallbackChain

	// Operation tracking
	operationHistory map[string][]OperationAttempt
	maxHistorySize   int

	// Circuit breaker
	circuitBreakers map[string]*CircuitBreaker

	// Adaptive retry parameters
	adaptiveSettings map[string]*AdaptiveSettings
	learningEnabled  bool

	// Metrics
	retryMetrics map[string]*RetryMetrics
}

// RetryPolicy defines how operations should be retried
type RetryPolicy struct {
	Name              string          `json:"name"`
	MaxAttempts       int             `json:"max_attempts"`
	BaseDelay         time.Duration   `json:"base_delay"`
	MaxDelay          time.Duration   `json:"max_delay"`
	BackoffStrategy   BackoffStrategy `json:"backoff_strategy"`
	BackoffMultiplier float64         `json:"backoff_multiplier"`
	Jitter            bool            `json:"jitter"`
	JitterRange       float64         `json:"jitter_range"` // 0.0 to 1.0

	// Conditional retries
	RetryableErrors    []string         `json:"retryable_errors"`
	NonRetryableErrors []string         `json:"non_retryable_errors"`
	RetryConditions    []RetryCondition `json:"retry_conditions"`

	// Timeouts
	OperationTimeout time.Duration `json:"operation_timeout"`
	TotalTimeout     time.Duration `json:"total_timeout"`

	// Advanced features
	EnableCircuitBreaker bool                 `json:"enable_circuit_breaker"`
	CircuitBreakerConfig CircuitBreakerConfig `json:"circuit_breaker_config"`
	EnableAdaptive       bool                 `json:"enable_adaptive"`
}

// BackoffStrategy defines different backoff strategies
type BackoffStrategy string

const (
	BackoffConstant    BackoffStrategy = "constant"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
	BackoffCubic       BackoffStrategy = "cubic"
	BackoffAdaptive    BackoffStrategy = "adaptive"
)

// RetryCondition defines conditions for retrying operations
type RetryCondition struct {
	Type        string      `json:"type"`     // "error_pattern", "response_code", "time_condition"
	Pattern     string      `json:"pattern"`  // Error pattern to match
	Operator    string      `json:"operator"` // "contains", "equals", "regex"
	Value       interface{} `json:"value"`    // Value to compare
	Description string      `json:"description"`
}

// FallbackChain defines a series of fallback strategies
type FallbackChain struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Strategies   []FallbackStrategy `json:"strategies"`
	Enabled      bool               `json:"enabled"`
	MaxFallbacks int                `json:"max_fallbacks"`
}

// FallbackStrategy defines a specific fallback approach
type FallbackStrategy struct {
	Type        FallbackType           `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Timeout     time.Duration          `json:"timeout"`
	Priority    int                    `json:"priority"` // Lower number = higher priority
	Conditions  []FallbackCondition    `json:"conditions"`
}

// FallbackType represents different types of fallback strategies
type FallbackType string

const (
	FallbackRegistrySwitch    FallbackType = "registry_switch"
	FallbackImageVariant      FallbackType = "image_variant"
	FallbackLocalBuild        FallbackType = "local_build"
	FallbackCachedImage       FallbackType = "cached_image"
	FallbackAlternativeMethod FallbackType = "alternative_method"
	FallbackDegradedMode      FallbackType = "degraded_mode"
	FallbackOfflineMode       FallbackType = "offline_mode"
)

// FallbackCondition defines when a fallback should be used
type FallbackCondition struct {
	Type        string      `json:"type"`     // "error_type", "failure_count", "time_condition"
	Operator    string      `json:"operator"` // "gt", "eq", "contains"
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

// OperationAttempt tracks individual operation attempts
type OperationAttempt struct {
	ID           string                 `json:"id"`
	Operation    string                 `json:"operation"`
	Attempt      int                    `json:"attempt"`
	Timestamp    time.Time              `json:"timestamp"`
	Duration     time.Duration          `json:"duration"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	ErrorType    string                 `json:"error_type,omitempty"`
	RetryPolicy  string                 `json:"retry_policy"`
	FallbackUsed string                 `json:"fallback_used,omitempty"`
	Parameters   map[string]interface{} `json:"parameters"`
	Context      map[string]string      `json:"context"`
}

// CircuitBreaker implements circuit breaker pattern for Docker operations
type CircuitBreaker struct {
	Name            string               `json:"name"`
	State           CircuitBreakerState  `json:"state"`
	FailureCount    int                  `json:"failure_count"`
	SuccessCount    int                  `json:"success_count"`
	LastFailure     time.Time            `json:"last_failure"`
	LastSuccess     time.Time            `json:"last_success"`
	LastStateChange time.Time            `json:"last_state_change"`
	Config          CircuitBreakerConfig `json:"config"`
	mutex           sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerOpen     CircuitBreakerState = "open"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`   // Failures to open circuit
	SuccessThreshold int           `json:"success_threshold"`   // Successes to close circuit
	Timeout          time.Duration `json:"timeout"`             // How long to keep circuit open
	HalfOpenMaxCalls int           `json:"half_open_max_calls"` // Max calls in half-open state
	MonitoringWindow time.Duration `json:"monitoring_window"`   // Window for failure counting
}

// AdaptiveSettings control adaptive retry behavior
type AdaptiveSettings struct {
	OperationType     string        `json:"operation_type"`
	AverageLatency    time.Duration `json:"average_latency"`
	SuccessRate       float64       `json:"success_rate"`
	OptimalRetryDelay time.Duration `json:"optimal_retry_delay"`
	LearningRate      float64       `json:"learning_rate"`
	LastUpdated       time.Time     `json:"last_updated"`
	SampleCount       int           `json:"sample_count"`
}

// RetryMetrics tracks retry operation metrics
type RetryMetrics struct {
	OperationType       string        `json:"operation_type"`
	TotalAttempts       int           `json:"total_attempts"`
	SuccessfulRetries   int           `json:"successful_retries"`
	FailedRetries       int           `json:"failed_retries"`
	AverageRetries      float64       `json:"average_retries"`
	MaxRetries          int           `json:"max_retries"`
	FallbacksUsed       int           `json:"fallbacks_used"`
	CircuitBreakerTrips int           `json:"circuit_breaker_trips"`
	TotalRetryTime      time.Duration `json:"total_retry_time"`
	AverageRetryTime    time.Duration `json:"average_retry_time"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// RetryManagerConfig configures the retry manager
type RetryManagerConfig struct {
	MaxHistorySize   int  `json:"max_history_size"`
	LearningEnabled  bool `json:"learning_enabled"`
	DefaultPolicies  bool `json:"default_policies"`
	DefaultFallbacks bool `json:"default_fallbacks"`
}

// NewDockerRetryManager creates a new Docker retry manager
func NewDockerRetryManager(config RetryManagerConfig, logger zerolog.Logger) *DockerRetryManager {
	manager := &DockerRetryManager{
		logger:           logger.With().Str("component", "docker_retry").Logger(),
		retryPolicies:    make(map[string]*RetryPolicy),
		fallbackChains:   make(map[string]*FallbackChain),
		operationHistory: make(map[string][]OperationAttempt),
		maxHistorySize:   config.MaxHistorySize,
		circuitBreakers:  make(map[string]*CircuitBreaker),
		adaptiveSettings: make(map[string]*AdaptiveSettings),
		learningEnabled:  config.LearningEnabled,
		retryMetrics:     make(map[string]*RetryMetrics),
	}

	// Set defaults
	if manager.maxHistorySize == 0 {
		manager.maxHistorySize = 1000
	}

	// Initialize default policies and fallbacks if requested
	if config.DefaultPolicies {
		manager.initializeDefaultRetryPolicies()
	}
	if config.DefaultFallbacks {
		manager.initializeDefaultFallbackChains()
	}

	return manager
}

// ExecuteWithRetry executes a Docker operation with retry and fallback logic
func (rm *DockerRetryManager) ExecuteWithRetry(
	ctx context.Context,
	operation string,
	operationFunc func(ctx context.Context, params map[string]interface{}) (interface{}, error),
	params map[string]interface{},
	policyName string,
) (interface{}, error) {

	policy, exists := rm.retryPolicies[policyName]
	if !exists {
		policy = rm.getDefaultPolicy()
	}

	// Check circuit breaker
	if policy.EnableCircuitBreaker {
		if !rm.canExecute(policyName) {
			return nil, fmt.Errorf("circuit breaker is open for operation: %s", operation)
		}
	}

	operationID := rm.generateOperationID()
	startTime := time.Now()

	var lastError error

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		attemptStart := time.Now()

		// Create operation context with timeout
		opCtx := ctx
		if policy.OperationTimeout > 0 {
			var cancel context.CancelFunc
			opCtx, cancel = context.WithTimeout(ctx, policy.OperationTimeout)
			defer cancel()
		}

		// Execute the operation
		opResult, err := operationFunc(opCtx, params)
		duration := time.Since(attemptStart)

		// Record attempt
		attemptRecord := OperationAttempt{
			ID:          fmt.Sprintf("%s_attempt_%d", operationID, attempt),
			Operation:   operation,
			Attempt:     attempt,
			Timestamp:   attemptStart,
			Duration:    duration,
			Success:     err == nil,
			RetryPolicy: policyName,
			Parameters:  params,
			Context:     rm.extractContext(ctx),
		}

		if err != nil {
			attemptRecord.Error = err.Error()
			attemptRecord.ErrorType = rm.categorizeError(err)
			lastError = err
		}

		rm.recordAttempt(operation, attemptRecord)

		// If successful, update metrics and return
		if err == nil {
			rm.updateSuccessMetrics(operation, attempt, time.Since(startTime))
			if policy.EnableCircuitBreaker {
				rm.recordSuccess(policyName)
			}
			if rm.learningEnabled {
				rm.updateAdaptiveSettings(operation, duration, true)
			}
			return opResult, nil
		}

		// If this was the last attempt, check for fallbacks
		if attempt == policy.MaxAttempts {
			if fallbackResult, fallbackErr := rm.tryFallbacks(ctx, operation, params, lastError); fallbackErr == nil {
				attemptRecord.FallbackUsed = "successful"
				rm.recordAttempt(operation, attemptRecord)
				rm.updateFallbackMetrics(operation)
				return fallbackResult, nil
			}
		}

		// Check if error is retryable
		if !rm.isRetryable(err, policy) {
			rm.logger.Debug().
				Err(err).
				Str("operation", operation).
				Int("attempt", attempt).
				Msg("Error is not retryable, stopping attempts")
			break
		}

		// Check total timeout
		if policy.TotalTimeout > 0 && time.Since(startTime) > policy.TotalTimeout {
			rm.logger.Debug().
				Str("operation", operation).
				Dur("elapsed", time.Since(startTime)).
				Dur("total_timeout", policy.TotalTimeout).
				Msg("Total timeout exceeded, stopping attempts")
			break
		}

		// If not the last attempt, wait before retrying
		if attempt < policy.MaxAttempts {
			delay := rm.calculateDelay(policy, attempt, operation)

			rm.logger.Debug().
				Err(err).
				Str("operation", operation).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Operation failed, retrying after delay")

			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// All attempts failed
	rm.updateFailureMetrics(operation, policy.MaxAttempts, time.Since(startTime))
	if policy.EnableCircuitBreaker {
		rm.recordFailure(policyName)
	}
	if rm.learningEnabled {
		rm.updateAdaptiveSettings(operation, time.Since(startTime), false)
	}

	return nil, fmt.Errorf("operation failed after %d attempts: %w", policy.MaxAttempts, lastError)
}

// AddRetryPolicy adds or updates a retry policy
func (rm *DockerRetryManager) AddRetryPolicy(policy *RetryPolicy) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.retryPolicies[policy.Name] = policy

	// Initialize circuit breaker if enabled
	if policy.EnableCircuitBreaker {
		rm.circuitBreakers[policy.Name] = &CircuitBreaker{
			Name:   policy.Name,
			State:  CircuitBreakerClosed,
			Config: policy.CircuitBreakerConfig,
		}
	}

	rm.logger.Info().
		Str("policy_name", policy.Name).
		Int("max_attempts", policy.MaxAttempts).
		Bool("circuit_breaker", policy.EnableCircuitBreaker).
		Msg("Added retry policy")
}

// AddFallbackChain adds or updates a fallback chain
func (rm *DockerRetryManager) AddFallbackChain(chain *FallbackChain) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.fallbackChains[chain.Name] = chain

	rm.logger.Info().
		Str("chain_name", chain.Name).
		Int("strategies", len(chain.Strategies)).
		Bool("enabled", chain.Enabled).
		Msg("Added fallback chain")
}

// GetRetryMetrics returns retry metrics for an operation
func (rm *DockerRetryManager) GetRetryMetrics(operation string) *RetryMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if metrics, exists := rm.retryMetrics[operation]; exists {
		metricsCopy := *metrics
		return &metricsCopy
	}
	return nil
}

// GetAllRetryMetrics returns all retry metrics
func (rm *DockerRetryManager) GetAllRetryMetrics() map[string]*RetryMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	result := make(map[string]*RetryMetrics)
	for operation, metrics := range rm.retryMetrics {
		metricsCopy := *metrics
		result[operation] = &metricsCopy
	}
	return result
}

// GetOperationHistory returns operation history for an operation
func (rm *DockerRetryManager) GetOperationHistory(operation string, limit int) []OperationAttempt {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	history, exists := rm.operationHistory[operation]
	if !exists {
		return []OperationAttempt{}
	}

	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	// Return most recent attempts
	start := len(history) - limit
	result := make([]OperationAttempt, limit)
	copy(result, history[start:])

	return result
}

// GetCircuitBreakerStatus returns circuit breaker status
func (rm *DockerRetryManager) GetCircuitBreakerStatus(operation string) *CircuitBreaker {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if breaker, exists := rm.circuitBreakers[operation]; exists {
		breakerCopy := *breaker
		return &breakerCopy
	}
	return nil
}

// ResetCircuitBreaker manually resets a circuit breaker
func (rm *DockerRetryManager) ResetCircuitBreaker(operation string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	breaker, exists := rm.circuitBreakers[operation]
	if !exists {
		return fmt.Errorf("circuit breaker not found for operation: %s", operation)
	}

	breaker.mutex.Lock()
	breaker.State = CircuitBreakerClosed
	breaker.FailureCount = 0
	breaker.SuccessCount = 0
	breaker.LastStateChange = time.Now()
	breaker.mutex.Unlock()

	rm.logger.Info().
		Str("operation", operation).
		Msg("Circuit breaker manually reset")

	return nil
}

// Private methods

func (rm *DockerRetryManager) getDefaultPolicy() *RetryPolicy {
	return &RetryPolicy{
		Name:              "default",
		MaxAttempts:       3,
		BaseDelay:         time.Second,
		MaxDelay:          30 * time.Second,
		BackoffStrategy:   BackoffExponential,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		JitterRange:       0.1,
		OperationTimeout:  5 * time.Minute,
		TotalTimeout:      15 * time.Minute,
	}
}

func (rm *DockerRetryManager) generateOperationID() string {
	return fmt.Sprintf("op_%d", time.Now().UnixNano())
}

func (rm *DockerRetryManager) extractContext(ctx context.Context) map[string]string {
	// Extract relevant context information
	contextMap := make(map[string]string)

	// Add request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		contextMap["request_id"] = fmt.Sprintf("%v", requestID)
	}

	// Add session ID if available
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		contextMap["session_id"] = fmt.Sprintf("%v", sessionID)
	}

	return contextMap
}

func (rm *DockerRetryManager) categorizeError(err error) string {
	errorMsg := err.Error()

	switch {
	case strings.Contains(errorMsg, "timeout"):
		return "timeout"
	case strings.Contains(errorMsg, "network"):
		return "network"
	case strings.Contains(errorMsg, "permission"):
		return "permission"
	case strings.Contains(errorMsg, "not found"):
		return "not_found"
	case strings.Contains(errorMsg, "unauthorized"):
		return "auth"
	case strings.Contains(errorMsg, "server error"):
		return "server_error"
	case strings.Contains(errorMsg, "rate limit"):
		return "rate_limit"
	default:
		return "unknown"
	}
}

func (rm *DockerRetryManager) recordAttempt(operation string, attempt OperationAttempt) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Add to operation history
	if rm.operationHistory[operation] == nil {
		rm.operationHistory[operation] = make([]OperationAttempt, 0)
	}

	rm.operationHistory[operation] = append(rm.operationHistory[operation], attempt)

	// Limit history size
	if len(rm.operationHistory[operation]) > rm.maxHistorySize {
		rm.operationHistory[operation] = rm.operationHistory[operation][1:]
	}
}

func (rm *DockerRetryManager) isRetryable(err error, policy *RetryPolicy) bool {
	errorMsg := err.Error()

	// Check non-retryable errors first
	for _, nonRetryablePattern := range policy.NonRetryableErrors {
		if strings.Contains(errorMsg, nonRetryablePattern) {
			return false
		}
	}

	// Check retryable errors
	if len(policy.RetryableErrors) > 0 {
		for _, retryablePattern := range policy.RetryableErrors {
			if strings.Contains(errorMsg, retryablePattern) {
				return true
			}
		}
		return false // If retryable list exists but doesn't match, not retryable
	}

	// Check retry conditions
	for _, condition := range policy.RetryConditions {
		if rm.evaluateRetryCondition(condition, err) {
			return true
		}
	}

	// Default: retry most errors except explicit non-retryable ones
	return !rm.isKnownNonRetryableError(err)
}

func (rm *DockerRetryManager) evaluateRetryCondition(condition RetryCondition, err error) bool {
	errorMsg := err.Error()

	switch condition.Type {
	case "error_pattern":
		switch condition.Operator {
		case "contains":
			return strings.Contains(errorMsg, condition.Pattern)
		case "equals":
			return errorMsg == condition.Pattern
		}
	}

	return false
}

func (rm *DockerRetryManager) isKnownNonRetryableError(err error) bool {
	errorMsg := err.Error()

	nonRetryablePatterns := []string{
		"invalid argument",
		"bad request",
		"unauthorized",
		"forbidden",
		"not found",
		"conflict",
		"gone",
		"unsupported",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (rm *DockerRetryManager) calculateDelay(policy *RetryPolicy, attempt int, operation string) time.Duration {
	var delay time.Duration

	switch policy.BackoffStrategy {
	case BackoffConstant:
		delay = policy.BaseDelay
	case BackoffLinear:
		delay = time.Duration(attempt) * policy.BaseDelay
	case BackoffExponential:
		delay = time.Duration(float64(policy.BaseDelay) * math.Pow(policy.BackoffMultiplier, float64(attempt-1)))
	case BackoffCubic:
		delay = time.Duration(float64(policy.BaseDelay) * math.Pow(float64(attempt), 3))
	case BackoffAdaptive:
		delay = rm.getAdaptiveDelay(operation, attempt)
	default:
		delay = policy.BaseDelay
	}

	// Apply maximum delay limit
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Apply jitter if enabled
	if policy.Jitter {
		jitterAmount := float64(delay) * policy.JitterRange
		jitter := time.Duration(jitterAmount * (2.0*math.Sin(float64(time.Now().UnixNano())) - 1.0))
		delay += jitter

		// Ensure delay is not negative
		if delay < 0 {
			delay = policy.BaseDelay
		}
	}

	return delay
}

func (rm *DockerRetryManager) getAdaptiveDelay(operation string, attempt int) time.Duration {
	rm.mutex.RLock()
	settings, exists := rm.adaptiveSettings[operation]
	rm.mutex.RUnlock()

	if !exists || settings.OptimalRetryDelay == 0 {
		return time.Second * time.Duration(attempt) // Default fallback
	}

	// Use learned optimal delay with attempt multiplier
	return settings.OptimalRetryDelay * time.Duration(attempt)
}

func (rm *DockerRetryManager) tryFallbacks(ctx context.Context, operation string, params map[string]interface{}, originalError error) (interface{}, error) {
	rm.mutex.RLock()
	fallbackChain, exists := rm.fallbackChains[operation]
	if !exists {
		// Try default fallback chain
		fallbackChain, exists = rm.fallbackChains["default"]
	}
	rm.mutex.RUnlock()

	if !exists || !fallbackChain.Enabled {
		return nil, fmt.Errorf("no fallback available")
	}

	// Sort strategies by priority
	strategies := make([]FallbackStrategy, len(fallbackChain.Strategies))
	copy(strategies, fallbackChain.Strategies)
	for i := 0; i < len(strategies)-1; i++ {
		for j := i + 1; j < len(strategies); j++ {
			if strategies[j].Priority < strategies[i].Priority {
				strategies[i], strategies[j] = strategies[j], strategies[i]
			}
		}
	}

	for _, strategy := range strategies {
		if !rm.shouldUseFallback(strategy, originalError) {
			continue
		}

		rm.logger.Info().
			Str("operation", operation).
			Str("fallback_type", string(strategy.Type)).
			Str("fallback_name", strategy.Name).
			Msg("Attempting fallback strategy")

		result, err := rm.executeFallbackStrategy(ctx, strategy, params, originalError)
		if err == nil {
			rm.logger.Info().
				Str("operation", operation).
				Str("fallback_type", string(strategy.Type)).
				Msg("Fallback strategy succeeded")
			return result, nil
		}

		rm.logger.Debug().
			Err(err).
			Str("operation", operation).
			Str("fallback_type", string(strategy.Type)).
			Msg("Fallback strategy failed")
	}

	return nil, fmt.Errorf("all fallback strategies failed")
}

func (rm *DockerRetryManager) shouldUseFallback(strategy FallbackStrategy, originalError error) bool {
	for _, condition := range strategy.Conditions {
		if !rm.evaluateFallbackCondition(condition, originalError) {
			return false
		}
	}
	return len(strategy.Conditions) == 0 || len(strategy.Conditions) > 0 // At least one condition exists or no conditions
}

func (rm *DockerRetryManager) evaluateFallbackCondition(condition FallbackCondition, originalError error) bool {
	switch condition.Type {
	case "error_type":
		errorType := rm.categorizeError(originalError)
		return errorType == condition.Value
	case "failure_count":
		// Would need to track failure counts
		return true // Simplified for now
	}
	return true
}

func (rm *DockerRetryManager) executeFallbackStrategy(ctx context.Context, strategy FallbackStrategy, params map[string]interface{}, originalError error) (interface{}, error) {
	switch strategy.Type {
	case FallbackRegistrySwitch:
		return rm.executeRegistrySwitch(ctx, strategy, params)
	case FallbackImageVariant:
		return rm.executeImageVariant(ctx, strategy, params)
	case FallbackCachedImage:
		return rm.executeCachedImage(ctx, strategy, params)
	case FallbackDegradedMode:
		return rm.executeDegradedMode(ctx, strategy, params)
	default:
		return nil, fmt.Errorf("unsupported fallback strategy: %s", strategy.Type)
	}
}

func (rm *DockerRetryManager) executeRegistrySwitch(ctx context.Context, strategy FallbackStrategy, params map[string]interface{}) (interface{}, error) {
	// Switch to alternative Docker registry
	alternativeRegistry, ok := strategy.Parameters["alternative_registry"].(string)
	if !ok {
		return nil, fmt.Errorf("alternative_registry parameter required for registry switch")
	}

	// Modify image reference to use alternative registry
	originalImage, ok := params["image"].(string)
	if !ok {
		return nil, fmt.Errorf("image parameter required")
	}

	// Simple registry substitution (in real implementation, would be more sophisticated)
	newImage := alternativeRegistry + "/" + originalImage

	rm.logger.Info().
		Str("original_image", originalImage).
		Str("fallback_image", newImage).
		Msg("Switching to alternative registry")

	return map[string]interface{}{
		"fallback_type":  "registry_switch",
		"new_image":      newImage,
		"original_image": originalImage,
	}, nil
}

func (rm *DockerRetryManager) executeImageVariant(ctx context.Context, strategy FallbackStrategy, params map[string]interface{}) (interface{}, error) {
	// Use alternative image variant (e.g., alpine instead of ubuntu)
	variantMapping, ok := strategy.Parameters["variant_mapping"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("variant_mapping parameter required for image variant fallback")
	}

	originalImage, ok := params["image"].(string)
	if !ok {
		return nil, fmt.Errorf("image parameter required")
	}

	// Find variant mapping
	for baseImage, variant := range variantMapping {
		if strings.Contains(originalImage, baseImage) {
			newImage := strings.Replace(originalImage, baseImage, variant, 1)

			rm.logger.Info().
				Str("original_image", originalImage).
				Str("variant_image", newImage).
				Msg("Using image variant fallback")

			return map[string]interface{}{
				"fallback_type":  "image_variant",
				"new_image":      newImage,
				"original_image": originalImage,
			}, nil
		}
	}

	return nil, fmt.Errorf("no variant mapping found for image: %s", originalImage)
}

func (rm *DockerRetryManager) executeCachedImage(ctx context.Context, strategy FallbackStrategy, params map[string]interface{}) (interface{}, error) {
	// Use locally cached image if available
	originalImage, ok := params["image"].(string)
	if !ok {
		return nil, fmt.Errorf("image parameter required")
	}

	// In real implementation, would check Docker daemon for cached images
	cachedImages := []string{"nginx:latest", "alpine:latest", "ubuntu:latest"} // Simplified

	for _, cachedImage := range cachedImages {
		if strings.Contains(originalImage, strings.Split(cachedImage, ":")[0]) {
			rm.logger.Info().
				Str("original_image", originalImage).
				Str("cached_image", cachedImage).
				Msg("Using cached image fallback")

			return map[string]interface{}{
				"fallback_type":  "cached_image",
				"cached_image":   cachedImage,
				"original_image": originalImage,
			}, nil
		}
	}

	return nil, fmt.Errorf("no cached image available for: %s", originalImage)
}

func (rm *DockerRetryManager) executeDegradedMode(ctx context.Context, strategy FallbackStrategy, params map[string]interface{}) (interface{}, error) {
	// Return minimal success response for degraded mode
	rm.logger.Info().Msg("Operating in degraded mode")

	return map[string]interface{}{
		"fallback_type": "degraded_mode",
		"status":        "degraded",
		"message":       "Operation completed in degraded mode",
	}, nil
}

// Circuit breaker methods

func (rm *DockerRetryManager) canExecute(operation string) bool {
	rm.mutex.RLock()
	breaker, exists := rm.circuitBreakers[operation]
	rm.mutex.RUnlock()

	if !exists {
		return true
	}

	breaker.mutex.RLock()
	defer breaker.mutex.RUnlock()

	switch breaker.State {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if timeout period has passed
		if time.Since(breaker.LastStateChange) > breaker.Config.Timeout {
			breaker.mutex.RUnlock()
			rm.transitionToHalfOpen(operation)
			breaker.mutex.RLock()
			return breaker.State == CircuitBreakerHalfOpen
		}
		return false
	case CircuitBreakerHalfOpen:
		// Allow limited calls in half-open state
		return true // Simplified - should track call count
	default:
		return false
	}
}

func (rm *DockerRetryManager) recordSuccess(operation string) {
	rm.mutex.RLock()
	breaker, exists := rm.circuitBreakers[operation]
	rm.mutex.RUnlock()

	if !exists {
		return
	}

	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()

	breaker.SuccessCount++
	breaker.LastSuccess = time.Now()

	if breaker.State == CircuitBreakerHalfOpen && breaker.SuccessCount >= breaker.Config.SuccessThreshold {
		breaker.State = CircuitBreakerClosed
		breaker.FailureCount = 0
		breaker.LastStateChange = time.Now()

		rm.logger.Info().
			Str("operation", operation).
			Msg("Circuit breaker transitioned to closed")
	}
}

func (rm *DockerRetryManager) recordFailure(operation string) {
	rm.mutex.RLock()
	breaker, exists := rm.circuitBreakers[operation]
	rm.mutex.RUnlock()

	if !exists {
		return
	}

	breaker.mutex.Lock()
	defer breaker.mutex.Unlock()

	breaker.FailureCount++
	breaker.LastFailure = time.Now()

	if breaker.State == CircuitBreakerClosed && breaker.FailureCount >= breaker.Config.FailureThreshold {
		breaker.State = CircuitBreakerOpen
		breaker.LastStateChange = time.Now()

		rm.logger.Warn().
			Str("operation", operation).
			Int("failure_count", breaker.FailureCount).
			Msg("Circuit breaker opened due to failures")
	}
}

func (rm *DockerRetryManager) transitionToHalfOpen(operation string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	breaker, exists := rm.circuitBreakers[operation]
	if !exists {
		return
	}

	breaker.mutex.Lock()
	breaker.State = CircuitBreakerHalfOpen
	breaker.SuccessCount = 0
	breaker.LastStateChange = time.Now()
	breaker.mutex.Unlock()

	rm.logger.Info().
		Str("operation", operation).
		Msg("Circuit breaker transitioned to half-open")
}

// Metrics methods

func (rm *DockerRetryManager) updateSuccessMetrics(operation string, attempts int, totalTime time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	metrics, exists := rm.retryMetrics[operation]
	if !exists {
		metrics = &RetryMetrics{
			OperationType: operation,
		}
		rm.retryMetrics[operation] = metrics
	}

	metrics.TotalAttempts += attempts
	if attempts > 1 {
		metrics.SuccessfulRetries++
	}

	if attempts > metrics.MaxRetries {
		metrics.MaxRetries = attempts
	}

	metrics.TotalRetryTime += totalTime
	metrics.AverageRetries = float64(metrics.TotalAttempts) / float64(metrics.SuccessfulRetries+metrics.FailedRetries+1)
	metrics.AverageRetryTime = metrics.TotalRetryTime / time.Duration(metrics.SuccessfulRetries+metrics.FailedRetries+1)
	metrics.LastUpdated = time.Now()
}

func (rm *DockerRetryManager) updateFailureMetrics(operation string, attempts int, totalTime time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	metrics, exists := rm.retryMetrics[operation]
	if !exists {
		metrics = &RetryMetrics{
			OperationType: operation,
		}
		rm.retryMetrics[operation] = metrics
	}

	metrics.TotalAttempts += attempts
	metrics.FailedRetries++

	if attempts > metrics.MaxRetries {
		metrics.MaxRetries = attempts
	}

	metrics.TotalRetryTime += totalTime
	metrics.AverageRetries = float64(metrics.TotalAttempts) / float64(metrics.SuccessfulRetries+metrics.FailedRetries)
	if metrics.SuccessfulRetries+metrics.FailedRetries > 0 {
		metrics.AverageRetryTime = metrics.TotalRetryTime / time.Duration(metrics.SuccessfulRetries+metrics.FailedRetries)
	}
	metrics.LastUpdated = time.Now()
}

func (rm *DockerRetryManager) updateFallbackMetrics(operation string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	metrics, exists := rm.retryMetrics[operation]
	if !exists {
		metrics = &RetryMetrics{
			OperationType: operation,
		}
		rm.retryMetrics[operation] = metrics
	}

	metrics.FallbacksUsed++
	metrics.LastUpdated = time.Now()
}

// Adaptive learning methods

func (rm *DockerRetryManager) updateAdaptiveSettings(operation string, duration time.Duration, success bool) {
	if !rm.learningEnabled {
		return
	}

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	settings, exists := rm.adaptiveSettings[operation]
	if !exists {
		settings = &AdaptiveSettings{
			OperationType: operation,
			LearningRate:  0.1,
			SampleCount:   0,
		}
		rm.adaptiveSettings[operation] = settings
	}

	settings.SampleCount++

	// Update average latency using exponential moving average
	if settings.AverageLatency == 0 {
		settings.AverageLatency = duration
	} else {
		alpha := settings.LearningRate
		settings.AverageLatency = time.Duration(float64(settings.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}

	// Update success rate
	if success {
		if settings.SuccessRate == 0 {
			settings.SuccessRate = 1.0
		} else {
			alpha := settings.LearningRate
			settings.SuccessRate = settings.SuccessRate*(1-alpha) + 1.0*alpha
		}
	} else {
		if settings.SuccessRate == 0 {
			settings.SuccessRate = 0.0
		} else {
			alpha := settings.LearningRate
			settings.SuccessRate = settings.SuccessRate * (1 - alpha)
		}
	}

	// Calculate optimal retry delay based on observed latency and success rate
	// Higher latency or lower success rate -> longer retry delay
	latencyFactor := float64(settings.AverageLatency) / float64(time.Second)
	successFactor := 2.0 - settings.SuccessRate // Ranges from 1.0 (perfect) to 2.0 (no success)
	settings.OptimalRetryDelay = time.Duration(latencyFactor * successFactor * float64(time.Second))

	settings.LastUpdated = time.Now()
}

// Initialize default configurations

func (rm *DockerRetryManager) initializeDefaultRetryPolicies() {
	// Standard retry policy
	rm.AddRetryPolicy(&RetryPolicy{
		Name:               "standard",
		MaxAttempts:        3,
		BaseDelay:          time.Second,
		MaxDelay:           30 * time.Second,
		BackoffStrategy:    BackoffExponential,
		BackoffMultiplier:  2.0,
		Jitter:             true,
		JitterRange:        0.1,
		RetryableErrors:    []string{"timeout", "network", "server error", "rate limit"},
		NonRetryableErrors: []string{"unauthorized", "forbidden", "not found", "bad request"},
		OperationTimeout:   5 * time.Minute,
		TotalTimeout:       15 * time.Minute,
	})

	// Aggressive retry policy for critical operations
	rm.AddRetryPolicy(&RetryPolicy{
		Name:                 "aggressive",
		MaxAttempts:          5,
		BaseDelay:            500 * time.Millisecond,
		MaxDelay:             60 * time.Second,
		BackoffStrategy:      BackoffExponential,
		BackoffMultiplier:    1.5,
		Jitter:               true,
		JitterRange:          0.2,
		RetryableErrors:      []string{"timeout", "network", "server error", "rate limit", "connection"},
		OperationTimeout:     10 * time.Minute,
		TotalTimeout:         30 * time.Minute,
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          5 * time.Minute,
			HalfOpenMaxCalls: 3,
			MonitoringWindow: 10 * time.Minute,
		},
		EnableAdaptive: true,
	})

	// Conservative retry policy
	rm.AddRetryPolicy(&RetryPolicy{
		Name:              "conservative",
		MaxAttempts:       2,
		BaseDelay:         2 * time.Second,
		MaxDelay:          10 * time.Second,
		BackoffStrategy:   BackoffLinear,
		BackoffMultiplier: 1.0,
		Jitter:            false,
		RetryableErrors:   []string{"timeout", "rate limit"},
		OperationTimeout:  2 * time.Minute,
		TotalTimeout:      5 * time.Minute,
	})
}

func (rm *DockerRetryManager) initializeDefaultFallbackChains() {
	// Docker pull fallback chain
	rm.AddFallbackChain(&FallbackChain{
		Name:         "docker_pull",
		Description:  "Fallback strategies for Docker pull operations",
		Enabled:      true,
		MaxFallbacks: 3,
		Strategies: []FallbackStrategy{
			{
				Type:        FallbackRegistrySwitch,
				Name:        "alternative_registry",
				Description: "Switch to alternative Docker registry",
				Priority:    1,
				Parameters: map[string]interface{}{
					"alternative_registry": "docker.io",
				},
				Conditions: []FallbackCondition{
					{
						Type:     "error_type",
						Operator: "eq",
						Value:    "network",
					},
				},
			},
			{
				Type:        FallbackImageVariant,
				Name:        "image_variant",
				Description: "Use alternative image variant",
				Priority:    2,
				Parameters: map[string]interface{}{
					"variant_mapping": map[string]string{
						"ubuntu": "alpine",
						"centos": "alpine",
					},
				},
			},
			{
				Type:        FallbackCachedImage,
				Name:        "cached_image",
				Description: "Use locally cached image",
				Priority:    3,
			},
		},
	})

	// General fallback chain
	rm.AddFallbackChain(&FallbackChain{
		Name:         "default",
		Description:  "Default fallback strategies for all operations",
		Enabled:      true,
		MaxFallbacks: 2,
		Strategies: []FallbackStrategy{
			{
				Type:        FallbackDegradedMode,
				Name:        "degraded_mode",
				Description: "Continue in degraded mode",
				Priority:    10, // Low priority, last resort
			},
		},
	})
}
