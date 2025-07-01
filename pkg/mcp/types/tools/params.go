package tools

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/types/config"
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

// DeployToolParams represents parameters for Kubernetes deployment operations
type DeployToolParams struct {
	SessionID   string              `json:"session_id" validate:"required"`
	Config      config.DeployConfig `json:"config"`
	Namespace   string              `json:"namespace,omitempty"`
	ImageRef    string              `json:"image_ref,omitempty"`
	ManifestDir string              `json:"manifest_dir,omitempty"`
	DryRun      bool                `json:"dry_run,omitempty"`
	Wait        bool                `json:"wait,omitempty"`
	Timeout     time.Duration       `json:"timeout,omitempty"`
}

// Validate implements ToolParams interface
func (p DeployToolParams) Validate() error {
	if p.SessionID == "" {
		return NewValidationError("session_id", "required field cannot be empty")
	}
	return p.Config.Validate()
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

// GenerateManifestsParams represents parameters for manifest generation
type GenerateManifestsParams struct {
	SessionID      string              `json:"session_id" validate:"required"`
	Config         config.DeployConfig `json:"config"`
	ImageRef       string              `json:"image_ref" validate:"required"`
	OutputDir      string              `json:"output_dir,omitempty"`
	IncludeIngress bool                `json:"include_ingress,omitempty"`
	IncludeService bool                `json:"include_service,omitempty"`
	CustomLabels   map[string]string   `json:"custom_labels,omitempty"`
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

// Using ValidationError from validation.go
