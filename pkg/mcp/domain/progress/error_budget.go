package progress

import (
	"sync"
	"time"
)

// ErrorBudget provides circuit breaker functionality for workflow steps.
type ErrorBudget struct {
	maxErrors    int
	resetWindow  time.Duration
	errors       []time.Time
	successCount int
	mu           sync.RWMutex
}

// ErrorBudgetStatus represents the current state of the error budget.
type ErrorBudgetStatus int

const (
	ErrorBudgetHealthy ErrorBudgetStatus = iota
	ErrorBudgetWarning
	ErrorBudgetExhausted
)

func (s ErrorBudgetStatus) String() string {
	switch s {
	case ErrorBudgetHealthy:
		return "healthy"
	case ErrorBudgetWarning:
		return "warning"
	case ErrorBudgetExhausted:
		return "exhausted"
	default:
		return "unknown"
	}
}

// NewErrorBudget creates a new error budget.
func NewErrorBudget(maxErrors int, resetWindow time.Duration) *ErrorBudget {
	return &ErrorBudget{
		maxErrors:   maxErrors,
		resetWindow: resetWindow,
		errors:      make([]time.Time, 0, maxErrors),
	}
}

// RecordError records an error and returns true if within budget.
func (eb *ErrorBudget) RecordError(err error) bool {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	now := time.Now()

	// Clean up old errors outside the reset window
	eb.cleanupOldErrors(now)

	// Add new error
	eb.errors = append(eb.errors, now)

	// Check if we're within budget
	return len(eb.errors) <= eb.maxErrors
}

// RecordSuccess records a successful operation.
func (eb *ErrorBudget) RecordSuccess() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.successCount++
}

// IsCircuitOpen returns true if the circuit breaker is open (too many errors).
func (eb *ErrorBudget) IsCircuitOpen() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	now := time.Now()
	// Count recent errors
	recentErrors := 0
	for _, errTime := range eb.errors {
		if now.Sub(errTime) <= eb.resetWindow {
			recentErrors++
		}
	}

	return recentErrors > eb.maxErrors
}

// GetStatus returns the current error budget status.
func (eb *ErrorBudget) GetStatus() ErrorBudgetStatus {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	now := time.Now()
	recentErrors := 0
	for _, errTime := range eb.errors {
		if now.Sub(errTime) <= eb.resetWindow {
			recentErrors++
		}
	}

	if recentErrors > eb.maxErrors {
		return ErrorBudgetExhausted
	} else if recentErrors > eb.maxErrors/2 {
		return ErrorBudgetWarning
	}

	return ErrorBudgetHealthy
}

// cleanupOldErrors removes errors outside the reset window.
func (eb *ErrorBudget) cleanupOldErrors(now time.Time) {
	cutoff := now.Add(-eb.resetWindow)

	// Find first error within window
	i := 0
	for i < len(eb.errors) && eb.errors[i].Before(cutoff) {
		i++
	}

	// Remove old errors
	if i > 0 {
		eb.errors = eb.errors[i:]
	}
}
