package workflow

import (
	"context"
	"time"
)

// Additional workflow types to complement existing simple_types.go

// StageType defines the type of workflow stage
type StageType string

const (
	StageTypeAnalysis     StageType = "analysis"
	StageTypeBuild        StageType = "build"
	StageTypeDeploy       StageType = "deployment"
	StageTypeScan         StageType = "scan"
	StageTypeValidation   StageType = "validation"
	StageTypeNotification StageType = "notification"
	StageTypeCustom       StageType = "custom"
)

// ExecutionStatus represents the status of workflow execution
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusPaused    ExecutionStatus = "paused"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
	StatusTimedOut  ExecutionStatus = "timed_out"
)

// CheckpointStatus represents the status of a checkpoint
type CheckpointStatus string

const (
	CheckpointCreated   CheckpointStatus = "created"
	CheckpointValidated CheckpointStatus = "validated"
	CheckpointRestored  CheckpointStatus = "restored"
	CheckpointExpired   CheckpointStatus = "expired"
	CheckpointCorrupted CheckpointStatus = "corrupted"
)

// ConditionType defines the type of condition
type ConditionType string

const (
	ConditionTypeVariable ConditionType = "variable"
	ConditionTypeOutput   ConditionType = "output"
	ConditionTypeStatus   ConditionType = "status"
	ConditionTypeCustom   ConditionType = "custom"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffMode     BackoffMode   `json:"backoff_mode"`
	Multiplier      float64       `json:"multiplier,omitempty"`
	RetryableErrors []string      `json:"retryable_errors,omitempty"`
}

// BackoffMode defines the backoff strategy
type BackoffMode string

const (
	BackoffLinear      BackoffMode = "linear"
	BackoffExponential BackoffMode = "exponential"
	BackoffConstant    BackoffMode = "constant"
)

// FailureActionType defines types of failure actions
type FailureActionType string

const (
	FailureActionFail     FailureActionType = "fail"
	FailureActionContinue FailureActionType = "continue"
	FailureActionRedirect FailureActionType = "redirect"
	FailureActionRetry    FailureActionType = "retry"
	FailureActionSkip     FailureActionType = "skip"
)

// StageAction defines actions to take after stage completion
type StageAction struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// ExecutionError represents an error during execution
type ExecutionError struct {
	StageID   string    `json:"stage_id"`
	Error     string    `json:"error"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
}

// LogEntry represents a log entry during execution
type LogEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Level     string      `json:"level"`
	StageID   string      `json:"stage_id,omitempty"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

// WorkflowInput defines workflow input parameters
type WorkflowInput struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Validation  string      `json:"validation,omitempty"`
}

// WorkflowOutput defines workflow output parameters
type WorkflowOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"`
}

// ExtendedExecutionConfig holds execution configuration
type ExtendedExecutionConfig struct {
	Context     context.Context
	Variables   map[string]interface{}
	Inputs      map[string]interface{}
	DryRun      bool
	SkipStages  []string
	OnlyStages  []string
	Timeout     time.Duration
	CallbackURL string
	Metadata    map[string]interface{}
}

// ExecutionOptionFunc for workflow execution configuration
type ExecutionOptionFunc func(*ExtendedExecutionConfig)

// WithContext sets the execution context
func WithContext(ctx context.Context) ExecutionOptionFunc {
	return func(cfg *ExtendedExecutionConfig) {
		cfg.Context = ctx
	}
}

// WithVariables sets execution variables
func WithVariables(vars map[string]interface{}) ExecutionOptionFunc {
	return func(cfg *ExtendedExecutionConfig) {
		cfg.Variables = vars
	}
}

// WithDryRun enables dry run mode
func WithDryRun() ExecutionOptionFunc {
	return func(cfg *ExtendedExecutionConfig) {
		cfg.DryRun = true
	}
}

// Service integration types

// ServiceConfig defines service configuration
type ServiceConfig struct {
	Type        string                 `json:"type"`
	Endpoint    string                 `json:"endpoint,omitempty"`
	Credentials map[string]interface{} `json:"credentials,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// ToolExecution represents a tool execution
type ToolExecution struct {
	ID        string                 `json:"id"`
	Tool      string                 `json:"tool"`
	Stage     string                 `json:"stage,omitempty"`
	Input     map[string]interface{} `json:"input"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowPersistenceState for workflow persistence support
type WorkflowPersistenceState struct {
	SessionID    string                 `json:"session_id"`
	CheckpointID string                 `json:"checkpoint_id,omitempty"`
	WorkflowID   string                 `json:"workflow_id"`
	StateData    map[string]interface{} `json:"state_data,omitempty"`
	LastUpdated  time.Time              `json:"last_updated"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
