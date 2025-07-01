package tools

import (
	"context"
	"io"
)

// ToolParams is a constraint for tool parameter types
type ToolParams interface {
	Validate() error
	GetSessionID() string
}

// ToolResult is a constraint for tool result types
type ToolResult interface {
	IsSuccess() bool
}

// ParallelResults wraps multiple results from parallel execution
type ParallelResults[TResult ToolResult] struct {
	Results []TResult
	Success bool
}

// IsSuccess returns true if all parallel results are successful
func (p ParallelResults[TResult]) IsSuccess() bool {
	return p.Success
}

// Tool is the generic interface for all tools with strongly-typed parameters and results
type Tool[TParams ToolParams, TResult ToolResult] interface {
	// Execute runs the tool with the given parameters
	Execute(ctx context.Context, params TParams) (TResult, error)

	// GetName returns the tool's unique name
	GetName() string

	// GetDescription returns a human-readable description
	GetDescription() string

	// GetSchema returns the JSON schema for parameters
	GetSchema() Schema[TParams, TResult]
}

// ConfigurableTool extends Tool with configuration capabilities
type ConfigurableTool[TParams ToolParams, TResult ToolResult, TConfig any] interface {
	Tool[TParams, TResult]

	// Configure applies configuration to the tool
	Configure(config TConfig) error

	// GetConfig returns the current configuration
	GetConfig() TConfig
}

// StatefulTool extends Tool with state management
type StatefulTool[TParams ToolParams, TResult ToolResult, TState any] interface {
	Tool[TParams, TResult]

	// GetState returns the current tool state
	GetState() TState

	// SetState updates the tool state
	SetState(state TState) error

	// ResetState resets the tool to initial state
	ResetState() error
}

// StreamingTool extends Tool with streaming capabilities
type StreamingTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// ExecuteStream executes the tool and streams results
	ExecuteStream(ctx context.Context, params TParams, writer io.Writer) error

	// SupportsStreaming indicates if streaming is supported
	SupportsStreaming() bool
}

// BatchTool extends Tool with batch processing capabilities
type BatchTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// ExecuteBatch processes multiple parameter sets
	ExecuteBatch(ctx context.Context, paramsBatch []TParams) ([]BatchResult[TResult], error)

	// MaxBatchSize returns the maximum batch size
	MaxBatchSize() int
}

// BatchResult wraps a result with its index and error
type BatchResult[TResult ToolResult] struct {
	Index  int
	Result TResult
	Error  error
}

// AsyncTool extends Tool with asynchronous execution
type AsyncTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// ExecuteAsync starts asynchronous execution
	ExecuteAsync(ctx context.Context, params TParams) (JobID, error)

	// GetResult retrieves the result of an async job
	GetResult(ctx context.Context, jobID JobID) (TResult, error)

	// GetStatus returns the status of an async job
	GetStatus(ctx context.Context, jobID JobID) (JobStatus, error)

	// Cancel cancels an async job
	Cancel(ctx context.Context, jobID JobID) error
}

// JobID represents an asynchronous job identifier
type JobID string

// JobStatus represents the status of an async job
type JobStatus struct {
	ID        JobID
	State     JobState
	Progress  float64 // 0.0 to 1.0
	Message   string
	StartedAt int64
	UpdatedAt int64
}

// JobState represents the state of an async job
type JobState string

const (
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

// CacheableTool extends Tool with caching capabilities
type CacheableTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// CacheKey generates a cache key for the given parameters
	CacheKey(params TParams) string

	// CacheTTL returns the cache time-to-live
	CacheTTL() int64 // seconds

	// IsCacheable indicates if results should be cached
	IsCacheable(params TParams) bool
}

// RetryableTool extends Tool with retry capabilities
type RetryableTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// MaxRetries returns the maximum number of retries
	MaxRetries() int

	// RetryDelay returns the delay between retries in milliseconds
	RetryDelay() int

	// IsRetryable determines if an error is retryable
	IsRetryable(err error) bool
}

// ObservableTool extends Tool with observability features
type ObservableTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// GetMetrics returns tool-specific metrics
	GetMetrics() ToolMetrics

	// ResetMetrics resets all metrics
	ResetMetrics()
}

// ToolMetrics contains execution metrics for a tool
type ToolMetrics struct {
	TotalExecutions   int64
	SuccessfulRuns    int64
	FailedRuns        int64
	AverageLatency    float64 // milliseconds
	P95Latency        float64 // milliseconds
	P99Latency        float64 // milliseconds
	LastExecutionTime int64   // unix timestamp
}

// ComposableTool allows tools to be composed together
type ComposableTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// GetComposer returns a composer for chaining operations
	GetComposer() ToolComposer[TParams, TResult]
}

// ToolComposer handles tool composition operations
type ToolComposer[TParams ToolParams, TResult ToolResult] interface {
	// ChainWith creates a new tool that executes this tool followed by another
	ChainWith(next interface{}) interface{}

	// ParallelWith creates a new tool that executes multiple tools in parallel
	ParallelWith(tools ...Tool[TParams, TResult]) Tool[TParams, ParallelResults[TResult]]
}

// ValidatableTool extends Tool with enhanced validation
type ValidatableTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// ValidateParams performs deep parameter validation
	ValidateParams(params TParams) []ValidationError

	// ValidateResult validates the result before returning
	ValidateResult(result TResult) []ValidationError
}

// ValidationError represents a validation issue
type ValidationError struct {
	Field   string
	Message string
	Code    string
	Value   interface{}
}

// VersionedTool extends Tool with versioning support
type VersionedTool[TParams ToolParams, TResult ToolResult] interface {
	Tool[TParams, TResult]

	// GetVersion returns the tool version
	GetVersion() string

	// IsCompatible checks if params are compatible with this version
	IsCompatible(params TParams) bool

	// MigrateParams migrates params from an older version
	MigrateParams(fromVersion string, params TParams) (TParams, error)
}

// Schema represents the JSON schema for tool parameters and results
type Schema[TParams ToolParams, TResult ToolResult] struct {
	Name           string
	Description    string
	Version        string
	ParamsSchema   interface{} // JSON Schema for parameters
	ResultSchema   interface{} // JSON Schema for results
	Examples       []Example[TParams, TResult]
	Deprecated     bool
	DeprecationMsg string
}

// Example represents an example usage of a tool
type Example[TParams ToolParams, TResult ToolResult] struct {
	Name        string
	Description string
	Params      TParams
	Result      TResult
}
