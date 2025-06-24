package tools

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

// SimplifiedInterfaces provides a cleaner, consolidated interface structure
// This file will eventually replace interfaces.go after migration

// Core Tool Interface - simplified from generic ExecutableTool
type SimpleTool interface {
	// Metadata
	GetName() string
	GetDescription() string
	GetVersion() string
	GetCapabilities() contract.ToolCapabilities

	// Execution - using interface{} instead of generics for simplicity
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	Validate(ctx context.Context, args interface{}) error
}

// PipelineOperations consolidates all pipeline operations
// This replaces the PipelineOperations interface with clearer naming
type PipelineOperations interface {
	// Repository operations
	AnalyzeRepository(sessionID, repoPath string) (*analysis.AnalysisResult, error)
	CloneRepository(sessionID, repoURL, branch string) (*git.CloneResult, error)

	// Docker operations
	GenerateDockerfile(sessionID, language, framework string) (string, error)
	BuildDockerImage(sessionID, imageName, dockerfilePath string) (*docker.BuildResult, error)
	PushDockerImage(sessionID, imageName, registryURL string) (*docker.RegistryPushResult, error)
	TagDockerImage(sessionID, sourceImage, targetImage string) (*docker.TagResult, error)
	PullDockerImage(sessionID, imageRef string) (*docker.PullResult, error)

	// Kubernetes operations
	GenerateKubernetesManifests(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*kubernetes.ManifestGenerationResult, error)
	DeployToKubernetes(sessionID, manifestPath, namespace string) (*kubernetes.DeploymentResult, error)
	CheckApplicationHealth(sessionID, namespace, labelSelector string, timeout time.Duration) (*kubernetes.HealthCheckResult, error)
	PreviewDeployment(sessionID, manifestPath, namespace string) (string, error)

	// Session operations
	GetSessionWorkspace(sessionID string) string
	SaveAnalysisCache(sessionID string, result *analysis.AnalysisResult) error

	// Context management for request lifecycle (legacy support)
	SetContext(sessionID string, ctx context.Context)
	GetContext(sessionID string) context.Context
	ClearContext(sessionID string)
}

// SessionOperations consolidates session management
// This replaces *session.SessionManager with clearer naming and combines with Session struct
type SessionOperations interface {
	GetSession(sessionID string) (*UnifiedSession, error)
	SaveSession(session *UnifiedSession) error
	CreateSession() (*UnifiedSession, error)
	GetOrCreateSession(sessionID string) (*UnifiedSession, error)
}

// UnifiedSession consolidates Session and SessionState into a single structure
type UnifiedSession struct {
	// Core fields
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time

	// Workspace
	WorkspaceDir string

	// Repository state
	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo

	// Build state
	DockerfileGenerated bool
	DockerfilePath      string
	ImageBuilt          bool
	ImageRef            string
	ImagePushed         bool

	// Deployment state
	ManifestsGenerated  bool
	ManifestPaths       []string
	DeploymentValidated bool

	// Tracking
	CurrentStage string
	Errors       []string
	Metadata     map[string]interface{}

	// Security
	SecurityScan *SimplifiedSecurityScan
}

// RepositoryInfo replaces RepositoryScanSummary with clearer structure
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

// FileStructure provides clearer file organization info
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

// SimplifiedSecurityScan simplifies security scan summary
type SimplifiedSecurityScan struct {
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

// ConversationOperations replaces ConversationHandler
type ConversationOperations interface {
	HandleConversation(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error)
}

// ToolSessionManager defines the session management interface used by atomic tools
// This allows for testing with mocks while production uses the concrete SessionManager
type ToolSessionManager interface {
	GetSession(sessionID string) (*sessiontypes.SessionState, error)
	GetOrCreateSession(sessionID string) (*sessiontypes.SessionState, error)
	UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error
}

// ToolCapabilities is imported from contract package
type ToolCapabilities = contract.ToolCapabilities

// ChatToolArgs and ChatToolResult are defined in chat_tool.go
