package types

import (
	"fmt"
	"time"
)

// Version constants for schema evolution
const (
	CurrentSchemaVersion = "v1.0.0"
	ToolAPIVersion       = "2024.12.17"
)

// BaseToolResponse provides common response structure for all tools
type BaseToolResponse struct {
	Version   string    `json:"version"`
	Tool      string    `json:"tool"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	DryRun    bool      `json:"dry_run"`
}

// BaseToolArgs provides common arguments for all tools
type BaseToolArgs struct {
	DryRun    bool   `json:"dry_run,omitempty" description:"Preview changes without executing"`
	SessionID string `json:"session_id,omitempty" description:"Session ID for state correlation"`
}

// NewBaseResponse creates a base response with current metadata
func NewBaseResponse(tool, sessionID string, dryRun bool) BaseToolResponse {
	return BaseToolResponse{
		Version:   CurrentSchemaVersion,
		Tool:      tool,
		Timestamp: time.Now(),
		SessionID: sessionID,
		DryRun:    dryRun,
	}
}

// ImageReference provides normalized image referencing across tools
type ImageReference struct {
	Registry   string `json:"registry,omitempty"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
}

func (ir ImageReference) String() string {
	result := ir.Repository
	if ir.Registry != "" {
		result = ir.Registry + "/" + result
	}
	if ir.Tag != "" {
		result += ":" + ir.Tag
	}
	if ir.Digest != "" {
		result += "@" + ir.Digest
	}
	return result
}

// ResourceRequests defines Kubernetes resource requirements
type ResourceRequests struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// SecretRef defines references to secrets in Kubernetes manifests
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Env  string `json:"env"`
}

// PortForward defines port forwarding for Kind cluster testing
type PortForward struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Service    string `json:"service,omitempty"`
	Pod        string `json:"pod,omitempty"`
}

// ResourceUtilization tracks system resource usage
type ResourceUtilization struct {
	CPU         float64 `json:"cpu_percent"`
	Memory      float64 `json:"memory_percent"`
	Disk        float64 `json:"disk_percent"`
	DiskFree    int64   `json:"disk_free_bytes"`
	LoadAverage float64 `json:"load_average"`
}

// ServiceHealth tracks health of external services
type ServiceHealth struct {
	Status       string        `json:"status"`
	LastCheck    time.Time     `json:"last_check"`
	ResponseTime time.Duration `json:"response_time,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// RepositoryScanSummary summarizes repository analysis results
type RepositoryScanSummary struct {
	// Core analysis results
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Port         int      `json:"port"`
	Dependencies []string `json:"dependencies"`

	// File structure insights
	FilesAnalyzed    int      `json:"files_analyzed"`
	ConfigFilesFound []string `json:"config_files_found"`
	EntryPointsFound []string `json:"entry_points_found"`
	TestFilesFound   []string `json:"test_files_found"`
	BuildFilesFound  []string `json:"build_files_found"`

	// Ecosystem insights
	PackageManagers []string `json:"package_managers"`
	DatabaseFiles   []string `json:"database_files"`
	DockerFiles     []string `json:"docker_files"`
	K8sFiles        []string `json:"k8s_files"`

	// Repository metadata
	Branch             string   `json:"branch,omitempty"`
	LastCommit         string   `json:"last_commit,omitempty"`
	ReadmeFound        bool     `json:"readme_found"`
	LicenseType        string   `json:"license_type,omitempty"`
	DocumentationFound []string `json:"documentation_found"`
	HasGitIgnore       bool     `json:"has_gitignore"`
	HasReadme          bool     `json:"has_readme"`
	HasLicense         bool     `json:"has_license"`
	HasCI              bool     `json:"has_ci"`
	RepositorySize     int64    `json:"repository_size_bytes"`

	// Cache metadata
	CachedAt         time.Time `json:"cached_at"`
	AnalysisDuration float64   `json:"analysis_duration_seconds"`
	RepoPath         string    `json:"repo_path"`
	RepoURL          string    `json:"repo_url,omitempty"`

	// Suggestions for reuse
	ContainerizationSuggestions []string `json:"containerization_suggestions"`
	NextStepSuggestions         []string `json:"next_step_suggestions"`
}

// ConsolidatedConversationStage represents the current stage in the containerization workflow
type ConsolidatedConversationStage string

const (
	StageWelcome    ConsolidatedConversationStage = "welcome"
	StagePreFlight  ConsolidatedConversationStage = "preflight"
	StageInit       ConsolidatedConversationStage = "init"
	StageAnalysis   ConsolidatedConversationStage = "analysis"
	StageDockerfile ConsolidatedConversationStage = "dockerfile"
	StageBuild      ConsolidatedConversationStage = "build"
	StagePush       ConsolidatedConversationStage = "push"
	StageManifests  ConsolidatedConversationStage = "manifests"
	StageDeployment ConsolidatedConversationStage = "deployment"
	StageCompleted  ConsolidatedConversationStage = "completed"
)

// UserPreferences stores user's choices throughout the conversation
type UserPreferences struct {
	// Global preferences
	SkipConfirmations bool `json:"skip_confirmations"`

	// Repository preferences
	SkipFileTree bool   `json:"skip_file_tree"`
	Branch       string `json:"branch,omitempty"`

	// Dockerfile preferences
	Optimization       string            `json:"optimization"` // "size", "speed", "security"
	IncludeHealthCheck bool              `json:"include_health_check"`
	BaseImage          string            `json:"base_image,omitempty"`
	BuildArgs          map[string]string `json:"build_args,omitempty"`
	Platform           string            `json:"platform,omitempty"`

	// Kubernetes preferences
	Namespace       string         `json:"namespace,omitempty"`
	Replicas        int            `json:"replicas"`
	ServiceType     string         `json:"service_type"` // ClusterIP, LoadBalancer, NodePort
	AutoScale       bool           `json:"auto_scale"`
	ResourceLimits  ResourceLimits `json:"resource_limits"`
	ImagePullPolicy string         `json:"image_pull_policy"` // Always, IfNotPresent, Never

	// Deployment preferences
	TargetCluster   string `json:"target_cluster,omitempty"`
	DryRun          bool   `json:"dry_run"`
	AutoRollback    bool   `json:"auto_rollback"`
	ValidationLevel string `json:"validation_level"` // basic, thorough, security
}

// ResourceLimits defines resource constraints for containers
type ResourceLimits struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// ToolMetrics represents metrics for tool execution
type ToolMetrics struct {
	Tool       string        `json:"tool"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	DryRun     bool          `json:"dry_run"`
	TokensUsed int           `json:"tokens_used"`
}

// K8sManifest represents a Kubernetes manifest
type K8sManifest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Applied bool   `json:"applied"`
	Status  string `json:"status"`
}

// ToolError represents enhanced error information for tool operations
type ToolError struct {
	Type        string                 `json:"type"`        // Error classification
	Message     string                 `json:"message"`     // Human-readable error message
	Retryable   bool                   `json:"retryable"`   // Whether the operation can be retried
	RetryCount  int                    `json:"retry_count"` // Current retry attempt
	MaxRetries  int                    `json:"max_retries"` // Maximum retry attempts
	Suggestions []string               `json:"suggestions"` // Suggested remediation steps
	Context     map[string]interface{} `json:"context"`     // Additional error context
	Timestamp   time.Time              `json:"timestamp"`   // When the error occurred
}

// Error implements the error interface
func (e *ToolError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}
