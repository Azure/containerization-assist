package api

import (
	"context"
	"time"
)

// MCPServer represents the main MCP server interface
type MCPServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Note: Tool interface is reserved for future use when implementing
// a more structured tool abstraction layer over MCP tools

// Transport defines the interface for MCP transports
type Transport interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(message interface{}) error
	Receive() (interface{}, error)
}

// Validation types defined at API layer for clean architecture

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors,omitempty"`
	Warnings []ValidationWarning    `json:"warnings,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// BuildValidationResult represents the result of build validation
type BuildValidationResult struct {
	ValidationResult
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	BuildContext   string            `json:"build_context,omitempty"`
	Issues         []ValidationError `json:"issues,omitempty"`
}

// ManifestValidationResult represents the result of manifest validation
type ManifestValidationResult struct {
	ValidationResult
	ManifestPath string            `json:"manifest_path,omitempty"`
	Resource     string            `json:"resource,omitempty"`
	Issues       []ValidationError `json:"issues,omitempty"`
}

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
