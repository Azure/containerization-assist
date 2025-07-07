package session

import (
	"time"
)

// Supporting types for workflow operations

// WorkflowSpec represents workflow specification for session creation
// This consolidates workflow-specific types into the session package
type WorkflowSpec struct {
	Metadata WorkflowMetadata `json:"metadata"`
	Spec     WorkflowSpecData `json:"spec"`
}

// WorkflowMetadata represents workflow metadata
type WorkflowMetadata struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// WorkflowSpecData represents workflow specification data
type WorkflowSpecData struct {
	Variables map[string]interface{} `json:"variables,omitempty"`
	Stages    []WorkflowStage        `json:"stages,omitempty"`
}

// WorkflowStage represents a workflow stage
type WorkflowStage struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type,omitempty"`
	Tools        []string               `json:"tools,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
}

// WorkflowSession represents workflow-specific session data
// This extends SessionState with workflow-specific fields
type WorkflowSession struct {
	*SessionState

	// Workflow-specific fields
	WorkflowID       string                 `json:"workflow_id"`
	WorkflowName     string                 `json:"workflow_name"`
	WorkflowVersion  string                 `json:"workflow_version"`
	Status           WorkflowStatus         `json:"status"`
	Stages           []WorkflowStage        `json:"stages,omitempty"`
	CurrentStage     string                 `json:"current_stage"`
	CompletedStages  []string               `json:"completed_stages"`
	FailedStages     []string               `json:"failed_stages"`
	SkippedStages    []string               `json:"skipped_stages"`
	StageResults     map[string]interface{} `json:"stage_results"`
	SharedContext    map[string]interface{} `json:"shared_context"`
	Checkpoints      []WorkflowCheckpoint   `json:"checkpoints"`
	ResourceBindings map[string]interface{} `json:"resource_bindings"`
	StartTime        time.Time              `json:"start_time"`
	EndTime          *time.Time             `json:"end_time,omitempty"`
	LastActivity     time.Time              `json:"last_activity"`

	// Type-safe alternatives to interface{} fields
	TypedVariables        *WorkflowVariables  `json:"typed_variables,omitempty"`
	TypedContext          *WorkflowContext    `json:"typed_context,omitempty"`
	TypedSharedContext    *SharedWorkflowData `json:"typed_shared_context,omitempty"`
	TypedResourceBindings *ResourceBindings   `json:"typed_resource_bindings,omitempty"`
	TypedStageResults     *StageResults       `json:"typed_stage_results,omitempty"`
}

// WorkflowStatus represents workflow execution status
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

// WorkflowCheckpoint represents a workflow checkpoint
type WorkflowCheckpoint struct {
	Stage     string    `json:"stage"`
	Timestamp time.Time `json:"timestamp"`
	State     string    `json:"state"`
}

// Type-safe workflow data structures

// WorkflowVariables represents type-safe workflow variables
type WorkflowVariables struct {
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	ConfigValues   map[string]string `json:"config_values,omitempty"`
	UserParameters map[string]string `json:"user_parameters,omitempty"`
}

// WorkflowContext represents type-safe workflow context
type WorkflowContext struct {
	SessionID  string            `json:"session_id"`
	Properties map[string]string `json:"properties,omitempty"`
}

// SharedWorkflowData represents type-safe shared workflow data
type SharedWorkflowData struct {
	CustomData map[string]string `json:"custom_data,omitempty"`
}

// ResourceBindings represents type-safe resource bindings
type ResourceBindings struct {
	ImageBindings     map[string]string `json:"image_bindings,omitempty"`
	NamespaceBindings map[string]string `json:"namespace_bindings,omitempty"`
	ConfigMapBindings map[string]string `json:"configmap_bindings,omitempty"`
	SecretBindings    map[string]string `json:"secret_bindings,omitempty"`
	VolumeBindings    map[string]string `json:"volume_bindings,omitempty"`
}

// StageResults represents type-safe stage results
type StageResults struct {
	Results map[string]*TypedStageResult `json:"results,omitempty"`
}

// TypedStageResult represents a type-safe stage result
type TypedStageResult struct {
	Success   bool              `json:"success"`
	Message   string            `json:"message,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Timestamp time.Time         `json:"timestamp"`
}

// UnifiedSessionManagerConfig represents configuration for the unified session manager
type UnifiedSessionManagerConfig struct {
	// Base configuration
	WorkspaceDir      string        `json:"workspace_dir"`
	MaxSessions       int           `json:"max_sessions"`
	SessionTTL        time.Duration `json:"session_ttl"`
	MaxDiskPerSession int64         `json:"max_disk_per_session"`
	TotalDiskLimit    int64         `json:"total_disk_limit"`
	StorePath         string        `json:"store_path"`

	// Workflow configuration
	EnableWorkflows       bool          `json:"enable_workflows"`
	WorkflowSessionTTL    time.Duration `json:"workflow_session_ttl"`
	MaxWorkflowSessions   int           `json:"max_workflow_sessions"`
	WorkflowCleanupWindow time.Duration `json:"workflow_cleanup_window"`

	// Performance configuration
	CleanupInterval       time.Duration `json:"cleanup_interval"`
	GCBatchSize           int           `json:"gc_batch_size"`
	EnableAsyncOperations bool          `json:"enable_async_operations"`
}
