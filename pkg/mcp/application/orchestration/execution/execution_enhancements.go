package execution

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Type-Safe Execution Enhancements
// ============================================================================

// TypedExecutionStage represents a fully type-safe execution stage
type TypedExecutionStage struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Type        string                `json:"type"`
	Tools       []string              `json:"tools"`
	DependsOn   []string              `json:"depends_on"`
	Variables   *StageVariables       `json:"variables"`
	Timeout     *time.Duration        `json:"timeout"`
	MaxRetries  int                   `json:"max_retries"`
	Parallel    bool                  `json:"parallel"`
	Conditions  []TypedStageCondition `json:"conditions"`
	RetryPolicy *RetryPolicyExecution `json:"retry_policy"`
	OnFailure   *FailureAction        `json:"on_failure,omitempty"`
}

// TypedStageCondition represents a type-safe stage condition
type TypedStageCondition struct {
	Type        string            `json:"type"`
	Condition   string            `json:"condition"`
	Variables   map[string]string `json:"variables"`
	Key         string            `json:"key,omitempty"`
	Operator    string            `json:"operator,omitempty"`
	StringValue string            `json:"string_value,omitempty"`
	IntValue    *int64            `json:"int_value,omitempty"`
	BoolValue   *bool             `json:"bool_value,omitempty"`
	FloatValue  *float64          `json:"float_value,omitempty"`
}

// TypedExecutionSession represents a fully type-safe execution session
type TypedExecutionSession struct {
	SessionID                string              `json:"session_id"`
	ID                       string              `json:"id"` // Legacy field for compatibility
	WorkflowID               string              `json:"workflow_id"`
	WorkflowName             string              `json:"workflow_name"`
	Variables                *WorkflowVariables  `json:"variables"`
	Context                  *WorkflowContext    `json:"context"`
	StartTime                time.Time           `json:"start_time"`
	Status                   string              `json:"status"`
	CurrentStage             string              `json:"current_stage"`
	CompletedStages          []string            `json:"completed_stages"`
	FailedStages             []string            `json:"failed_stages"`
	SkippedStages            []string            `json:"skipped_stages"`
	SharedContext            *SharedWorkflowData `json:"shared_context"`
	ResourceBindings         *ResourceBindings   `json:"resource_bindings"`
	LastActivity             time.Time           `json:"last_activity"`
	StageResults             *StageResults       `json:"stage_results"`
	CreatedAt                time.Time           `json:"created_at"`
	UpdatedAt                time.Time           `json:"updated_at"`
	Checkpoints              []TypedCheckpoint   `json:"checkpoints"`
	ConsolidatedErrorContext *ErrorContextData   `json:"error_context,omitempty"`
	WorkflowVersion          string              `json:"workflow_version"`
	Labels                   map[string]string   `json:"labels"`
	EndTime                  *time.Time          `json:"end_time"`
}

// TypedCheckpoint represents a type-safe workflow checkpoint
type TypedCheckpoint struct {
	ID           string             `json:"id"`
	WorkflowID   string             `json:"workflow_id"`
	SessionID    string             `json:"session_id"`
	StageID      string             `json:"stage_id"`
	StageName    string             `json:"stage_name"`
	Timestamp    time.Time          `json:"timestamp"`
	State        *CheckpointState   `json:"state"`
	WorkflowSpec *TypedWorkflowSpec `json:"workflow_spec,omitempty"`
	SessionState *SessionStateData  `json:"session_state,omitempty"`
	StageResults *StageResults      `json:"stage_results,omitempty"`
	Message      string             `json:"message,omitempty"`
}

// CheckpointState represents typed checkpoint state
type CheckpointState struct {
	Variables        map[string]string `json:"variables,omitempty"`
	Progress         float64           `json:"progress,omitempty"`
	CurrentOperation string            `json:"current_operation,omitempty"`
	Artifacts        []string          `json:"artifacts,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// SessionStateData represents typed session state
type SessionStateData struct {
	Status           string            `json:"status"`
	CompletedStages  []string          `json:"completed_stages"`
	ActiveStages     []string          `json:"active_stages"`
	PendingStages    []string          `json:"pending_stages"`
	Variables        map[string]string `json:"variables,omitempty"`
	SharedData       map[string]string `json:"shared_data,omitempty"`
	ResourceBindings map[string]string `json:"resource_bindings,omitempty"`
}

// TypedWorkflowSpec represents a type-safe workflow specification
type TypedWorkflowSpec struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	Version    string                  `json:"version"`
	Stages     []TypedExecutionStage   `json:"stages"`
	Variables  *WorkflowVariables      `json:"variables"`
	APIVersion string                  `json:"apiVersion,omitempty"`
	Kind       string                  `json:"kind,omitempty"`
	Metadata   WorkflowMetadata        `json:"metadata,omitempty"`
	Spec       TypedWorkflowDefinition `json:"spec,omitempty"`
}

// TypedWorkflowDefinition represents a type-safe workflow definition
type TypedWorkflowDefinition struct {
	Stages      []TypedExecutionStage `json:"stages"`
	Variables   *WorkflowVariables    `json:"variables,omitempty"`
	ErrorPolicy *ErrorPolicy          `json:"error_policy,omitempty"`
	Timeout     time.Duration         `json:"timeout,omitempty"`
}

// TypedExecutionOption represents type-safe execution options
type TypedExecutionOption struct {
	Parallel   bool               `json:"parallel"`
	MaxRetries int                `json:"max_retries"`
	Timeout    time.Duration      `json:"timeout"`
	Variables  *WorkflowVariables `json:"variables"`
}

// TypedWorkflowResult represents a type-safe workflow result
type TypedWorkflowResult struct {
	Success         bool                                 `json:"success"`
	Results         map[string]*TypedStageResultEnhanced `json:"results"`
	Error           *WorkflowError                       `json:"error,omitempty"`
	Duration        time.Duration                        `json:"duration"`
	Artifacts       []TypedExecutionArtifact             `json:"artifacts"`
	SessionID       string                               `json:"session_id"`
	StagesCompleted int                                  `json:"stages_completed"`
}

// TypedExecutionArtifact represents a type-safe execution artifact
type TypedExecutionArtifact struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Path      string            `json:"path"`
	Size      int64             `json:"size"`
	Metadata  *ArtifactMetadata `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

// ArtifactMetadata represents typed artifact metadata
type ArtifactMetadata struct {
	ContentType string            `json:"content_type,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	Encoding    string            `json:"encoding,omitempty"`
	Compressed  bool              `json:"compressed"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// TypedStageResultEnhanced represents a fully type-safe stage result with enhanced metrics
type TypedStageResultEnhanced struct {
	StageID   string                   `json:"stage_id"`
	StageName string                   `json:"stage_name"`
	Success   bool                     `json:"success"`
	Error     *WorkflowError           `json:"error,omitempty"`
	Results   map[string]string        `json:"results"`
	Duration  time.Duration            `json:"duration"`
	Artifacts []TypedExecutionArtifact `json:"artifacts"`
	Metrics   *StageMetrics            `json:"metrics"`
}

// StageMetrics represents typed stage metrics
type StageMetrics struct {
	ExecutionTime  time.Duration     `json:"execution_time"`
	QueueTime      time.Duration     `json:"queue_time,omitempty"`
	RetryCount     int               `json:"retry_count"`
	ResourceUsage  *ResourceUsage    `json:"resource_usage,omitempty"`
	ToolExecutions map[string]int    `json:"tool_executions,omitempty"`
	CustomMetrics  map[string]string `json:"custom_metrics,omitempty"`
}

// ResourceUsage represents resource usage metrics
type ResourceUsage struct {
	CPUPercent   float64 `json:"cpu_percent,omitempty"`
	MemoryMB     int64   `json:"memory_mb,omitempty"`
	DiskReadMB   int64   `json:"disk_read_mb,omitempty"`
	DiskWriteMB  int64   `json:"disk_write_mb,omitempty"`
	NetworkInMB  int64   `json:"network_in_mb,omitempty"`
	NetworkOutMB int64   `json:"network_out_mb,omitempty"`
}

// TypedExecutionResult represents a type-safe execution result
type TypedExecutionResult struct {
	Success   bool                     `json:"success"`
	Results   map[string]string        `json:"results"`
	Artifacts []TypedExecutionArtifact `json:"artifacts"`
	Metrics   *StageMetrics            `json:"metrics"`
	Duration  time.Duration            `json:"duration"`
	Error     *ExecutionError          `json:"error,omitempty"`
}

// TypedVariableContext represents type-safe variable context
type TypedVariableContext struct {
	WorkflowVars    map[string]string `json:"workflow_vars"`
	StageVars       map[string]string `json:"stage_vars"`
	SessionContext  map[string]string `json:"session_context"`
	EnvironmentVars map[string]string `json:"environment_vars"`
	Secrets         map[string]string `json:"secrets"`
}

// ============================================================================
// Conversion Functions
// ============================================================================

// ConvertToTypedExecutionStage converts ExecutionStage to TypedExecutionStage
func ConvertToTypedExecutionStage(stage *ExecutionStage) *TypedExecutionStage {
	if stage == nil {
		return nil
	}

	typed := &TypedExecutionStage{
		ID:          stage.ID,
		Name:        stage.Name,
		Type:        stage.Type,
		Tools:       stage.Tools,
		DependsOn:   stage.DependsOn,
		Timeout:     stage.Timeout,
		MaxRetries:  stage.MaxRetries,
		Parallel:    stage.Parallel,
		RetryPolicy: stage.RetryPolicy,
		OnFailure:   stage.OnFailure,
	}

	// Convert variables
	if stage.TypedVariables != nil {
		typed.Variables = stage.TypedVariables
	} else if stage.Variables != nil {
		// Convert from map[string]interface{} to StageVariables
		typed.Variables = &StageVariables{
			InputVariables:  make(map[string]string),
			OutputVariables: make(map[string]string),
			Configuration:   make(map[string]string),
			Environment:     make(map[string]string),
		}
		for k, v := range stage.Variables {
			if str, ok := v.(string); ok {
				typed.Variables.InputVariables[k] = str
			}
		}
	}

	// Convert conditions
	typed.Conditions = make([]TypedStageCondition, 0, len(stage.Conditions))
	for _, cond := range stage.Conditions {
		typedCond := TypedStageCondition{
			Type:      cond.Type,
			Condition: cond.Condition,
			Key:       cond.Key,
			Operator:  cond.Operator,
			Variables: make(map[string]string),
		}

		// Convert variables
		for k, v := range cond.Variables {
			if str, ok := v.(string); ok {
				typedCond.Variables[k] = str
			}
		}

		// Convert value based on type
		switch val := cond.Value.(type) {
		case string:
			typedCond.StringValue = val
		case int, int32, int64:
			intVal := fmt.Sprintf("%v", val)
			if i, err := fmt.Sscanf(intVal, "%d", new(int64)); err == nil && i == 1 {
				v := new(int64)
				fmt.Sscanf(intVal, "%d", v)
				typedCond.IntValue = v
			}
		case float32, float64:
			floatVal := fmt.Sprintf("%v", val)
			if i, err := fmt.Sscanf(floatVal, "%f", new(float64)); err == nil && i == 1 {
				v := new(float64)
				fmt.Sscanf(floatVal, "%f", v)
				typedCond.FloatValue = v
			}
		case bool:
			typedCond.BoolValue = &val
		}

		typed.Conditions = append(typed.Conditions, typedCond)
	}

	return typed
}

// ConvertToTypedExecutionSession converts ExecutionSession to TypedExecutionSession
func ConvertToTypedExecutionSession(session *ExecutionSession) *TypedExecutionSession {
	if session == nil {
		return nil
	}

	typed := &TypedExecutionSession{
		SessionID:       session.SessionID,
		ID:              session.ID,
		WorkflowID:      session.WorkflowID,
		WorkflowName:    session.WorkflowName,
		StartTime:       session.StartTime,
		Status:          session.Status,
		CurrentStage:    session.CurrentStage,
		CompletedStages: session.CompletedStages,
		FailedStages:    session.FailedStages,
		SkippedStages:   session.SkippedStages,
		LastActivity:    session.LastActivity,
		CreatedAt:       session.CreatedAt,
		UpdatedAt:       session.UpdatedAt,
		WorkflowVersion: session.WorkflowVersion,
		Labels:          session.Labels,
		EndTime:         session.EndTime,
	}

	// Use typed fields if available
	if session.TypedVariables != nil {
		typed.Variables = session.TypedVariables
	}
	if session.TypedContext != nil {
		typed.Context = session.TypedContext
	}
	if session.TypedSharedContext != nil {
		typed.SharedContext = session.TypedSharedContext
	}
	if session.TypedResourceBindings != nil {
		typed.ResourceBindings = session.TypedResourceBindings
	}
	if session.TypedStageResults != nil {
		typed.StageResults = session.TypedStageResults
	}
	if session.TypedErrorContext != nil {
		typed.ConsolidatedErrorContext = session.TypedErrorContext
	}

	// Convert checkpoints
	typed.Checkpoints = make([]TypedCheckpoint, 0, len(session.Checkpoints))
	for _, cp := range session.Checkpoints {
		typed.Checkpoints = append(typed.Checkpoints, ConvertToTypedCheckpoint(cp))
	}

	return typed
}

// ConvertToTypedCheckpoint converts WorkflowCheckpoint to TypedCheckpoint
func ConvertToTypedCheckpoint(checkpoint WorkflowCheckpoint) TypedCheckpoint {
	typed := TypedCheckpoint{
		ID:         checkpoint.ID,
		WorkflowID: checkpoint.WorkflowID,
		SessionID:  checkpoint.SessionID,
		StageID:    checkpoint.StageID,
		StageName:  checkpoint.StageName,
		Timestamp:  checkpoint.Timestamp,
		Message:    checkpoint.Message,
	}

	// Convert state
	if checkpoint.State != nil {
		typed.State = &CheckpointState{
			Variables: make(map[string]string),
			Metadata:  make(map[string]string),
		}

		for k, v := range checkpoint.State {
			if str, ok := v.(string); ok {
				typed.State.Variables[k] = str
			}
		}
	}

	// Convert session state
	if checkpoint.SessionState != nil {
		typed.SessionState = &SessionStateData{
			Variables:        make(map[string]string),
			SharedData:       make(map[string]string),
			ResourceBindings: make(map[string]string),
		}

		if status, ok := checkpoint.SessionState["status"].(string); ok {
			typed.SessionState.Status = status
		}
	}

	return typed
}

// ConvertVariableContext converts VariableContext to TypedVariableContext
func ConvertVariableContext(ctx *VariableContext) *TypedVariableContext {
	if ctx == nil {
		return nil
	}

	typed := &TypedVariableContext{
		WorkflowVars:    ctx.WorkflowVars,
		EnvironmentVars: ctx.EnvironmentVars,
		Secrets:         ctx.Secrets,
		StageVars:       make(map[string]string),
		SessionContext:  make(map[string]string),
	}

	// Convert stage vars
	for k, v := range ctx.StageVars {
		if str, ok := v.(string); ok {
			typed.StageVars[k] = str
		}
	}

	// Convert session context
	for k, v := range ctx.SessionContext {
		if str, ok := v.(string); ok {
			typed.SessionContext[k] = str
		}
	}

	return typed
}

// ============================================================================
// Type-Safe Executor Interface
// ============================================================================

// TypedExecutor interface removed - use types.InternalContextTool instead

// TypedExecuteToolFunc is the signature for type-safe tool execution functions
type TypedExecuteToolFunc func(
	ctx context.Context,
	toolName string,
	stage *TypedExecutionStage,
	session *TypedExecutionSession,
) (*TypedToolResult, error)

// TypedToolResult represents a type-safe tool execution result
type TypedToolResult struct {
	Success   bool                     `json:"success"`
	Outputs   map[string]string        `json:"outputs"`
	Artifacts []TypedExecutionArtifact `json:"artifacts"`
	Metrics   map[string]string        `json:"metrics"`
	Error     string                   `json:"error,omitempty"`
}
