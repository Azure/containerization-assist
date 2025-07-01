package errors

import (
	"time"
)

// Orchestration error types and routing (from internal/orchestration/error_*.go)

// ErrorRoutingRule defines how to route specific types of errors
type ErrorRoutingRule struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Conditions  []RoutingCondition        `json:"conditions"`
	Action      string                    `json:"action"` // retry, redirect, skip, fail, recover
	RedirectTo  string                    `json:"redirect_to,omitempty"`
	RetryPolicy *OrchestrationRetryPolicy `json:"retry_policy,omitempty"`
	Parameters  *ErrorRoutingParameters   `json:"parameters,omitempty"`
	Priority    int                       `json:"priority"` // Higher number = higher priority
	Enabled     bool                      `json:"enabled"`
}

// RoutingCondition defines conditions for matching errors
type RoutingCondition struct {
	Field         string      `json:"field"`    // error_type, stage_name, tool_name, message, severity
	Operator      string      `json:"operator"` // equals, contains, matches, not_equals
	Value         interface{} `json:"value"`
	CaseSensitive bool        `json:"case_sensitive,omitempty"`
}

// ErrorRoutingParameters contains additional parameters for error routing
type ErrorRoutingParameters struct {
	IncreaseTimeout   bool              `json:"increase_timeout,omitempty"`
	TimeoutMultiplier float64           `json:"timeout_multiplier,omitempty"`
	ValidationMode    string            `json:"validation_mode,omitempty"`
	FixErrors         bool              `json:"fix_errors,omitempty"`
	AddWarning        bool              `json:"add_warning,omitempty"`
	ContinueWorkflow  bool              `json:"continue_workflow,omitempty"`
	CustomParams      map[string]string `json:"custom_params,omitempty"`
}

// OrchestrationRetryPolicy defines how to retry failed operations
type OrchestrationRetryPolicy struct {
	MaxAttempts  int           `json:"max_attempts"`
	BackoffMode  string        `json:"backoff_mode"` // fixed, linear, exponential
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay,omitempty"`
	Multiplier   float64       `json:"multiplier,omitempty"`
}

// RecoveryStrategy defines how to recover from specific types of errors
type RecoveryStrategy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	AppliesTo   []string         `json:"applies_to"` // List of error types/patterns
	Actions     []RecoveryAction `json:"actions"`
	Conditions  []string         `json:"conditions,omitempty"`
	Priority    int              `json:"priority"`
	Enabled     bool             `json:"enabled"`
}

// RecoveryAction defines a specific recovery action
type RecoveryAction struct {
	Type        string                 `json:"type"` // cleanup, rollback, compensate, skip
	Description string                 `json:"description"`
	Target      string                 `json:"target,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
}

// ErrorClassification provides classification metadata for errors
type ErrorClassification struct {
	Category     ErrorCategory `json:"category"`
	Severity     Severity      `json:"severity"`
	Retryable    bool          `json:"retryable"`
	Recoverable  bool          `json:"recoverable"`
	UserFacing   bool          `json:"user_facing"`
	RequiresAuth bool          `json:"requires_auth"`
	Tags         []string      `json:"tags,omitempty"`
}

// ClassifyError classifies an error based on its characteristics
func ClassifyError(err error) *ErrorClassification {
	classification := &ErrorClassification{
		Category:    CategoryInternal,
		Severity:    SeverityMedium,
		Retryable:   false,
		Recoverable: true,
		UserFacing:  false,
	}

	// Check if it's a CoreError
	if coreErr, ok := err.(*CoreError); ok {
		classification.Category = coreErr.Category
		classification.Severity = coreErr.Severity
		classification.Retryable = coreErr.Retryable
		classification.Recoverable = coreErr.Recoverable

		// Determine if user-facing based on category
		switch coreErr.Category {
		case CategoryValidation, CategoryAuth, CategoryConfig:
			classification.UserFacing = true
		case CategoryInternal, CategoryNetwork:
			classification.UserFacing = false
		}

		// Check if requires auth
		if coreErr.Category == CategoryAuth {
			classification.RequiresAuth = true
		}

		return classification
	}

	// Check if it's a ToolError
	if toolErr, ok := err.(*ToolError); ok {
		// Map error types to categories
		switch toolErr.Type {
		case ErrTypeValidation:
			classification.Category = CategoryValidation
		case ErrTypeNetwork:
			classification.Category = CategoryNetwork
		case ErrTypeBuild:
			classification.Category = CategoryBuild
		case ErrTypeDeployment:
			classification.Category = CategoryDeploy
		case ErrTypeSecurity:
			classification.Category = CategorySecurity
		case ErrTypeConfig:
			classification.Category = CategoryConfig
		case ErrTypePermission:
			classification.Category = CategoryAuth
			classification.RequiresAuth = true
		}

		classification.Severity = toolErr.Severity
		return classification
	}

	// Default classification for unknown errors
	return classification
}

// ShouldRetry determines if an error should be retried based on classification
func ShouldRetry(err error, attemptNumber int) bool {
	classification := ClassifyError(err)

	// Don't retry if not retryable
	if !classification.Retryable {
		return false
	}

	// Check attempt limits based on severity
	maxAttempts := 3
	switch classification.Severity {
	case SeverityCritical:
		maxAttempts = 1 // Only one retry for critical errors
	case SeverityHigh:
		maxAttempts = 2
	case SeverityMedium:
		maxAttempts = 3
	case SeverityLow:
		maxAttempts = 5
	}

	return attemptNumber < maxAttempts
}

// GetRetryDelay calculates retry delay based on error and attempt
func GetRetryDelay(err error, attemptNumber int) time.Duration {
	baseDelay := 1 * time.Second

	classification := ClassifyError(err)

	// Adjust base delay based on error category
	switch classification.Category {
	case CategoryNetwork:
		baseDelay = 2 * time.Second // Network errors need more time
	case CategoryResource:
		baseDelay = 5 * time.Second // Resource contention needs longer wait
	case CategoryTimeout:
		baseDelay = 3 * time.Second
	}

	// Exponential backoff
	delay := baseDelay * time.Duration(1<<uint(attemptNumber-1))

	// Cap at 30 seconds
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
