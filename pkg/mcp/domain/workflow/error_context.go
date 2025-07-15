// Package workflow provides error context accumulation for better AI-assisted recovery
package workflow

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrorContext represents detailed context about a failure
type ErrorContext struct {
	Step      string                 `json:"step"`
	Error     string                 `json:"error"`
	Timestamp time.Time              `json:"timestamp"`
	Attempt   int                    `json:"attempt"`
	Context   map[string]interface{} `json:"context"`
	Fixes     []string               `json:"fixes_attempted"`
}

// ProgressiveErrorContext accumulates error context across workflow execution
type ProgressiveErrorContext struct {
	mu          sync.RWMutex
	errors      []ErrorContext
	maxHistory  int
	stepSummary map[string]string // Summary of errors per step
}

// NewProgressiveErrorContext creates a new error context accumulator
func NewProgressiveErrorContext(maxHistory int) *ProgressiveErrorContext {
	return &ProgressiveErrorContext{
		errors:      make([]ErrorContext, 0, maxHistory),
		maxHistory:  maxHistory,
		stepSummary: make(map[string]string),
	}
}

// AddError adds a new error to the context
func (pec *ProgressiveErrorContext) AddError(step string, err error, attempt int, context map[string]interface{}) {
	pec.mu.Lock()
	defer pec.mu.Unlock()

	errorContext := ErrorContext{
		Step:      step,
		Error:     err.Error(),
		Timestamp: time.Now(),
		Attempt:   attempt,
		Context:   context,
		Fixes:     []string{},
	}

	pec.errors = append(pec.errors, errorContext)

	// Maintain max history
	if len(pec.errors) > pec.maxHistory {
		pec.errors = pec.errors[len(pec.errors)-pec.maxHistory:]
	}

	// Update step summary
	pec.updateStepSummary(step, err.Error())
}

// AddFixAttempt records a fix that was attempted
func (pec *ProgressiveErrorContext) AddFixAttempt(step string, fix string) {
	pec.mu.Lock()
	defer pec.mu.Unlock()

	// Find the most recent error for this step
	for i := len(pec.errors) - 1; i >= 0; i-- {
		if pec.errors[i].Step == step {
			pec.errors[i].Fixes = append(pec.errors[i].Fixes, fix)
			break
		}
	}
}

// updateStepSummary maintains a summary of errors per step
func (pec *ProgressiveErrorContext) updateStepSummary(step string, errorMsg string) {
	if existing, ok := pec.stepSummary[step]; ok {
		// Append to existing summary
		pec.stepSummary[step] = fmt.Sprintf("%s; %s", existing, errorMsg)
	} else {
		pec.stepSummary[step] = errorMsg
	}
}

// GetRecentErrors returns the most recent errors
func (pec *ProgressiveErrorContext) GetRecentErrors(count int) []ErrorContext {
	pec.mu.RLock()
	defer pec.mu.RUnlock()

	// Handle negative count
	if count <= 0 {
		return []ErrorContext{}
	}

	if count > len(pec.errors) {
		count = len(pec.errors)
	}

	start := len(pec.errors) - count
	if start < 0 {
		start = 0
	}

	// Return a copy to avoid data races
	result := make([]ErrorContext, count)
	copy(result, pec.errors[start:])
	return result
}

// GetStepErrors returns all errors for a specific step
func (pec *ProgressiveErrorContext) GetStepErrors(step string) []ErrorContext {
	pec.mu.RLock()
	defer pec.mu.RUnlock()

	stepErrors := []ErrorContext{}
	for _, err := range pec.errors {
		if err.Step == step {
			stepErrors = append(stepErrors, err)
		}
	}
	return stepErrors
}

// GetSummary returns a human-readable summary of all errors
func (pec *ProgressiveErrorContext) GetSummary() string {
	pec.mu.RLock()
	defer pec.mu.RUnlock()

	if len(pec.errors) == 0 {
		return "No errors recorded"
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Error History (%d total errors):\n", len(pec.errors)))

	// Group by step
	stepGroups := make(map[string][]ErrorContext)
	for _, err := range pec.errors {
		stepGroups[err.Step] = append(stepGroups[err.Step], err)
	}

	// Summarize each step
	for step, errors := range stepGroups {
		summary.WriteString(fmt.Sprintf("\n%s (%d errors):\n", step, len(errors)))
		for i, err := range errors {
			summary.WriteString(fmt.Sprintf("  [Attempt %d] %s\n", err.Attempt, err.Error))
			if len(err.Fixes) > 0 {
				summary.WriteString("    Fixes attempted:\n")
				for _, fix := range err.Fixes {
					summary.WriteString(fmt.Sprintf("    - %s\n", fix))
				}
			}
			// Only show last 2 errors per step in summary
			if i >= 1 {
				remaining := len(errors) - i - 1
				if remaining > 0 {
					summary.WriteString(fmt.Sprintf("    ... and %d more errors\n", remaining))
				}
				break
			}
		}
	}

	return summary.String()
}

// GetAIContext returns context formatted for AI analysis
func (pec *ProgressiveErrorContext) GetAIContext() string {
	var context strings.Builder

	context.WriteString("PREVIOUS ERRORS AND ATTEMPTS:\n")
	context.WriteString("============================\n\n")

	// Include recent errors with full context
	recentErrors := pec.GetRecentErrors(5)
	for i, err := range recentErrors {
		context.WriteString(fmt.Sprintf("Error %d:\n", i+1))
		context.WriteString(fmt.Sprintf("- Step: %s\n", err.Step))
		context.WriteString(fmt.Sprintf("- Error: %s\n", err.Error))
		context.WriteString(fmt.Sprintf("- Attempt: %d\n", err.Attempt))

		if len(err.Context) > 0 {
			context.WriteString("- Context:\n")
			for k, v := range err.Context {
				context.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
		}

		if len(err.Fixes) > 0 {
			context.WriteString("- Previous fixes attempted:\n")
			for _, fix := range err.Fixes {
				context.WriteString(fmt.Sprintf("  • %s\n", fix))
			}
		}
		context.WriteString("\n")
	}

	// Add step summaries (with lock protection)
	pec.mu.RLock()
	stepSummaryLen := len(pec.stepSummary)
	stepSummaryCopy := make(map[string]string, stepSummaryLen)
	for k, v := range pec.stepSummary {
		stepSummaryCopy[k] = v
	}
	pec.mu.RUnlock()

	if stepSummaryLen > 0 {
		context.WriteString("STEP ERROR PATTERNS:\n")
		context.WriteString("===================\n")
		for step, summary := range stepSummaryCopy {
			context.WriteString(fmt.Sprintf("- %s: %s\n", step, summary))
		}
	}

	return context.String()
}

// HasRepeatedErrors checks if the same error has occurred multiple times
func (pec *ProgressiveErrorContext) HasRepeatedErrors(step string, threshold int) bool {
	pec.mu.RLock()
	defer pec.mu.RUnlock()

	count := 0
	lastError := ""

	for _, err := range pec.errors {
		if err.Step == step {
			if err.Error == lastError {
				count++
				if count >= threshold {
					return true
				}
			} else {
				lastError = err.Error
				count = 1
			}
		}
	}

	return false
}

// ShouldEscalate determines if errors should be escalated based on patterns
func (pec *ProgressiveErrorContext) ShouldEscalate(step string) bool {
	// Note: HasRepeatedErrors and GetStepErrors handle their own locking
	// Escalate if:
	// 1. Same error repeated 3+ times
	if pec.HasRepeatedErrors(step, 3) {
		return true
	}

	// 2. More than 5 different errors for the same step
	stepErrors := pec.GetStepErrors(step)
	if len(stepErrors) > 5 {
		return true
	}

	// 3. Fixes have been attempted but errors persist
	for _, err := range stepErrors {
		if len(err.Fixes) >= 2 {
			return true
		}
	}

	return false
}
