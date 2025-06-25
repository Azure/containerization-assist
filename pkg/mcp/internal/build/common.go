package build

import (
	"context"
	"time"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// BuildStrategy defines the interface for different build strategies
type BuildStrategy interface {
	// Name returns the strategy name
	Name() string

	// Description returns a human-readable description
	Description() string

	// Build executes the build using this strategy
	Build(ctx BuildContext) (*BuildResult, error)

	// SupportsFeature checks if the strategy supports a specific feature
	SupportsFeature(feature string) bool

	// Validate checks if the strategy can be used with the given context
	Validate(ctx BuildContext) error
}

// BuildContext contains all information needed for a build
type BuildContext struct {
	SessionID      string
	WorkspaceDir   string
	ImageName      string
	ImageTag       string
	DockerfilePath string
	BuildPath      string
	Platform       string
	NoCache        bool
	BuildArgs      map[string]string
	Labels         map[string]string
}

// BuildResult contains the results of a build operation
type BuildResult struct {
	Success        bool
	ImageID        string
	FullImageRef   string
	Duration       time.Duration
	LayerCount     int
	ImageSizeBytes int64
	BuildLogs      []string
	CacheHits      int
	CacheMisses    int
}

// BuildValidator defines the interface for build validation
type BuildValidator interface {
	// ValidateDockerfile checks if the Dockerfile is valid
	ValidateDockerfile(dockerfilePath string) (*ValidationResult, error)

	// ValidateBuildContext checks if the build context is valid
	ValidateBuildContext(ctx BuildContext) (*ValidationResult, error)

	// ValidateSecurityRequirements checks for security issues
	ValidateSecurityRequirements(dockerfilePath string) (*SecurityValidationResult, error)
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
	Info     []string
}

// ValidationError represents a validation error
type ValidationError struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// SecurityValidationResult contains security validation results
type SecurityValidationResult struct {
	Secure               bool
	CriticalIssues       []SecurityIssue
	HighIssues           []SecurityIssue
	MediumIssues         []SecurityIssue
	LowIssues            []SecurityIssue
	BestPractices        []string
	ComplianceViolations []ComplianceViolation
}

// SecurityIssue represents a security issue found during validation
type SecurityIssue struct {
	Severity    string
	Type        string
	Message     string
	Line        int
	Remediation string
}

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	Standard string
	Rule     string
	Message  string
	Line     int
}

// BuildExecutor defines the interface for build execution
type BuildExecutor interface {
	// Execute runs the build with the selected strategy
	Execute(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy) (*ExecutionResult, error)

	// ExecuteWithProgress runs the build with progress reporting
	ExecuteWithProgress(ctx context.Context, buildCtx BuildContext, strategy BuildStrategy, reporter BuildProgressReporter) (*ExecutionResult, error)

	// Monitor monitors a running build
	Monitor(buildID string) (*BuildStatus, error)

	// Cancel cancels a running build
	Cancel(buildID string) error
}

// ExecutionResult contains the complete results of a build execution
type ExecutionResult struct {
	BuildResult      *BuildResult
	ValidationResult *ValidationResult
	SecurityResult   *SecurityValidationResult
	Performance      *PerformanceMetrics
	Artifacts        []BuildArtifact
}

// PerformanceMetrics contains build performance metrics
type PerformanceMetrics struct {
	TotalDuration     time.Duration
	ValidationTime    time.Duration
	BuildTime         time.Duration
	PushTime          time.Duration
	CacheUtilization  float64
	NetworkTransferMB float64
	DiskUsageMB       float64
	CPUUsagePercent   float64
	MemoryUsageMB     float64
}

// BuildArtifact represents an artifact produced by the build
type BuildArtifact struct {
	Type     string
	Name     string
	Path     string
	Size     int64
	Checksum string
}

// BuildStatus represents the current status of a build
type BuildStatus struct {
	BuildID       string
	State         string
	Progress      float64
	CurrentStage  string
	Message       string
	StartTime     time.Time
	EstimatedTime time.Duration
}

// ProgressReporter defines the interface for progress reporting
type ProgressReporter interface {
	ReportProgress(progress float64, stage string, message string)
	ReportError(err error)
	ReportWarning(message string)
	ReportInfo(message string)
}

// BuildProgressReporter defines the interface for build progress reporting
// This extends the core progress reporting functionality
type BuildProgressReporter interface {
	ReportStage(stageProgress float64, message string)
	NextStage(message string)
	SetStage(stageIndex int, message string)
	ReportOverall(progress float64, message string)
	GetCurrentStage() (int, mcptypes.ProgressStage)
	ReportError(err error)
	ReportWarning(message string)
	ReportInfo(message string)
}

// BuildOptions contains additional options for builds
type BuildOptions struct {
	Timeout          time.Duration
	CPULimit         string
	MemoryLimit      string
	NetworkMode      string
	SecurityOpts     []string
	EnableBuildKit   bool
	ExperimentalOpts map[string]string
}

// BuildError represents a build-specific error
type BuildError struct {
	Code    string
	Message string
	Stage   string
	Line    int
	Type    string
}

func (e *BuildError) Error() string {
	return e.Message
}

// NewBuildError creates a new build error
func NewBuildError(code, message, stage string, errType string) *BuildError {
	return &BuildError{
		Code:    code,
		Message: message,
		Stage:   stage,
		Type:    errType,
	}
}

// Common build stage names
const (
	StageValidation = "validation"
	StagePreBuild   = "pre-build"
	StageBuild      = "build"
	StagePostBuild  = "post-build"
	StagePush       = "push"
	StageScan       = "scan"
)

// Common build features
const (
	FeatureMultiStage   = "multi-stage"
	FeatureBuildKit     = "buildkit"
	FeatureSecrets      = "secrets"
	FeatureSBOM         = "sbom"
	FeatureProvenance   = "provenance"
	FeatureCrossCompile = "cross-compile"
)
