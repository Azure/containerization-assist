package api

import (
	"context"
	"encoding/json"
	"time"
)

// TypedToolInput represents strongly typed tool input with generic context
type TypedToolInput[TData any, TContext any] struct {
	// SessionID identifies the session this tool execution belongs to
	SessionID string `json:"session_id"`

	// Data contains the tool-specific input parameters
	Data TData `json:"data"`

	// Context provides additional execution context
	Context TContext `json:"context,omitempty"`
}

// TypedToolOutput represents strongly typed tool output
type TypedToolOutput[TData any, TDetails any] struct {
	// Success indicates whether the tool execution succeeded
	Success bool `json:"success"`

	// Data contains the tool-specific output data
	Data TData `json:"data"`

	// Error contains any error message if Success is false
	Error string `json:"error,omitempty"`

	// Details provides additional execution details
	Details TDetails `json:"details,omitempty"`
}

// TypedTool is a fully type-safe tool interface
type TypedTool[TInputData, TInputContext, TOutputData, TOutputDetails any] interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Execute runs the tool with type-safe input and output
	Execute(ctx context.Context, input TypedToolInput[TInputData, TInputContext]) (TypedToolOutput[TOutputData, TOutputDetails], error)

	// Schema returns the strongly typed schema for the tool
	Schema() TypedToolSchema[TInputData, TInputContext, TOutputData, TOutputDetails]
}

// TypedToolSchema provides type-safe schema information
type TypedToolSchema[TInputData, TInputContext, TOutputData, TOutputDetails any] struct {
	// Name is the tool's unique identifier
	Name string `json:"name"`

	// Description explains what the tool does
	Description string `json:"description"`

	// Version indicates the tool's version
	Version string `json:"version"`

	// InputExample provides a sample input
	InputExample TypedToolInput[TInputData, TInputContext] `json:"input_example"`

	// OutputExample provides a sample output
	OutputExample TypedToolOutput[TOutputData, TOutputDetails] `json:"output_example"`

	// Tags categorizes the tool
	Tags []string `json:"tags,omitempty"`

	// Category groups related tools
	Category ToolCategory `json:"category,omitempty"`
}

// GetJSONSchema generates JSON schema from the typed schema
func (s TypedToolSchema[TI, TC, TO, TD]) GetJSONSchema() (input json.RawMessage, output json.RawMessage, err error) {
	// This would use reflection or code generation to create JSON schemas
	// from the Go types. For now, returning empty schemas.
	return json.RawMessage(`{}`), json.RawMessage(`{}`), nil
}

// ============================================================================
// Specialized Context Types
// ============================================================================

// ExecutionContext provides common execution context fields
type ExecutionContext struct {
	// RequestID uniquely identifies this request
	RequestID string `json:"request_id"`

	// UserID identifies the user making the request
	UserID string `json:"user_id,omitempty"`

	// TraceID for distributed tracing
	TraceID string `json:"trace_id,omitempty"`

	// Timeout for this execution
	Timeout time.Duration `json:"timeout,omitempty"`

	// Priority indicates execution priority
	Priority int `json:"priority,omitempty"`
}

// AnalysisContext provides context for analysis operations
type AnalysisContext struct {
	ExecutionContext

	// Branch to analyze
	Branch string `json:"branch,omitempty"`

	// CommitID to analyze
	CommitID string `json:"commit_id,omitempty"`

	// AnalysisDepth controls how deep to analyze
	AnalysisDepth int `json:"analysis_depth,omitempty"`
}

// BuildContext provides context for build operations
type BuildContext struct {
	ExecutionContext

	// Registry to push to
	Registry string `json:"registry,omitempty"`

	// CacheFrom specifies cache sources
	CacheFrom []string `json:"cache_from,omitempty"`

	// Labels to add to the image
	Labels map[string]string `json:"labels,omitempty"`
}

// DeployContext provides context for deployment operations
type DeployContext struct {
	ExecutionContext

	// Environment to deploy to
	Environment string `json:"environment,omitempty"`

	// RollbackOnFailure controls rollback behavior
	RollbackOnFailure bool `json:"rollback_on_failure"`

	// MaxRetries for deployment attempts
	MaxRetries int `json:"max_retries,omitempty"`
}

// ============================================================================
// Specialized Details Types
// ============================================================================

// ExecutionDetails provides common execution details
type ExecutionDetails struct {
	// Duration of the execution
	Duration time.Duration `json:"duration"`

	// StartTime when execution began
	StartTime time.Time `json:"start_time"`

	// EndTime when execution completed
	EndTime time.Time `json:"end_time"`

	// ResourcesUsed tracks resource consumption
	ResourcesUsed ResourceUsage `json:"resources_used,omitempty"`
}

// ResourceUsage tracks resource consumption
type ResourceUsage struct {
	// CPUTime in milliseconds
	CPUTime int64 `json:"cpu_time_ms"`

	// MemoryPeak in bytes
	MemoryPeak int64 `json:"memory_peak_bytes"`

	// NetworkIO in bytes
	NetworkIO int64 `json:"network_io_bytes"`

	// DiskIO in bytes
	DiskIO int64 `json:"disk_io_bytes"`
}

// AnalysisDetails provides details for analysis operations
type AnalysisDetails struct {
	ExecutionDetails

	// FilesScanned count
	FilesScanned int `json:"files_scanned"`

	// IssuesFound count
	IssuesFound int `json:"issues_found"`

	// CodeCoverage percentage
	CodeCoverage float64 `json:"code_coverage,omitempty"`
}

// BuildDetails provides details for build operations
type BuildDetails struct {
	ExecutionDetails

	// ImageSize in bytes
	ImageSize int64 `json:"image_size"`

	// LayerCount in the image
	LayerCount int `json:"layer_count"`

	// CacheHit indicates if cache was used
	CacheHit bool `json:"cache_hit"`

	// BuildSteps executed
	BuildSteps []BuildStep `json:"build_steps,omitempty"`
}

// BuildStep represents a single build step
type BuildStep struct {
	// Name of the step
	Name string `json:"name"`

	// Duration of the step
	Duration time.Duration `json:"duration"`

	// Success indicates if the step succeeded
	Success bool `json:"success"`

	// Error if the step failed
	Error string `json:"error,omitempty"`
}

// DeployDetails provides details for deployment operations
type DeployDetails struct {
	ExecutionDetails

	// ResourcesCreated lists created resources
	ResourcesCreated []string `json:"resources_created"`

	// ResourcesUpdated lists updated resources
	ResourcesUpdated []string `json:"resources_updated"`

	// ResourcesDeleted lists deleted resources
	ResourcesDeleted []string `json:"resources_deleted"`

	// RollbackPerformed indicates if rollback occurred
	RollbackPerformed bool `json:"rollback_performed"`
}
