package config

import (
	"time"
)

// BuildConfig represents typed configuration for Docker build operations
type BuildConfig struct {
	// Image configuration
	ImageName      string            `json:"image_name" validate:"required"`
	Tags           []string          `json:"tags,omitempty"`
	DockerfilePath string            `json:"dockerfile_path" validate:"required"`
	ContextPath    string            `json:"context_path" validate:"required"`
	
	// Build arguments and environment
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	
	// Build options
	NoCache        bool              `json:"no_cache,omitempty"`
	PullParent     bool              `json:"pull_parent,omitempty"`
	ForceRebuild   bool              `json:"force_rebuild,omitempty"`
	Squash         bool              `json:"squash,omitempty"`
	
	// Resource limits
	Memory         int64             `json:"memory,omitempty"`         // in bytes
	CPUs           float64           `json:"cpus,omitempty"`           // CPU limit
	DiskSpace      int64             `json:"disk_space,omitempty"`     // in bytes
	
	// Timeout and retry configuration
	Timeout        time.Duration     `json:"timeout" validate:"required,min=1s"`
	Retries        int               `json:"retries" validate:"min=0,max=10"`
	RetryDelay     time.Duration     `json:"retry_delay,omitempty"`
	
	// Registry configuration
	RegistryURL    string            `json:"registry_url,omitempty"`
	RegistryAuth   *RegistryAuth     `json:"registry_auth,omitempty"`
	
	// Build stages and targets
	Target         string            `json:"target,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	
	// Caching configuration
	CacheFrom      []string          `json:"cache_from,omitempty"`
	CacheTo        string            `json:"cache_to,omitempty"`
	
	// Output configuration
	Output         string            `json:"output,omitempty"`
	BuildOutput    []string          `json:"build_output,omitempty"`
	
	// Security and compliance
	SecurityScan   bool              `json:"security_scan,omitempty"`
	ScanSeverity   string            `json:"scan_severity,omitempty"`
	
	// Metadata
	Labels         map[string]string `json:"labels,omitempty"`
	CreatedBy      string            `json:"created_by,omitempty"`
	BuildID        string            `json:"build_id,omitempty"`
}

// RegistryAuth represents authentication configuration for container registries
type RegistryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	AuthFile string `json:"auth_file,omitempty"`
}

// Validate validates the build configuration
func (bc *BuildConfig) Validate() error {
	if bc.ImageName == "" {
		return NewValidationError("image_name", "required field cannot be empty")
	}
	
	if bc.DockerfilePath == "" {
		return NewValidationError("dockerfile_path", "required field cannot be empty")
	}
	
	if bc.ContextPath == "" {
		return NewValidationError("context_path", "required field cannot be empty")
	}
	
	if bc.Timeout < time.Second {
		return NewValidationError("timeout", "must be at least 1 second")
	}
	
	if bc.Retries < 0 || bc.Retries > 10 {
		return NewValidationError("retries", "must be between 0 and 10")
	}
	
	return nil
}

// SetDefaults sets default values for build configuration
func (bc *BuildConfig) SetDefaults() {
	if bc.Timeout == 0 {
		bc.Timeout = 10 * time.Minute
	}
	
	if bc.Retries == 0 {
		bc.Retries = 3
	}
	
	if bc.RetryDelay == 0 {
		bc.RetryDelay = 5 * time.Second
	}
	
	if bc.ContextPath == "" {
		bc.ContextPath = "."
	}
	
	if bc.DockerfilePath == "" {
		bc.DockerfilePath = "Dockerfile"
	}
	
	if bc.CPUs == 0 {
		bc.CPUs = 2.0
	}
	
	if bc.Memory == 0 {
		bc.Memory = 2 * 1024 * 1024 * 1024 // 2GB
	}
}

// IsValid checks if the configuration is valid
func (bc *BuildConfig) IsValid() bool {
	return bc.Validate() == nil
}

// GetRegistryURL returns the registry URL or default
func (bc *BuildConfig) GetRegistryURL() string {
	if bc.RegistryURL != "" {
		return bc.RegistryURL
	}
	return "docker.io" // Docker Hub default
}

// HasRegistryAuth checks if registry authentication is configured
func (bc *BuildConfig) HasRegistryAuth() bool {
	return bc.RegistryAuth != nil && 
		(bc.RegistryAuth.Username != "" || 
		 bc.RegistryAuth.Token != "" || 
		 bc.RegistryAuth.AuthFile != "")
}

// GetFullImageName returns the fully qualified image name
func (bc *BuildConfig) GetFullImageName() string {
	registry := bc.GetRegistryURL()
	if registry == "docker.io" {
		return bc.ImageName
	}
	return registry + "/" + bc.ImageName
}

// GetPrimaryTag returns the primary tag for the image
func (bc *BuildConfig) GetPrimaryTag() string {
	if len(bc.Tags) > 0 {
		return bc.Tags[0]
	}
	return "latest"
}