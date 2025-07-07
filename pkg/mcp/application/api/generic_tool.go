package api

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// GenericTool is the type-safe interface for all MCP tools.
// TInput and TOutput must implement their respective constraint interfaces.
type GenericTool[TInput ToolInputConstraint, TOutput ToolOutputConstraint] interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Execute runs the tool with type-safe input and output
	Execute(ctx context.Context, input TInput) (TOutput, error)

	// Schema returns the JSON schema for the tool
	Schema() ToolSchema

	// Validate validates the input before execution
	Validate(ctx context.Context, input TInput) error

	// GetTimeout returns the tool's execution timeout
	GetTimeout() time.Duration
}

// ToolInputConstraint defines what types can be used as tool input
type ToolInputConstraint interface {
	// GetSessionID returns the session identifier
	GetSessionID() string

	// Validate performs input validation
	Validate() error

	// GetContext returns execution context data
	GetContext() map[string]interface{}
}

// ToolOutputConstraint defines what types can be used as tool output
type ToolOutputConstraint interface {
	// IsSuccess indicates if the operation was successful
	IsSuccess() bool

	// GetData returns the result data
	GetData() interface{}

	// GetError returns any error message
	GetError() string
}

// StreamingTool extends GenericTool with streaming capabilities
type StreamingTool[TInput ToolInputConstraint, TOutput ToolOutputConstraint] interface {
	GenericTool[TInput, TOutput]

	// Stream executes the tool and streams results
	Stream(ctx context.Context, input TInput) (<-chan TOutput, <-chan error)

	// SupportsStreaming indicates if this tool supports streaming
	SupportsStreaming() bool

	// GetStreamBufferSize returns the recommended buffer size for streaming
	GetStreamBufferSize() int
}

// BatchTool extends GenericTool with batch processing capabilities
type BatchTool[TInput ToolInputConstraint, TOutput ToolOutputConstraint] interface {
	GenericTool[TInput, TOutput]

	// ExecuteBatch processes multiple inputs in a single operation
	ExecuteBatch(ctx context.Context, inputs []TInput) ([]BatchResult[TOutput], error)

	// GetMaxBatchSize returns the maximum number of items in a batch
	GetMaxBatchSize() int

	// GetBatchTimeout returns the timeout for batch operations
	GetBatchTimeout() time.Duration
}

// BatchResult represents the result of a single item in a batch operation
type BatchResult[TOutput ToolOutputConstraint] struct {
	// Index identifies which input this result corresponds to
	Index int `json:"index"`

	// Output contains the result for this item
	Output TOutput `json:"output"`

	// Error contains any error specific to this item
	Error error `json:"error,omitempty"`

	// Duration indicates how long this item took to process
	Duration time.Duration `json:"duration"`
}

// ============================================================================
// Domain-Specific Input Types
// ============================================================================

// AnalyzeInput represents input for repository analysis tools
type AnalyzeInput struct {
	SessionID            string                 `json:"session_id"`
	RepoURL              string                 `json:"repo_url"`
	Branch               string                 `json:"branch,omitempty"`
	LanguageHint         string                 `json:"language_hint,omitempty"`
	IncludeDependencies  bool                   `json:"include_dependencies"`
	IncludeSecurityScan  bool                   `json:"include_security_scan"`
	IncludeBuildAnalysis bool                   `json:"include_build_analysis"`
	CustomOptions        map[string]string      `json:"custom_options,omitempty"`
	Context              map[string]interface{} `json:"context,omitempty"`
}

// GetSessionID implements ToolInputConstraint
func (a *AnalyzeInput) GetSessionID() string {
	return a.SessionID
}

// Validate implements ToolInputConstraint
func (a *AnalyzeInput) Validate() error {
	if a.SessionID == "" {
		return errors.Validation("api", "session_id is required")
	}
	if a.RepoURL == "" {
		return errors.Validation("api", "repo_url is required")
	}
	return nil
}

// GetContext implements ToolInputConstraint
func (a *AnalyzeInput) GetContext() map[string]interface{} {
	if a.Context == nil {
		return make(map[string]interface{})
	}
	return a.Context
}

// BuildInput represents input for image building tools
type BuildInput struct {
	SessionID     string                 `json:"session_id"`
	Image         string                 `json:"image"`
	Dockerfile    string                 `json:"dockerfile"`
	Context       string                 `json:"context"`
	BuildArgs     map[string]string      `json:"build_args,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	NoCache       bool                   `json:"no_cache"`
	Platform      string                 `json:"platform,omitempty"`
	CustomOptions map[string]string      `json:"custom_options,omitempty"`
	ContextData   map[string]interface{} `json:"context,omitempty"`
}

// GetSessionID implements ToolInputConstraint
func (b *BuildInput) GetSessionID() string {
	return b.SessionID
}

// Validate implements ToolInputConstraint
func (b *BuildInput) Validate() error {
	if b.SessionID == "" {
		return errors.Validation("api", "session_id is required")
	}
	if b.Image == "" {
		return errors.Validation("api", "image is required")
	}
	return nil
}

// GetContext implements ToolInputConstraint
func (b *BuildInput) GetContext() map[string]interface{} {
	if b.ContextData == nil {
		return make(map[string]interface{})
	}
	return b.ContextData
}

// DeployInput represents input for deployment tools
type DeployInput struct {
	SessionID     string                 `json:"session_id"`
	Manifests     []string               `json:"manifests"`
	Namespace     string                 `json:"namespace,omitempty"`
	DryRun        bool                   `json:"dry_run"`
	Wait          bool                   `json:"wait"`
	Timeout       time.Duration          `json:"timeout,omitempty"`
	CustomOptions map[string]string      `json:"custom_options,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
}

// GetSessionID implements ToolInputConstraint
func (d *DeployInput) GetSessionID() string {
	return d.SessionID
}

// Validate implements ToolInputConstraint
func (d *DeployInput) Validate() error {
	if d.SessionID == "" {
		return errors.Validation("api", "session_id is required")
	}
	if len(d.Manifests) == 0 {
		return errors.Validation("api", "manifests are required")
	}
	return nil
}

// GetContext implements ToolInputConstraint
func (d *DeployInput) GetContext() map[string]interface{} {
	if d.Context == nil {
		return make(map[string]interface{})
	}
	return d.Context
}

// ============================================================================
// Domain-Specific Output Types
// ============================================================================

// AnalyzeOutput represents output from repository analysis tools
type AnalyzeOutput struct {
	Success              bool                   `json:"success"`
	SessionID            string                 `json:"session_id"`
	Language             string                 `json:"language"`
	Framework            string                 `json:"framework"`
	Dependencies         []Dependency           `json:"dependencies,omitempty"`
	SecurityIssues       []SecurityIssue        `json:"security_issues,omitempty"`
	BuildRecommendations []string               `json:"build_recommendations,omitempty"`
	AnalysisTime         time.Duration          `json:"analysis_time"`
	FilesAnalyzed        int                    `json:"files_analyzed"`
	ErrorMsg             string                 `json:"error,omitempty"`
	Data                 map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess implements ToolOutputConstraint
func (a *AnalyzeOutput) IsSuccess() bool {
	return a.Success
}

// GetData implements ToolOutputConstraint
func (a *AnalyzeOutput) GetData() interface{} {
	return a.Data
}

// GetError implements ToolOutputConstraint
func (a *AnalyzeOutput) GetError() string {
	return a.ErrorMsg
}

// BuildOutput represents output from image building tools
type BuildOutput struct {
	Success   bool                   `json:"success"`
	SessionID string                 `json:"session_id"`
	ImageID   string                 `json:"image_id,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Size      int64                  `json:"size,omitempty"`
	BuildTime time.Duration          `json:"build_time"`
	Digest    string                 `json:"digest,omitempty"`
	ErrorMsg  string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess implements ToolOutputConstraint
func (b *BuildOutput) IsSuccess() bool {
	return b.Success
}

// GetData implements ToolOutputConstraint
func (b *BuildOutput) GetData() interface{} {
	return b.Data
}

// GetError implements ToolOutputConstraint
func (b *BuildOutput) GetError() string {
	return b.ErrorMsg
}

// DeployOutput represents output from deployment tools
type DeployOutput struct {
	Success           bool                   `json:"success"`
	SessionID         string                 `json:"session_id"`
	DeployedResources []DeployedResource     `json:"deployed_resources,omitempty"`
	DeployTime        time.Duration          `json:"deploy_time"`
	Namespace         string                 `json:"namespace,omitempty"`
	ErrorMsg          string                 `json:"error,omitempty"`
	Data              map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess implements ToolOutputConstraint
func (d *DeployOutput) IsSuccess() bool {
	return d.Success
}

// GetData implements ToolOutputConstraint
func (d *DeployOutput) GetData() interface{} {
	return d.Data
}

// GetError implements ToolOutputConstraint
func (d *DeployOutput) GetError() string {
	return d.ErrorMsg
}

// ============================================================================
// Supporting Types
// ============================================================================

// Dependency represents a project dependency
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type,omitempty"`
}

// SecurityIssue represents a security vulnerability
type SecurityIssue struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	FixVersion  string `json:"fix_version,omitempty"`
}

// DeployedResource represents a deployed Kubernetes resource
type DeployedResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

// ============================================================================
// Domain-Specific Tool Type Aliases
// ============================================================================

// AnalyzeTool is a type-safe tool for repository analysis
type AnalyzeTool = GenericTool[*AnalyzeInput, *AnalyzeOutput]

// BuildTool is a type-safe tool for image building
type BuildTool = GenericTool[*BuildInput, *BuildOutput]

// DeployTool is a type-safe tool for deployment
type DeployTool = GenericTool[*DeployInput, *DeployOutput]

// StreamingAnalyzeTool is a streaming repository analysis tool
type StreamingAnalyzeTool = StreamingTool[*AnalyzeInput, *AnalyzeOutput]

// BatchBuildTool is a batch image building tool
type BatchBuildTool = BatchTool[*BuildInput, *BuildOutput]
