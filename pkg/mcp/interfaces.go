package mcp

import (
	"context"
	"fmt"
	"time"
)

// Package mcp defines core interfaces for the Model Context Protocol

// Tool represents the interface for MCP tools
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata represents tool metadata
type ToolMetadata struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Dependencies []string          `json:"dependencies"`
	Capabilities []string          `json:"capabilities"`
	Requirements []string          `json:"requirements"`
	Parameters   map[string]string `json:"parameters"`
	Examples     []ToolExample     `json:"examples"`
}

// ToolExample represents tool usage example
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// Session represents session management interface
type Session interface {
	// ID returns session identifier
	ID() string

	// GetWorkspace returns the workspace directory path
	GetWorkspace() string

	// UpdateState applies a function to update the session state
	UpdateState(func(*SessionState))
}

// SessionState represents session state
type SessionState struct {
	SessionID string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time

	WorkspaceDir string

	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo
	RepoURL            string

	DockerfileGenerated bool
	DockerfilePath      string
	ImageBuilt          bool
	ImageRef            string
	ImagePushed         bool

	ManifestsGenerated  bool
	ManifestPaths       []string
	DeploymentValidated bool

	CurrentStage string
	Status       string
	Stage        string
	Errors       []string
	Metadata     map[string]interface{}

	SecurityScan *SecurityScanResult
}

// RepositoryInfo represents repository information
type RepositoryInfo struct {
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework"`
	Dependencies []string               `json:"dependencies"`
	EntryPoint   string                 `json:"entry_point"`
	Port         int                    `json:"port"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// SecurityScanResult represents security scan results
type SecurityScanResult struct {
	HasVulnerabilities bool      `json:"has_vulnerabilities"`
	CriticalCount      int       `json:"critical_count"`
	HighCount          int       `json:"high_count"`
	MediumCount        int       `json:"medium_count"`
	LowCount           int       `json:"low_count"`
	Vulnerabilities    []string  `json:"vulnerabilities"`
	ScanTime           time.Time `json:"scan_time"`
}

// Transport represents MCP transport interface
type Transport interface {
	// Serve starts transport
	Serve(ctx context.Context) error

	// Stop stops transport
	Stop() error

	// Name returns transport name
	Name() string

	// SetHandler sets request handler
	SetHandler(handler RequestHandler)
}

// RequestHandler handles MCP requests
type RequestHandler interface {
	HandleRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error)
}

// MCPRequest represents MCP request
type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// MCPResponse represents MCP response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Orchestrator represents tool orchestration interface
type Orchestrator interface {
	ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
	RegisterTool(name string, tool Tool) error
}

// ToolRegistry manages tool registration
type ToolRegistry interface {
	Register(name string, factory ToolFactory) error
	Get(name string) (ToolFactory, error)
	List() []string
	GetMetadata() map[string]ToolMetadata
}

// StronglyTypedToolRegistry manages typed tool registration
type StronglyTypedToolRegistry interface {
	RegisterTyped(name string, factory StronglyTypedToolFactory[Tool]) error
	GetTyped(name string) (StronglyTypedToolFactory[Tool], error)
	List() []string
	GetMetadata() map[string]ToolMetadata
}

// StandardRegistry combines tool registration approaches
type StandardRegistry interface {
	ToolRegistry
	StronglyTypedToolRegistry

	RegisterStandard(name string, tool Tool) error

	GetTool(name string) (Tool, error)
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

// BaseToolResponse provides common fields for all tool responses
type BaseToolResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Errors  []string               `json:"errors,omitempty"`
}

// GetSuccess implements ToolResult interface
func (b BaseToolResponse) GetSuccess() bool {
	return b.Success
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

// =============================================================================
// PROGRESS REPORTING INTERFACE
// =============================================================================

// ProgressReporter provides stage-aware progress reporting
type ProgressReporter interface {
	ReportStage(stageProgress float64, message string)
	NextStage(message string)
	SetStage(stageIndex int, message string)
	ReportOverall(progress float64, message string)
	GetCurrentStage() (int, ProgressStage)
}

// ProgressStage represents a stage in a multi-step operation
type ProgressStage struct {
	Name        string  // Human-readable stage name
	Weight      float64 // Relative weight (0.0-1.0) of this stage in overall progress
	Description string  // Optional detailed description
}

// =============================================================================
// HEALTH CHECKING INTERFACE
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
