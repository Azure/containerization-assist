// Package api provides the pure interface definitions for the MCP system.
// This package contains only interfaces and data contracts with NO implementation code.
//
// Dependency flow: Infrastructure → Application → Domain → API
// The API layer must remain stable and contain no business logic.
package api

import (
	"context"
	"time"
)

// ============================================================================
// Core MCP Server Interface
// ============================================================================

// MCPServer represents the main MCP server interface
type MCPServer interface {
	// Start starts the server
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server
	Stop(ctx context.Context) error
}

// ============================================================================
// Tool Interfaces
// ============================================================================

// Tool is the canonical interface for all MCP tools
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

// ============================================================================
// Data Structures (Pure DTOs - No Methods)
// ============================================================================

// ToolInput represents the input structure for tools
type ToolInput struct {
	SessionID string                 `json:"session_id"`
	Data      map[string]interface{} `json:"data"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// ToolOutput represents the output structure for tools
type ToolOutput struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolSchema represents the schema definition for a tool
type ToolSchema struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Examples     []ToolExample          `json:"examples,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Category     string                 `json:"category,omitempty"`
	Version      string                 `json:"version,omitempty"`
}

// ToolExample demonstrates how to use a tool
type ToolExample struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Input       ToolInput  `json:"input"`
	Output      ToolOutput `json:"output"`
}

// ============================================================================
// Registry Interface
// ============================================================================

// Registry manages tool registration and execution
type Registry interface {
	// Register adds a tool to the registry
	Register(tool Tool) error

	// Get retrieves a tool by name
	Get(name string) (Tool, error)

	// List returns all registered tool names
	List() []string

	// Execute runs a tool with the given input
	Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)
}

// ============================================================================
// Orchestrator Interface
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
}

// ============================================================================
// Transport Interface
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
// Validation Interfaces
// ============================================================================

// Validator defines the core validation interface
type Validator[T any] interface {
	// Validate validates a value and returns validation result
	Validate(ctx context.Context, value T) ValidationResult

	// Name returns the validator name for error reporting
	Name() string
}

// ============================================================================
// Validation Data Structures
// ============================================================================

// ValidationResult holds validation outcome
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// BuildValidationResult represents the result of build validation
type BuildValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ManifestValidationResult represents the result of manifest validation
type ManifestValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ============================================================================
// Workflow Data Structures
// ============================================================================

// WorkflowResult represents the result of executing a workflow
type WorkflowResult struct {
	WorkflowID  string        `json:"workflow_id"`
	Success     bool          `json:"success"`
	StepResults []StepResult  `json:"step_results"`
	Error       string        `json:"error,omitempty"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
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
}

// ============================================================================
// Session Data Structures
// ============================================================================

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata"`
	State     map[string]interface{} `json:"state"`
}

// ============================================================================
// Build Data Structures
// ============================================================================

// BuildArgs represents arguments for a build operation
type BuildArgs struct {
	SessionID  string            `json:"session_id"`
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	ImageName  string            `json:"image_name"`
	Tags       []string          `json:"tags"`
	BuildArgs  map[string]string `json:"build_args"`
	Target     string            `json:"target,omitempty"`
	Platform   string            `json:"platform,omitempty"`
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	BuildID   string        `json:"build_id"`
	ImageID   string        `json:"image_id"`
	ImageName string        `json:"image_name"`
	Tags      []string      `json:"tags"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}
