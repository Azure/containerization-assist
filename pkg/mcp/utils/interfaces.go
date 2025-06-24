package utils

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

// Core Tool Interface - standardized across all tools
type Tool interface {
	// Metadata
	GetName() string
	GetDescription() string
	GetVersion() string
	GetCapabilities() contract.ToolCapabilities

	// Execution - using interface{} for flexibility
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	Validate(ctx context.Context, args interface{}) error
}

// SessionManager defines the session management interface used by tools
type SessionManager interface {
	GetSession(sessionID string) (*sessiontypes.SessionState, error)
	GetOrCreateSession(sessionID string) (*sessiontypes.SessionState, error)
	UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error
}

// SessionData represents session information for management tools
type SessionData struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	CurrentStage string                 `json:"current_stage"`
	Metadata     map[string]interface{} `json:"metadata"`
	IsActive     bool                   `json:"is_active"`
	LastAccess   time.Time              `json:"last_access"`
}

// SessionManagerStats represents statistics about session management
type SessionManagerStats struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	ExpiredSessions int     `json:"expired_sessions"`
	AverageAge      float64 `json:"average_age_hours"`
	OldestSession   string  `json:"oldest_session_id"`
	NewestSession   string  `json:"newest_session_id"`
}

// ProgressStage represents a stage in a multi-step operation
type ProgressStage struct {
	Name        string  // Human-readable stage name
	Weight      float64 // Relative weight (0.0-1.0) of this stage in overall progress
	Description string  // Optional detailed description
}

// ProgressReporter provides stage-aware progress reporting
type ProgressReporter interface {
	// ReportStage reports progress for the current stage
	ReportStage(stageProgress float64, message string)

	// NextStage advances to the next stage and reports its start
	NextStage(message string)

	// SetStage explicitly sets the current stage index
	SetStage(stageIndex int, message string)

	// ReportOverall reports overall progress directly (bypassing stage calculation)
	ReportOverall(progress float64, message string)

	// GetCurrentStage returns the current stage information
	GetCurrentStage() (int, ProgressStage)
}

// ProgressTracker provides centralized progress reporting for tools
type ProgressTracker interface {
	// RunWithProgress executes an operation with standardized progress reporting
	RunWithProgress(
		ctx context.Context,
		operation string,
		stages []ProgressStage,
		fn func(ctx context.Context, reporter ProgressReporter) error,
	) error
}

// PipelineOperations consolidates all pipeline operations
type PipelineOperations interface {
	// Repository operations
	AnalyzeRepository(sessionID, repoPath string) (interface{}, error)
	CloneRepository(sessionID, repoURL, branch string) (interface{}, error)

	// Docker operations
	GenerateDockerfile(sessionID, language, framework string) (string, error)
	BuildDockerImage(sessionID, imageName, dockerfilePath string) (interface{}, error)
	PushDockerImage(sessionID, imageName, registryURL string) (interface{}, error)
	TagDockerImage(sessionID, sourceImage, targetImage string) (interface{}, error)
	PullDockerImage(sessionID, imageRef string) (interface{}, error)

	// Kubernetes operations
	GenerateKubernetesManifests(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (interface{}, error)
	DeployToKubernetes(sessionID, manifestPath, namespace string) (interface{}, error)
	CheckApplicationHealth(sessionID, namespace, labelSelector string, timeout time.Duration) (interface{}, error)
	PreviewDeployment(sessionID, manifestPath, namespace string) (string, error)

	// Session operations
	GetSessionWorkspace(sessionID string) string
	SaveAnalysisCache(sessionID string, result interface{}) error

	// Context management for request lifecycle
	SetContext(sessionID string, ctx context.Context)
	GetContext(sessionID string) context.Context
	ClearContext(sessionID string)
}

// RepositoryInfo provides structured repository analysis information
type RepositoryInfo struct {
	// Core analysis
	Language     string
	Framework    string
	Port         int
	Dependencies []string

	// File insights
	Structure FileStructure

	// Repository metadata
	Size      int64
	HasCI     bool
	HasReadme bool

	// Cache info
	CachedAt         time.Time
	AnalysisDuration time.Duration

	// Suggestions
	Recommendations []string
}

// FileStructure provides file organization information
type FileStructure struct {
	TotalFiles      int
	ConfigFiles     []string
	EntryPoints     []string
	TestFiles       []string
	BuildFiles      []string
	DockerFiles     []string
	KubernetesFiles []string
	PackageManagers []string
}

// SecurityScan provides security scan information
type SecurityScan struct {
	Success         bool
	ScannedAt       time.Time
	ImageRef        string
	Scanner         string
	Vulnerabilities VulnerabilityCount
	FixableCount    int
}

// VulnerabilityCount provides clear vulnerability counts
type VulnerabilityCount struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Unknown  int
	Total    int
}

// Standard stage definitions for consistent progress reporting across tools

// StandardBuildStages provides common stages for build operations
func StandardBuildStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Analyze", Weight: 0.20, Description: "Analyzing build context and dependencies"},
		{Name: "Build", Weight: 0.50, Description: "Building Docker image"},
		{Name: "Verify", Weight: 0.15, Description: "Running post-build verification"},
		{Name: "Finalize", Weight: 0.05, Description: "Cleaning up and saving results"},
	}
}

// StandardDeployStages provides common stages for deployment operations
func StandardDeployStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Generate", Weight: 0.30, Description: "Generating Kubernetes manifests"},
		{Name: "Deploy", Weight: 0.40, Description: "Deploying to cluster"},
		{Name: "Verify", Weight: 0.15, Description: "Verifying deployment health"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving deployment status"},
	}
}

// StandardScanStages provides common stages for security scanning operations
func StandardScanStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Preparing scan environment"},
		{Name: "Scan", Weight: 0.60, Description: "Running security analysis"},
		{Name: "Analyze", Weight: 0.20, Description: "Processing scan results"},
		{Name: "Report", Weight: 0.10, Description: "Generating security report"},
	}
}

// StandardAnalysisStages provides common stages for repository analysis
func StandardAnalysisStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Setting up analysis environment"},
		{Name: "Discover", Weight: 0.30, Description: "Discovering project structure"},
		{Name: "Analyze", Weight: 0.40, Description: "Analyzing dependencies and frameworks"},
		{Name: "Generate", Weight: 0.15, Description: "Generating recommendations"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving analysis results"},
	}
}

// StandardPushStages provides common stages for registry push operations
func StandardPushStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Authenticate", Weight: 0.15, Description: "Authenticating with registry"},
		{Name: "Push", Weight: 0.60, Description: "Pushing Docker image layers"},
		{Name: "Verify", Weight: 0.10, Description: "Verifying push results"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// StandardGenerateStages provides common stages for generation operations
func StandardGenerateStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and analyzing requirements"},
		{Name: "Template", Weight: 0.30, Description: "Selecting and preparing templates"},
		{Name: "Generate", Weight: 0.40, Description: "Generating files"},
		{Name: "Validate", Weight: 0.15, Description: "Validating generated content"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving generated files"},
	}
}

// StandardValidationStages provides common stages for validation operations
func StandardValidationStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and preparing validation"},
		{Name: "Parse", Weight: 0.20, Description: "Parsing and loading files"},
		{Name: "Validate", Weight: 0.50, Description: "Running validation checks"},
		{Name: "Report", Weight: 0.15, Description: "Generating validation report"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving validation results"},
	}
}

// StandardHealthStages provides common stages for health check operations
func StandardHealthStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Preparing health checks"},
		{Name: "Connect", Weight: 0.20, Description: "Connecting to services"},
		{Name: "Check", Weight: 0.50, Description: "Running health checks"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing health status"},
		{Name: "Report", Weight: 0.05, Description: "Generating health report"},
	}
}
