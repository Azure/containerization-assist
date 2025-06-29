package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// ============================================================================
// BACKWARD COMPATIBILITY: Re-export core interfaces
// ============================================================================

// Core Tool Interface (re-exported from core)
type Tool = core.Tool
type ToolMetadata = core.ToolMetadata
type ToolExample = core.ToolExample

// Progress Reporting (re-exported from core)
type ProgressReporter = core.ProgressReporter
type ProgressToken = core.ProgressToken
type ProgressStage = core.ProgressStage

// Repository Analysis (re-exported from core)
type RepositoryAnalyzer = core.RepositoryAnalyzer
type RepositoryInfo = core.RepositoryInfo
type DockerfileInfo = core.DockerfileInfo
type HealthCheckInfo = core.HealthCheckInfo
type BuildRecommendations = core.BuildRecommendations

// Transport (re-exported from core)
type Transport = core.Transport
type RequestHandler = core.RequestHandler

// Tool Registry (re-exported from core)
type ToolRegistry = core.ToolRegistry

// Session Management (re-exported from core)
type SessionManager = core.SessionManager
type Session = core.Session
type SessionState = core.SessionState
type SecurityScanResult = core.SecurityScanResult
type VulnerabilityCount = core.VulnerabilityCount
type SecurityFinding = core.SecurityFinding

// MCP Protocol (re-exported from core)
type MCPRequest = core.MCPRequest
type MCPResponse = core.MCPResponse
type MCPError = core.MCPError

// Base Types (re-exported from core)
type BaseToolResponse = core.BaseToolResponse

// Server Types (re-exported from core)
type Server = core.Server
type ServerConfig = core.ServerConfig
type ConversationConfig = core.ConversationConfig
type ServerStats = core.ServerStats
type SessionManagerStats = core.SessionManagerStats
type WorkspaceStats = core.WorkspaceStats
type AlternativeStrategy = core.AlternativeStrategy
type ConversationStage = core.ConversationStage

// Constants (re-exported from core)
const (
	ConversationStagePreFlight  = core.ConversationStagePreFlight
	ConversationStageAnalyze    = core.ConversationStageAnalyze
	ConversationStageDockerfile = core.ConversationStageDockerfile
	ConversationStageBuild      = core.ConversationStageBuild
	ConversationStagePush       = core.ConversationStagePush
	ConversationStageManifests  = core.ConversationStageManifests
	ConversationStageDeploy     = core.ConversationStageDeploy
	ConversationStageScan       = core.ConversationStageScan
	ConversationStageCompleted  = core.ConversationStageCompleted
	ConversationStageError      = core.ConversationStageError
)

// ============================================================================
// LEGACY TYPES AND INTERFACES (maintained for compatibility during transition)
// ============================================================================

// Orchestrator represents tool orchestration interface
type Orchestrator interface {
	ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
	RegisterTool(name string, tool Tool) error
}

// StronglyTypedToolRegistry manages typed tool registration
type StronglyTypedToolRegistry interface {
	RegisterTyped(name string, factory StronglyTypedToolFactory[Tool]) error
	GetTyped(name string) (StronglyTypedToolFactory[Tool], error)
	List() []string
	GetAllMetadata() map[string]ToolMetadata
}

// StandardRegistry combines tool registration approaches
type StandardRegistry interface {
	ToolRegistry
	StronglyTypedToolRegistry

	RegisterStandard(name string, tool Tool) error
	GetToolInfo(name string) (*StandardToolInfo, error)
	IsRegistered(name string) bool
	Count() int
	Clear()
}

// StandardToolInfo represents tool information
type StandardToolInfo struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies"`
	Capabilities []string `json:"capabilities"`
}

// ToolFactory creates tool instances
type ToolFactory func() Tool

// StronglyTypedToolFactory creates typed tool instances
type StronglyTypedToolFactory[T Tool] interface {
	Create() T
	GetType() string
	GetMetadata() ToolMetadata
}

// TypedFactoryFunc represents typed factory function
type TypedFactoryFunc[T Tool] func() T

// ToolInstanceRegistry provides thread-safe storage for tool instances
type ToolInstanceRegistry interface {
	RegisterInstance(toolName string, factory ToolFactory) error
	GetInstance(toolName string) (Tool, error)
	ListTools() []string
	HasTool(toolName string) bool
}

// ToolExecutionRequest represents a tool execution request
type ToolExecutionRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ToolExecutionResult represents the result of tool execution
type ToolExecutionResult struct {
	Result   interface{}            `json:"result"`
	Error    error                  `json:"error"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ToolDispatcher manages tool instances and execution
type ToolDispatcher interface {
	RegisterTool(name string, factory ToolFactory, converter ArgConverter) error
	ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error)
	ListTools() []string
	GetToolsByCategory(category string) []string
}

// ToolOrchestrationExecutor handles tool execution within orchestration context
type ToolOrchestrationExecutor interface {
	ExecuteTool(ctx context.Context, request ToolExecutionRequest) (*ToolExecutionResult, error)
	GetDispatcher() ToolDispatcher
}

// NewStronglyTypedFactory creates typed factory
func NewStronglyTypedFactory[T Tool](factoryFunc TypedFactoryFunc[T], toolType string, metadata ToolMetadata) StronglyTypedToolFactory[T] {
	return &stronglyTypedFactory[T]{
		factoryFunc: factoryFunc,
		toolType:    toolType,
		metadata:    metadata,
	}
}

// stronglyTypedFactory implements StronglyTypedToolFactory
type stronglyTypedFactory[T Tool] struct {
	factoryFunc TypedFactoryFunc[T]
	toolType    string
	metadata    ToolMetadata
}

func (f *stronglyTypedFactory[T]) Create() T {
	return f.factoryFunc()
}

func (f *stronglyTypedFactory[T]) GetType() string {
	return f.toolType
}

func (f *stronglyTypedFactory[T]) GetMetadata() ToolMetadata {
	return f.metadata
}

// ToolArgs represents tool arguments
type ToolArgs interface {
	// GetSessionID returns session ID
	GetSessionID() string
	// Validate validates arguments
	Validate() error
}

// ToolResult is a marker interface for tool-specific result types
type ToolResult interface {
	// GetSuccess returns whether the tool execution was successful
	GetSuccess() bool
}

// BaseToolArgs provides common fields for all tool arguments
type BaseToolArgs struct {
	SessionID string `json:"session_id" jsonschema:"required,description=Unique identifier for the session"`
}

// GetSessionID implements ToolArgs interface
func (b BaseToolArgs) GetSessionID() string {
	return b.SessionID
}

// Validate implements ToolArgs interface
func (b BaseToolArgs) Validate() error {
	if b.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	return nil
}


// =============================================================================
// ERROR HANDLING INTERFACES
// =============================================================================

// RichError represents an enriched error with context - concrete implementation
type RichError struct {
	// Basic error information
	Code      string    `json:"code"`      // Error code (e.g., "BUILD_FAILED", "DEPLOY_ERROR")
	Message   string    `json:"message"`   // Human-readable error message
	Type      string    `json:"type"`      // Error type category
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Timestamp time.Time `json:"timestamp"` // When the error occurred

	// Context information
	Context     ErrorContext     `json:"context"`     // Rich context about the error
	Diagnostics ErrorDiagnostics `json:"diagnostics"` // Diagnostic information
	Resolution  ErrorResolution  `json:"resolution"`  // Suggested resolutions

	// Session and retry information
	SessionState   *SessionStateSnapshot `json:"session_state,omitempty"`
	Tool           string                `json:"tool"`
	AttemptNumber  int                   `json:"attempt_number"`
	PreviousErrors []string              `json:"previous_errors,omitempty"`
	Environment    map[string]string     `json:"environment,omitempty"`
}

// Error implements the error interface
func (e *RichError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Type, e.Message)
}

// GetCode returns the error code (for interface compatibility)
func (e *RichError) GetCode() string {
	return e.Code
}

// GetContext returns the error context (for interface compatibility)
func (e *RichError) GetContext() map[string]interface{} {
	if e.Context.Metadata == nil {
		return make(map[string]interface{})
	}
	return e.Context.Metadata.Custom
}

// GetSeverity returns the error severity (for interface compatibility)
func (e *RichError) GetSeverity() string {
	return e.Severity
}

// SystemState captures system state information
type SystemState struct {
	DockerAvailable bool   `json:"docker_available"`
	K8sConnected    bool   `json:"k8s_connected"`
	DiskSpaceMB     int64  `json:"disk_space_mb"`
	WorkspaceQuota  int64  `json:"workspace_quota_mb"`
	NetworkStatus   string `json:"network_status"`
}

// ErrorContext provides detailed context about where and why the error occurred
type ErrorContext struct {
	// Operation context
	Operation string `json:"operation"` // What operation was being performed
	Stage     string `json:"stage"`     // What stage of the operation
	Component string `json:"component"` // Which component failed

	// Input/output context
	Input         map[string]interface{} `json:"input,omitempty"`          // Input that caused the error
	PartialOutput map[string]interface{} `json:"partial_output,omitempty"` // Any partial results

	// System context
	SystemState   SystemState   `json:"system_state"`   // System state at error time
	ResourceUsage ResourceUsage `json:"resource_usage"` // Resource usage info

	// Additional context
	RelatedFiles []string       `json:"related_files,omitempty"` // Files involved in the error
	Logs         []LogEntry     `json:"logs,omitempty"`          // Relevant log entries
	Metadata     *ErrorMetadata `json:"metadata,omitempty"`      // Structured metadata
}

// ErrorDiagnostics provides diagnostic information for troubleshooting
type ErrorDiagnostics struct {
	// Error analysis
	RootCause    string `json:"root_cause"`    // Identified root cause
	ErrorPattern string `json:"error_pattern"` // Common error pattern identified
	Category     string `json:"category"`      // Error category

	// Diagnostic checks
	Checks   []DiagnosticCheck `json:"checks"`   // Diagnostic checks performed
	Symptoms []string          `json:"symptoms"` // Observed symptoms

	// Related information
	SimilarErrors []SimilarError `json:"similar_errors,omitempty"` // Similar past errors
	Documentation []string       `json:"documentation,omitempty"`  // Relevant docs
}

// ErrorResolution provides actionable resolution suggestions
type ErrorResolution struct {
	// Immediate actions
	ImmediateSteps []ResolutionStep `json:"immediate_steps"` // Steps to resolve now

	// Alternative approaches
	Alternatives []Alternative `json:"alternatives"` // Alternative approaches

	// Prevention
	Prevention []string `json:"prevention"` // How to prevent in future

	// Retry guidance
	RetryStrategy RetryStrategy `json:"retry_strategy"` // How/when to retry

	// Manual intervention
	ManualSteps []string `json:"manual_steps,omitempty"` // Manual steps if needed
}

// Supporting error types

// SessionStateSnapshot captures session state at error time
type SessionStateSnapshot struct {
	ID              string                 `json:"id"`
	CurrentStage    string                 `json:"current_stage"`
	CompletedStages []string               `json:"completed_stages"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ResourceUsage captures resource usage at error time
type ResourceUsage struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryMB         int64   `json:"memory_mb"`
	DiskUsageMB      int64   `json:"disk_usage_mb"`
	NetworkBandwidth string  `json:"network_bandwidth"`
}

// LogEntry represents a relevant log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
}

// DiagnosticCheck represents a diagnostic check performed
type DiagnosticCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SimilarError represents a similar past error
type SimilarError struct {
	Code       string    `json:"code"`
	Occurred   time.Time `json:"occurred"`
	Resolution string    `json:"resolution"`
	Successful bool      `json:"successful"`
}

// ResolutionStep represents a step to resolve the error
type ResolutionStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	ToolCall    string `json:"tool_call,omitempty"`
	Expected    string `json:"expected"`
}

// Alternative represents an alternative approach
type Alternative struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	TradeOffs   []string `json:"trade_offs"`
	Confidence  float64  `json:"confidence"`
}

// RetryStrategy defines how to retry after the error
type RetryStrategy struct {
	Recommended     bool          `json:"recommended"`
	WaitTime        time.Duration `json:"wait_time"`
	MaxAttempts     int           `json:"max_attempts"`
	BackoffStrategy string        `json:"backoff_strategy"`
	Conditions      []string      `json:"conditions"`
}

// ErrorMetadata provides structured error metadata
type ErrorMetadata struct {
	SessionID   string                 `json:"session_id"`
	Tool        string                 `json:"tool"`
	Operation   string                 `json:"operation"`
	ExecutionID string                 `json:"execution_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration"`
	RequestID   string                 `json:"request_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// AddCustom adds a custom metadata field
func (em *ErrorMetadata) AddCustom(key string, value interface{}) {
	if em.Custom == nil {
		em.Custom = make(map[string]interface{})
	}
	em.Custom[key] = value
}

// NewErrorMetadata creates a new ErrorMetadata instance
func NewErrorMetadata(sessionID, tool, operation string) *ErrorMetadata {
	return &ErrorMetadata{
		SessionID: sessionID,
		Tool:      tool,
		Operation: operation,
		Timestamp: time.Now(),
		Custom:    make(map[string]interface{}),
	}
}

// NewRichError creates a new rich error with basic information
func NewRichError(code, message, errorType string) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      errorType,
		Severity:  "medium",
		Timestamp: time.Now(),
		Context: ErrorContext{
			SystemState:   SystemState{},
			ResourceUsage: ResourceUsage{},
		},
		Diagnostics: ErrorDiagnostics{
			Checks:   make([]DiagnosticCheck, 0),
			Symptoms: make([]string, 0),
		},
		Resolution: ErrorResolution{
			ImmediateSteps: make([]ResolutionStep, 0),
			Alternatives:   make([]Alternative, 0),
			Prevention:     make([]string, 0),
		},
	}
}

// ErrorBuilder provides a fluent interface for building rich errors
type ErrorBuilder struct {
	error *RichError
}

// NewErrorBuilder creates a new error builder with basic information
func NewErrorBuilder(code, message, errorType string) *ErrorBuilder {
	return &ErrorBuilder{
		error: NewRichError(code, message, errorType),
	}
}

// WithSeverity sets the error severity
func (eb *ErrorBuilder) WithSeverity(severity string) *ErrorBuilder {
	eb.error.Severity = severity
	return eb
}

// WithOperation sets the operation context
func (eb *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	eb.error.Context.Operation = operation
	return eb
}

// WithStage sets the stage context
func (eb *ErrorBuilder) WithStage(stage string) *ErrorBuilder {
	eb.error.Context.Stage = stage
	return eb
}

// WithComponent sets the component context
func (eb *ErrorBuilder) WithComponent(component string) *ErrorBuilder {
	eb.error.Context.Component = component
	return eb
}

// WithRootCause sets the root cause in diagnostics
func (eb *ErrorBuilder) WithRootCause(rootCause string) *ErrorBuilder {
	eb.error.Diagnostics.RootCause = rootCause
	return eb
}

// WithImmediateStep adds an immediate resolution step
func (eb *ErrorBuilder) WithImmediateStep(order int, action, description string) *ErrorBuilder {
	step := ResolutionStep{
		Order:       order,
		Action:      action,
		Description: description,
	}
	eb.error.Resolution.ImmediateSteps = append(eb.error.Resolution.ImmediateSteps, step)
	return eb
}

// WithField adds a custom field to the error metadata
func (eb *ErrorBuilder) WithField(key string, value interface{}) *ErrorBuilder {
	if eb.error.Context.Metadata == nil {
		eb.error.Context.Metadata = NewErrorMetadata("", "", "")
	}
	eb.error.Context.Metadata.AddCustom(key, value)
	return eb
}

// Build returns the constructed RichError
func (eb *ErrorBuilder) Build() *RichError {
	return eb.error
}

// ValidationErrorBuilder provides specialized building for validation errors
type ValidationErrorBuilder struct {
	*ErrorBuilder
}

// NewValidationErrorBuilder creates a new validation error builder
func NewValidationErrorBuilder(message, field string, value interface{}) *ValidationErrorBuilder {
	builder := NewErrorBuilder("VALIDATION_ERROR", message, "validation_error")
	builder.WithField("field", field)
	builder.WithField("value", value)
	return &ValidationErrorBuilder{ErrorBuilder: builder}
}

// NewValidationError creates a validation error
func NewValidationError(message string) *RichError {
	return NewRichError("VALIDATION_ERROR", message, "validation_error")
}

// =============================================================================
// ADDITIONAL INTERFACES FOR COMPATIBILITY
// =============================================================================

// HealthChecker defines the interface for health checking operations
type HealthChecker interface {
	GetSystemResources() SystemResources
	GetSessionStats() SessionHealthStats
	GetCircuitBreakerStats() map[string]CircuitBreakerStatus
	CheckServiceHealth(ctx context.Context) []ServiceHealth
	GetJobQueueStats() JobQueueStats
	GetRecentErrors(limit int) []RecentError
}

// SystemResources represents system resource information
type SystemResources struct {
	CPUUsage    float64   `json:"cpu_usage_percent"`
	MemoryUsage float64   `json:"memory_usage_percent"`
	DiskUsage   float64   `json:"disk_usage_percent"`
	OpenFiles   int       `json:"open_files"`
	GoRoutines  int       `json:"goroutines"`
	HeapSize    int64     `json:"heap_size_bytes"`
	LastUpdated time.Time `json:"last_updated"`
}

// SessionHealthStats represents session-related health statistics
type SessionHealthStats struct {
	ActiveSessions    int     `json:"active_sessions"`
	TotalSessions     int     `json:"total_sessions"`
	FailedSessions    int     `json:"failed_sessions"`
	AverageSessionAge float64 `json:"average_session_age_minutes"`
	SessionErrors     int     `json:"session_errors_last_hour"`
}

// CircuitBreakerStatus represents the status of a circuit breaker
type CircuitBreakerStatus struct {
	State         string    `json:"state"` // open, closed, half-open
	FailureCount  int       `json:"failure_count"`
	LastFailure   time.Time `json:"last_failure"`
	NextRetry     time.Time `json:"next_retry"`
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
}

// ServiceHealth represents the health of an external service
type ServiceHealth struct {
	Name         string                 `json:"name"`
	Status       string                 `json:"status"` // healthy, degraded, unhealthy
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// JobQueueStats represents job queue statistics
type JobQueueStats struct {
	QueuedJobs      int     `json:"queued_jobs"`
	RunningJobs     int     `json:"running_jobs"`
	CompletedJobs   int64   `json:"completed_jobs"`
	FailedJobs      int64   `json:"failed_jobs"`
	AverageWaitTime float64 `json:"average_wait_time_seconds"`
}

// RecentError represents a recent error for debugging
type RecentError struct {
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Severity  string                 `json:"severity"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Circuit breaker states
const (
	CircuitBreakerClosed   = "closed"
	CircuitBreakerOpen     = "open"
	CircuitBreakerHalfOpen = "half-open"
)

// =============================================================================
// SESSION MANAGEMENT INTERFACES (from types package)
// =============================================================================

// ToolSessionManager interface for managing tool sessions
type ToolSessionManager interface {
	GetSession(sessionID string) (interface{}, error)
	GetSessionInterface(sessionID string) (interface{}, error)
	GetOrCreateSession(sessionID string) (interface{}, error)
	GetOrCreateSessionFromRepo(repoURL string) (interface{}, error)
	UpdateSession(sessionID string, updateFunc func(interface{})) error
	DeleteSession(ctx context.Context, sessionID string) error

	ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)
	FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error)
}

// SessionMetadata contains metadata about sessions
type SessionMetadata struct {
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	WorkspaceSize  int64     `json:"workspace_size"`
	OperationCount int       `json:"operation_count"`
	CurrentStage   string    `json:"current_stage"`
	Labels         []string  `json:"labels"`
}

// SessionData additional session data structure
type SessionData struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	CurrentStage string                 `json:"current_stage"`
	Metadata     map[string]interface{} `json:"metadata"`
	IsActive     bool                   `json:"is_active"`
	LastAccess   time.Time              `json:"last_access"`
}

// =============================================================================
// PIPELINE OPERATIONS INTERFACE (from types package)
// =============================================================================

// PipelineOperations interface for pipeline operations
type PipelineOperations interface {
	GetSessionWorkspace(sessionID string) string
	UpdateSessionFromDockerResults(sessionID string, result interface{}) error

	BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*BuildResult, error)
	PullDockerImage(sessionID, imageRef string) error
	PushDockerImage(sessionID, imageRef string) error
	TagDockerImage(sessionID, sourceRef, targetRef string) error
	ConvertToDockerState(sessionID string) (*DockerState, error)

	GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*KubernetesManifestResult, error)
	DeployToKubernetes(sessionID string, manifests []string) (*KubernetesDeploymentResult, error)
	CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*HealthCheckResult, error)

	AcquireResource(sessionID, resourceType string) error
	ReleaseResource(sessionID, resourceType string) error
}

// BuildResult different structure for pipeline operations
type BuildResult struct {
	ImageID  string      `json:"image_id"`
	ImageRef string      `json:"image_ref"`
	Success  bool        `json:"success"`
	Error    *BuildError `json:"error,omitempty"`
	Logs     string      `json:"logs,omitempty"`
}

// BuildError build-specific error
type BuildError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// DockerState docker state information
type DockerState struct {
	Images     []string `json:"images"`
	Containers []string `json:"containers"`
	Networks   []string `json:"networks"`
	Volumes    []string `json:"volumes"`
}

// HealthCheckResult different structure for deployment health
type HealthCheckResult struct {
	Healthy     bool              `json:"healthy"`
	Status      string            `json:"status"`
	PodStatuses []PodStatus       `json:"pod_statuses"`
	Error       *HealthCheckError `json:"error,omitempty"`
}

// PodStatus kubernetes pod status
type PodStatus struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// HealthCheckError health check specific error
type HealthCheckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// KubernetesManifestResult k8s manifest generation result
type KubernetesManifestResult struct {
	Success   bool                `json:"success"`
	Manifests []GeneratedManifest `json:"manifests"`
	Error     *RichError          `json:"error,omitempty"`
}

// GeneratedManifest generated k8s manifest
type GeneratedManifest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// KubernetesDeploymentResult k8s deployment result
type KubernetesDeploymentResult struct {
	Success     bool       `json:"success"`
	Namespace   string     `json:"namespace"`
	Deployments []string   `json:"deployments"`
	Services    []string   `json:"services"`
	Error       *RichError `json:"error,omitempty"`
}

// =============================================================================
// ITERATIVE FIXING INTERFACES (from types package)
// =============================================================================

// IterativeFixer interface for iterative fixing
type IterativeFixer interface {
	Fix(ctx context.Context, issue interface{}) (*FixingResult, error)
	AttemptFix(ctx context.Context, issue interface{}, attempt int) (*FixingResult, error)
	SetMaxAttempts(max int)
	GetFixHistory() []FixAttempt
	GetFailureRouting() map[string]string
	GetFixStrategies() []string
}

// FixingResult result of fix attempts
type FixingResult struct {
	Success         bool                   `json:"success"`
	Error           error                  `json:"error,omitempty"`
	FixApplied      string                 `json:"fix_applied"`
	Attempts        int                    `json:"attempts"`
	Duration        time.Duration          `json:"duration"`
	TotalDuration   time.Duration          `json:"total_duration"`
	TotalAttempts   int                    `json:"total_attempts"`
	FixHistory      []FixAttempt           `json:"fix_history"`
	AllAttempts     []FixAttempt           `json:"all_attempts"`
	FinalAttempt    *FixAttempt            `json:"final_attempt"`
	RecommendedNext []string               `json:"recommended_next"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// FixAttempt individual fix attempt details
type FixAttempt struct {
	AttemptNumber  int                    `json:"attempt_number"`
	Strategy       string                 `json:"strategy"`
	FixStrategy    FixStrategy            `json:"fix_strategy"`
	Error          error                  `json:"error,omitempty"`
	Success        bool                   `json:"success"`
	Duration       time.Duration          `json:"duration"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	AnalysisPrompt string                 `json:"analysis_prompt,omitempty"`
	AnalysisResult string                 `json:"analysis_result,omitempty"`
	Changes        []string               `json:"changes"`
	FixedContent   string                 `json:"fixed_content,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// FixStrategy strategy for fixing errors
type FixStrategy struct {
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	Type          string                             `json:"type"`
	Priority      int                                `json:"priority"`
	EstimatedTime time.Duration                      `json:"estimated_time"`
	Applicable    func(error) bool                   `json:"-"`
	Apply         func(context.Context, error) error `json:"-"`
	FileChanges   []FileChange                       `json:"file_changes,omitempty"`
	Commands      []string                           `json:"commands,omitempty"`
	Metadata      map[string]interface{}             `json:"metadata"`
}

// FileChange file change description
type FileChange struct {
	FilePath   string `json:"file_path"`
	Operation  string `json:"operation"`
	Content    string `json:"content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	Reason     string `json:"reason"`
}

// FixableOperation interface for fixable operations
type FixableOperation interface {
	ExecuteOnce(ctx context.Context) error
	GetFailureAnalysis(ctx context.Context, err error) (*RichError, error)
	PrepareForRetry(ctx context.Context, fixAttempt *FixAttempt) error
	Execute(ctx context.Context) error
	CanRetry() bool
	GetLastError() error
}

// =============================================================================
// AI ANALYSIS INTERFACES (from types package)
// =============================================================================

// AIAnalyzer interface for AI analysis
type AIAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() TokenUsage
	ResetTokenUsage()
}

// TokenUsage token usage tracking
type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// =============================================================================
// PROGRESS TRACKING INTERFACES (from types package)
// =============================================================================

// ProgressTracker different from ProgressReporter
type ProgressTracker interface {
	RunWithProgress(
		ctx context.Context,
		operation string,
		stages []LocalProgressStage,
		fn func(ctx context.Context, reporter interface{}) error,
	) error
}

// LocalProgressStage different from ProgressStage
type LocalProgressStage struct {
	Name        string
	Weight      float64
	Description string
}

// =============================================================================
// UTILITY INTERFACES (from types package)
// =============================================================================

// ContextSharer interface for sharing context
type ContextSharer interface {
	ShareContext(ctx context.Context, key string, value interface{}) error
	GetSharedContext(ctx context.Context, key string) (interface{}, bool)
}

// ArgConverter function type for argument conversion
type ArgConverter func(args map[string]interface{}) (interface{}, error)

// ResultConverter function type for result conversion
type ResultConverter func(result interface{}) (map[string]interface{}, error)

// =============================================================================
// MCP CLIENT TYPES (from types package)
// =============================================================================

// MCPClients contains client instances for external services
type MCPClients struct {
	Docker     interface{} `json:"-"`
	Kubernetes interface{} `json:"-"`
	Kind       interface{} `json:"-"`
	Registry   interface{} `json:"-"`
	Analyzer   interface{} `json:"-"`
	Kube       interface{} `json:"-"`
}

// =============================================================================
// ERROR CONSTANTS (from types package)
// =============================================================================

const (
	ErrorCodeInvalidRequest = -32600
)

// =============================================================================
// BASE TYPES FOR AI CONTEXT (from types package)
// =============================================================================

// BaseAIContextResult provides common fields for AI-powered tool results
type BaseAIContextResult struct {
	Context         string        `json:"context"`
	TokensUsed      int           `json:"tokens_used"`
	IsSuccessful    bool          `json:"is_successful"`
	Duration        time.Duration `json:"duration"`
	Recommendations []string      `json:"recommendations,omitempty"`
}

// NewBaseAIContextResult creates a new base AI context result
func NewBaseAIContextResult(context string, successful bool, duration time.Duration) BaseAIContextResult {
	return BaseAIContextResult{
		Context:      context,
		IsSuccessful: successful,
		Duration:     duration,
	}
}

// Recommendation represents a recommendation with confidence level
type Recommendation struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	Confidence  float64 `json:"confidence"`
	Rationale   string  `json:"rationale"`
}

// =============================================================================
// AI CONTEXT TYPES (from types package)
// =============================================================================

// AIContext provides context for AI-based analysis
type AIContext interface {
	GetAssessment() *UnifiedAssessment
	GenerateRecommendations() []Recommendation
	GetToolContext() *ToolContext
	GetMetadata() map[string]interface{}
}

// UnifiedAssessment represents a unified assessment result
type UnifiedAssessment struct {
	// Add fields as needed based on actual implementation
}

// ToolContext represents tool-specific context
type ToolContext struct {
	// Add fields as needed based on actual implementation
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NewGoMCPProgressAdapter creates a progress adapter for GoMCP integration
// This function signature is based on the implementation in internal/gomcp_progress_adapter.go
func NewGoMCPProgressAdapter(serverCtx interface{}, stages []LocalProgressStage) interface{} {
	// This is a placeholder that returns an interface{} to avoid import cycles
	// The actual implementation is in internal/gomcp_progress_adapter.go
	return nil
}

// NewMCPClients creates a new MCPClients instance
func NewMCPClients(docker, kind, kube interface{}) *MCPClients {
	return &MCPClients{
		Docker: docker,
		Kind:   kind,
		Kube:   kube,
	}
}

// =============================================================================
// ANALYSIS AND VALIDATION INTERFACES (from types package)
// =============================================================================

// ScoreCalculator interface for score calculation
type ScoreCalculator interface {
	CalculateScore(data interface{}) int
	DetermineRiskLevel(score int, factors map[string]interface{}) string
	CalculateConfidence(evidence []string) int
}

// TradeoffAnalyzer interface for tradeoff analysis
type TradeoffAnalyzer interface {
	AnalyzeTradeoffs(options []string, context map[string]interface{}) []TradeoffAnalysis
	CompareAlternatives(alternatives []AlternativeStrategy) *ComparisonMatrix
	RecommendBestOption(analysis []TradeoffAnalysis) *DecisionRecommendation
}

// TradeoffAnalysis represents tradeoff analysis
type TradeoffAnalysis struct {
	// Add fields as needed based on actual implementation
}

// ComparisonMatrix represents comparison matrix
type ComparisonMatrix struct {
	// Add fields as needed based on actual implementation
}

// DecisionRecommendation represents decision recommendation
type DecisionRecommendation struct {
	// Add fields as needed based on actual implementation
}

// BaseAnalysisOptions common options for analysis operations
type BaseAnalysisOptions struct {
	Depth                   string
	Aspects                 []string
	GenerateRecommendations bool
	CustomParams            map[string]interface{}
}

// BaseValidationOptions common options for validation operations
type BaseValidationOptions struct {
	Severity     string
	IgnoreRules  []string
	StrictMode   bool
	CustomParams map[string]interface{}
}

// BaseAnalysisResult common result structure for analysis
type BaseAnalysisResult struct {
	Summary         BaseAnalysisSummary
	Findings        []BaseFinding
	Recommendations []BaseRecommendation
	Metrics         map[string]interface{}
	RiskAssessment  BaseRiskAssessment
	Context         map[string]interface{}
	Metadata        BaseAnalysisMetadata
}

// BaseValidationResult common result structure for validation
type BaseValidationResult struct {
	IsValid bool
	Score   int

	Errors   []BaseValidationError
	Warnings []BaseValidationWarning

	TotalIssues    int
	CriticalIssues int

	Context  map[string]interface{}
	Metadata BaseValidationMetadata
}

// BaseAnalyzerCapabilities analyzer capabilities description
type BaseAnalyzerCapabilities struct {
	SupportedTypes   []string
	SupportedAspects []string
	RequiresContext  bool
	SupportsDeepScan bool
}

// BaseAnalysisSummary summary of analysis results
type BaseAnalysisSummary struct {
	TotalFindings    int
	CriticalFindings int
	Strengths        []string
	Weaknesses       []string
	OverallScore     int
}

// BaseFinding represents a finding from analysis
type BaseFinding struct {
	ID          string
	Type        string
	Category    string
	Severity    string
	Title       string
	Description string
	Evidence    []string
	Impact      string
	Location    BaseFindingLocation
}

// BaseFindingLocation location information for findings
type BaseFindingLocation struct {
	File      string
	Line      int
	Component string
	Context   string
}

// BaseRecommendation represents a recommendation
type BaseRecommendation struct {
	ID          string
	Priority    string
	Category    string
	Title       string
	Description string
	Benefits    []string
	Effort      string
	Impact      string
}

// BaseRiskAssessment represents risk assessment
type BaseRiskAssessment struct {
	OverallRisk string
	RiskFactors []BaseRiskFactor
	Mitigations []BaseMitigation
}

// BaseRiskFactor represents a risk factor
type BaseRiskFactor struct {
	ID          string
	Category    string
	Description string
	Likelihood  string
	Impact      string
	Score       int
}

// BaseMitigation represents a mitigation strategy
type BaseMitigation struct {
	RiskID        string
	Description   string
	Effort        string
	Effectiveness string
}

// BaseAnalysisMetadata metadata for analysis operations
type BaseAnalysisMetadata struct {
	AnalyzerName    string
	AnalyzerVersion string
	Duration        time.Duration
	Timestamp       time.Time
	Parameters      map[string]interface{}
}

// BaseValidationError represents a validation error
type BaseValidationError struct {
	Code          string
	Type          string
	Message       string
	Severity      string
	Location      BaseErrorLocation
	Fix           string
	Documentation string
}

// BaseValidationWarning represents a validation warning
type BaseValidationWarning struct {
	Code       string
	Type       string
	Message    string
	Suggestion string
	Impact     string
	Location   BaseWarningLocation
}

// BaseErrorLocation location information for errors
type BaseErrorLocation struct {
	File   string
	Line   int
	Column int
	Path   string
}

// BaseWarningLocation location information for warnings
type BaseWarningLocation struct {
	File string
	Line int
	Path string
}

// BaseValidationMetadata metadata for validation operations
type BaseValidationMetadata struct {
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}

// =============================================================================
// UTILITY FUNCTIONS (from types package)
// =============================================================================

// UpdateSessionHelper provides a generic helper for session updates
func UpdateSessionHelper[T any](manager ToolSessionManager, sessionID string, updater func(*T)) error {
	return manager.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*T); ok {
			updater(session)
		}
	})
}