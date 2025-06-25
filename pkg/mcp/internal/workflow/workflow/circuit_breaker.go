package workflow

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"    // Normal operation
	CircuitBreakerOpen     CircuitBreakerState = "open"      // Failing, rejecting requests
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open" // Testing if service recovered
)

// CircuitBreakerConfig defines circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening
	SuccessThreshold int           // Number of successes to close from half-open
	Timeout          time.Duration // Time to wait before half-open
	WindowSize       time.Duration // Rolling window for failure counting
	MaxRetries       int           // Maximum retries per stage
}

// CircuitBreaker implements circuit breaker pattern for stage execution
type CircuitBreaker struct {
	logger          zerolog.Logger
	config          CircuitBreakerConfig
	state           CircuitBreakerState
	failures        int
	successes       int
	lastFailureTime time.Time
	window          *RollingWindow
	mu              sync.RWMutex
}

// RollingWindow tracks failures in a time window
type RollingWindow struct {
	events     []time.Time
	windowSize time.Duration
	mu         sync.Mutex
}

// NewRollingWindow creates a new rolling window
func NewRollingWindow(windowSize time.Duration) *RollingWindow {
	return &RollingWindow{
		events:     make([]time.Time, 0),
		windowSize: windowSize,
	}
}

// AddEvent adds an event to the rolling window
func (rw *RollingWindow) AddEvent() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now()
	rw.events = append(rw.events, now)

	// Remove old events outside the window
	cutoff := now.Add(-rw.windowSize)
	var filtered []time.Time
	for _, event := range rw.events {
		if event.After(cutoff) {
			filtered = append(filtered, event)
		}
	}
	rw.events = filtered
}

// Count returns the number of events in the current window
func (rw *RollingWindow) Count() int {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rw.windowSize)

	count := 0
	for _, event := range rw.events {
		if event.After(cutoff) {
			count++
		}
	}

	return count
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(logger zerolog.Logger, config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	if config.WindowSize <= 0 {
		config.WindowSize = 10 * time.Minute
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	return &CircuitBreaker{
		logger: logger.With().Str("component", "circuit_breaker").Logger(),
		config: config,
		state:  CircuitBreakerClosed,
		window: NewRollingWindow(config.WindowSize),
	}
}

// CanExecute checks if execution should be allowed
func (cb *CircuitBreaker) CanExecute(stageName string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.logger.Info().
				Str("stage_name", stageName).
				Msg("Circuit breaker transitioning to half-open")
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful execution
func (cb *CircuitBreaker) RecordSuccess(stageName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		// Reset failure count on success
		cb.failures = 0
	case CircuitBreakerHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.logger.Info().
				Str("stage_name", stageName).
				Int("successes", cb.successes).
				Msg("Circuit breaker closing after successful recoveries")
			cb.state = CircuitBreakerClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed execution
func (cb *CircuitBreaker) RecordFailure(stageName string, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.window.AddEvent()
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.window.Count() >= cb.config.FailureThreshold {
			cb.logger.Warn().
				Str("stage_name", stageName).
				Int("failures", cb.failures).
				Int("window_failures", cb.window.Count()).
				Err(err).
				Msg("Circuit breaker opening due to failures")
			cb.state = CircuitBreakerOpen
		}
	case CircuitBreakerHalfOpen:
		cb.logger.Warn().
			Str("stage_name", stageName).
			Err(err).
			Msg("Circuit breaker reopening after failure in half-open state")
		cb.state = CircuitBreakerOpen
		cb.successes = 0
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		WindowFailures:  cb.window.Count(),
		LastFailureTime: cb.lastFailureTime,
		TimeUntilRetry:  cb.getTimeUntilRetry(),
	}
}

// getTimeUntilRetry calculates time until next retry attempt
func (cb *CircuitBreaker) getTimeUntilRetry() time.Duration {
	if cb.state != CircuitBreakerOpen {
		return 0
	}

	elapsed := time.Since(cb.lastFailureTime)
	if elapsed >= cb.config.Timeout {
		return 0
	}

	return cb.config.Timeout - elapsed
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitBreakerClosed
	cb.failures = 0
	cb.successes = 0
	cb.window = NewRollingWindow(cb.config.WindowSize)

	cb.logger.Info().Msg("Circuit breaker manually reset")
}

// CircuitBreakerStats contains circuit breaker statistics
type CircuitBreakerStats struct {
	State           CircuitBreakerState `json:"state"`
	Failures        int                 `json:"failures"`
	Successes       int                 `json:"successes"`
	WindowFailures  int                 `json:"window_failures"`
	LastFailureTime time.Time           `json:"last_failure_time"`
	TimeUntilRetry  time.Duration       `json:"time_until_retry"`
}

// StageCircuitBreakerManager manages circuit breakers for different stage types
type StageCircuitBreakerManager struct {
	logger   zerolog.Logger
	breakers map[string]*CircuitBreaker
	configs  map[string]CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewStageCircuitBreakerManager creates a new stage circuit breaker manager
func NewStageCircuitBreakerManager(logger zerolog.Logger) *StageCircuitBreakerManager {
	manager := &StageCircuitBreakerManager{
		logger:   logger.With().Str("component", "stage_circuit_breaker_manager").Logger(),
		breakers: make(map[string]*CircuitBreaker),
		configs:  make(map[string]CircuitBreakerConfig),
	}

	// Set default configs for different stage types
	manager.setDefaultConfigs()

	return manager
}

// setDefaultConfigs sets default circuit breaker configs for different stage types
func (scbm *StageCircuitBreakerManager) setDefaultConfigs() {
	scbm.configs["build"] = CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          5 * time.Minute,
		WindowSize:       15 * time.Minute,
		MaxRetries:       3,
	}

	scbm.configs["test"] = CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          2 * time.Minute,
		WindowSize:       10 * time.Minute,
		MaxRetries:       2,
	}

	scbm.configs["deploy"] = CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          10 * time.Minute,
		WindowSize:       30 * time.Minute,
		MaxRetries:       5,
	}

	scbm.configs["analysis"] = CircuitBreakerConfig{
		FailureThreshold: 4,
		SuccessThreshold: 2,
		Timeout:          3 * time.Minute,
		WindowSize:       15 * time.Minute,
		MaxRetries:       3,
	}

	// Default config for unknown stage types
	scbm.configs["default"] = CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          5 * time.Minute,
		WindowSize:       15 * time.Minute,
		MaxRetries:       3,
	}
}

// GetCircuitBreaker returns the circuit breaker for a stage type
func (scbm *StageCircuitBreakerManager) GetCircuitBreaker(stageType string) *CircuitBreaker {
	scbm.mu.RLock()
	if breaker, exists := scbm.breakers[stageType]; exists {
		scbm.mu.RUnlock()
		return breaker
	}
	scbm.mu.RUnlock()

	// Create new circuit breaker
	scbm.mu.Lock()
	defer scbm.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := scbm.breakers[stageType]; exists {
		return breaker
	}

	config, exists := scbm.configs[stageType]
	if !exists {
		config = scbm.configs["default"]
	}

	breaker := NewCircuitBreaker(scbm.logger, config)
	scbm.breakers[stageType] = breaker

	scbm.logger.Info().
		Str("stage_type", stageType).
		Msg("Created new circuit breaker for stage type")

	return breaker
}

// SetConfig sets a custom circuit breaker config for a stage type
func (scbm *StageCircuitBreakerManager) SetConfig(stageType string, config CircuitBreakerConfig) {
	scbm.mu.Lock()
	defer scbm.mu.Unlock()

	scbm.configs[stageType] = config

	// Reset existing breaker to use new config
	if breaker, exists := scbm.breakers[stageType]; exists {
		breaker.config = config
		breaker.Reset()
	}
}

// GetAllStats returns statistics for all circuit breakers
func (scbm *StageCircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	scbm.mu.RLock()
	defer scbm.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for stageType, breaker := range scbm.breakers {
		stats[stageType] = breaker.GetStats()
	}

	return stats
}

// ResetAll resets all circuit breakers
func (scbm *StageCircuitBreakerManager) ResetAll() {
	scbm.mu.RLock()
	defer scbm.mu.RUnlock()

	for _, breaker := range scbm.breakers {
		breaker.Reset()
	}

	scbm.logger.Info().Msg("Reset all circuit breakers")
}
