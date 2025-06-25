package mcp

import (
	"context"
	"fmt"
	"time"
)

// Unified MCP Interfaces - Single Source of Truth
// This file consolidates all MCP interfaces as specified in REORG.md

// =============================================================================
// CORE TOOL INTERFACE
// =============================================================================

// Tool represents the unified interface for all MCP tools
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata contains comprehensive information about a tool
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

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// =============================================================================
// SESSION INTERFACE
// =============================================================================

// Session represents the unified interface for session management
type Session interface {
	// ID returns the unique session identifier
	ID() string

	// GetWorkspace returns the workspace directory path
	GetWorkspace() string

	// UpdateState applies a function to update the session state
	UpdateState(func(*SessionState))
}

// SessionState represents the current state of a session
type SessionState struct {
	SessionID string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time

	// Workspace
	WorkspaceDir string

	// Repository state
	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo
	RepoURL            string

	// Build state
	DockerfileGenerated bool
	DockerfilePath      string
	ImageBuilt          bool
	ImageRef            string
	ImagePushed         bool

	// Deployment state
	ManifestsGenerated  bool
	ManifestPaths       []string
	DeploymentValidated bool

	// Progress tracking
	CurrentStage string
	Status       string
	Stage        string
	Errors       []string
	Metadata     map[string]interface{}

	// Security
	SecurityScan *SecurityScanResult
}

// RepositoryInfo contains repository analysis information
type RepositoryInfo struct {
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework"`
	Dependencies []string               `json:"dependencies"`
	EntryPoint   string                 `json:"entry_point"`
	Port         int                    `json:"port"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// SecurityScanResult contains security scan information
type SecurityScanResult struct {
	HasVulnerabilities bool      `json:"has_vulnerabilities"`
	CriticalCount      int       `json:"critical_count"`
	HighCount          int       `json:"high_count"`
	MediumCount        int       `json:"medium_count"`
	LowCount           int       `json:"low_count"`
	Vulnerabilities    []string  `json:"vulnerabilities"`
	ScanTime           time.Time `json:"scan_time"`
}

// =============================================================================
// TRANSPORT INTERFACE
// =============================================================================

// Transport represents the unified interface for MCP transport mechanisms
type Transport interface {
	// Serve starts the transport and serves requests
	Serve(ctx context.Context) error

	// Stop gracefully stops the transport
	Stop() error

	// Name returns the transport name
	Name() string

	// SetHandler sets the request handler
	SetHandler(handler RequestHandler)
}

// RequestHandler processes MCP requests
type RequestHandler interface {
	HandleRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error)
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// =============================================================================
// ORCHESTRATOR INTERFACE
// =============================================================================

// Orchestrator defines the unified interface for tool orchestration
type Orchestrator interface {
	ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
	RegisterTool(name string, tool Tool) error
}

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	Register(name string, factory ToolFactory) error
	Get(name string) (ToolFactory, error)
	List() []string
	GetMetadata() map[string]ToolMetadata
}

// ToolFactory creates new instances of tools
type ToolFactory func() Tool

// =============================================================================
// TOOL ARGUMENT AND RESULT INTERFACES
// =============================================================================

// ToolArgs is a marker interface for tool-specific argument types
type ToolArgs interface {
	// GetSessionID returns the session ID for this tool execution
	GetSessionID() string
	// Validate validates the arguments
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
