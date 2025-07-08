package build

import (
	"context"
	"time"

	validationcore "github.com/Azure/container-kit/pkg/common/validation-core/core" // For validator only
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/knowledge"
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
	Target         string
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

// CommonBuildError is an alias for BuildError for backward compatibility
type CommonBuildError struct {
	Message   string
	Code      string
	Details   map[string]interface{}
	Timestamp time.Time
	Stage     string
	Type      string // Additional field for error type
	Line      int    // Line number where error occurred
}

// Error implements the error interface for CommonBuildError
func (e *CommonBuildError) Error() string {
	return e.Message
}

// NewCommonBuildError creates a new CommonBuildError
func NewCommonBuildError(code, message, stage, errType string) *CommonBuildError {
	return &CommonBuildError{
		Code:      code,
		Message:   message,
		Stage:     stage,
		Type:      errType,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Line:      0,
	}
}

// BuildValidationResult represents the result of build validation
type BuildValidationResult = core.BuildValidationResult

// Helper functions for tests

// determineImpact determines the impact of a warning type
func determineImpact(warningType string) string {
	switch warningType {
	case "security":
		return "security"
	case "best_practice":
		return "maintainability"
	case "performance":
		return "performance"
	default:
		return "performance"
	}
}

// ConvertCoreResult converts a core validation result to a build validation result
func ConvertCoreResult(coreResult *core.BuildValidationResult) *BuildValidationResult {
	// Since BuildValidationResult is an alias for core.BuildValidationResult,
	// we can just return it directly
	return coreResult
}

// ValidationError represents a validation error
type ValidationError struct {
	Type    string
	Message string
	File    string
	Line    int
	Column  int
	Rule    string
}

// ComplianceViolation represents a compliance standard violation
type ComplianceViolation struct {
	Standard string
	Rule     string
	Message  string
	Line     int
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

// Feature constants are defined in buildkit_strategy.go

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
	validator, exists := validationcore.GetValidator("build")
	if !exists {
		return errors.NewError().Messagef("validator 'build' not found").Build()
	}
	return validator.Validate(vc.Context, vc.Config, nil)
}

// AtomicBuildImageArgs represents arguments for atomic build operations
type AtomicBuildImageArgs struct {
	SessionID      string            `json:"session_id"`
	ImageName      string            `json:"image_name"`
	ImageTag       string            `json:"image_tag"`
	DockerfilePath string            `json:"dockerfile_path"`
	BuildContext   string            `json:"build_context"`
	Platform       string            `json:"platform,omitempty"`
	NoCache        bool              `json:"no_cache,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	PushAfterBuild bool              `json:"push_after_build,omitempty"`
	RegistryURL    string            `json:"registry_url,omitempty"`
	DryRun         bool              `json:"dry_run,omitempty"`
}

// AtomicBuildImageResult represents the result of an atomic build operation
type AtomicBuildImageResult struct {
	SessionID            string                `json:"session_id"`
	ImageName            string                `json:"image_name"`
	ImageTag             string                `json:"image_tag"`
	Platform             string                `json:"platform,omitempty"`
	BuildContext         string                `json:"build_context"`
	DockerfilePath       string                `json:"dockerfile_path"`
	FullImageRef         string                `json:"full_image_ref"`
	WorkspaceDir         string                `json:"workspace_dir"`
	BuildContext_Info    *BuildContextInfo     `json:"build_context_info,omitempty"`
	Success              bool                  `json:"success"`
	BuildDuration        time.Duration         `json:"build_duration"`
	PushDuration         time.Duration         `json:"push_duration"`
	TotalDuration        time.Duration         `json:"total_duration"`
	OptimizationResult   *OptimizationResult   `json:"optimization_result,omitempty"`
	BuildFailureAnalysis *BuildFailureAnalysis `json:"build_failure_analysis,omitempty"`
	PerformanceReport    *PerformanceReport    `json:"performance_report,omitempty"`
	BaseToolResponse     interface{}           `json:"base_tool_response,omitempty"`
	BaseAIContextResult  interface{}           `json:"base_ai_context_result,omitempty"`
}

// BuildContextInfo is defined in build_context.go

// OptimizationResult is defined in build_optimizer.go
// OptimizationRecommendation is defined in build_optimizer.go

// BuildFailureAnalysis represents analysis of build failures
type BuildFailureAnalysis struct {
	ErrorType             string                        `json:"error_type"`
	ErrorMessage          string                        `json:"error_message"`
	Suggestions           []string                      `json:"suggestions"`
	Context               map[string]string             `json:"context"`
	FailureReason         string                        `json:"failure_reason"`
	FailureType           string                        `json:"failure_type"`
	FailureStage          string                        `json:"failure_stage"`
	CommonCauses          []FailureCause                `json:"common_causes"`
	SuggestedFixes        []BuildFix                    `json:"suggested_fixes"`
	AlternativeStrategies []BuildStrategyRecommendation `json:"alternative_strategies"`
	SecurityImplications  []string                      `json:"security_implications"`
	PerformanceImpact     string                        `json:"performance_impact"`
}

// PerformanceReport represents build performance metrics
type PerformanceReport struct {
	TotalTime     time.Duration `json:"total_time"`
	BuildTime     time.Duration `json:"build_time"`
	ContextTime   time.Duration `json:"context_time"`
	NetworkTime   time.Duration `json:"network_time"`
	CacheHitRate  float64       `json:"cache_hit_rate"`
	TotalDuration time.Duration `json:"total_duration"`
}

// PushOptions represents options for Docker push operations
type PushOptions struct {
	Registry string `json:"registry,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Force    bool   `json:"force,omitempty"`
}

// BuildSecurityScanner represents a security scanner for builds
type BuildSecurityScanner struct {
	Logger interface{}
}

// NewBuildSecurityScanner creates a new build security scanner
func NewBuildSecurityScanner(logger interface{}) *BuildSecurityScanner {
	return &BuildSecurityScanner{Logger: logger}
}

// PerformanceMonitor represents a performance monitor for builds
type PerformanceMonitor struct {
	Logger interface{}
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger interface{}) *PerformanceMonitor {
	return &PerformanceMonitor{Logger: logger}
}

// NewBuildMetrics creates a new BuildMetrics instance
func NewBuildMetrics() *BuildMetrics {
	return &BuildMetrics{
		Performance: make(map[string]interface{}),
	}
}

// BuildMonitor represents a build monitor
type BuildMonitor struct {
	Logger interface{}
}

// StartBuildMonitoring starts monitoring a build operation
func (pm *PerformanceMonitor) StartBuildMonitoring(ctx context.Context, operation *BuildOperation) *BuildMonitor {
	return &BuildMonitor{Logger: pm.Logger}
}

// Complete completes the build monitoring
func (bm *BuildMonitor) Complete(success bool, message string, imageInfo *BuiltImageInfo) {
	// Implementation stub
}

// GetReport gets the performance report
func (bm *BuildMonitor) GetReport() *PerformanceReport {
	return &PerformanceReport{}
}

// BuildOperation represents a build operation
type BuildOperation struct {
	Name        string
	Tool        string
	Type        string
	Strategy    string
	SessionID   string
	ContextSize int64
}

// BuiltImageInfo represents information about a built image
type BuiltImageInfo struct {
	Name       string
	Tag        string
	Size       int64
	LayerCount int
}

// BuildStrategy represents a build strategy interface
type BuildStrategy interface {
	Name() string
	Description() string
	CanHandle(context BuildContext) bool
	Build(context BuildContext) (*BuildResult, error)
	Execute(context BuildContext) (*BuildResult, error)
	SupportsFeature(feature string) bool
	Validate(context BuildContext) error
	ScoreCompatibility(info interface{}) int
}

// KnowledgeBase interface for build insights
type KnowledgeBase interface {
	GetBuildInsights(ctx context.Context, input interface{}) ([]string, error)
	StoreInsights(ctx context.Context, insights *ToolInsights) error
}

// DockerClient interface for Docker operations
type DockerClient interface {
	InspectImage(ctx context.Context, imageID string) (*ImageInfo, error)
	BuildImage(ctx context.Context, buildContext string, options BuildOptions) (*BuildResult, error)
	Build(ctx context.Context, options BuildOptions) (*BuildResult, error)
	Push(ctx context.Context, image string, options PushOptions) error
	Tag(ctx context.Context, source, target string) error
}

// ImageInfo represents Docker image information
type ImageInfo struct {
	ID      string    `json:"id"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
	RootFS  struct {
		Layers []string `json:"layers"`
	} `json:"rootfs"`
}

// BuildValidator represents a build validator interface
type BuildValidator interface {
	Validate(context BuildContext) error
}

// ErrorRouter represents an error router
type ErrorRouter struct {
	Logger interface{}
}

// RouteError routes an error to appropriate handlers
func (e *ErrorRouter) RouteError(ctx *ConsolidatedErrorContext) (*RoutingDecision, error) {
	// Simple implementation for now
	return &RoutingDecision{
		Action:     "retry",
		Confidence: 0.5,
		Reasoning:  "Default routing decision",
		Parameters: make(map[string]interface{}),
	}, nil
}

// TypedPipelineOperations represents typed pipeline operations
type TypedPipelineOperations interface {
	BuildImageTyped(ctx context.Context, sessionID string, params core.BuildImageParams) (*core.BuildImageResult, error)
	PushImageTyped(ctx context.Context, sessionID string, params core.PushImageParams) (*core.PushImageResult, error)
	TagImageTyped(ctx context.Context, sessionID string, params core.TagImageParams) (*core.TagImageResult, error)
	GetSessionWorkspace(sessionID string) string
}

// OptimizedBuildStrategy represents an optimized build strategy
type OptimizedBuildStrategy struct {
	Logger           interface{}
	Name             string
	Description      string
	Steps            []*BuildStep
	ExpectedDuration time.Duration
}

// ToolInsights is now in the knowledge package
// This is kept for backward compatibility
type ToolInsights = knowledge.ToolInsights

// FailurePattern is now in the knowledge package
// This is kept for backward compatibility
type FailurePattern = knowledge.FailurePattern

// SharedKnowledge is now in the knowledge package
// This is kept for backward compatibility
type SharedKnowledge = knowledge.SharedKnowledge

// GeneralOptimizationTip is now in the knowledge package
// This is kept for backward compatibility
type GeneralOptimizationTip = knowledge.GeneralOptimizationTip

// AtomicPushImageArgs represents arguments for atomic push operations
type AtomicPushImageArgs struct {
	SessionID   string `json:"session_id"`
	ImageName   string `json:"image_name"`
	ImageTag    string `json:"image_tag"`
	RegistryURL string `json:"registry_url,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Force       bool   `json:"force,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

// AtomicPushImageResult represents the result of an atomic push operation
type AtomicPushImageResult struct {
	SessionID           string        `json:"session_id"`
	ImageName           string        `json:"image_name"`
	ImageTag            string        `json:"image_tag"`
	FullImageRef        string        `json:"full_image_ref"`
	RegistryURL         string        `json:"registry_url"`
	WorkspaceDir        string        `json:"workspace_dir"`
	Success             bool          `json:"success"`
	PushDuration        time.Duration `json:"push_duration"`
	TotalDuration       time.Duration `json:"total_duration"`
	BaseToolResponse    interface{}   `json:"base_tool_response,omitempty"`
	BaseAIContextResult interface{}   `json:"base_ai_context_result,omitempty"`
}

// BuildArgs represents build arguments
type BuildArgs struct {
	Args           map[string]string `json:"args,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	ContextPath    string            `json:"context_path,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	NoCache        bool              `json:"no_cache,omitempty"`
	Target         string            `json:"target,omitempty"`
	ImageName      string            `json:"image_name,omitempty"`
}

// SecurityValidator represents a security validator
type SecurityValidator struct {
	Logger interface{}
}

// ProgressReporter represents a progress reporter
type ProgressReporter interface {
	Report(message string, progress int, total int)
}

// SecurityPolicy is defined in security_types.go
// PolicyViolation is defined in security_types.go

// These types and constructors are defined in their respective files:
// - BuildAnalyzer and NewBuildAnalyzer in build_analysis.go
// - BuildValidatorImpl and NewBuildValidator in build_validator.go
// - BuildTroubleshooter and NewBuildTroubleshooter in build_troubleshooting.go
// - BuildOptimizer and NewBuildOptimizer in build_optimizer.go

// ComplianceFramework is defined in security_types.go

// ImageReference represents a Docker image reference
type ImageReference struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
	FullRef    string `json:"full_ref"`
}

// ToolError represents a tool error
type ToolError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ExecutionResult represents the result of an execution
type ExecutionResult struct {
	Success     bool                   `json:"success"`
	Data        interface{}            `json:"data,omitempty"`
	Error       *ToolError             `json:"error,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	Performance *PerformanceReport     `json:"performance,omitempty"`
}

// AdvancedBuildFixer represents an advanced build fixer
type AdvancedBuildFixer struct {
	Logger     interface{}
	strategies []interface{}
}

// NewAdvancedBuildFixerUnified creates a new advanced build fixer
func NewAdvancedBuildFixerUnified(logger interface{}, analyzer ...interface{}) *AdvancedBuildFixer {
	return &AdvancedBuildFixer{
		Logger:     logger,
		strategies: make([]interface{}, 0),
	}
}

// RegisterStrategy registers a recovery strategy
func (a *AdvancedBuildFixer) RegisterStrategy(name string, strategy interface{}) {
	a.strategies = append(a.strategies, strategy)
}

// NetworkErrorRecoveryStrategy represents a network error recovery strategy
type NetworkErrorRecoveryStrategy struct {
	Logger interface{}
}

// NewNetworkErrorRecoveryStrategy creates a new network error recovery strategy
func NewNetworkErrorRecoveryStrategy(logger interface{}) *NetworkErrorRecoveryStrategy {
	return &NetworkErrorRecoveryStrategy{Logger: logger}
}

// PermissionErrorRecoveryStrategy represents a permission error recovery strategy
type PermissionErrorRecoveryStrategy struct {
	Logger interface{}
}

// NewPermissionErrorRecoveryStrategy creates a new permission error recovery strategy
func NewPermissionErrorRecoveryStrategy(logger interface{}) *PermissionErrorRecoveryStrategy {
	return &PermissionErrorRecoveryStrategy{Logger: logger}
}

// DockerfileErrorRecoveryStrategy represents a dockerfile error recovery strategy
type DockerfileErrorRecoveryStrategy struct {
	Logger interface{}
}

// NewDockerfileErrorRecoveryStrategy creates a new dockerfile error recovery strategy
func NewDockerfileErrorRecoveryStrategy(logger interface{}) *DockerfileErrorRecoveryStrategy {
	return &DockerfileErrorRecoveryStrategy{Logger: logger}
}

// DependencyErrorRecoveryStrategy represents a dependency error recovery strategy
type DependencyErrorRecoveryStrategy struct {
	Logger interface{}
}

// NewDependencyErrorRecoveryStrategy creates a new dependency error recovery strategy
func NewDependencyErrorRecoveryStrategy(logger interface{}) *DependencyErrorRecoveryStrategy {
	return &DependencyErrorRecoveryStrategy{Logger: logger}
}

// NewAtomicDockerBuildOperation creates a new atomic docker build operation
func NewAtomicDockerBuildOperation(config interface{}) (*AtomicDockerBuildOperation, error) {
	return &AtomicDockerBuildOperation{}, nil
}

// ConsolidatedFixableOperation represents a consolidated fixable operation
type ConsolidatedFixableOperation interface {
	Execute(ctx context.Context) error
	ExecuteOnce(ctx context.Context) error
	GetFailureAnalysis(ctx context.Context, err error) (*ConsolidatedFailureAnalysis, error)
	PrepareForRetry(ctx context.Context, fixAttempt interface{}) error
}

// ConsolidatedFailureAnalysis represents consolidated failure analysis
type ConsolidatedFailureAnalysis struct {
	FailureType              string   `json:"failure_type"`
	IsCritical               bool     `json:"is_critical"`
	IsRetryable              bool     `json:"is_retryable"`
	RootCauses               []string `json:"root_causes"`
	SuggestedFixes           []string `json:"suggested_fixes"`
	ConsolidatedErrorContext string   `json:"consolidated_error_context"`
}

// Error implements the error interface
func (cfa *ConsolidatedFailureAnalysis) Error() string {
	return cfa.ConsolidatedErrorContext
}

// FixingResult represents a fixing result
type FixingResult struct {
	Success       bool          `json:"success"`
	Fixed         bool          `json:"fixed"`
	Changes       []string      `json:"changes,omitempty"`
	Error         *ToolError    `json:"error,omitempty"`
	TotalAttempts int           `json:"total_attempts"`
	AllAttempts   []interface{} `json:"all_attempts"`
	Duration      time.Duration `json:"duration"`
	AttemptsUsed  int           `json:"attempts_used"`
}

// DefaultIterativeFixer is defined in iterative_fixer.go
// DefaultContextSharer is defined in context_sharer.go

// ConsolidatedLocalProgressStage represents a consolidated local progress stage
type ConsolidatedLocalProgressStage struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// FixRequest represents a request for fixing
type FixRequest struct {
	SessionID     string
	ToolName      string
	OperationType string
	Error         error
	MaxAttempts   int
	BaseDir       string
}

// AtomicDockerBuildOperation represents an atomic Docker build operation
type AtomicDockerBuildOperation struct {
	SessionID      string
	ImageName      string
	BuildContext   string
	DockerfilePath string
}

// BuildOptimizationRequest represents a build optimization request
type BuildOptimizationRequest struct {
	SessionID      string
	DockerfilePath string
	BuildContext   string
	ImageName      string
	Options        map[string]interface{}
	ProjectType    string
	Goals          *OptimizationGoals
	Constraints    *BuildConstraints
}

// FailureCause represents a failure cause
type FailureCause struct {
	Type        string
	Message     string
	Severity    string
	Suggestions []string
	Description string
	Likelihood  string
	Evidence    []string
}

// BuildFix represents a build fix
type BuildFix struct {
	Type        string
	Description string
	Steps       []string
	Difficulty  string
	Priority    string
	Commands    []string
	Command     string // Single command variant
	Validation  string
}

// BuildFixerOptions represents options for the build fixer
type BuildFixerOptions struct {
	NetworkTimeout    int
	NetworkRetries    int
	NetworkRetryDelay time.Duration
	ForceRootUser     bool
	NoCache           bool
	ForceRM           bool
	Squash            bool
}

// BuildStrategyRecommendation represents a build strategy recommendation
type BuildStrategyRecommendation struct {
	Strategy    string
	Confidence  float64
	Reasoning   string
	Benefits    []string
	Drawbacks   []string
	Description string
	Name        string
	Complexity  string
	Example     string
}

// BuildFixerPerformanceAnalysis represents build fixer performance analysis
type BuildFixerPerformanceAnalysis struct {
	FixTime         time.Duration
	SuccessRate     float64
	CommonFixes     []string
	Metrics         map[string]interface{}
	CacheEfficiency string
	BuildTime       time.Duration
	CacheHitRate    float64
	ImageSize       int64
	Optimizations   []string
	Bottlenecks     []string
}

// ComplianceResult represents a compliance result
type ComplianceResult struct {
	Passed          bool          `json:"passed"`
	Compliant       bool          `json:"compliant"`
	Score           float64       `json:"score"`
	Issues          []string      `json:"issues"`
	Violations      []interface{} `json:"violations"`
	Recommendations []string      `json:"recommendations"`
}

// RoutingDecision represents a routing decision
type RoutingDecision struct {
	Action     string                 `json:"action"`
	Confidence float64                `json:"confidence"`
	Reasoning  string                 `json:"reasoning"`
	Parameters map[string]interface{} `json:"parameters"`
}

// AnalysisRequest is now in the knowledge package
// This is kept for backward compatibility
type AnalysisRequest = knowledge.AnalysisRequest

// RelatedFailure is now in the knowledge package
// This is kept for backward compatibility
type RelatedFailure = knowledge.RelatedFailure

// AggregatedMetrics is now in the knowledge package
// This is kept for backward compatibility
type AggregatedMetrics = knowledge.AggregatedMetrics

// Recommendation represents a recommendation
type Recommendation struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
}

// AlternativeStrategy represents an alternative strategy
type AlternativeStrategy struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
}

// ConsolidatedErrorContext represents consolidated error context
type ConsolidatedErrorContext struct {
	SessionID      string                 `json:"session_id"`
	SourceTool     string                 `json:"source_tool"`
	ErrorType      string                 `json:"error_type"`
	ErrorCode      string                 `json:"error_code"`
	ErrorMessage   string                 `json:"error_message"`
	Timestamp      time.Time              `json:"timestamp"`
	ExecutionTrace []string               `json:"execution_trace,omitempty"`
	ToolContext    map[string]interface{} `json:"tool_context,omitempty"`
	Context        map[string]interface{} `json:"context,omitempty"`
	Stack          []string               `json:"stack,omitempty"`
}
