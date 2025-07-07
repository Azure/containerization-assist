package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ExecutionStage represents a stage in workflow execution
type ExecutionStage struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Tools       []string               `json:"tools"`
	DependsOn   []string               `json:"depends_on"`
	Variables   map[string]interface{} `json:"variables"` // Deprecated: use TypedVariables
	Timeout     *time.Duration         `json:"timeout"`
	MaxRetries  int                    `json:"max_retries"`
	Parallel    bool                   `json:"parallel"`
	Conditions  []StageCondition       `json:"conditions"`
	RetryPolicy *RetryPolicyExecution  `json:"retry_policy"`
	OnFailure   *FailureAction         `json:"on_failure,omitempty"`

	// Type-safe alternative
	TypedVariables *StageVariables `json:"typed_variables,omitempty"`
}

// ExecutionSession represents an execution session
type ExecutionSession struct {
	SessionID                string                 `json:"session_id"`
	ID                       string                 `json:"id"` // Legacy field for compatibility
	WorkflowID               string                 `json:"workflow_id"`
	WorkflowName             string                 `json:"workflow_name"`
	Variables                map[string]interface{} `json:"variables,omitempty"` // Deprecated: use TypedVariables
	Context                  map[string]interface{} `json:"context,omitempty"`   // Deprecated: use TypedContext
	StartTime                time.Time              `json:"start_time"`
	Status                   string                 `json:"status"`
	CurrentStage             string                 `json:"current_stage"`
	CompletedStages          []string               `json:"completed_stages"`
	FailedStages             []string               `json:"failed_stages"`
	SkippedStages            []string               `json:"skipped_stages"`
	SharedContext            map[string]interface{} `json:"shared_context,omitempty"`    // Deprecated: use TypedSharedContext
	ResourceBindings         map[string]interface{} `json:"resource_bindings,omitempty"` // Deprecated: use TypedResourceBindings
	LastActivity             time.Time              `json:"last_activity"`
	StageResults             map[string]interface{} `json:"stage_results,omitempty"` // Deprecated: use TypedStageResults
	CreatedAt                time.Time              `json:"created_at"`
	UpdatedAt                time.Time              `json:"updated_at"`
	Checkpoints              []WorkflowCheckpoint   `json:"checkpoints"`
	ConsolidatedErrorContext map[string]interface{} `json:"error_context,omitempty"` // Deprecated: use TypedErrorContext

	// Type-safe alternatives
	TypedVariables        *WorkflowVariables  `json:"typed_variables,omitempty"`
	TypedContext          *WorkflowContext    `json:"typed_context,omitempty"`
	TypedSharedContext    *SharedWorkflowData `json:"typed_shared_context,omitempty"`
	TypedResourceBindings *ResourceBindings   `json:"typed_resource_bindings,omitempty"`
	TypedStageResults     *StageResults       `json:"typed_stage_results,omitempty"`
	TypedErrorContext     *ErrorContextData   `json:"typed_error_context,omitempty"`
	WorkflowVersion       string              `json:"workflow_version"`
	Labels                map[string]string   `json:"labels"`
	EndTime               *time.Time          `json:"end_time"`
}

// ExecutionArtifact represents an artifact from execution
type ExecutionArtifact struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Path      string                 `json:"path"`
	Size      int64                  `json:"size"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
}

// Legacy workflow types for backward compatibility
type WorkflowSession = ExecutionSession
type WorkflowStage = ExecutionStage
type WorkflowStatus = string
type WorkflowSpec struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Version    string                 `json:"version"`
	Stages     []ExecutionStage       `json:"stages"`
	Variables  map[string]interface{} `json:"variables"`
	APIVersion string                 `json:"apiVersion,omitempty"`
	Kind       string                 `json:"kind,omitempty"`
	Metadata   WorkflowMetadata       `json:"metadata,omitempty"`
	Spec       WorkflowDefinition     `json:"spec,omitempty"`
}

type WorkflowCheckpoint struct {
	ID           string                 `json:"id"`
	WorkflowID   string                 `json:"workflow_id"`
	SessionID    string                 `json:"session_id"`
	StageID      string                 `json:"stage_id"`
	StageName    string                 `json:"stage_name"`
	Timestamp    time.Time              `json:"timestamp"`
	State        map[string]interface{} `json:"state"`
	WorkflowSpec *WorkflowSpec          `json:"workflow_spec,omitempty"`
	SessionState map[string]interface{} `json:"session_state,omitempty"`
	StageResults map[string]interface{} `json:"stage_results,omitempty"`
	Message      string                 `json:"message,omitempty"`
}

type WorkflowError struct {
	ID         string    `json:"id"`
	Message    string    `json:"message"`
	Code       string    `json:"code"`
	Type       string    `json:"type"`
	ErrorType  string    `json:"error_type"`
	Severity   string    `json:"severity"`
	Retryable  bool      `json:"retryable"`
	StageName  string    `json:"stage_name"`
	ToolName   string    `json:"tool_name"`
	Timestamp  time.Time `json:"timestamp"`
	RootCauses []string  `json:"root_causes,omitempty"`
}

type Engine struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewEngine creates a new workflow engine
func NewEngine() *Engine {
	return &Engine{
		Name:    "workflow-engine",
		Version: "1.0.0",
	}
}

// ExecuteWorkflow executes a workflow (stub implementation)
func (e *Engine) ExecuteWorkflow(ctx context.Context, spec *WorkflowSpec, options ...ExecutionOption) (*WorkflowResult, error) {
	return &WorkflowResult{
		Success: true,
		Results: make(map[string]interface{}),
	}, nil
}

// ValidateWorkflow validates a workflow specification
func (e *Engine) ValidateWorkflow(spec *WorkflowSpec) error {
	return nil
}

// PauseWorkflow pauses a running workflow
func (e *Engine) PauseWorkflow(sessionID string) error {
	return nil
}

// ResumeWorkflow resumes a paused workflow
func (e *Engine) ResumeWorkflow(ctx context.Context, sessionID string, spec *WorkflowSpec) (*WorkflowResult, error) {
	return &WorkflowResult{
		Success:   true,
		Results:   make(map[string]interface{}),
		SessionID: sessionID,
	}, nil
}

// CancelWorkflow cancels a running workflow
func (e *Engine) CancelWorkflow(sessionID string) error {
	return nil
}

// Additional legacy types
type StageCondition struct {
	Type      string                 `json:"type"`
	Condition string                 `json:"condition"`
	Variables map[string]interface{} `json:"variables"`
	Key       string                 `json:"key,omitempty"`
	Operator  string                 `json:"operator,omitempty"`
	Value     interface{}            `json:"value,omitempty"`
}

type ExecutionOption struct {
	Parallel   bool                   `json:"parallel"`
	MaxRetries int                    `json:"max_retries"`
	Timeout    time.Duration          `json:"timeout"`
	Variables  map[string]interface{} `json:"variables"`
}

type WorkflowResult struct {
	Success         bool                   `json:"success"`
	Status          string                 `json:"status"`
	Message         string                 `json:"message"`
	Results         map[string]interface{} `json:"results"`
	Error           *WorkflowError         `json:"error,omitempty"`
	Duration        time.Duration          `json:"duration"`
	Artifacts       []ExecutionArtifact    `json:"artifacts"`
	SessionID       string                 `json:"session_id"`
	StagesExecuted  int                    `json:"stages_executed"`
	StagesCompleted int                    `json:"stages_completed"`
	StagesFailed    int                    `json:"stages_failed"`
}

type SessionFilter struct {
	Status       string            `json:"status,omitempty"`
	WorkflowID   string            `json:"workflow_id,omitempty"`
	WorkflowName string            `json:"workflow_name,omitempty"`
	StartAfter   time.Time         `json:"start_after,omitempty"`
	StartTime    *time.Time        `json:"start_time,omitempty"`
	EndTime      *time.Time        `json:"end_time,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Offset       int               `json:"offset,omitempty"`
	Limit        int               `json:"limit,omitempty"`
}

type StageResult struct {
	StageID   string                 `json:"stage_id"`
	StageName string                 `json:"stage_name"`
	Success   bool                   `json:"success"`
	Error     *WorkflowError         `json:"error,omitempty"`
	Results   map[string]interface{} `json:"results"`
	Duration  time.Duration          `json:"duration"`
	Artifacts []ExecutionArtifact    `json:"artifacts"`
	Metrics   map[string]interface{} `json:"metrics"`
}

type WorkflowSpecWorkflowStage = ExecutionStage
type WorkflowArtifact = ExecutionArtifact

// Workflow status constants
const (
	WorkflowStatusPending   = "pending"
	WorkflowStatusRunning   = "running"
	WorkflowStatusDone      = "done"
	WorkflowStatusFailed    = "failed"
	WorkflowStatusPaused    = "paused"
	WorkflowStatusCompleted = "completed"
	WorkflowStatusCancelled = "cancelled"
)

// RetryPolicyExecution defines retry behavior for execution stages
type RetryPolicyExecution struct {
	MaxAttempts  int           `json:"max_attempts"`
	Delay        time.Duration `json:"delay"`
	BackoffType  string        `json:"backoff_type"`
	BackoffMode  string        `json:"backoff_mode"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	Multiplier   float64       `json:"multiplier"`
}

// Workflow-related types for examples
type WorkflowMetadata struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type WorkflowDefinition struct {
	Stages      []WorkflowStage        `json:"stages"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	ErrorPolicy *ErrorPolicy           `json:"error_policy,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
}

type ErrorPolicy struct {
	Action      string         `json:"action"`
	Rules       []ErrorRule    `json:"rules,omitempty"`
	Mode        string         `json:"mode,omitempty"`
	MaxFailures int            `json:"max_failures,omitempty"`
	Routing     []ErrorRouting `json:"routing,omitempty"`
}

type ErrorRouting struct {
	Default    string `json:"default,omitempty"`
	FromTool   string `json:"from_tool,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`
	Action     string `json:"action,omitempty"`
	RedirectTo string `json:"redirect_to,omitempty"`
}

type ErrorRule struct {
	Type   string `json:"type"`
	Action string `json:"action"`
}

type FailureAction struct {
	Action     string                `json:"action"`
	Retry      *RetryPolicyExecution `json:"retry,omitempty"`
	RedirectTo string                `json:"redirect_to,omitempty"`
}

// ExecuteToolFunc is the signature for tool execution functions
type ExecuteToolFunc func(
	ctx context.Context,
	toolName string,
	stage *ExecutionStage,
	session *ExecutionSession,
) (interface{}, error)

// ExecutionResult represents the result of executing tools
type ExecutionResult struct {
	Success   bool                   `json:"success"`
	Results   map[string]interface{} `json:"results"`
	Artifacts []ExecutionArtifact    `json:"artifacts"`
	Metrics   map[string]interface{} `json:"metrics"`
	Duration  time.Duration          `json:"duration"`
	Error     *ExecutionError        `json:"error,omitempty"`
}

// ExecutionError provides detailed error information
type ExecutionError struct {
	ToolName string `json:"tool_name"`
	Index    int    `json:"index"`
	Error    error  `json:"error"`
	Type     string `json:"type"`
}

// Executor interface removed - use types.InternalBatchTool instead

// Option functions for ExecutionOption
func WithVariables(vars map[string]interface{}) ExecutionOption {
	return ExecutionOption{Variables: vars}
}

func WithCreateCheckpoints(enable bool) ExecutionOption {
	return ExecutionOption{}
}

func WithEnableParallel(enable bool) ExecutionOption {
	return ExecutionOption{Parallel: enable}
}

func WithMaxRetries(retries int) ExecutionOption {
	return ExecutionOption{MaxRetries: retries}
}

func WithTimeout(timeout time.Duration) ExecutionOption {
	return ExecutionOption{Timeout: timeout}
}

// VariableContext contains variables available for expansion
type VariableContext struct {
	WorkflowVars    map[string]string      `json:"workflow_vars"`
	StageVars       map[string]interface{} `json:"stage_vars"`
	SessionContext  map[string]interface{} `json:"session_context"`
	EnvironmentVars map[string]string      `json:"environment_vars"`
	Secrets         map[string]string      `json:"secrets"`
}

// VariableResolver handles variable expansion
type VariableResolver struct {
	logger zerolog.Logger
}

// NewVariableResolver creates a new variable resolver
func NewVariableResolver(logger zerolog.Logger) *VariableResolver {
	return &VariableResolver{
		logger: logger.With().Str("component", "variable_resolver").Logger(),
	}
}

// Expand expands variables in the given string using the provided context
func (vr *VariableResolver) Expand(input string, context *VariableContext) string {
	// Simple variable expansion implementation
	result := input

	// Replace workflow variables
	for key, value := range context.WorkflowVars {
		placeholder := fmt.Sprintf("${%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Replace stage variables
	for key, value := range context.StageVars {
		if strValue, ok := value.(string); ok {
			placeholder := fmt.Sprintf("${%s}", key)
			result = strings.ReplaceAll(result, placeholder, strValue)
		}
	}

	// Replace session context variables
	for key, value := range context.SessionContext {
		if strValue, ok := value.(string); ok {
			placeholder := fmt.Sprintf("${%s}", key)
			result = strings.ReplaceAll(result, placeholder, strValue)
		}
	}

	// Replace environment variables
	for key, value := range context.EnvironmentVars {
		placeholder := fmt.Sprintf("${%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// ResolveVariables is an alias for Expand for backward compatibility
func (vr *VariableResolver) ResolveVariables(input string, context *VariableContext) string {
	return vr.Expand(input, context)
}

// ============================================================================
// Type-safe structures for ExecutionSession
// ============================================================================

// WorkflowVariables represents typed workflow variables
type WorkflowVariables struct {
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	ConfigValues   map[string]string `json:"config_values,omitempty"`
	UserParameters map[string]string `json:"user_parameters,omitempty"`
}

// WorkflowContext represents typed workflow context
type WorkflowContext struct {
	SessionID    string            `json:"session_id"`
	WorkspaceDir string            `json:"workspace_dir"`
	Repository   string            `json:"repository,omitempty"`
	Branch       string            `json:"branch,omitempty"`
	ImageRef     string            `json:"image_ref,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
}

// SharedWorkflowData represents typed shared context data
type SharedWorkflowData struct {
	AnalysisResults   *AnalysisData     `json:"analysis_results,omitempty"`
	BuildResults      *BuildData        `json:"build_results,omitempty"`
	DeploymentResults *DeploymentData   `json:"deployment_results,omitempty"`
	SecurityResults   *SecurityData     `json:"security_results,omitempty"`
	CustomData        map[string]string `json:"custom_data,omitempty"`
}

// AnalysisData represents analysis stage results
type AnalysisData struct {
	RepositoryPath string            `json:"repository_path"`
	Language       string            `json:"language"`
	Framework      string            `json:"framework"`
	Dependencies   map[string]string `json:"dependencies,omitempty"`
	EntryPoint     string            `json:"entry_point,omitempty"`
	Port           int               `json:"port,omitempty"`
}

// BuildData represents build stage results
type BuildData struct {
	ImageRef   string            `json:"image_ref"`
	ImageID    string            `json:"image_id"`
	BuildTime  time.Duration     `json:"build_time"`
	Tags       []string          `json:"tags,omitempty"`
	Platform   string            `json:"platform,omitempty"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	LayerCount int               `json:"layer_count,omitempty"`
}

// DeploymentData represents deployment stage results
type DeploymentData struct {
	Namespace     string   `json:"namespace"`
	AppName       string   `json:"app_name"`
	ManifestPaths []string `json:"manifest_paths"`
	Resources     []string `json:"resources"`
	ServiceURL    string   `json:"service_url,omitempty"`
	ReadyReplicas int      `json:"ready_replicas"`
	TotalReplicas int      `json:"total_replicas"`
}

// SecurityData represents security scan results
type SecurityData struct {
	ScanTime         time.Time `json:"scan_time"`
	TotalFindings    int       `json:"total_findings"`
	CriticalFindings int       `json:"critical_findings"`
	HighFindings     int       `json:"high_findings"`
	MediumFindings   int       `json:"medium_findings"`
	LowFindings      int       `json:"low_findings"`
	ScanReport       string    `json:"scan_report,omitempty"`
	HasSecrets       bool      `json:"has_secrets"`
}

// ResourceBindings represents typed resource bindings
type ResourceBindings struct {
	ImageBindings     map[string]string `json:"image_bindings,omitempty"`
	NamespaceBindings map[string]string `json:"namespace_bindings,omitempty"`
	ConfigMapBindings map[string]string `json:"config_map_bindings,omitempty"`
	SecretBindings    map[string]string `json:"secret_bindings,omitempty"`
	VolumeBindings    map[string]string `json:"volume_bindings,omitempty"`
}

// StageResults represents typed stage execution results
type StageResults struct {
	Results map[string]*TypedStageResult `json:"results"`
}

// TypedStageResult represents a single stage execution result with typed fields
type TypedStageResult struct {
	StageID   string            `json:"stage_id"`
	Status    string            `json:"status"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Duration  time.Duration     `json:"duration"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Outputs   map[string]string `json:"outputs,omitempty"`
	Metrics   map[string]string `json:"metrics,omitempty"`
}

// ErrorContextData represents typed error context
type ErrorContextData struct {
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	StageID      string            `json:"stage_id,omitempty"`
	ToolName     string            `json:"tool_name,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
	StackTrace   string            `json:"stack_trace,omitempty"`
	Context      map[string]string `json:"context,omitempty"`
}

// StageVariables represents typed stage variables
type StageVariables struct {
	InputVariables  map[string]string `json:"input_variables,omitempty"`
	OutputVariables map[string]string `json:"output_variables,omitempty"`
	Configuration   map[string]string `json:"configuration,omitempty"`
	Environment     map[string]string `json:"environment,omitempty"`
}
