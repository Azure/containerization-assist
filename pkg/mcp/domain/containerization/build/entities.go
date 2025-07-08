// Package build contains pure business entities and rules for container build operations.
// This package has no external dependencies and represents the core build domain.
package build

import (
	"time"
)

// BuildRequest represents a request to build a container image
type BuildRequest struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"session_id"`
	Context      string            `json:"context"`
	Dockerfile   string            `json:"dockerfile"`
	ImageName    string            `json:"image_name"`
	Tags         []string          `json:"tags"`
	BuildArgs    map[string]string `json:"build_args,omitempty"`
	Target       string            `json:"target,omitempty"`
	Platform     string            `json:"platform,omitempty"`
	NoCache      bool              `json:"no_cache,omitempty"`
	PullParent   bool              `json:"pull_parent,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Options      BuildOptions      `json:"options,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// BuildOptions contains additional build configuration options
type BuildOptions struct {
	Strategy         BuildStrategy `json:"strategy,omitempty"`
	Timeout          time.Duration `json:"timeout,omitempty"`
	MemoryLimit      string        `json:"memory_limit,omitempty"`
	CPULimit         string        `json:"cpu_limit,omitempty"`
	NetworkMode      string        `json:"network_mode,omitempty"`
	EnableBuildKit   bool          `json:"enable_buildkit,omitempty"`
	RemoveIntermediate bool        `json:"remove_intermediate,omitempty"`
	Squash           bool          `json:"squash,omitempty"`
	SecurityOpt      []string      `json:"security_opt,omitempty"`
}

// BuildStrategy represents different build strategies
type BuildStrategy string

const (
	BuildStrategyDocker   BuildStrategy = "docker"
	BuildStrategyBuildKit BuildStrategy = "buildkit"
	BuildStrategyPodman   BuildStrategy = "podman"
	BuildStrategyKaniko   BuildStrategy = "kaniko"
	BuildStrategyImg      BuildStrategy = "img"
)

// BuildResult represents the result of a build operation
type BuildResult struct {
	BuildID     string        `json:"build_id"`
	RequestID   string        `json:"request_id"`
	SessionID   string        `json:"session_id"`
	ImageID     string        `json:"image_id"`
	ImageName   string        `json:"image_name"`
	Tags        []string      `json:"tags"`
	Size        int64         `json:"size"`
	Status      BuildStatus   `json:"status"`
	Error       string        `json:"error,omitempty"`
	Logs        []BuildLog    `json:"logs,omitempty"`
	Duration    time.Duration `json:"duration"`
	CreatedAt   time.Time     `json:"created_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Metadata    BuildMetadata `json:"metadata"`
}

// BuildStatus represents the current status of a build
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusQueued    BuildStatus = "queued"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
	BuildStatusTimeout   BuildStatus = "timeout"
)

// BuildLog represents a log entry from the build process
type BuildLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Step      string    `json:"step,omitempty"`
	Stream    string    `json:"stream,omitempty"`
}

// LogLevel represents the level of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// BuildMetadata contains additional build information
type BuildMetadata struct {
	Strategy      BuildStrategy     `json:"strategy"`
	Platform      string            `json:"platform,omitempty"`
	BaseImage     string            `json:"base_image,omitempty"`
	Layers        int               `json:"layers"`
	CacheHits     int               `json:"cache_hits"`
	CacheMisses   int               `json:"cache_misses"`
	ResourceUsage ResourceUsage     `json:"resource_usage"`
	Optimizations []Optimization    `json:"optimizations,omitempty"`
	SecurityScan  *SecurityScanResult `json:"security_scan,omitempty"`
}

// ResourceUsage represents resource consumption during build
type ResourceUsage struct {
	CPUTime    time.Duration `json:"cpu_time"`
	MemoryPeak int64         `json:"memory_peak"`
	DiskIO     int64         `json:"disk_io"`
	NetworkIO  int64         `json:"network_io"`
}

// Optimization represents a build optimization that was applied
type Optimization struct {
	Type        OptimizationType `json:"type"`
	Description string           `json:"description"`
	Savings     OptimizationSavings `json:"savings,omitempty"`
	Applied     bool             `json:"applied"`
}

// OptimizationType represents the type of optimization
type OptimizationType string

const (
	OptimizationTypeLayerMerging   OptimizationType = "layer_merging"
	OptimizationTypeCache          OptimizationType = "cache"
	OptimizationTypeMultiStage     OptimizationType = "multi_stage"
	OptimizationTypeBaseImage      OptimizationType = "base_image"
	OptimizationTypePackageManager OptimizationType = "package_manager"
	OptimizationTypeFileSystem     OptimizationType = "filesystem"
)

// OptimizationSavings represents the savings from an optimization
type OptimizationSavings struct {
	SizeReduction int64         `json:"size_reduction,omitempty"`
	TimeReduction time.Duration `json:"time_reduction,omitempty"`
	LayerReduction int          `json:"layer_reduction,omitempty"`
}

// SecurityScanResult represents the result of a security scan on the built image
type SecurityScanResult struct {
	Scanner        string             `json:"scanner"`
	ScanTime       time.Time          `json:"scan_time"`
	Vulnerabilities []Vulnerability   `json:"vulnerabilities"`
	Summary        VulnerabilitySummary `json:"summary"`
	Passed         bool               `json:"passed"`
}

// Vulnerability represents a security vulnerability found in the image
type Vulnerability struct {
	ID          string        `json:"id"`
	Severity    SeverityLevel `json:"severity"`
	Package     string        `json:"package"`
	Version     string        `json:"version"`
	FixedIn     string        `json:"fixed_in,omitempty"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	References  []string      `json:"references,omitempty"`
}

// VulnerabilitySummary provides a summary of vulnerabilities found
type VulnerabilitySummary struct {
	Total    int            `json:"total"`
	Critical int            `json:"critical"`
	High     int            `json:"high"`
	Medium   int            `json:"medium"`
	Low      int            `json:"low"`
	BySeverity map[SeverityLevel]int `json:"by_severity"`
}

// SeverityLevel represents the severity of a vulnerability
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityHigh     SeverityLevel = "high"
	SeverityMedium   SeverityLevel = "medium"
	SeverityLow      SeverityLevel = "low"
	SeverityInfo     SeverityLevel = "info"
	SeverityUnknown  SeverityLevel = "unknown"
)

// ImagePushRequest represents a request to push an image to a registry
type ImagePushRequest struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	ImageID   string    `json:"image_id"`
	ImageName string    `json:"image_name"`
	Tag       string    `json:"tag"`
	Registry  string    `json:"registry"`
	CreatedAt time.Time `json:"created_at"`
}

// ImagePushResult represents the result of pushing an image
type ImagePushResult struct {
	PushID      string      `json:"push_id"`
	RequestID   string      `json:"request_id"`
	ImageName   string      `json:"image_name"`
	Tag         string      `json:"tag"`
	Registry    string      `json:"registry"`
	Status      PushStatus  `json:"status"`
	Error       string      `json:"error,omitempty"`
	Digest      string      `json:"digest,omitempty"`
	Size        int64       `json:"size"`
	Duration    time.Duration `json:"duration"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// PushStatus represents the status of an image push operation
type PushStatus string

const (
	PushStatusPending   PushStatus = "pending"
	PushStatusUploading PushStatus = "uploading"
	PushStatusCompleted PushStatus = "completed"
	PushStatusFailed    PushStatus = "failed"
)

// ImageTagRequest represents a request to tag an image
type ImageTagRequest struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	ImageID   string    `json:"image_id"`
	NewTag    string    `json:"new_tag"`
	CreatedAt time.Time `json:"created_at"`
}

// ImageTagResult represents the result of tagging an image
type ImageTagResult struct {
	TagID     string     `json:"tag_id"`
	RequestID string     `json:"request_id"`
	ImageID   string     `json:"image_id"`
	OldTag    string     `json:"old_tag,omitempty"`
	NewTag    string     `json:"new_tag"`
	Status    TagStatus  `json:"status"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TagStatus represents the status of an image tag operation
type TagStatus string

const (
	TagStatusCompleted TagStatus = "completed"
	TagStatusFailed    TagStatus = "failed"
)

// BuildProgress represents the progress of an ongoing build
type BuildProgress struct {
	BuildID       string        `json:"build_id"`
	Status        BuildStatus   `json:"status"`
	CurrentStep   string        `json:"current_step"`
	StepNumber    int           `json:"step_number"`
	TotalSteps    int           `json:"total_steps"`
	Percentage    float64       `json:"percentage"`
	ElapsedTime   time.Duration `json:"elapsed_time"`
	EstimatedTime *time.Duration `json:"estimated_time,omitempty"`
	LastUpdate    time.Time     `json:"last_update"`
}

// BuildStats represents statistics about build operations
type BuildStats struct {
	TotalBuilds     int64         `json:"total_builds"`
	SuccessfulBuilds int64        `json:"successful_builds"`
	FailedBuilds    int64         `json:"failed_builds"`
	AverageDuration time.Duration `json:"average_duration"`
	TotalSize       int64         `json:"total_size"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	LastBuild       *time.Time    `json:"last_build,omitempty"`
}