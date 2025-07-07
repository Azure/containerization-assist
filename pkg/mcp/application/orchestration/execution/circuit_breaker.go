package execution

import (
	"context"
	"log/slog"
	"sync"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for external services
type CircuitBreaker struct {
	name             string
	failureThreshold int
	successThreshold int           // Number of successes needed to close from half-open
	timeout          time.Duration // Time to wait before trying half-open

	// State
	state           CircuitState
	failureCount    int
	successCount    int // For half-open state
	lastFailure     time.Time
	lastStateChange time.Time

	mutex  sync.RWMutex
	logger *slog.Logger
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	Name             string
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	Logger           *slog.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:             config.Name,
		failureThreshold: config.FailureThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
		state:            CircuitClosed,
		lastStateChange:  time.Now(),
		logger:           config.Logger,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if we can execute
	if err := cb.canExecute(); err != nil {
		return err
	}

	// Execute the function
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Record the result
	cb.recordResult(err, duration)

	return err
}

// ExecuteWithTimeout runs a function with circuit breaker protection and timeout
func (cb *CircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	// Check if we can execute
	if err := cb.canExecute(); err != nil {
		return err
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute with timeout
	start := time.Now()
	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	var err error
	select {
	case err = <-done:
		// Function completed
	case <-ctx.Done():
		// Timeout or cancellation
		err = ctx.Err()
	}

	duration := time.Since(start)
	cb.recordResult(err, duration)

	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() error {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			// Double-check after acquiring write lock
			if cb.state == CircuitOpen && time.Since(cb.lastFailure) > cb.timeout {
				cb.state = CircuitHalfOpen
				cb.successCount = 0
				cb.lastStateChange = time.Now()
				cb.logger.Info("Circuit breaker transitioning to half-open", "circuit", cb.name)
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()

			if cb.state == CircuitHalfOpen {
				return nil
			}
		}

		return errors.NewError().Messagef("circuit breaker %s is open", cb.name).Build()
	case CircuitHalfOpen:
		return nil
	default:
		return errors.NewError().Messagef("unknown circuit breaker state").WithLocation().Build()
	}
}

func (cb *CircuitBreaker) recordResult(err error, duration time.Duration) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	// Log the execution
	cb.logger.Debug("Circuit breaker execution recorded",
		"circuit", cb.name,
		"state", cb.state.String(),
		"duration", duration,
		"success", err == nil,
		"failure_count", cb.failureCount)
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
			cb.logger.Warn("Circuit breaker opened due to failures",
				"circuit", cb.name,
				"failure_count", cb.failureCount,
				"threshold", cb.failureThreshold)
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.successCount = 0 // Reset success count when transitioning to open
		cb.lastStateChange = time.Now()
		cb.logger.Warn("Circuit breaker opened from half-open due to failure",
			"circuit", cb.name)
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case CircuitClosed:
		// Reset failure count on success
		cb.failureCount = 0
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.lastStateChange = time.Now()
			cb.logger.Info("Circuit breaker closed from half-open",
				"circuit", cb.name,
				"success_count", cb.successCount)
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() *CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return &CircuitBreakerStats{
		Name:             cb.name,
		State:            cb.state.String(),
		FailureCount:     cb.failureCount,
		SuccessCount:     cb.successCount,
		LastFailure:      cb.lastFailure,
		LastStateChange:  cb.lastStateChange,
		FailureThreshold: cb.failureThreshold,
		SuccessThreshold: cb.successThreshold,
		Timeout:          cb.timeout,
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()

	cb.logger.Info("Circuit breaker manually reset", "circuit", cb.name)
}

// CircuitBreakerStats provides statistics about a circuit breaker
type CircuitBreakerStats struct {
	Name             string        `json:"name"`
	State            string        `json:"state"`
	FailureCount     int           `json:"failure_count"`
	SuccessCount     int           `json:"success_count"`
	LastFailure      time.Time     `json:"last_failure"`
	LastStateChange  time.Time     `json:"last_state_change"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
	logger   *slog.Logger
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry(logger *slog.Logger) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
	}
}

// Register adds a circuit breaker to the registry
func (cbr *CircuitBreakerRegistry) Register(name string, breaker *CircuitBreaker) {
	cbr.mutex.Lock()
	defer cbr.mutex.Unlock()

	cbr.breakers[name] = breaker
	cbr.logger.Info("Registered circuit breaker", "circuit", name)
}

// Get retrieves a circuit breaker by name
func (cbr *CircuitBreakerRegistry) Get(name string) (*CircuitBreaker, bool) {
	cbr.mutex.RLock()
	defer cbr.mutex.RUnlock()

	breaker, exists := cbr.breakers[name]
	return breaker, exists
}

// GetStats returns statistics for all circuit breakers
func (cbr *CircuitBreakerRegistry) GetStats() map[string]*CircuitBreakerStats {
	cbr.mutex.RLock()
	defer cbr.mutex.RUnlock()

	stats := make(map[string]*CircuitBreakerStats)
	for name, breaker := range cbr.breakers {
		stats[name] = breaker.GetStats()
	}

	return stats
}

// ResetAll resets all circuit breakers
func (cbr *CircuitBreakerRegistry) ResetAll() {
	cbr.mutex.RLock()
	defer cbr.mutex.RUnlock()

	for name, breaker := range cbr.breakers {
		breaker.Reset()
		cbr.logger.Info("Reset circuit breaker", "circuit", name)
	}
}

// DefaultCircuitBreakers creates commonly used circuit breakers
func CreateDefaultCircuitBreakers(logger *slog.Logger) *CircuitBreakerRegistry {
	registry := NewCircuitBreakerRegistry(logger)

	// Docker circuit breaker
	dockerBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "docker",
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		Logger:           logger.With("component", "circuit_breaker", "service", "docker"),
	})
	registry.Register("docker", dockerBreaker)

	// Kubernetes circuit breaker
	kubernetesBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "kubernetes",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		Logger:           logger.With("component", "circuit_breaker", "service", "kubernetes"),
	})
	registry.Register("kubernetes", kubernetesBreaker)

	// Registry circuit breaker
	registryBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "registry",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          45 * time.Second,
		Logger:           logger.With("component", "circuit_breaker", "service", "registry"),
	})
	registry.Register("registry", registryBreaker)

	// Git circuit breaker
	gitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
		Name:             "git",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		Logger:           logger.With("component", "circuit_breaker", "service", "git"),
	})
	registry.Register("git", gitBreaker)

	logger.Info("Created default circuit breakers")
	return registry
}
