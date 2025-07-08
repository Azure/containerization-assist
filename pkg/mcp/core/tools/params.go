package tools

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/config"
)

// BuildToolParams represents parameters for Docker build operations
type BuildToolParams struct {
	SessionID      string             `json:"session_id" validate:"required"`
	Config         config.BuildConfig `json:"config"`
	ImageName      string             `json:"image_name,omitempty"`
	DockerfilePath string             `json:"dockerfile_path,omitempty"`
	ContextPath    string             `json:"context_path,omitempty"`
	BuildArgs      map[string]string  `json:"build_args,omitempty"`
	Tags           []string           `json:"tags,omitempty"`
	NoCache        bool               `json:"no_cache,omitempty"`
	Target         string             `json:"target,omitempty"`
	Platform       string             `json:"platform,omitempty"`
}

// Validate implements ToolParams interface
func (p BuildToolParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	return p.Config.Validate()
}

// GetSessionID implements ToolParams interface
func (p BuildToolParams) GetSessionID() string {
	return p.SessionID
}

// DeployToolParams represents parameters for Kubernetes deployment operations
type DeployToolParams struct {
	SessionID   string        `json:"session_id" validate:"required"`
	Config      config.Deploy `json:"config"`
	Namespace   string        `json:"namespace,omitempty"`
	ImageRef    string        `json:"image_ref,omitempty"`
	ManifestDir string        `json:"manifest_dir,omitempty"`
	DryRun      bool          `json:"dry_run,omitempty"`
	Wait        bool          `json:"wait,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
}

// Validate implements ToolParams interface
func (p DeployToolParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	return p.Config.Validate()
}

// GetSessionID implements ToolParams interface
func (p DeployToolParams) GetSessionID() string {
	return p.SessionID
}

// ScanToolParams represents parameters for security scanning operations
type ScanToolParams struct {
	SessionID     string            `json:"session_id" validate:"required"`
	Config        config.ScanConfig `json:"config"`
	Target        string            `json:"target,omitempty"`
	ScanType      string            `json:"scan_type,omitempty"`
	OutputFormat  string            `json:"output_format,omitempty"`
	Severity      string            `json:"severity,omitempty"`
	IgnoreUnfixed bool              `json:"ignore_unfixed,omitempty"`
	OfflineMode   bool              `json:"offline_mode,omitempty"`
}

// Validate implements ToolParams interface
func (p ScanToolParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	return p.Config.Validate()
}

// GetSessionID implements ToolParams interface
func (p ScanToolParams) GetSessionID() string {
	return p.SessionID
}

// AnalyzeToolParams represents parameters for repository analysis operations
type AnalyzeToolParams struct {
	SessionID                   string `json:"session_id" validate:"required"`
	RepositoryPath              string `json:"repository_path" validate:"required"`
	RepositoryURL               string `json:"repository_url,omitempty"`
	Branch                      string `json:"branch,omitempty"`
	IncludeBuildRecommendations bool   `json:"include_build_recommendations,omitempty"`
	IncludeSecurityAnalysis     bool   `json:"include_security_analysis,omitempty"`
	IncludeDependencyAnalysis   bool   `json:"include_dependency_analysis,omitempty"`
}

// Validate implements ToolParams interface
func (p AnalyzeToolParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	if p.RepositoryPath == "" {
		return NewValidationError("repository_path", "required field cannot be empty")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p AnalyzeToolParams) GetSessionID() string {
	return p.SessionID
}

// GenerateManifestsParams represents parameters for manifest generation
type GenerateManifestsParams struct {
	SessionID      string            `json:"session_id" validate:"required"`
	Config         config.Deploy     `json:"config"`
	ImageRef       string            `json:"image_ref" validate:"required"`
	OutputDir      string            `json:"output_dir,omitempty"`
	IncludeIngress bool              `json:"include_ingress,omitempty"`
	IncludeService bool              `json:"include_service,omitempty"`
	CustomLabels   map[string]string `json:"custom_labels,omitempty"`
}

// Validate implements ToolParams interface
func (p GenerateManifestsParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	if p.ImageRef == "" {
		return NewValidationError("image_ref", "required field cannot be empty")
	}
	return p.Config.Validate()
}

// GetSessionID implements ToolParams interface
func (p GenerateManifestsParams) GetSessionID() string {
	return p.SessionID
}

// SessionParams represents basic session parameters
type SessionParams struct {
	SessionID string `json:"session_id" validate:"required"`
	UserID    string `json:"user_id,omitempty"`
}

// Validate implements ToolParams interface
func (p SessionParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p SessionParams) GetSessionID() string {
	return p.SessionID
}

// Using ValidationError from validation.go

// ============================================================================
// Additional Atomic Tool Parameters for Type Safety
// ============================================================================

// AtomicAnalyzeRepositoryParams represents parameters for atomic repository analysis
type AtomicAnalyzeRepositoryParams struct {
	SessionParams
	RepoURL      string `json:"repo_url" validate:"required" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch       string `json:"branch,omitempty" description:"Git branch to analyze (default: main)"`
	Context      string `json:"context,omitempty" description:"Additional context about the application"`
	LanguageHint string `json:"language_hint,omitempty" description:"Primary programming language hint"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
}

// Validate implements ToolParams interface
func (p AtomicAnalyzeRepositoryParams) Validate() error {
	if err := p.SessionParams.Validate(); err != nil {
		return err
	}
	if p.RepoURL == "" {
		return NewValidationError("repo_url", "required field cannot be empty")
	}
	return nil
}

// AtomicBuildImageParams represents parameters for atomic image building
type AtomicBuildImageParams struct {
	SessionParams
	ImageName      string            `json:"image_name" validate:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*$" description:"Docker image name (e.g., my-app)"`
	ImageTag       string            `json:"image_tag,omitempty" description:"Image tag (default: latest)"`
	DockerfilePath string            `json:"dockerfile_path,omitempty" description:"Path to Dockerfile (default: ./Dockerfile)"`
	BuildContext   string            `json:"build_context,omitempty" description:"Build context directory (default: session workspace)"`
	Platform       string            `json:"platform,omitempty" description:"Target platform (default: linux/amd64)"`
	NoCache        bool              `json:"no_cache,omitempty" description:"Build without cache"`
	BuildArgs      map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	PushAfterBuild bool              `json:"push_after_build,omitempty" description:"Push image after successful build"`
	RegistryURL    string            `json:"registry_url,omitempty" description:"Registry URL for pushing (if push_after_build=true)"`
}

// Validate implements ToolParams interface
func (p AtomicBuildImageParams) Validate() error {
	if err := p.SessionParams.Validate(); err != nil {
		return err
	}
	if p.ImageName == "" {
		return NewValidationError("image_name", "required field cannot be empty")
	}
	return nil
}

// AtomicDeployKubernetesParams represents parameters for atomic Kubernetes deployment
type AtomicDeployKubernetesParams struct {
	SessionParams
	ImageRef        string            `json:"image_ref" validate:"required" description:"Container image reference"`
	AppName         string            `json:"app_name,omitempty" description:"Application name (default: from image name)"`
	Namespace       string            `json:"namespace,omitempty" description:"Kubernetes namespace (default: default)"`
	Replicas        int               `json:"replicas,omitempty" description:"Number of replicas (default: 1)"`
	Port            int               `json:"port,omitempty" description:"Application port (default: 80)"`
	ServiceType     string            `json:"service_type,omitempty" description:"Service type: ClusterIP, NodePort, LoadBalancer"`
	IncludeIngress  bool              `json:"include_ingress,omitempty" description:"Generate and deploy Ingress resource"`
	Environment     map[string]string `json:"environment,omitempty" description:"Environment variables"`
	CPURequest      string            `json:"cpu_request,omitempty" description:"CPU request (e.g., 100m)"`
	MemoryRequest   string            `json:"memory_request,omitempty" description:"Memory request (e.g., 128Mi)"`
	CPULimit        string            `json:"cpu_limit,omitempty" description:"CPU limit (e.g., 500m)"`
	MemoryLimit     string            `json:"memory_limit,omitempty" description:"Memory limit (e.g., 512Mi)"`
	GenerateOnly    bool              `json:"generate_only,omitempty" description:"Only generate manifests, don't deploy"`
	WaitForReady    bool              `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready"`
	WaitTimeout     int               `json:"wait_timeout,omitempty" description:"Wait timeout in seconds"`
	SkipHealthCheck bool              `json:"skip_health_check,omitempty" description:"Skip health check validation after deployment"`
	ManifestPath    string            `json:"manifest_path,omitempty" description:"Custom path for generated manifests"`
	Force           bool              `json:"force,omitempty" description:"Force deployment even if validation fails"`
	DryRun          bool              `json:"dry_run,omitempty" description:"Preview changes without applying"`
}

// Validate implements ToolParams interface
func (p AtomicDeployKubernetesParams) Validate() error {
	if err := p.SessionParams.Validate(); err != nil {
		return err
	}
	if p.ImageRef == "" {
		return NewValidationError("image_ref", "required field cannot be empty")
	}
	return nil
}

// AtomicScanImageSecurityParams represents parameters for atomic image security scanning
type AtomicScanImageSecurityParams struct {
	SessionParams
	ImageRef       string   `json:"image_ref" validate:"required" description:"Container image reference to scan"`
	ScanTypes      []string `json:"scan_types,omitempty" description:"Types of scans to perform"`
	VulnTypes      []string `json:"vuln_types,omitempty" description:"Vulnerability types to scan for"`
	SecurityLevel  string   `json:"security_level,omitempty" description:"Security scanning level"`
	IgnoreRules    []string `json:"ignore_rules,omitempty" description:"Security rules to ignore"`
	IncludeSecrets bool     `json:"include_secrets,omitempty" description:"Include secrets scanning"`
	OutputFormat   string   `json:"output_format,omitempty" description:"Output format for results"`
	Severity       string   `json:"severity,omitempty" description:"Minimum severity level"`
	IgnoreUnfixed  bool     `json:"ignore_unfixed,omitempty" description:"Ignore vulnerabilities without fixes"`
	OfflineMode    bool     `json:"offline_mode,omitempty" description:"Use offline mode for scanning"`
}

// Validate implements ToolParams interface
func (p AtomicScanImageSecurityParams) Validate() error {
	if err := p.SessionParams.Validate(); err != nil {
		return err
	}
	if p.ImageRef == "" {
		return NewValidationError("image_ref", "required field cannot be empty")
	}
	return nil
}
