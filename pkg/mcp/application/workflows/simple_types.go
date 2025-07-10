package workflow

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ToolRegistry provides simple tool registration
type ToolRegistry struct {
	tools map[string]api.Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]api.Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(name string, tool api.Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (api.Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// GetTool retrieves a tool by name (alternative interface)
func (r *ToolRegistry) GetTool(name string) (api.Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeToolNotFound).
			Type(errors.ErrTypeTool).
			Severity(errors.SeverityMedium).
			Messagef("tool not found: %s", name).
			WithLocation().
			Build()
	}
	return tool, nil
}

// List returns all registered tools
func (r *ToolRegistry) List() map[string]api.Tool {
	result := make(map[string]api.Tool)
	for name, tool := range r.tools {
		result[name] = tool
	}
	return result
}

// ExecutionContext provides simple execution context
type ExecutionContext struct {
	Context context.Context
	Data    map[string]interface{}
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(ctx context.Context) *ExecutionContext {
	return &ExecutionContext{
		Context: ctx,
		Data:    make(map[string]interface{}),
	}
}

// ExecutionSession represents a workflow execution session
type ExecutionSession struct {
	ID         string                 `json:"id"`
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Context    *ExecutionContext      `json:"context"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowCheckpoint represents a checkpoint in workflow execution
type WorkflowCheckpoint struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	StageID     string                 `json:"stage_id"`
	Timestamp   time.Time              `json:"timestamp"`
	State       map[string]interface{} `json:"state"`
	Description string                 `json:"description,omitempty"`
}

// WorkflowSpec defines a workflow specification
type WorkflowSpec struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version"`
	Stages      []WorkflowStage        `json:"stages"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Metadata    WorkflowMetadata       `json:"metadata,omitempty"`
}

// WorkflowStage represents a stage in a workflow
type WorkflowStage struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	ToolName     string                 `json:"tool_name"`
	Tools        []string               `json:"tools,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Timeout      *time.Duration         `json:"timeout,omitempty"`
	RetryPolicy  *api.RetryPolicy       `json:"retry_policy,omitempty"`
	Conditions   []StageCondition       `json:"conditions,omitempty"`
	OnFailure    *FailureAction         `json:"on_failure,omitempty"`
}

// ExecutionOption provides options for workflow execution
type ExecutionOption struct {
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// StageCondition represents a condition for stage execution
type StageCondition struct {
	Type      string                 `json:"type"`
	Key       string                 `json:"key,omitempty"`
	Operator  string                 `json:"operator,omitempty"`
	Operation string                 `json:"operation,omitempty"`
	Value     interface{}            `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowStage extended fields
type WorkflowStageExtended struct {
	WorkflowStage
	Conditions []StageCondition `json:"conditions,omitempty"`
	OnFailure  *FailureAction   `json:"on_failure,omitempty"`
}

// FailureAction represents action to take on stage failure
type FailureAction struct {
	Action     string `json:"action"`
	RedirectTo string `json:"redirect_to,omitempty"`
}

// ExecutionStage represents a stage in workflow execution
type ExecutionStage struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Tools     []string               `json:"tools"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DependsOn []string               `json:"depends_on,omitempty"`
}

// WorkflowMetadata contains metadata about a workflow
type WorkflowMetadata struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	CreatedBy   string                 `json:"created_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	ModifiedAt  time.Time              `json:"modified_at,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// SessionFilter provides filtering options for session queries
type SessionFilter struct {
	SessionID    string                 `json:"session_id,omitempty"`
	WorkflowID   string                 `json:"workflow_id,omitempty"`
	WorkflowName string                 `json:"workflow_name,omitempty"`
	Status       string                 `json:"status,omitempty"`
	StartTime    *time.Time             `json:"start_time,omitempty"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	StartAfter   time.Time              `json:"start_after,omitempty"`
	Limit        int                    `json:"limit,omitempty"`
	Offset       int                    `json:"offset,omitempty"`
	Labels       map[string]string      `json:"labels,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Constants for workflow status
const (
	WorkflowStatusPending   = "pending"
	WorkflowStatusRunning   = "running"
	WorkflowStatusCompleted = "completed"
	WorkflowStatusFailed    = "failed"
	WorkflowStatusPaused    = "paused"
	WorkflowStatusCancelled = "cancelled"
)

// ExecutionSession extended fields
type ExecutionSessionExtended struct {
	ExecutionSession
	WorkflowName string            `json:"workflow_name,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	LastActivity time.Time         `json:"last_activity,omitempty"`
	FailedStages []string          `json:"failed_stages,omitempty"`
}

// WorkflowCheckpoint extended fields
type WorkflowCheckpointExtended struct {
	WorkflowCheckpoint
	WorkflowSpec *WorkflowSpec `json:"workflow_spec,omitempty"`
}
