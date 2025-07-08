package build

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// NOTE: The following interfaces have been consolidated into BuildService
// in unified_interface.go for better maintainability:
// - BuildStrategy
// - BuildValidator
// - BuildExecutor
// - BuildProgressReporter
// - ExtendedBuildReporter
//
// Use BuildService instead of these interfaces for new implementations.

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
	BuildArgs      map[string]string
	Labels         map[string]string
	Platform       string
	Tags           []string
	Warnings       []string
	Errors         []string
}

// BuildError represents an error during build operation
type BuildError struct {
	Message   string
	Code      string
	Details   map[string]interface{}
	Timestamp time.Time
	Stage     string
}

// BuildValidationResult represents the result of build validation
type BuildValidationResult struct {
	Valid   bool
	Issues  []BuildValidationIssue
	Score   int
	Metrics map[string]interface{}
}

// BuildValidationIssue represents a validation issue
type BuildValidationIssue struct {
	Type       string
	Severity   string
	Message    string
	File       string
	Line       int
	Suggestion string
	Fix        string
	Category   string
	RuleID     string
	RuleName   string
	Details    map[string]interface{}
	Timestamp  time.Time
}

// BuildMetrics represents build metrics
type BuildMetrics struct {
	Duration        time.Duration
	ImageSize       int64
	LayerCount      int
	CacheHits       int
	CacheMisses     int
	BuildSteps      int
	Dependencies    int
	Vulnerabilities int
	Performance     map[string]interface{}
}

// BuildProgressInfo represents build progress information
type BuildProgressInfo struct {
	Stage      string
	Step       int
	Total      int
	Message    string
	Complete   bool
	Percentage float64
	Duration   time.Duration
	Details    map[string]interface{}
	Logs       []string
	Metrics    BuildMetrics
	Warnings   []string
	Errors     []string
}

// BuildConfig represents build configuration
type BuildConfig struct {
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
	CustomOptions  map[string]string
	Timeout        time.Duration
}

// BuildSession represents a build session
type BuildSession struct {
	ID            string
	Status        string
	StartTime     time.Time
	Duration      time.Duration
	Configuration BuildConfig
	Result        *BuildResult
	Metrics       BuildMetrics
	Logs          []string
	Errors        []BuildError
}

// BuildOptions represents build options
type BuildOptions struct {
	NoCache        bool
	ForceRebuild   bool
	Pull           bool
	RmIntermediate bool
	Squash         bool
	Quiet          bool
	Verbose        bool
	BuildArgs      map[string]string
	Labels         map[string]string
	Platform       string
	Target         string
	CacheFrom      []string
	CacheTo        []string
}

// BuildStatus represents build status
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
)

// BuildStage represents build stage
type BuildStage string

const (
	BuildStageInit       BuildStage = "init"
	BuildStageAnalysis   BuildStage = "analysis"
	BuildStageValidation BuildStage = "validation"
	BuildStageBuilding   BuildStage = "building"
	BuildStageValidating BuildStage = "validating"
	BuildStageFinished   BuildStage = "finished"
	BuildStageError      BuildStage = "error"
)

// ValidationContextInterface defines the interface for validation context
type ValidationContextInterface interface {
	GetSessionID() string
	GetWorkspaceDir() string
	GetImageName() string
	GetImageTag() string
	GetDockerfilePath() string
	GetBuildPath() string
	GetPlatform() string
	GetBuildArgs() map[string]string
	GetLabels() map[string]string
	GetCustomOptions() map[string]string
	GetTimeout() time.Duration
	IsNoCache() bool
	Validate() error
}

// ValidationContext represents validation context
type ValidationContext struct {
	Context    context.Context
	SessionID  string
	Config     BuildConfig
	Options    BuildOptions
	Metrics    BuildMetrics
	Logs       []string
	Errors     []BuildError
	Warnings   []string
	Timestamp  time.Time
	Stage      BuildStage
	Step       int
	Total      int
	Percentage float64
	Details    map[string]interface{}
}

// Ensure ValidationContext implements ValidationContextInterface
var _ ValidationContextInterface = (*ValidationContext)(nil)

// Implementation of ValidationContextInterface
func (vc *ValidationContext) GetSessionID() string {
	return vc.SessionID
}

func (vc *ValidationContext) GetWorkspaceDir() string {
	return vc.Config.WorkspaceDir
}

func (vc *ValidationContext) GetImageName() string {
	return vc.Config.ImageName
}

func (vc *ValidationContext) GetImageTag() string {
	return vc.Config.ImageTag
}

func (vc *ValidationContext) GetDockerfilePath() string {
	return vc.Config.DockerfilePath
}

func (vc *ValidationContext) GetBuildPath() string {
	return vc.Config.BuildPath
}

func (vc *ValidationContext) GetPlatform() string {
	return vc.Config.Platform
}

func (vc *ValidationContext) GetBuildArgs() map[string]string {
	return vc.Config.BuildArgs
}

func (vc *ValidationContext) GetLabels() map[string]string {
	return vc.Config.Labels
}

func (vc *ValidationContext) GetCustomOptions() map[string]string {
	return vc.Config.CustomOptions
}

func (vc *ValidationContext) GetTimeout() time.Duration {
	return vc.Config.Timeout
}

func (vc *ValidationContext) IsNoCache() bool {
	return vc.Config.NoCache
}

func (vc *ValidationContext) Validate() error {
	validator := core.GetValidator()
	return validator.Struct(vc.Config)
}
