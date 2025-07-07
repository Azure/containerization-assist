package build

import (
	"time"
)

// DockerBuildParams defines parameters for Docker build operations
type DockerBuildParams struct {
	DockerfilePath string            `json:"dockerfile_path" validate:"required,file"`
	ContextPath    string            `json:"context_path" validate:"required,dir"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	NoCache        bool              `json:"no_cache,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	Target         string            `json:"target,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	BuildKit       bool              `json:"buildkit,omitempty"`
}

// Validate validates the parameters
func (p DockerBuildParams) Validate() error {
	if p.DockerfilePath == "" {
		return validationError("dockerfile_path", "required")
	}
	if p.ContextPath == "" {
		return validationError("context_path", "required")
	}
	return nil
}

// GetSessionID returns the session ID
func (p DockerBuildParams) GetSessionID() string {
	return p.SessionID
}

// DockerBuildResult contains the result of a Docker build operation
type DockerBuildResult struct {
	Success     bool          `json:"success"`
	ImageID     string        `json:"image_id,omitempty"`
	ImageSize   int64         `json:"image_size,omitempty"`
	Duration    time.Duration `json:"duration"`
	BuildLog    []string      `json:"build_log,omitempty"`
	CacheHits   int           `json:"cache_hits"`
	CacheMisses int           `json:"cache_misses"`
	SessionID   string        `json:"session_id"`
	Tags        []string      `json:"tags,omitempty"`
}

// IsSuccess implements ToolOutput interface
func (r DockerBuildResult) IsSuccess() bool {
	return r.Success
}

// GetData implements ToolOutput interface
func (r DockerBuildResult) GetData() interface{} {
	return r
}

// DockerPullParams defines parameters for Docker pull operations
type DockerPullParams struct {
	Image     string `json:"image" validate:"required"`
	Tag       string `json:"tag,omitempty"`
	Platform  string `json:"platform,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// Validate validates the parameters
func (p DockerPullParams) Validate() error {
	if p.Image == "" {
		return validationError("image", "required")
	}
	return nil
}

// GetSessionID returns the session ID
func (p DockerPullParams) GetSessionID() string {
	return p.SessionID
}

// DockerPullResult contains the result of a Docker pull operation
type DockerPullResult struct {
	Success   bool          `json:"success"`
	ImageID   string        `json:"image_id,omitempty"`
	ImageSize int64         `json:"image_size,omitempty"`
	Duration  time.Duration `json:"duration"`
	PullLog   []string      `json:"pull_log,omitempty"`
	SessionID string        `json:"session_id"`
}

// IsSuccess returns whether the operation was successful
func (r DockerPullResult) IsSuccess() bool {
	return r.Success
}

// DockerPushParams defines parameters for Docker push operations
type DockerPushParams struct {
	Image     string `json:"image" validate:"required"`
	Tag       string `json:"tag,omitempty"`
	Registry  string `json:"registry,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// Validate validates the parameters
func (p DockerPushParams) Validate() error {
	if p.Image == "" {
		return validationError("image", "required")
	}
	return nil
}

// GetSessionID returns the session ID
func (p DockerPushParams) GetSessionID() string {
	return p.SessionID
}

// DockerPushResult contains the result of a Docker push operation
type DockerPushResult struct {
	Success    bool          `json:"success"`
	ImageID    string        `json:"image_id,omitempty"`
	Duration   time.Duration `json:"duration"`
	PushLog    []string      `json:"push_log,omitempty"`
	SessionID  string        `json:"session_id"`
	Registry   string        `json:"registry,omitempty"`
	RemoteSize int64         `json:"remote_size,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r DockerPushResult) IsSuccess() bool {
	return r.Success
}

// DockerTagParams defines parameters for Docker tag operations
type DockerTagParams struct {
	SourceImage string `json:"source_image" validate:"required"`
	TargetImage string `json:"target_image" validate:"required"`
	SessionID   string `json:"session_id,omitempty"`
}

// Validate validates the parameters
func (p DockerTagParams) Validate() error {
	if p.SourceImage == "" {
		return validationError("source_image", "required")
	}
	if p.TargetImage == "" {
		return validationError("target_image", "required")
	}
	return nil
}

// GetSessionID returns the session ID
func (p DockerTagParams) GetSessionID() string {
	return p.SessionID
}

// DockerTagResult contains the result of a Docker tag operation
type DockerTagResult struct {
	Success     bool   `json:"success"`
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
	SessionID   string `json:"session_id"`
}

// IsSuccess returns whether the operation was successful
func (r DockerTagResult) IsSuccess() bool {
	return r.Success
}

// Helper function for validation errors
func validationError(field, message string) error {
	return &ToolValidationError{
		Field:   field,
		Message: message,
	}
}

// ToolValidationError represents a parameter validation error for build tools
type ToolValidationError struct {
	Field   string
	Message string
}

func (e *ToolValidationError) Error() string {
	return e.Field + ": " + e.Message
}
