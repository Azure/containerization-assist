package mcp

import (
	"context"
	"time"
)

// Unified MCP Interfaces - Single Source of Truth
// This file consolidates all interface definitions across the MCP system
// to eliminate duplication and provide consistent patterns.

// =============================================================================
// CORE TOOL INTERFACE
// =============================================================================

// Tool represents the unified interface for all MCP atomic tools
// Consolidates patterns from dispatch/interfaces.go, tools/interfaces.go, and utils/interfaces.go
type Tool interface {
	// Execute runs the tool with the provided arguments
	Execute(ctx context.Context, args interface{}) (interface{}, error)

	// GetMetadata returns comprehensive metadata about the tool
	GetMetadata() ToolMetadata

	// Validate checks if the provided arguments are valid for this tool
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata contains comprehensive information about a tool
// Consolidates metadata patterns from multiple existing interfaces
type ToolMetadata struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Dependencies []string          `json:"dependencies"`
	Capabilities []string          `json:"capabilities"`
	Requirements []string          `json:"requirements"`
	Parameters   map[string]string `json:"parameters"`
	Examples     []ToolExample     `json:"examples"`
}

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// =============================================================================
// SESSION INTERFACE
// =============================================================================

// Session represents the unified session management interface
// Consolidates session patterns from multiple packages
type Session interface {
	// ID returns the unique session identifier
	ID() string

	// GetWorkspace returns the workspace directory path
	GetWorkspace() string

	// UpdateState allows atomic updates to session state
	UpdateState(updateFunc func(*SessionState))

	// IsExpired checks if the session has expired
	IsExpired() bool

	// GetMetadata returns session metadata
	GetMetadata() SessionMetadata
}

// SessionState holds the unified session state
// Consolidates session state from tools/interfaces.go and types/session/state.go
type SessionState struct {
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

	// Progress tracking
	CurrentStage string
	Errors       []string
	Metadata     map[string]interface{}

	// Security
	SecurityScan *SecurityScanResult
}

// SessionMetadata contains session metadata
type SessionMetadata struct {
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	WorkspaceSize  int64     `json:"workspace_size"`
	OperationCount int       `json:"operation_count"`
	CurrentStage   string    `json:"current_stage"`
	Labels         []string  `json:"labels"`
}

// =============================================================================
// TRANSPORT INTERFACE
// =============================================================================

// Transport defines the unified interface for MCP transport mechanisms
// Consolidates transport patterns from transport/interface.go
type Transport interface {
	// Serve starts the transport and handles requests
	Serve(ctx context.Context) error

	// Stop gracefully shuts down the transport
	Stop() error

	// Name returns the transport name for identification
	Name() string

	// SetHandler sets the request handler for this transport
	SetHandler(handler RequestHandler)
}

// RequestHandler processes MCP requests
type RequestHandler interface {
	// HandleRequest processes an incoming MCP request
	HandleRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error)
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// =============================================================================
// ORCHESTRATOR INTERFACE
// =============================================================================

// Orchestrator defines the unified interface for tool orchestration
// Consolidates orchestration patterns from multiple packages
type Orchestrator interface {
	// ExecuteTool executes a tool by name with the provided arguments
	ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)

	// RegisterTool registers a tool with the orchestrator
	RegisterTool(name string, tool Tool) error

	// UnregisterTool removes a tool from the orchestrator
	UnregisterTool(name string) error

	// ListTools returns a list of registered tool names
	ListTools() []string

	// GetToolMetadata returns metadata for a specific tool
	GetToolMetadata(name string) (ToolMetadata, error)

	// ValidateToolArgs validates arguments for a specific tool
	ValidateToolArgs(ctx context.Context, name string, args interface{}) error
}

// =============================================================================
// SESSION MANAGER INTERFACE
// =============================================================================

// SessionManager defines the unified interface for session management
// Consolidates session management patterns from multiple packages
type SessionManager interface {
	// CreateSession creates a new session and returns it
	CreateSession() (*SessionState, error)

	// GetSession retrieves an existing session by ID
	GetSession(sessionID string) (*SessionState, error)

	// GetOrCreateSession gets an existing session or creates a new one
	GetOrCreateSession(sessionID string) (*SessionState, error)

	// UpdateSession atomically updates a session
	UpdateSession(sessionID string, updateFunc func(*SessionState)) error

	// DeleteSession removes a session
	DeleteSession(sessionID string) error

	// ListSessions returns a list of active session IDs
	ListSessions() ([]string, error)

	// CleanupExpiredSessions removes expired sessions
	CleanupExpiredSessions() error
}

// =============================================================================
// SUPPORTING TYPES
// =============================================================================

// RepositoryInfo contains information about analyzed repositories
// Consolidates repository information from multiple packages
type RepositoryInfo struct {
	// Core analysis
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Port         int      `json:"port"`
	Dependencies []string `json:"dependencies"`

	// File structure
	Structure FileStructure `json:"structure"`

	// Repository metadata
	Size      int64 `json:"size"`
	HasCI     bool  `json:"has_ci"`
	HasReadme bool  `json:"has_readme"`

	// Analysis metadata
	CachedAt         time.Time     `json:"cached_at"`
	AnalysisDuration time.Duration `json:"analysis_duration"`

	// Recommendations
	Recommendations []string `json:"recommendations"`
}

// FileStructure provides information about file organization
type FileStructure struct {
	TotalFiles      int      `json:"total_files"`
	ConfigFiles     []string `json:"config_files"`
	EntryPoints     []string `json:"entry_points"`
	TestFiles       []string `json:"test_files"`
	BuildFiles      []string `json:"build_files"`
	DockerFiles     []string `json:"docker_files"`
	KubernetesFiles []string `json:"kubernetes_files"`
	PackageManagers []string `json:"package_managers"`
}

// SecurityScanResult contains information about security scans
type SecurityScanResult struct {
	Success         bool               `json:"success"`
	ScannedAt       time.Time          `json:"scanned_at"`
	ImageRef        string             `json:"image_ref"`
	Scanner         string             `json:"scanner"`
	Vulnerabilities VulnerabilityCount `json:"vulnerabilities"`
	FixableCount    int                `json:"fixable_count"`
}

// VulnerabilityCount provides vulnerability counts by severity
type VulnerabilityCount struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Total    int `json:"total"`
}

// =============================================================================
// PIPELINE OPERATIONS INTERFACE
// =============================================================================

// PipelineOperations defines operations for the containerization pipeline
// Consolidates pipeline operations from multiple packages
type PipelineOperations interface {
	// Repository operations
	AnalyzeRepository(ctx context.Context, sessionID, repoPath string) (*RepositoryInfo, error)
	CloneRepository(ctx context.Context, sessionID, repoURL, branch string) (*CloneResult, error)

	// Docker operations
	GenerateDockerfile(ctx context.Context, sessionID, language, framework string) (string, error)
	BuildDockerImage(ctx context.Context, sessionID, imageName, dockerfilePath string) (*BuildResult, error)
	PushDockerImage(ctx context.Context, sessionID, imageName, registryURL string) (*RegistryPushResult, error)
	TagDockerImage(ctx context.Context, sessionID, sourceImage, targetImage string) (*TagResult, error)
	PullDockerImage(ctx context.Context, sessionID, imageRef string) (*PullResult, error)

	// Kubernetes operations
	GenerateKubernetesManifests(ctx context.Context, sessionID, imageName, appName string, port int, resources ResourceRequirements) (*ManifestGenerationResult, error)
	DeployToKubernetes(ctx context.Context, sessionID, manifestPath, namespace string) (*DeploymentResult, error)
	CheckApplicationHealth(ctx context.Context, sessionID, namespace, labelSelector string, timeout time.Duration) (*HealthCheckResult, error)
	PreviewDeployment(ctx context.Context, sessionID, manifestPath, namespace string) (string, error)
}

// =============================================================================
// RESULT TYPES
// =============================================================================

// Result types for pipeline operations
type CloneResult struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type BuildResult struct {
	ImageID string `json:"image_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Logs    string `json:"logs,omitempty"`
}

type RegistryPushResult struct {
	Digest  string `json:"digest"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type TagResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type PullResult struct {
	ImageID string `json:"image_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ManifestGenerationResult struct {
	ManifestPaths []string `json:"manifest_paths"`
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
}

type DeploymentResult struct {
	Resources []string `json:"resources"`
	Success   bool     `json:"success"`
	Error     string   `json:"error,omitempty"`
}

type HealthCheckResult struct {
	Healthy bool   `json:"healthy"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// ResourceRequirements defines Kubernetes resource requirements
type ResourceRequirements struct {
	CPURequest    string `json:"cpu_request"`
	MemoryRequest string `json:"memory_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryLimit   string `json:"memory_limit"`
}

// =============================================================================
// FACTORY AND REGISTRY INTERFACES
// =============================================================================

// ToolFactory creates new instances of tools
type ToolFactory func() Tool

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	// Register adds a tool to the registry
	Register(name string, factory ToolFactory) error

	// Unregister removes a tool from the registry
	Unregister(name string) error

	// Get retrieves a tool factory by name
	Get(name string) (ToolFactory, error)

	// List returns all registered tool names
	List() []string

	// GetMetadata returns metadata for all registered tools
	GetMetadata() map[string]ToolMetadata
}

// =============================================================================
// SERVER INTERFACE
// =============================================================================

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalDiskUsage int64 `json:"total_disk_usage"`
	SessionCount   int   `json:"session_count"`
}

// SessionManagerStats represents session manager statistics
type SessionManagerStats struct {
	ActiveSessions int `json:"active_sessions"`
	TotalSessions  int `json:"total_sessions"`
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	State        string     `json:"state"`
	FailureCount int        `json:"failure_count"`
	SuccessCount int64      `json:"success_count"`
	LastFailure  *time.Time `json:"last_failure,omitempty"`
}

// =============================================================================
// ERROR CODES
// =============================================================================

// Standard MCP error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	// Custom MCP error codes
	ErrorCodeSessionNotFound = -32001
	ErrorCodeQuotaExceeded   = -32002
	ErrorCodeCircuitOpen     = -32003
	ErrorCodeJobNotFound     = -32004
	ErrorCodeToolNotFound    = -32005
	ErrorCodeValidationError = -32006
)
