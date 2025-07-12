package progress

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ErrorBudget manages error thresholds and recovery strategies
type ErrorBudget struct {
	maxErrors     int
	currentErrors int
	resetDuration time.Duration
	lastReset     time.Time
	circuitOpen   bool
	mu            sync.RWMutex

	// Recovery strategies
	backoffMultiplier float64
	maxBackoffDelay   time.Duration
	retryCount        int
}

// NewErrorBudget creates a new error budget manager
func NewErrorBudget(maxErrors int, resetDuration time.Duration) *ErrorBudget {
	return &ErrorBudget{
		maxErrors:         maxErrors,
		currentErrors:     0,
		resetDuration:     resetDuration,
		lastReset:         time.Now(),
		circuitOpen:       false,
		backoffMultiplier: 2.0,
		maxBackoffDelay:   30 * time.Second,
		retryCount:        0,
	}
}

// RecordError records a new error and checks if budget is exceeded
func (eb *ErrorBudget) RecordError(err error) bool {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Check if we should reset the error count
	if time.Since(eb.lastReset) >= eb.resetDuration {
		eb.currentErrors = 0
		eb.lastReset = time.Now()
		eb.circuitOpen = false
		eb.retryCount = 0
	}

	eb.currentErrors++

	// Check if we've exceeded the error budget
	if eb.currentErrors >= eb.maxErrors {
		eb.circuitOpen = true
		return false // Budget exceeded
	}

	return true // Within budget
}

// RecordSuccess records a successful operation
func (eb *ErrorBudget) RecordSuccess() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Reset retry count on success
	eb.retryCount = 0

	// Gradually reduce error count on success (forgiveness)
	if eb.currentErrors > 0 {
		eb.currentErrors--
	}

	// Close circuit if errors are back to acceptable level
	if eb.currentErrors < eb.maxErrors/2 {
		eb.circuitOpen = false
	}
}

// IsCircuitOpen returns whether the circuit breaker is open
func (eb *ErrorBudget) IsCircuitOpen() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.circuitOpen
}

// GetCurrentErrors returns the current error count
func (eb *ErrorBudget) GetCurrentErrors() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.currentErrors
}

// GetBackoffDelay calculates the next backoff delay
func (eb *ErrorBudget) GetBackoffDelay() time.Duration {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.retryCount++
	baseDelay := time.Duration(float64(time.Second) * eb.backoffMultiplier)

	// Calculate exponential backoff
	delay := time.Duration(1<<(eb.retryCount-1)) * baseDelay

	// Cap at maximum delay
	if delay > eb.maxBackoffDelay {
		delay = eb.maxBackoffDelay
	}

	return delay
}

// CanRetry determines if an operation can be retried
func (eb *ErrorBudget) CanRetry() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Don't retry if circuit is open
	if eb.circuitOpen {
		return false
	}

	// Allow retries up to a reasonable limit
	return eb.retryCount < 5
}

// Reset manually resets the error budget
func (eb *ErrorBudget) Reset() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.currentErrors = 0
	eb.lastReset = time.Now()
	eb.circuitOpen = false
	eb.retryCount = 0
}

// GetStatus returns the current error budget status
func (eb *ErrorBudget) GetStatus() ErrorBudgetStatus {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	healthPercentage := float64(eb.maxErrors-eb.currentErrors) / float64(eb.maxErrors) * 100
	if healthPercentage < 0 {
		healthPercentage = 0
	}

	return ErrorBudgetStatus{
		MaxErrors:        eb.maxErrors,
		CurrentErrors:    eb.currentErrors,
		CircuitOpen:      eb.circuitOpen,
		HealthPercentage: healthPercentage,
		RetryCount:       eb.retryCount,
		TimeToReset:      eb.resetDuration - time.Since(eb.lastReset),
	}
}

// ErrorBudgetStatus represents the current status of an error budget
type ErrorBudgetStatus struct {
	MaxErrors        int           `json:"max_errors"`
	CurrentErrors    int           `json:"current_errors"`
	CircuitOpen      bool          `json:"circuit_open"`
	HealthPercentage float64       `json:"health_percentage"`
	RetryCount       int           `json:"retry_count"`
	TimeToReset      time.Duration `json:"time_to_reset"`
}

// String returns a human-readable status string
func (ebs ErrorBudgetStatus) String() string {
	status := "healthy"
	if ebs.CircuitOpen {
		status = "circuit_open"
	} else if ebs.HealthPercentage < 50 {
		status = "degraded"
	}

	return fmt.Sprintf("ErrorBudget[%s]: %d/%d errors (%.1f%% healthy), retries: %d, reset in: %v",
		status, ebs.CurrentErrors, ebs.MaxErrors, ebs.HealthPercentage,
		ebs.RetryCount, ebs.TimeToReset.Truncate(time.Second))
}

// RetryWithErrorBudget executes a function with error budget and retry logic
func RetryWithErrorBudget(ctx context.Context, budget *ErrorBudget, operation func() error) error {
	for {
		// Check if circuit is open
		if budget.IsCircuitOpen() {
			return fmt.Errorf("circuit breaker open, operation not allowed")
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute operation
		err := operation()
		if err == nil {
			budget.RecordSuccess()
			return nil
		}

		// Record error and check if we can retry
		if !budget.RecordError(err) {
			return fmt.Errorf("error budget exceeded: %w", err)
		}

		if !budget.CanRetry() {
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		// Wait for backoff delay
		delay := budget.GetBackoffDelay()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}
}

// MultiErrorBudget manages multiple error budgets for different operation types
type MultiErrorBudget struct {
	budgets map[string]*ErrorBudget
	mu      sync.RWMutex
}

// NewMultiErrorBudget creates a new multi-error budget manager
func NewMultiErrorBudget() *MultiErrorBudget {
	return &MultiErrorBudget{
		budgets: make(map[string]*ErrorBudget),
	}
}

// GetOrCreateBudget gets or creates an error budget for an operation type
func (meb *MultiErrorBudget) GetOrCreateBudget(operationType string, maxErrors int, resetDuration time.Duration) *ErrorBudget {
	meb.mu.Lock()
	defer meb.mu.Unlock()

	if budget, exists := meb.budgets[operationType]; exists {
		return budget
	}

	budget := NewErrorBudget(maxErrors, resetDuration)
	meb.budgets[operationType] = budget
	return budget
}

// GetBudget gets an existing error budget
func (meb *MultiErrorBudget) GetBudget(operationType string) (*ErrorBudget, bool) {
	meb.mu.RLock()
	defer meb.mu.RUnlock()

	budget, exists := meb.budgets[operationType]
	return budget, exists
}

// GetAllStatuses returns status for all error budgets
func (meb *MultiErrorBudget) GetAllStatuses() map[string]ErrorBudgetStatus {
	meb.mu.RLock()
	defer meb.mu.RUnlock()

	statuses := make(map[string]ErrorBudgetStatus)
	for opType, budget := range meb.budgets {
		statuses[opType] = budget.GetStatus()
	}

	return statuses
}
