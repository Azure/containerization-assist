package workflow

import (
	"time"
)

// WorkflowSpec defines a declarative workflow specification
type WorkflowSpec struct {
	APIVersion string             `yaml:"apiVersion" json:"apiVersion"`
	Kind       string             `yaml:"kind" json:"kind"`
	Metadata   WorkflowMetadata   `yaml:"metadata" json:"metadata"`
	Spec       WorkflowDefinition `yaml:"spec" json:"spec"`
}

// WorkflowMetadata contains workflow identification information
type WorkflowMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// WorkflowDefinition contains the workflow execution specification
type WorkflowDefinition struct {
	Stages                 []WorkflowStage         `yaml:"stages" json:"stages"`
	Variables              map[string]string       `yaml:"variables,omitempty" json:"variables,omitempty"`
	ErrorPolicy            ErrorPolicy             `yaml:"errorPolicy,omitempty" json:"errorPolicy,omitempty"`
	Timeout                *time.Duration          `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryPolicy            *RetryPolicy            `yaml:"retryPolicy,omitempty" json:"retryPolicy,omitempty"`
	ConcurrencyConfig      *ConcurrencyConfig      `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	StageTypeRetryPolicies map[string]*RetryPolicy `yaml:"stageTypeRetryPolicies,omitempty" json:"stageTypeRetryPolicies,omitempty"`
}

// WorkflowStage represents a single stage in the workflow
type WorkflowStage struct {
	Name        string            `yaml:"name" json:"name"`
	Type        string            `yaml:"type,omitempty" json:"type,omitempty"` // Stage type for categorization (build, test, deploy, etc.)
	Tools       []string          `yaml:"tools" json:"tools"`
	DependsOn   []string          `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Parallel    bool              `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	Conditions  []StageCondition  `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
	Timeout     *time.Duration    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryPolicy *RetryPolicy      `yaml:"retryPolicy,omitempty" json:"retryPolicy,omitempty"`
	OnFailure   *FailureAction    `yaml:"onFailure,omitempty" json:"onFailure,omitempty"`
}

// StageCondition defines conditions for stage execution
type StageCondition struct {
	Key      string      `yaml:"key" json:"key"`
	Operator string      `yaml:"operator" json:"operator"` // required, equals, not_equals, exists, not_exists
	Value    interface{} `yaml:"value,omitempty" json:"value,omitempty"`
}

// ErrorPolicy defines how errors should be handled at the workflow level
type ErrorPolicy struct {
	Mode        string         `yaml:"mode" json:"mode"` // fail_fast, continue, ignore
	MaxFailures int            `yaml:"maxFailures,omitempty" json:"maxFailures,omitempty"`
	Routing     []ErrorRouting `yaml:"routing,omitempty" json:"routing,omitempty"`
}

// ErrorRouting defines how specific errors should be routed
type ErrorRouting struct {
	FromTool   string            `yaml:"fromTool" json:"fromTool"`
	ErrorType  string            `yaml:"errorType" json:"errorType"`
	Action     string            `yaml:"action" json:"action"` // retry, redirect, skip, fail
	RedirectTo string            `yaml:"redirectTo,omitempty" json:"redirectTo,omitempty"`
	Parameters map[string]string `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts  int           `yaml:"maxAttempts" json:"maxAttempts"`
	BackoffMode  string        `yaml:"backoffMode" json:"backoffMode"` // fixed, exponential, linear
	InitialDelay time.Duration `yaml:"initialDelay" json:"initialDelay"`
	MaxDelay     time.Duration `yaml:"maxDelay,omitempty" json:"maxDelay,omitempty"`
	Multiplier   float64       `yaml:"multiplier,omitempty" json:"multiplier,omitempty"`
}

// FailureAction defines what to do when a stage fails
type FailureAction struct {
	Action     string            `yaml:"action" json:"action"` // retry, redirect, skip, fail
	RedirectTo string            `yaml:"redirectTo,omitempty" json:"redirectTo,omitempty"`
	Parameters map[string]string `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// WorkflowSession represents the execution state of a workflow
type WorkflowSession struct {
	ID               string                 `json:"id"`
	WorkflowID       string                 `json:"workflow_id"`
	WorkflowName     string                 `json:"workflow_name"`
	WorkflowVersion  string                 `json:"workflow_version"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Status           WorkflowStatus         `json:"status"`
	CurrentStage     string                 `json:"current_stage"`
	CompletedStages  []string               `json:"completed_stages"`
	FailedStages     []string               `json:"failed_stages"`
	SkippedStages    []string               `json:"skipped_stages"`
	StageResults     map[string]interface{} `json:"stage_results"`
	SharedContext    map[string]interface{} `json:"shared_context"`
	Checkpoints      []WorkflowCheckpoint   `json:"checkpoints"`
	ErrorContext     *WorkflowErrorContext  `json:"error_context,omitempty"`
	ResourceBindings map[string]string      `json:"resource_bindings"`
	StartTime        time.Time              `json:"start_time"`
	EndTime          *time.Time             `json:"end_time,omitempty"`
	ExecutionOptions *ExecutionOptions      `json:"execution_options,omitempty"`
	LastActivity     time.Time              `json:"last_activity"`
	Duration         time.Duration          `json:"duration"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// WorkflowStatus represents the current status of workflow execution
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

// WorkflowCheckpoint represents a point in workflow execution that can be resumed
type WorkflowCheckpoint struct {
	ID           string                 `json:"id"`
	StageName    string                 `json:"stage_name"`
	Timestamp    time.Time              `json:"timestamp"`
	SessionState map[string]interface{} `json:"session_state"`
	StageResults map[string]interface{} `json:"stage_results"`
	Message      string                 `json:"message"`
	WorkflowSpec *WorkflowSpec          `json:"workflow_spec"`
}

// WorkflowErrorContext provides detailed error information for workflow failures
type WorkflowErrorContext struct {
	LastError        *WorkflowError         `json:"last_error,omitempty"`
	ErrorHistory     []WorkflowError        `json:"error_history"`
	FailedStage      string                 `json:"failed_stage"`
	RetryCount       int                    `json:"retry_count"`
	MaxRetries       int                    `json:"max_retries"`
	RetryAttempts    map[string]int         `json:"retry_attempts,omitempty"`
	NextRetryAt      *time.Time             `json:"next_retry_at,omitempty"`
	RecoveryStrategy string                 `json:"recovery_strategy"`
	RecoveryOptions  map[string]interface{} `json:"recovery_options"`
}

// WorkflowError represents a structured error that occurred during workflow execution
type WorkflowError struct {
	ID        string                 `json:"id"`
	StageName string                 `json:"stage_name"`
	ToolName  string                 `json:"tool_name"`
	ErrorType string                 `json:"error_type"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Retryable bool                   `json:"retryable"`
}

// WorkflowResult represents the final result of workflow execution
type WorkflowResult struct {
	WorkflowID      string                 `json:"workflow_id"`
	SessionID       string                 `json:"session_id"`
	Status          WorkflowStatus         `json:"status"`
	Success         bool                   `json:"success"`
	Message         string                 `json:"message"`
	Duration        time.Duration          `json:"duration"`
	StagesExecuted  int                    `json:"stages_executed"`
	StagesCompleted int                    `json:"stages_completed"`
	StagesFailed    int                    `json:"stages_failed"`
	StagesSkipped   int                    `json:"stages_skipped"`
	Results         map[string]interface{} `json:"results"`
	Artifacts       []WorkflowArtifact     `json:"artifacts"`
	Metrics         WorkflowMetrics        `json:"metrics"`
	ErrorSummary    *WorkflowErrorSummary  `json:"error_summary,omitempty"`
}

// WorkflowArtifact represents an output artifact from workflow execution
type WorkflowArtifact struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // file, image, manifest, report
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Checksum    string            `json:"checksum"`
	Metadata    map[string]string `json:"metadata"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
}

// WorkflowMetrics contains performance and execution metrics
type WorkflowMetrics struct {
	TotalDuration       time.Duration            `json:"total_duration"`
	StageDurations      map[string]time.Duration `json:"stage_durations"`
	ToolExecutionCounts map[string]int           `json:"tool_execution_counts"`
	ResourceUtilization map[string]interface{}   `json:"resource_utilization"`
	PerformanceMetrics  map[string]float64       `json:"performance_metrics"`
	QualityMetrics      map[string]interface{}   `json:"quality_metrics"`
}

// WorkflowErrorSummary provides a summary of errors that occurred during execution
type WorkflowErrorSummary struct {
	TotalErrors       int            `json:"total_errors"`
	ErrorsByType      map[string]int `json:"errors_by_type"`
	ErrorsByStage     map[string]int `json:"errors_by_stage"`
	CriticalErrors    int            `json:"critical_errors"`
	RecoverableErrors int            `json:"recoverable_errors"`
	RetryAttempts     int            `json:"retry_attempts"`
	LastError         *WorkflowError `json:"last_error,omitempty"`
	Recommendations   []string       `json:"recommendations"`
}

// StageResult represents the result of executing a workflow stage
type StageResult struct {
	StageName  string                 `json:"stage_name"`
	Success    bool                   `json:"success"`
	Duration   time.Duration          `json:"duration"`
	Results    map[string]interface{} `json:"results"`
	Artifacts  []WorkflowArtifact     `json:"artifacts"`
	Metrics    map[string]interface{} `json:"metrics"`
	Error      *WorkflowError         `json:"error,omitempty"`
	NextStage  string                 `json:"next_stage,omitempty"`
	Checkpoint *WorkflowCheckpoint    `json:"checkpoint,omitempty"`
}

// ErrorAction represents an action to take in response to an error
type ErrorAction struct {
	Action     string                 `json:"action"` // retry, redirect, skip, fail
	RedirectTo string                 `json:"redirect_to,omitempty"`
	RetryAfter *time.Duration         `json:"retry_after,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Message    string                 `json:"message"`
}

// RecoveryOption represents a possible recovery action for an error
type RecoveryOption struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"`
	Parameters  map[string]interface{} `json:"parameters"`
	Probability float64                `json:"probability"` // 0.0-1.0 likelihood of success
	Cost        string                 `json:"cost"`        // low, medium, high
}

// SessionFilter defines criteria for filtering workflow sessions
type SessionFilter struct {
	WorkflowName string            `json:"workflow_name,omitempty"`
	Status       WorkflowStatus    `json:"status,omitempty"`
	StartTime    *time.Time        `json:"start_time,omitempty"`
	EndTime      *time.Time        `json:"end_time,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Limit        int               `json:"limit,omitempty"`
	Offset       int               `json:"offset,omitempty"`
}

// ConcurrencyConfig defines concurrency limits and behavior for workflow execution
type ConcurrencyConfig struct {
	MaxParallelStages int           `yaml:"maxParallelStages" json:"maxParallelStages"` // Maximum concurrent stages (0 = unlimited)
	StageTimeout      time.Duration `yaml:"stageTimeout" json:"stageTimeout"`           // Per-stage execution timeout
	QueueSize         int           `yaml:"queueSize" json:"queueSize"`                 // Size of stage execution queue
	WorkerPoolSize    int           `yaml:"workerPoolSize" json:"workerPoolSize"`       // Number of worker goroutines
}

// ExecutionOptions controls workflow execution behavior
type ExecutionOptions struct {
	EnableParallel       bool               // Enable parallel execution of stages
	DryRun               bool               // Perform validation without execution
	CreateCheckpoints    bool               // Create checkpoints after each stage group
	SessionID            string             // Resume existing session
	ResumeFromCheckpoint string             // Checkpoint ID to resume from
	Variables            map[string]string  // Additional variables for workflow
	Timeout              time.Duration      // Overall workflow timeout
	StageTimeout         time.Duration      // Default timeout for stages
	ConcurrencyConfig    *ConcurrencyConfig // Concurrency limits and behavior
}

// ExecutionOption is a function that modifies execution options
type ExecutionOption func(*ExecutionOptions)
