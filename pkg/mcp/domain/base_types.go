package domain

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// BaseToolResponse is an alias to avoid import cycles
type BaseToolResponse = domaintypes.BaseToolResponse

// ConversationStage is an alias for the conversation stage from internal types
type ConversationStage = domaintypes.ConversationStage

// ConversationStage constants
const (
	ConversationStageWelcome    ConversationStage = domaintypes.StageWelcome
	ConversationStagePreFlight  ConversationStage = domaintypes.StagePreFlight
	ConversationStageInit       ConversationStage = domaintypes.StageInit
	ConversationStageAnalyze    ConversationStage = domaintypes.StageAnalysis
	ConversationStageAnalysis   ConversationStage = domaintypes.StageAnalysis // Alias for backward compatibility
	ConversationStageDockerfile ConversationStage = domaintypes.StageDockerfile
	ConversationStageBuild      ConversationStage = domaintypes.StageBuild
	ConversationStagePush       ConversationStage = domaintypes.StagePush
	ConversationStageManifests  ConversationStage = domaintypes.StageManifests
	ConversationStageDeploy     ConversationStage = domaintypes.StageDeployment
	ConversationStageScan       ConversationStage = domaintypes.StageScan
	ConversationStageCompleted  ConversationStage = domaintypes.StageCompleted
	ConversationStageError      ConversationStage = domaintypes.StageError
)

// ImageReference is an alias for image reference from internal types
type ImageReference = domaintypes.ImageReference

// ToolError is an alias for tool error from internal types
type ToolError = domaintypes.ToolError

// ExecutionResult is an alias for execution result from internal types
type ExecutionResult = domaintypes.ExecutionResult

// NewBaseResponse creates a new BaseToolResponse with current timestamp
func NewBaseResponse(success bool, message string) BaseToolResponse {
	response := domaintypes.NewBaseResponse("", "", false)
	response.Success = success
	response.Message = message
	return response
}

// NewToolResponse creates a tool response with current metadata
func NewToolResponse(tool, sessionID string, dryRun bool) BaseToolResponse {
	return domaintypes.NewBaseResponse(tool, sessionID, dryRun)
}

// Recommendation represents an AI recommendation
// Moved from core package to break import cycles
type Recommendation struct {
	Type        string            `json:"type"`
	Priority    int               `json:"priority"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Action      string            `json:"action"`
	Metadata    map[string]string `json:"metadata"`
}

// AlternativeStrategy represents an alternative approach or strategy
// Moved from core package to break import cycles
type AlternativeStrategy struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
}

// ToolExample represents tool usage example
// Moved from core package to break import cycles
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// RepositoryInfo represents information about a repository
type RepositoryInfo struct {
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Language       string            `json:"language,omitempty"`
	Framework      string            `json:"framework,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	BuildTool      string            `json:"build_tool,omitempty"`
	PackageManager string            `json:"package_manager,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// DockerfileInfo represents information about a Dockerfile
type DockerfileInfo struct {
	Path         string            `json:"path"`
	BaseImage    string            `json:"base_image,omitempty"`
	Instructions []string          `json:"instructions,omitempty"`
	ExposedPorts []string          `json:"exposed_ports,omitempty"`
	WorkDir      string            `json:"work_dir,omitempty"`
	User         string            `json:"user,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// BuildRecommendations represents build recommendations
type BuildRecommendations struct {
	OptimizationSuggestions []Recommendation `json:"optimization_suggestions,omitempty"`
	SecurityRecommendations []Recommendation `json:"security_recommendations,omitempty"`
	PerformanceTips         []Recommendation `json:"performance_tips,omitempty"`
	BestPractices           []Recommendation `json:"best_practices,omitempty"`
}

// TokenUsage represents token usage for AI operations
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// BaseAIContextResult provides common AI context implementations for all atomic tool results
type BaseAIContextResult struct {
	IsSuccessful  bool          `json:"is_successful"`
	Duration      time.Duration `json:"duration"`
	OperationType string        `json:"operation_type"`
	ErrorCount    int           `json:"error_count"`
	WarningCount  int           `json:"warning_count"`
}

// NewBaseAIContextResult creates a new BaseAIContextResult
func NewBaseAIContextResult(operationType string, isSuccessful bool, duration time.Duration) BaseAIContextResult {
	return BaseAIContextResult{
		IsSuccessful:  isSuccessful,
		Duration:      duration,
		OperationType: operationType,
		ErrorCount:    0,
		WarningCount:  0,
	}
}

// Docker operation params and results
type BuildImageParams struct {
	SessionID      string            `json:"session_id"`
	DockerfilePath string            `json:"dockerfile_path"`
	ImageName      string            `json:"image_name"`
	ImageTag       string            `json:"image_tag,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	ContextPath    string            `json:"context_path,omitempty"`
	BuildContext   string            `json:"build_context,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	Pull           bool              `json:"pull,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	NoCache        bool              `json:"no_cache,omitempty"`
}

// Validate validates the BuildImageParams
func (p BuildImageParams) Validate() error {
	if p.SessionID == "" {
		return errors.MissingParameterError("session_id")
	}
	if p.DockerfilePath == "" {
		return errors.MissingParameterError("dockerfile_path")
	}
	if p.ImageName == "" {
		return errors.MissingParameterError("image_name")
	}
	return nil
}

type BuildImageResult struct {
	BaseToolResponse
	ImageID   string        `json:"image_id"`
	ImageName string        `json:"image_name"`
	Tags      []string      `json:"tags,omitempty"`
	BuildTime time.Duration `json:"build_time,omitempty"`
}

type PushImageParams struct {
	ImageName  string `json:"image_name"`
	ImageRef   string `json:"image_ref,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Registry   string `json:"registry,omitempty"`
	Repository string `json:"repository,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	Force      bool   `json:"force,omitempty"`
}

type PushImageResult struct {
	BaseToolResponse
	ImageName string `json:"image_name"`
	Registry  string `json:"registry"`
	Digest    string `json:"digest"`
}

type PullImageParams struct {
	ImageName string `json:"image_name"`
	ImageRef  string `json:"image_ref,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Registry  string `json:"registry,omitempty"`
}

type PullImageResult struct {
	BaseToolResponse
	ImageName string        `json:"image_name"`
	ImageID   string        `json:"image_id"`
	PullTime  time.Duration `json:"pull_time,omitempty"`
}

type TagImageParams struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

type TagImageResult struct {
	BaseToolResponse
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

// Kubernetes operation params and results
type GenerateManifestsParams struct {
	ImageName   string            `json:"image_name"`
	ImageRef    string            `json:"image_ref,omitempty"`
	ServiceName string            `json:"service_name"`
	AppName     string            `json:"app_name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Port        int               `json:"port,omitempty"`
	Replicas    int               `json:"replicas,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Resources   ResourceLimits    `json:"resources,omitempty"`
	HealthCheck HealthCheckConfig `json:"health_check,omitempty"`
}

type GenerateManifestsResult struct {
	BaseToolResponse
	ManifestPaths []string `json:"manifest_paths"`
	ManifestCount int      `json:"manifest_count"`
	Namespace     string   `json:"namespace"`
}

type DeployParams struct {
	SessionID     string        `json:"session_id"`
	ManifestPath  string        `json:"manifest_path"`
	ManifestPaths []string      `json:"manifest_paths,omitempty"`
	Namespace     string        `json:"namespace,omitempty"`
	Wait          bool          `json:"wait,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	DryRun        bool          `json:"dry_run,omitempty"`
}

// Validate validates the DeployParams
func (p DeployParams) Validate() error {
	if p.SessionID == "" {
		return errors.MissingParameterError("session_id")
	}
	if p.ManifestPath == "" && len(p.ManifestPaths) == 0 {
		return errors.MissingParameterError("manifest_path")
	}
	return nil
}

type DeployResult struct {
	BaseToolResponse
	DeploymentName string                 `json:"deployment_name"`
	ServiceName    string                 `json:"service_name"`
	Namespace      string                 `json:"namespace"`
	Status         string                 `json:"status"`
	Endpoints      []string               `json:"endpoints,omitempty"`
	DeploymentTime time.Time              `json:"deployment_time,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
	Errors         []string               `json:"errors,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
}

type HealthCheckParams struct {
	DeploymentName string `json:"deployment_name"`
	AppName        string `json:"app_name"`
	Namespace      string `json:"namespace"`
	Timeout        int    `json:"timeout,omitempty"`
	WaitTimeout    int    `json:"wait_timeout,omitempty"`
}

type HealthCheckResult struct {
	BaseToolResponse
	Healthy          bool     `json:"healthy"`
	ReadyReplicas    int      `json:"ready_replicas"`
	TotalReplicas    int      `json:"total_replicas"`
	Status           string   `json:"status"`
	OverallHealth    string   `json:"overall_health"`
	ResourceStatuses []string `json:"resource_statuses"`
	Error            string   `json:"error,omitempty"`
	StatusCode       int      `json:"status_code,omitempty"`
	Checked          bool     `json:"checked"`
	Endpoint         string   `json:"endpoint,omitempty"`
}

// Analysis operation params and results
type AnalyzeParams struct {
	Path           string   `json:"path,omitempty"`
	RepositoryPath string   `json:"repository_path,omitempty"`
	Language       string   `json:"language,omitempty"`
	Framework      string   `json:"framework,omitempty"`
	IncludeFiles   []string `json:"include_files,omitempty"`
	ExcludeFiles   []string `json:"exclude_files,omitempty"`
	DeepAnalysis   bool     `json:"deep_analysis,omitempty"`
}

type AnalyzeResult struct {
	BaseToolResponse
	RepositoryInfo       RepositoryInfo       `json:"repository_info"`
	DockerfileInfo       DockerfileInfo       `json:"dockerfile_info,omitempty"`
	BuildRecommendations BuildRecommendations `json:"recommendations,omitempty"`
	Recommendations      BuildRecommendations `json:"build_recommendations,omitempty"`
}

type ValidateParams struct {
	DockerfilePath string   `json:"dockerfile_path"`
	Strict         bool     `json:"strict,omitempty"`
	StrictMode     bool     `json:"strict_mode,omitempty"`
	Rules          []string `json:"rules,omitempty"`
}

type ConsolidatedValidateResult struct {
	BaseToolResponse
	Valid         bool     `json:"valid"`
	Score         float64  `json:"score,omitempty"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	BestPractices []string `json:"best_practices,omitempty"`
}

// Security operation params and results
type ConsolidatedScanParams struct {
	SessionID         string   `json:"session_id,omitempty"`
	ImageRef          string   `json:"image_ref,omitempty"`
	ImageName         string   `json:"image_name"`
	ScanType          string   `json:"scan_type,omitempty"`
	SeverityThreshold string   `json:"severity_threshold,omitempty"`
	SeverityFilter    string   `json:"severity_filter,omitempty"`
	VulnTypes         []string `json:"vuln_types,omitempty"`
	OutputFile        string   `json:"output_file,omitempty"`
}

// Validate validates the ConsolidatedScanParams
func (p ConsolidatedScanParams) Validate() error {
	if p.ImageName == "" && p.ImageRef == "" {
		return errors.MissingParameterError("image_name")
	}
	return nil
}

type ScanResult struct {
	BaseToolResponse
	VulnerabilityCount   int                    `json:"vulnerability_count"`
	CriticalCount        int                    `json:"critical_count"`
	HighCount            int                    `json:"high_count"`
	MediumCount          int                    `json:"medium_count"`
	LowCount             int                    `json:"low_count"`
	Vulnerabilities      []string               `json:"vulnerabilities,omitempty"`
	ScanReport           map[string]interface{} `json:"scan_report,omitempty"`
	VulnerabilityDetails []interface{}          `json:"vulnerability_details,omitempty"`
}

type ScanSecretsParams struct {
	Path            string   `json:"path,omitempty"`
	FilePatterns    []string `json:"file_patterns,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	Recursive       bool     `json:"recursive,omitempty"`
	FileTypes       []string `json:"file_types,omitempty"`
	ExcludeDirs     []string `json:"exclude_dirs,omitempty"`
}

type ScanSecretsResult struct {
	BaseToolResponse
	SecretsFound int      `json:"secrets_found"`
	FilesScanned int      `json:"files_scanned"`
	Files        []string `json:"files,omitempty"`
	Secrets      []string `json:"secrets,omitempty"`
}

// Missing core types needed by kubernetes_operations_test.go
type ResourceSpec struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

type ResourceLimits struct {
	Limits   ResourceSpec `json:"limits,omitempty"`
	Requests ResourceSpec `json:"requests,omitempty"`
}

type HealthCheckConfig struct {
	Enabled             bool   `json:"enabled,omitempty"`
	Path                string `json:"path,omitempty"`
	Port                int    `json:"port,omitempty"`
	InitialDelaySeconds int    `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int    `json:"period_seconds,omitempty"`
	TimeoutSeconds      int    `json:"timeout_seconds,omitempty"`
	FailureThreshold    int    `json:"failure_threshold,omitempty"`
}

// Missing DomainAnalyzer type for partial_analyzers.go
type DomainAnalyzer interface {
	Analyze(path string) (*AnalyzeResult, error)
	Validate(params ValidateParams) error
}

// Missing GenerateDockerfileResult type for generate_dockerfile.go
type GenerateDockerfileResult struct {
	BaseToolResponse
	DockerfilePath string   `json:"dockerfile_path"`
	Content        string   `json:"content"`
	BaseImage      string   `json:"base_image,omitempty"`
	Instructions   []string `json:"instructions,omitempty"`
	Optimizations  []string `json:"optimizations,omitempty"`
}
