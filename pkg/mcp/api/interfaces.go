package api

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/validation"
)

// MCPServer represents the main MCP server interface
type MCPServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Tool is the canonical interface for all MCP tools
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
	Schema() ToolSchema
}

// ToolInput represents the input structure for tools
type ToolInput struct {
	SessionID string                 `json:"session_id"`
	Data      map[string]interface{} `json:"data"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type ToolOutput struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

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

type ToolExample struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Input       ToolInput  `json:"input"`
	Output      ToolOutput `json:"output"`
}

// Registry manages tool registration and execution
type Registry interface {
	Register(tool Tool) error

	Get(name string) (Tool, error)

	List() []string

	Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)
}

// Transport defines the interface for MCP transports
type Transport interface {
	Start(ctx context.Context) error

	Stop(ctx context.Context) error

	Send(message interface{}) error

	Receive() (interface{}, error)

	ReceiveStream() (<-chan interface{}, error)

	IsConnected() bool
}

// Validator defines the core validation interface
type Validator[T any] interface {
	Validate(ctx context.Context, value T) ValidationResult

	Name() string
}

// Use unified validation types from the core validation package

// ValidationResult is an alias to the unified validation result
type ValidationResult = validation.ValidationResult

// ValidationError is an alias to the unified validation error
type ValidationError = validation.ValidationError

// ValidationWarning is an alias to the unified validation warning
type ValidationWarning = validation.ValidationWarning

// BuildValidationResult is an alias to the unified build validation result
type BuildValidationResult = validation.BuildValidationResult

// ManifestValidationResult is an alias to the unified manifest validation result
type ManifestValidationResult = validation.ManifestValidationResult

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

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata"`
	State     map[string]interface{} `json:"state"`
}

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

type BuildResult struct {
	BuildID   string        `json:"build_id"`
	ImageID   string        `json:"image_id"`
	ImageName string        `json:"image_name"`
	Tags      []string      `json:"tags"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// ProgressEmitter provides clean interface for progress reporting across transports
type ProgressEmitter interface {
	Emit(ctx context.Context, stage string, percent int, message string) error

	EmitDetailed(ctx context.Context, update ProgressUpdate) error

	Close() error
}

// ProgressUpdate represents a structured progress report
type ProgressUpdate struct {
	Step       int                    `json:"step"`
	Total      int                    `json:"total"`
	Stage      string                 `json:"stage"`
	Message    string                 `json:"message"`
	Percentage int                    `json:"percentage"` // 0-100
	StartedAt  time.Time              `json:"started_at"`
	ETA        time.Duration          `json:"eta,omitempty"`
	Status     string                 `json:"status,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
