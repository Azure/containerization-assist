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

// RichError represents an enriched error with context
type RichError interface {
	error
	Code() string
	Context() map[string]interface{}
	Severity() string
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
