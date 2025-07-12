// Package api provides the single source of truth for all MCP interfaces.
// This package consolidates all interface definitions to prevent duplication and
// ensure consistency across the codebase.
package api

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// ============================================================================
// Core Tool Interfaces
// ============================================================================

var (
	// ErrorInvalidInput indicates invalid input
	ErrorInvalidInput = errors.New(errors.CodeValidationFailed, "api", "invalid input", nil)
)

// Simple validation result types for core package compatibility
type BuildValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`
}

type ManifestValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Tool is the canonical interface for all MCP tools.
// This is the single source of truth, replacing all other Tool interface definitions.
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Execute runs the tool with the given input
	Execute(ctx context.Context, input ToolInput) (ToolOutput, error)

	// Schema returns the JSON schema for the tool's parameters and results
	Schema() ToolSchema
}

// ToolInput represents the canonical input structure for all tools
type ToolInput struct {
	// SessionID identifies the session this tool execution belongs to
	SessionID string `json:"session_id"`

	// Data contains the tool-specific input parameters
	Data map[string]interface{} `json:"data"`

	// Context provides additional execution context
	Context map[string]interface{} `json:"context,omitempty"`
}

// GetSessionID implements compatibility with ToolInputConstraint
func (t *ToolInput) GetSessionID() string {
	return t.SessionID
}

// Validate implements basic validation
func (t *ToolInput) Validate() error {
	if t.SessionID == "" {
		return ErrorInvalidInput
	}
	return nil
}

// GetContext returns execution context for compatibility
func (t *ToolInput) GetContext() map[string]interface{} {
	if t.Context == nil {
		return make(map[string]interface{})
	}
	return t.Context
}

// ToolOutput represents the canonical output structure for all tools
type ToolOutput struct {
	// Success indicates if the tool execution was successful
	Success bool `json:"success"`

	// Data contains the tool-specific output
	Data map[string]interface{} `json:"data"`

	// Error contains any error message if Success is false
	Error string `json:"error,omitempty"`

	// Metadata contains additional information about the execution
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IsSuccess implements compatibility with ToolOutputConstraint
func (t *ToolOutput) IsSuccess() bool {
	return t.Success
}

// GetData implements compatibility with ToolOutputConstraint
func (t *ToolOutput) GetData() interface{} {
	return t.Data
}

// GetError implements compatibility with ToolOutputConstraint
func (t *ToolOutput) GetError() string {
	return t.Error
}

// ToolSchema represents the schema definition for a tool
type ToolSchema struct {
	// Name is the tool name
	Name string `json:"name"`

	// Description describes what the tool does
	Description string `json:"description"`

	// InputSchema defines the expected input structure
	InputSchema map[string]interface{} `json:"input_schema"`

	// OutputSchema defines the output structure
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`

	// Examples provides usage examples
	Examples []ToolExample `json:"examples,omitempty"`

	// Tags categorizes the tool
	Tags []string `json:"tags,omitempty"`

	// Category groups related tools
	Category ToolCategory `json:"category,omitempty"`

	// Deprecated indicates if this tool is deprecated
	Deprecated bool `json:"deprecated,omitempty"`

	// Status indicates the operational status
	Status ToolStatus `json:"status,omitempty"`

	// Documentation provides links to documentation
	Documentation string `json:"documentation,omitempty"`

	// Version indicates the tool's version
	Version string `json:"version,omitempty"`
}

// ToolExample demonstrates how to use a tool
type ToolExample struct {
	// Name identifies this example
	Name string `json:"name"`

	// Description explains what this example demonstrates
	Description string `json:"description"`

	// Input shows example input data
	Input ToolInput `json:"input"`

	// Output shows expected output
	Output ToolOutput `json:"output"`
}

// ============================================================================
// Registry Interfaces
// ============================================================================

// Registry is the canonical interface for tool registration and management.
// This consolidates all Registry interface variants.
type Registry interface {
	// Register adds a tool to the registry
	Register(tool Tool, opts ...RegistryOption) error

	// Unregister removes a tool from the registry
	Unregister(name string) error

	// Get retrieves a tool by name
	Get(name string) (Tool, error)

	// List returns all registered tool names
	List() []string

	// ListByCategory returns tools filtered by category
	ListByCategory(category ToolCategory) []string

	// ListByTags returns tools that match any of the given tags
	ListByTags(tags ...string) []string

	// Execute runs a tool with the given input
	Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)

	// ExecuteWithRetry runs a tool with automatic retry on failure
	ExecuteWithRetry(ctx context.Context, name string, input ToolInput, policy RetryPolicy) (ToolOutput, error)

	// GetMetadata returns metadata about a registered tool
	GetMetadata(name string) (ToolMetadata, error)

	// GetStatus returns the current status of a tool
	GetStatus(name string) (ToolStatus, error)

	// SetStatus updates the status of a tool
	SetStatus(name string, status ToolStatus) error

	// Close releases all resources used by the registry
	Close() error

	// GetMetrics returns registry metrics (optional monitoring)
	GetMetrics() RegistryMetrics

	// Subscribe registers a callback for registry events (optional monitoring)
	Subscribe(event RegistryEventType, callback RegistryEventCallback) error

	// Unsubscribe removes a callback (optional monitoring)
	Unsubscribe(event RegistryEventType, callback RegistryEventCallback) error
}

// ToolMetadata provides detailed information about a tool
type ToolMetadata struct {
	// Name is the tool's unique identifier
	Name string `json:"name"`

	// Description explains what the tool does
	Description string `json:"description"`

	// Version indicates the tool's version
	Version string `json:"version"`

	// Category groups related tools
	Category ToolCategory `json:"category"`

	// Tags for categorization and filtering
	Tags []string `json:"tags"`

	// Status indicates the tool's operational state
	Status ToolStatus `json:"status"`

	// Dependencies lists other tools this tool depends on
	Dependencies []string `json:"dependencies,omitempty"`

	// Capabilities describes what this tool can do
	Capabilities []string `json:"capabilities,omitempty"`

	// Requirements lists system requirements
	Requirements []string `json:"requirements,omitempty"`

	// RegisteredAt indicates when the tool was registered
	RegisteredAt time.Time `json:"registered_at"`

	// LastModified indicates when the tool was last updated
	LastModified time.Time `json:"last_modified"`

	// ExecutionCount tracks how many times the tool has been executed
	ExecutionCount int64 `json:"execution_count"`

	// LastExecuted indicates when the tool was last executed
	LastExecuted *time.Time `json:"last_executed,omitempty"`

	// AverageExecutionTime tracks the average execution duration
	AverageExecutionTime time.Duration `json:"average_execution_time,omitempty"`
}

// ToolCategory represents different categories of tools
type ToolCategory string

// ToolStatus represents the operational status of a tool
type ToolStatus string

// RegistryOption provides configuration for tool registration
type RegistryOption func(*RegistryConfig)

// RegistryConfig contains configuration for tool registration
type RegistryConfig struct {
	// Namespace groups related tools
	Namespace string

	// Tags for categorization
	Tags []string

	// Priority determines execution order when multiple tools match
	Priority int

	// Enabled indicates if the tool should be active upon registration
	Enabled bool

	// Metadata provides additional tool-specific configuration
	Metadata map[string]interface{}

	// Concurrency limits concurrent executions of this tool
	Concurrency int

	// Timeout sets the maximum execution time
	Timeout time.Duration

	// RetryPolicy defines retry behavior
	RetryPolicy *RetryPolicy

	// CacheEnabled indicates if results should be cached
	CacheEnabled bool

	// CacheDuration sets how long results are cached
	CacheDuration time.Duration

	// RateLimitPerMinute sets the maximum executions per minute
	RateLimitPerMinute int
}

// RetryPolicy defines how tools should handle retries
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `json:"max_attempts"`

	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration `json:"initial_delay"`

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// RetryableErrors defines which errors should trigger a retry
	RetryableErrors []string `json:"retryable_errors,omitempty"`
}

// Registry configuration options

// WithNamespace sets the namespace for the tool
func WithNamespace(namespace string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Namespace = namespace
	}
}

// WithTags adds tags to the tool
func WithTags(tags ...string) RegistryOption {
	return func(c *RegistryConfig) {
		c.Tags = append(c.Tags, tags...)
	}
}

// WithPriority sets the tool priority
func WithPriority(priority int) RegistryOption {
	return func(c *RegistryConfig) {
		c.Priority = priority
	}
}

// WithEnabled sets whether the tool is enabled
func WithEnabled(enabled bool) RegistryOption {
	return func(c *RegistryConfig) {
		c.Enabled = enabled
	}
}

// WithMetadata adds metadata to the tool
func WithMetadata(key string, value interface{}) RegistryOption {
	return func(c *RegistryConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]interface{})
		}
		c.Metadata[key] = value
	}
}

// WithConcurrency sets the maximum concurrent executions
func WithConcurrency(maxConcurrency int) RegistryOption {
	return func(c *RegistryConfig) {
		c.Concurrency = maxConcurrency
	}
}

// WithTimeout sets the execution timeout
func WithTimeout(timeout time.Duration) RegistryOption {
	return func(c *RegistryConfig) {
		c.Timeout = timeout
	}
}

// WithRetryPolicy sets the retry policy
func WithRetryPolicy(policy RetryPolicy) RegistryOption {
	return func(c *RegistryConfig) {
		c.RetryPolicy = &policy
	}
}

// WithCache enables caching with the specified duration
func WithCache(duration time.Duration) RegistryOption {
	return func(c *RegistryConfig) {
		c.CacheEnabled = true
		c.CacheDuration = duration
	}
}

// WithRateLimit sets the rate limit per minute
func WithRateLimit(perMinute int) RegistryOption {
	return func(c *RegistryConfig) {
		c.RateLimitPerMinute = perMinute
	}
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []string{"timeout", "network", "temporary"},
	}
}

// NOTE: ObservableRegistry has been merged into Registry for simplicity.
// Registry now includes optional monitoring methods.

// RegistryMetrics provides metrics about registry operations
type RegistryMetrics struct {
	// TotalTools is the number of registered tools
	TotalTools int `json:"total_tools"`

	// ActiveTools is the number of active tools
	ActiveTools int `json:"active_tools"`

	// TotalExecutions is the total number of executions
	TotalExecutions int64 `json:"total_executions"`

	// FailedExecutions is the number of failed executions
	FailedExecutions int64 `json:"failed_executions"`

	// AverageExecutionTime is the average execution duration
	AverageExecutionTime time.Duration `json:"average_execution_time"`

	// UpTime is how long the registry has been running
	UpTime time.Duration `json:"up_time"`

	// LastExecution is when a tool was last executed
	LastExecution *time.Time `json:"last_execution,omitempty"`
}

// RegistryEventType defines types of registry events
type RegistryEventType string

const (
	// EventToolRegistered fires when a tool is registered
	EventToolRegistered RegistryEventType = "tool_registered"

	// EventToolUnregistered fires when a tool is unregistered
	EventToolUnregistered RegistryEventType = "tool_unregistered"

	// EventToolExecuted fires when a tool is executed
	EventToolExecuted RegistryEventType = "tool_executed"

	// EventToolFailed fires when a tool execution fails
	EventToolFailed RegistryEventType = "tool_failed"

	// EventToolStatusChanged fires when a tool's status changes
	EventToolStatusChanged RegistryEventType = "tool_status_changed"
)

// RegistryEvent represents an event in the registry
type RegistryEvent struct {
	// Type identifies the event type
	Type RegistryEventType `json:"type"`

	// ToolName identifies the tool involved
	ToolName string `json:"tool_name"`

	// Timestamp indicates when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Details provides event-specific information
	Details map[string]interface{} `json:"details,omitempty"`

	// Error contains any error information
	Error error `json:"error,omitempty"`
}

// RegistryEventCallback is a function that handles registry events
type RegistryEventCallback func(event RegistryEvent)

// ============================================================================
// Manager Interfaces
// ============================================================================

// SessionManager interface removed as part of EPSILON workstream.
// Replaced by focused service interfaces:
// - services.SessionStore for session persistence
// - services.SessionState for session state management
// These provide the same functionality with better testability and separation of concerns.

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata"`
	State     map[string]interface{} `json:"state"`
}

// WorkflowManager interface removed as part of EPSILON workstream.
// Replaced by services.WorkflowExecutor interface which provides
// focused workflow orchestration without the complexity of job and template management.

// BuildManager interface removed as part of EPSILON workstream.
// Replaced by services.BuildExecutor interface which provides
// focused container build operations without session management complexity.

// RegistryManager interface removed as part of EPSILON workstream.
// Replaced by services.ToolRegistry interface which provides
// focused tool registration and discovery without orchestration complexity.

// ConfigManager interface removed as part of EPSILON workstream.
// Replaced by services.ConfigValidator interface which provides
// focused configuration validation using BETA's validation framework.

// ============================================================================
// Orchestration Interfaces
// ============================================================================

// Orchestrator provides tool orchestration functionality
type Orchestrator interface {
	// RegisterTool registers a tool with the orchestrator
	RegisterTool(name string, tool Tool) error

	// ExecuteTool executes a tool with the given arguments
	ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)

	// GetTool retrieves a registered tool
	GetTool(name string) (Tool, bool)

	// ListTools returns a list of all registered tools
	ListTools() []string

	// GetStats returns orchestrator statistics
	GetStats() interface{}

	// ValidateToolArgs validates tool arguments
	ValidateToolArgs(toolName string, args interface{}) error

	// GetToolMetadata retrieves metadata for a specific tool
	GetToolMetadata(toolName string) (*ToolMetadata, error)

	// RegisterGenericTool registers a tool with generic interface
	RegisterGenericTool(name string, tool interface{}) error

	// GetTypedToolMetadata retrieves typed metadata for a specific tool
	GetTypedToolMetadata(toolName string) (*ToolMetadata, error)
}

// ============================================================================
// Workflow Interfaces
// ============================================================================

// Workflow represents a workflow configuration
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Steps       []WorkflowStep         `json:"steps"`
	Variables   map[string]interface{} `json:"variables"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Tool       string                 `json:"tool"`
	Input      map[string]interface{} `json:"input"`
	DependsOn  []string               `json:"depends_on"`
	Condition  string                 `json:"condition"`
	MaxRetries int                    `json:"max_retries"`
	Timeout    time.Duration          `json:"timeout"`
}

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Steps       []WorkflowStep         `json:"steps"`
	Parameters  map[string]interface{} `json:"parameters"`
	Tags        []string               `json:"tags"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WorkflowResult represents the result of executing a workflow
type WorkflowResult struct {
	WorkflowID   string        `json:"workflow_id"`
	Success      bool          `json:"success"`
	StepResults  []StepResult  `json:"step_results"`
	Error        string        `json:"error,omitempty"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	TotalSteps   int           `json:"total_steps"`
	SuccessSteps int           `json:"success_steps"`
	FailedSteps  int           `json:"failed_steps"`
}

// StepResult represents the result of executing a workflow step
type StepResult struct {
	StepID    string                 `json:"step_id"`
	StepName  string                 `json:"step_name"`
	Success   bool                   `json:"success"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Retries   int                    `json:"retries"`
}

// WorkflowStatus represents the status of a running workflow
type WorkflowStatus struct {
	WorkflowID     string    `json:"workflow_id"`
	Status         string    `json:"status"`
	CurrentStep    string    `json:"current_step"`
	StartTime      time.Time `json:"start_time"`
	LastUpdate     time.Time `json:"last_update"`
	CompletedSteps int       `json:"completed_steps"`
	TotalSteps     int       `json:"total_steps"`
}

// ============================================================================
// MCP Server Interfaces
// ============================================================================

// MCPServer represents the main MCP server interface
type MCPServer interface {
	// Start starts the server
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server
	Stop(ctx context.Context) error
}

// GomcpManager manages the gomcp server lifecycle
type GomcpManager interface {
	// Start starts the gomcp server
	Start(ctx context.Context) error

	// Stop stops the mcp-go server
	Stop(ctx context.Context) error

	// RegisterTool registers a tool with mcp-go
	RegisterTool(name, description string, handler interface{}) error

	// GetServer returns the underlying mcp-go server
	GetServer() interface{}

	// IsRunning checks if the server is running
	IsRunning() bool
}

// ============================================================================
// Transport Interfaces
// ============================================================================

// Transport defines the interface for MCP transports
type Transport interface {
	// Start starts the transport
	Start(ctx context.Context) error

	// Stop stops the transport
	Stop(ctx context.Context) error

	// Send sends a message
	Send(message interface{}) error

	// Receive receives a message
	Receive() (interface{}, error)

	// IsConnected checks if the transport is connected
	IsConnected() bool
}

// ============================================================================
// Error Types
// ============================================================================

// ErrorType represents different types of errors
type ErrorType string

// ============================================================================
// Logging Interface
// ============================================================================

// Logger interface removed - use domain/*slog.Logger directly

// ============================================================================
// Build Types
// ============================================================================

// BuildArgs represents arguments for a build operation
type BuildArgs struct {
	SessionID  string                 `json:"session_id"`
	Dockerfile string                 `json:"dockerfile"`
	Context    string                 `json:"context"`
	ImageName  string                 `json:"image_name"`
	Tags       []string               `json:"tags"`
	BuildArgs  map[string]string      `json:"build_args"`
	Target     string                 `json:"target,omitempty"`
	Platform   string                 `json:"platform,omitempty"`
	NoCache    bool                   `json:"no_cache,omitempty"`
	PullParent bool                   `json:"pull_parent,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	BuildID     string                 `json:"build_id"`
	ImageID     string                 `json:"image_id"`
	ImageName   string                 `json:"image_name"`
	Tags        []string               `json:"tags"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	Size        int64                  `json:"size"`
	Duration    time.Duration          `json:"duration"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BuildStatus represents the status of an ongoing build
type BuildStatus struct {
	BuildID                 string     `json:"build_id"`
	Status                  BuildState `json:"status"`
	Progress                float64    `json:"progress"`
	CurrentStep             string     `json:"current_step"`
	StartTime               time.Time  `json:"start_time"`
	LastUpdate              time.Time  `json:"last_update"`
	EstimatedCompletionTime *time.Time `json:"estimated_completion_time,omitempty"`
}

// BuildInfo represents information about a build
type BuildInfo struct {
	BuildID     string                 `json:"build_id"`
	SessionID   string                 `json:"session_id"`
	ImageName   string                 `json:"image_name"`
	Status      BuildState             `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Size        int64                  `json:"size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BuildState represents the state of a build operation
type BuildState string

const (
	BuildStateQueued    BuildState = "queued"
	BuildStateRunning   BuildState = "running"
	BuildStateCompleted BuildState = "completed"
	BuildStateFailed    BuildState = "failed"
	BuildStateCancelled BuildState = "cancelled"
)

// BuildStrategy represents different build strategies
type BuildStrategy string

const (
	BuildStrategyDocker   BuildStrategy = "docker"
	BuildStrategyBuildkit BuildStrategy = "buildkit"
	BuildStrategyPodman   BuildStrategy = "podman"
	BuildStrategyKaniko   BuildStrategy = "kaniko"
)

// CacheStats represents build cache statistics
type CacheStats struct {
	TotalSize     int64     `json:"total_size"`
	UsedSize      int64     `json:"used_size"`
	AvailableSize int64     `json:"available_size"`
	HitRate       float64   `json:"hit_rate"`
	LastCleanup   time.Time `json:"last_cleanup"`
	Entries       int       `json:"entries"`
}

// RegistryStats represents registry statistics
type RegistryStats struct {
	TotalTools       int           `json:"total_tools"`
	ActiveTools      int           `json:"active_tools"`
	TotalExecutions  int64         `json:"total_executions"`
	FailedExecutions int64         `json:"failed_executions"`
	AverageExecTime  time.Duration `json:"average_execution_time"`
	LastExecution    *time.Time    `json:"last_execution,omitempty"`
	UpTime           time.Duration `json:"up_time"`
}

// ============================================================================
// Factory Interfaces
// ============================================================================

// ToolFactory defines the interface for creating tools without direct dependencies on internal packages
type ToolFactory interface {
	// CreateTool creates a tool by category and name
	CreateTool(category string, name string) (Tool, error)

	// CreateAnalyzer creates an analyzer (special case due to interfaces)
	CreateAnalyzer(aiAnalyzer interface{}) interface{}

	// CreateEnhancedBuildAnalyzer creates an enhanced build analyzer
	CreateEnhancedBuildAnalyzer() interface{} // Returns interface{} to avoid import

	// CreateSessionStateManager creates a session state manager
	CreateSessionStateManager(sessionID string) interface{} // Returns interface{} to avoid import

	// RegisterToolCreator registers a tool creator function for a category and name
	RegisterToolCreator(category string, name string, creator ToolCreator)
}

// ToolCreator is a function that creates a tool
type ToolCreator func() (Tool, error)

// ============================================================================
// Pipeline Interfaces - Unified Pipeline System
// ============================================================================

// Pipeline defines unified orchestration interface
type Pipeline interface {
	// Execute runs pipeline with context and metrics
	Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error)

	// AddStage adds a stage to the pipeline
	AddStage(stage PipelineStage) Pipeline

	// WithTimeout sets pipeline timeout
	WithTimeout(timeout time.Duration) Pipeline

	// WithRetry sets retry policy
	WithRetry(policy PipelineRetryPolicy) Pipeline

	// WithMetrics enables metrics collection
	WithMetrics(collector MetricsCollector) Pipeline
}

// PipelineStage represents a single pipeline stage
type PipelineStage interface {
	Name() string
	Execute(ctx context.Context, input interface{}) (interface{}, error)
	Validate(input interface{}) error
}

// PipelineBuilder provides fluent API for pipeline construction
type PipelineBuilder interface {
	New() Pipeline
	FromTemplate(template string) Pipeline
	WithStages(stages ...PipelineStage) PipelineBuilder
	Build() Pipeline
}

// CommandRouter provides map-based command routing
type CommandRouter interface {
	Register(command string, handler CommandHandler) error
	Route(ctx context.Context, command string, args interface{}) (interface{}, error)
	ListCommands() []string
	Unregister(command string) error
	GetHandler(command string) (CommandHandler, error)
	RegisterFunc(command string, handler func(ctx context.Context, args interface{}) (interface{}, error)) error
}

// CommandHandler handles command execution
type CommandHandler interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
}

// PipelineRequest represents input to pipeline execution
type PipelineRequest struct {
	Input    interface{}            `json:"input"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PipelineResponse represents output from pipeline execution
type PipelineResponse struct {
	Output   interface{}            `json:"output"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PipelineRetryPolicy defines retry behavior for pipelines
type PipelineRetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	BackoffDuration time.Duration `json:"backoff_duration"`
	Multiplier      float64       `json:"multiplier,omitempty"`
}

// MetricsCollector interface for pipeline metrics
type MetricsCollector interface {
	RecordStageExecution(stageName string, duration time.Duration, err error)
}

// ============================================================================
// Validation Interfaces - Domain Validation System
// ============================================================================

// Validator defines the core validation interface
type Validator[T any] interface {
	// Validate validates a value and returns validation result
	Validate(ctx context.Context, value T) ValidationResult

	// Name returns the validator name for error reporting
	Name() string
}

// ValidationResult holds validation outcome
type ValidationResult struct {
	Valid    bool
	Errors   []error
	Warnings []string
	Context  ValidationContext
}

// ValidationContext provides validation execution context
type ValidationContext struct {
	Field    string
	Path     string
	Metadata map[string]interface{}
}

// ValidatorChain allows composing multiple validators
type ValidatorChain[T any] struct {
	validators []Validator[T]
	strategy   ChainStrategy
}

// ChainStrategy defines how validators are executed
type ChainStrategy int

const (
	// StopOnFirstError stops chain on first validation error
	StopOnFirstError ChainStrategy = iota
	// ContinueOnError continues chain collecting all errors
	ContinueOnError
	// StopOnFirstWarning stops chain on first warning
	StopOnFirstWarning
)

// DomainValidator extends basic validation with domain-specific metadata
type DomainValidator[T any] interface {
	Validator[T]

	// Domain returns the validation domain (e.g., "kubernetes", "docker", "security")
	Domain() string

	// Category returns the validation category (e.g., "manifest", "config", "policy")
	Category() string

	// Priority returns validation priority for ordering (higher = earlier)
	Priority() int

	// Dependencies returns validator names this depends on
	Dependencies() []string
}

// ValidatorRegistry manages domain validators with dependency resolution
type ValidatorRegistry interface {
	// Register a domain validator
	Register(validator DomainValidator[interface{}]) error

	// Unregister a validator by name
	Unregister(name string) error

	// Get validators by domain and category
	GetValidators(domain, category string) []DomainValidator[interface{}]

	// Get all validators for a domain
	GetDomainValidators(domain string) []DomainValidator[interface{}]

	// Validate using all applicable validators
	ValidateAll(ctx context.Context, data interface{}, domain, category string) ValidationResult

	// List all registered validators
	ListValidators() []ValidatorInfo
}

// ValidatorInfo provides metadata about registered validators
type ValidatorInfo struct {
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	Category     string   `json:"category"`
	Priority     int      `json:"priority"`
	Dependencies []string `json:"dependencies"`
}

// NewValidatorChain creates a new validator chain
func NewValidatorChain[T any](strategy ChainStrategy) *ValidatorChain[T] {
	return &ValidatorChain[T]{
		validators: make([]Validator[T], 0),
		strategy:   strategy,
	}
}

// Add adds a validator to the chain
func (c *ValidatorChain[T]) Add(validator Validator[T]) *ValidatorChain[T] {
	c.validators = append(c.validators, validator)
	return c
}

// Validate executes the validator chain
func (c *ValidatorChain[T]) Validate(ctx context.Context, value T) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	for _, validator := range c.validators {
		validationResult := validator.Validate(ctx, value)

		// Collect errors and warnings
		result.Errors = append(result.Errors, validationResult.Errors...)
		result.Warnings = append(result.Warnings, validationResult.Warnings...)

		// Apply strategy
		if !validationResult.Valid {
			result.Valid = false
			if c.strategy == StopOnFirstError {
				break
			}
		}

		if len(validationResult.Warnings) > 0 && c.strategy == StopOnFirstWarning {
			break
		}
	}

	return result
}

// Name returns the chain name
func (c *ValidatorChain[T]) Name() string {
	return "ValidatorChain"
}
