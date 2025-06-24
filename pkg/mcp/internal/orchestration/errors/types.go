package errors

import (
	"time"
)

// ErrorRoutingRule defines how to route specific types of errors
type ErrorRoutingRule struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Conditions  []RoutingCondition      `json:"conditions"`
	Action      string                  `json:"action"` // retry, redirect, skip, fail, recover
	RedirectTo  string                  `json:"redirect_to,omitempty"`
	RetryPolicy *RetryPolicy            `json:"retry_policy,omitempty"`
	Parameters  *ErrorRoutingParameters `json:"parameters,omitempty"`
	Priority    int                     `json:"priority"` // Higher number = higher priority
	Enabled     bool                    `json:"enabled"`
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

// RetryPolicy defines how to retry failed operations
type RetryPolicy struct {
	MaxAttempts  int           `json:"max_attempts"`
	BackoffMode  string        `json:"backoff_mode"` // fixed, linear, exponential
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay,omitempty"`
	Multiplier   float64       `json:"multiplier,omitempty"`
}

// RecoveryStrategy defines how to recover from specific types of errors
type RecoveryStrategy struct {
	ID                 string                      `json:"id"`
	Name               string                      `json:"name"`
	Description        string                      `json:"description"`
	ApplicableErrors   []string                    `json:"applicable_errors"`
	AutoRecovery       bool                        `json:"auto_recovery"`
	RecoverySteps      []RecoveryStep              `json:"recovery_steps"`
	SuccessProbability float64                     `json:"success_probability"`
	EstimatedDuration  time.Duration               `json:"estimated_duration"`
	Requirements       []string                    `json:"requirements"`
	Parameters         *RecoveryStrategyParameters `json:"parameters"`
}

// RecoveryStep represents a single step in a recovery strategy
type RecoveryStep struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Action      string                  `json:"action"`
	Parameters  *RecoveryStepParameters `json:"parameters"`
	Timeout     time.Duration           `json:"timeout"`
	RetryOnFail bool                    `json:"retry_on_fail"`
	IgnoreError bool                    `json:"ignore_error"`
}

// RecoveryStepParameters contains parameters for a recovery step
type RecoveryStepParameters struct {
	CustomParams map[string]string `json:"custom_params,omitempty"`
}

// RecoveryStrategyParameters contains parameters for a recovery strategy
type RecoveryStrategyParameters struct {
	CustomParams map[string]string `json:"custom_params,omitempty"`
}

// RecoveryOption represents an available recovery option
type RecoveryOption struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Probability float64                `json:"probability"`
	Cost        string                 `json:"cost"` // low, medium, high
}

// ErrorAction represents the action to take for an error
type ErrorAction struct {
	Action     string                 `json:"action"` // retry, redirect, skip, fail, recover
	RedirectTo string                 `json:"redirect_to,omitempty"`
	RetryAfter *time.Duration         `json:"retry_after,omitempty"`
	Message    string                 `json:"message"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// RedirectionPlan contains detailed information about an error redirection
type RedirectionPlan struct {
	SourceStage          string                   `json:"source_stage"`
	TargetStage          string                   `json:"target_stage"`
	RedirectionType      string                   `json:"redirection_type"`
	CreatedAt            time.Time                `json:"created_at"`
	EstimatedDuration    time.Duration            `json:"estimated_duration"`
	ContextPreservation  bool                     `json:"context_preservation"`
	RequiredContext      []string                 `json:"required_context"`
	MissingContext       []string                 `json:"missing_context"`
	Parameters           map[string]interface{}   `json:"parameters"`
	ExpectedOutcome      string                   `json:"expected_outcome"`
	InterventionRequired bool                     `json:"intervention_required"`
	OriginalError        *RedirectionErrorContext `json:"original_error"`
}

// RedirectionErrorContext preserves error information for redirection
type RedirectionErrorContext struct {
	ErrorID      string    `json:"error_id"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Severity     string    `json:"severity"`
	Timestamp    time.Time `json:"timestamp"`
}
