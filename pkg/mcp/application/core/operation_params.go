package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// BuildImageParams represents parameters for build operations
type BuildImageParams struct {
	SessionID      string            `json:"session_id"`
	DockerfilePath string            `json:"dockerfile_path"`
	ContextPath    string            `json:"context_path"`
	ImageName      string            `json:"image_name"`
	Tags           []string          `json:"tags"`
	BuildArgs      map[string]string `json:"build_args"`
	NoCache        bool              `json:"no_cache"`
	Pull           bool              `json:"pull"`
}

// Validate implements ToolParams interface
func (p BuildImageParams) Validate() error {
	if p.SessionID == "" {
		return errors.MissingParameterError("session_id")
	}
	if p.DockerfilePath == "" {
		return errors.MissingParameterError("dockerfile_path")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p BuildImageParams) GetSessionID() string {
	return p.SessionID
}

// BuildImageResult represents the result of a build operation
type BuildImageResult struct {
	types.BaseToolResponse
	ImageID    string   `json:"image_id"`
	ImageRef   string   `json:"image_ref"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	BuildTime  float64  `json:"build_time_seconds"`
	LayerCount int      `json:"layer_count"`
}

// PushImageParams represents parameters for push operations
type PushImageParams struct {
	ImageRef   string `json:"image_ref"`
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

// PushImageResult represents the result of a push operation
type PushImageResult struct {
	types.BaseToolResponse
	ImageRef  string  `json:"image_ref"`
	Registry  string  `json:"registry"`
	PushTime  float64 `json:"push_time_seconds"`
	ImageSize int64   `json:"image_size"`
}

// PullImageParams represents parameters for pull operations
type PullImageParams struct {
	ImageRef string `json:"image_ref"`
	Platform string `json:"platform,omitempty"`
}

// PullImageResult represents the result of a pull operation
type PullImageResult struct {
	types.BaseToolResponse
	ImageRef  string  `json:"image_ref"`
	ImageID   string  `json:"image_id"`
	PullTime  float64 `json:"pull_time_seconds"`
	ImageSize int64   `json:"image_size"`
}

// TagImageParams represents parameters for tag operations
type TagImageParams struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

// TagImageResult represents the result of a tag operation
type TagImageResult struct {
	types.BaseToolResponse
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

// GenerateManifestsParams represents parameters for manifest generation
type GenerateManifestsParams struct {
	ImageRef    string            `json:"image_ref"`
	AppName     string            `json:"app_name"`
	Namespace   string            `json:"namespace"`
	Port        int               `json:"port"`
	Replicas    int               `json:"replicas"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Resources   ResourceLimits    `json:"resources"`
	HealthCheck HealthCheckConfig `json:"health_check"`
}

// GenerateManifestsResult represents the result of manifest generation
type GenerateManifestsResult struct {
	types.BaseToolResponse
	ManifestPaths []string `json:"manifest_paths"`
	ManifestCount int      `json:"manifest_count"`
	Resources     []string `json:"resources"`
	Warnings      []string `json:"warnings"`
}

// DeployParams represents parameters for deployment operations
type DeployParams struct {
	SessionID     string   `json:"session_id"`
	ManifestPaths []string `json:"manifest_paths"`
	Namespace     string   `json:"namespace"`
	DryRun        bool     `json:"dry_run"`
	Wait          bool     `json:"wait"`
	Timeout       int      `json:"timeout_seconds"`
}

// Validate implements ToolParams interface
func (p DeployParams) Validate() error {
	if p.SessionID == "" {
		return errors.MissingParameterError("session_id")
	}
	if len(p.ManifestPaths) == 0 {
		return errors.MissingParameterError("manifest_paths")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p DeployParams) GetSessionID() string {
	return p.SessionID
}

// DeployResult represents the result of a deployment operation
type DeployResult struct {
	types.BaseToolResponse
	DeployedResources []string `json:"deployed_resources"`
	Namespace         string   `json:"namespace"`
	Status            string   `json:"status"`
	Warnings          []string `json:"warnings"`
}

// HealthCheckParams represents parameters for health check operations
type HealthCheckParams struct {
	Namespace   string   `json:"namespace"`
	AppName     string   `json:"app_name"`
	Resources   []string `json:"resources"`
	WaitTimeout int      `json:"wait_timeout_seconds"`
}

// HealthCheckResult represents the result of a health check operation
type HealthCheckResult struct {
	types.BaseToolResponse
	HealthyResources   []string          `json:"healthy_resources"`
	UnhealthyResources []string          `json:"unhealthy_resources"`
	ResourceStatuses   map[string]string `json:"resource_statuses"`
	OverallHealth      string            `json:"overall_health"`
}

// AnalyzeParams represents parameters for analysis operations
type AnalyzeParams struct {
	RepositoryPath string   `json:"repository_path"`
	IncludeFiles   []string `json:"include_files"`
	ExcludeFiles   []string `json:"exclude_files"`
	DeepAnalysis   bool     `json:"deep_analysis"`
}

// AnalyzeResult represents the result of an analysis operation
type AnalyzeResult struct {
	types.BaseToolResponse
	RepositoryInfo   *RepositoryInfo        `json:"repository_info"`
	Recommendations  []types.Recommendation `json:"recommendations"`
	SecurityIssues   []string               `json:"security_issues"`
	PerformanceHints []string               `json:"performance_hints"`
}

// ValidateParams represents parameters for validation operations
type ValidateParams struct {
	DockerfilePath string   `json:"dockerfile_path"`
	Rules          []string `json:"rules"`
	StrictMode     bool     `json:"strict_mode"`
}

// ScanSecretsParams represents parameters for secrets scan operations
type ScanSecretsParams struct {
	Path        string   `json:"path"`
	Recursive   bool     `json:"recursive"`
	FileTypes   []string `json:"file_types"`
	ExcludeDirs []string `json:"exclude_dirs"`
}

// ScanSecretsResult represents the result of a secrets scan operation
type ScanSecretsResult struct {
	types.BaseToolResponse
	SecretsFound []ConsolidatedSecretFinding `json:"secrets_found"`
	FilesScanned int                         `json:"files_scanned"`
	SecretTypes  []string                    `json:"secret_types"`
	RiskLevel    string                      `json:"risk_level"`
}

// ResourceLimits represents resource limits and requests
type ResourceLimits struct {
	Requests ResourceSpec `json:"requests"`
	Limits   ResourceSpec `json:"limits"`
}

// ResourceSpec represents CPU and memory specifications
type ResourceSpec struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled             bool   `json:"enabled"`
	Path                string `json:"path"`
	Port                int    `json:"port"`
	InitialDelaySeconds int    `json:"initial_delay_seconds"`
	PeriodSeconds       int    `json:"period_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	FailureThreshold    int    `json:"failure_threshold"`
}

// ConsolidatedFixableOperation represents an operation that can be fixed when it fails
type ConsolidatedFixableOperation interface {
	Execute(ctx context.Context) error
	ExecuteOnce(ctx context.Context) error
	CanRetry(err error) bool
	GetFailureAnalysis(ctx context.Context, err error) (*ConsolidatedFailureAnalysis, error)
	PrepareForRetry(ctx context.Context, fixAttempt interface{}) error
}

// ConsolidatedFailureAnalysis represents analysis of an operation failure
type ConsolidatedFailureAnalysis struct {
	FailureType              string   `json:"failure_type"`
	IsCritical               bool     `json:"is_critical"`
	IsRetryable              bool     `json:"is_retryable"`
	RootCauses               []string `json:"root_causes"`
	SuggestedFixes           []string `json:"suggested_fixes"`
	ConsolidatedErrorContext string   `json:"error_context"`
}

// Error implements the error interface for ConsolidatedFailureAnalysis
func (fa *ConsolidatedFailureAnalysis) Error() string {
	if fa == nil {
		return "failure analysis: <nil>"
	}
	return fmt.Sprintf("failure analysis: %s (%s)", fa.FailureType, fa.ConsolidatedErrorContext)
}

// ConsolidatedErrorContext provides contextual information about errors
type ConsolidatedErrorContext struct {
	SessionID     string            `json:"session_id"`
	OperationType string            `json:"operation_type"`
	Phase         string            `json:"phase"`
	ErrorCode     string            `json:"error_code"`
	Metadata      map[string]string `json:"metadata"`
	Timestamp     time.Time         `json:"timestamp"`
}

// ConsolidatedLocalProgressStage represents a local progress stage
type ConsolidatedLocalProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Progress    int     `json:"progress"`
	Status      string  `json:"status"`
	Weight      float64 `json:"weight"`
}

// ConsolidatedConversationConfig holds configuration for conversation mode
type ConsolidatedConversationConfig struct {
	EnableTelemetry          bool
	TelemetryPort            int
	PreferencesDBPath        string
	PreferencesEncryptionKey string

	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ConsolidatedConversationStage represents different stages of conversation
type ConsolidatedConversationStage string

const (
	ConsolidatedConversationStagePreFlight  ConsolidatedConversationStage = "preflight"
	ConsolidatedConversationStageAnalyze    ConsolidatedConversationStage = "analyze"
	ConsolidatedConversationStageDockerfile ConsolidatedConversationStage = "dockerfile"
	ConsolidatedConversationStageBuild      ConsolidatedConversationStage = "build"
	ConsolidatedConversationStagePush       ConsolidatedConversationStage = "push"
	ConsolidatedConversationStageManifests  ConsolidatedConversationStage = "manifests"
	ConsolidatedConversationStageDeploy     ConsolidatedConversationStage = "deploy"
	ConsolidatedConversationStageScan       ConsolidatedConversationStage = "scan"
	ConsolidatedConversationStageCompleted  ConsolidatedConversationStage = "completed"
	ConsolidatedConversationStageError      ConsolidatedConversationStage = "error"
)

// ConsolidatedValidateResult represents the result of a validation operation
type ConsolidatedValidateResult struct {
	types.BaseToolResponse
	Valid       bool     `json:"valid"`
	Score       float64  `json:"score"`
	Suggestions []string `json:"suggestions"`
}

// ConsolidatedScanParams represents parameters for security scan operations
type ConsolidatedScanParams struct {
	SessionID      string `json:"session_id"`
	ImageRef       string `json:"image_ref"`
	ScanType       string `json:"scan_type"`
	OutputFile     string `json:"output_file,omitempty"`
	SeverityFilter string `json:"severity_filter,omitempty"`
}

// Validate implements ToolParams interface
func (p ConsolidatedScanParams) Validate() error {
	if p.SessionID == "" {
		return errors.MissingParameterError("session_id")
	}
	if p.ImageRef == "" {
		return errors.MissingParameterError("image_ref")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p ConsolidatedScanParams) GetSessionID() string {
	return p.SessionID
}

// ScanResult represents the result of a security scan operation
type ScanResult struct {
	types.BaseToolResponse
	ScanReport           *SecurityScanResult `json:"scan_report"`
	VulnerabilityDetails []SecurityFinding   `json:"vulnerability_details"`
	ComplianceIssues     []string            `json:"compliance_issues"`
	ReportPath           string              `json:"report_path"`
}

// ConsolidatedSecretFinding represents a detected secret
type ConsolidatedSecretFinding struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
	RuleID      string `json:"rule_id"`
}
