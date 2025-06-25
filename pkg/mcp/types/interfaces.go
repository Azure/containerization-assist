package types

import (
	"context"
	"time"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
)

// Unified MCP Interface Types
// This package contains only the interface types to avoid circular imports

// =============================================================================
// CORE TOOL INTERFACE
// =============================================================================

// Tool represents the unified interface for all MCP atomic tools
type Tool interface {
	// Execute runs the tool with the provided arguments
	Execute(ctx context.Context, args interface{}) (interface{}, error)

	// GetMetadata returns comprehensive metadata about the tool
	GetMetadata() ToolMetadata

	// Validate checks if the provided arguments are valid for this tool
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata contains comprehensive information about a tool
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

// ToolArgs is a marker interface for tool-specific argument types
type ToolArgs interface {
	// Validate checks if the arguments are valid
	Validate() error
}

// ToolResult is a marker interface for tool-specific result types
type ToolResult interface {
	// IsSuccess indicates if the tool execution was successful
	IsSuccess() bool

	// GetError returns any error that occurred during execution
	GetError() error
}

// ArgConverter converts generic arguments to tool-specific types
type ArgConverter func(args map[string]interface{}) (ToolArgs, error)

// ResultConverter converts tool-specific results to generic types
type ResultConverter func(result ToolResult) (map[string]interface{}, error)

// =============================================================================
// SESSION INTERFACE
// =============================================================================

// Session represents the unified session management interface
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
type SessionState struct {
	// Core fields
	ID        string
	SessionID string // Alias for ID for compatibility
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time

	// Workspace
	WorkspaceDir string

	// Repository state
	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo
	RepoURL            string // Repository URL

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
	Status       string // Session status
	Stage        string // Current stage alias
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
// AI CONTEXT INTERFACES
// =============================================================================

// AIContext provides essential AI context capabilities for tool responses
type AIContext interface {
	// Assessment capabilities
	GetAssessment() *UnifiedAssessment

	// Recommendation capabilities
	GenerateRecommendations() []Recommendation

	// Context enrichment
	GetToolContext() *ToolContext

	// Essential metadata
	GetMetadata() map[string]interface{}
}

// ScoreCalculator provides unified scoring algorithms
type ScoreCalculator interface {
	CalculateScore(data interface{}) int
	DetermineRiskLevel(score int, factors map[string]interface{}) string
	CalculateConfidence(evidence []string) int
}

// TradeoffAnalyzer provides unified trade-off analysis
type TradeoffAnalyzer interface {
	AnalyzeTradeoffs(options []string, context map[string]interface{}) []TradeoffAnalysis
	CompareAlternatives(alternatives []AlternativeStrategy) *ComparisonMatrix
	RecommendBestOption(analysis []TradeoffAnalysis) *DecisionRecommendation
}

// AI Context supporting types (placeholders - to be defined based on usage)
type UnifiedAssessment struct{}
type Recommendation struct{}
type ToolContext struct{}
type TradeoffAnalysis struct{}
type AlternativeStrategy struct{}
type ComparisonMatrix struct{}
type DecisionRecommendation struct{}

// =============================================================================
// FIXING INTERFACES
// =============================================================================

// IterativeFixer provides iterative fixing capabilities
type IterativeFixer interface {
	// Fix attempts to fix an issue iteratively
	Fix(ctx context.Context, issue interface{}) (*FixingResult, error)

	// SetMaxAttempts sets the maximum number of fix attempts
	SetMaxAttempts(max int)

	// GetFixHistory returns the history of fix attempts
	GetFixHistory() []FixAttempt
}

// ContextSharer provides context sharing capabilities
type ContextSharer interface {
	// ShareContext shares context between operations
	ShareContext(ctx context.Context, key string, value interface{}) error

	// GetSharedContext retrieves shared context
	GetSharedContext(ctx context.Context, key string) (interface{}, bool)
}

// FixingResult represents the result of a fixing operation
type FixingResult struct {
	Success         bool                   `json:"success"`
	Error           error                  `json:"error,omitempty"`
	FixApplied      string                 `json:"fix_applied"`
	Attempts        int                    `json:"attempts"`
	Duration        time.Duration          `json:"duration"`
	TotalDuration   time.Duration          `json:"total_duration"`
	TotalAttempts   int                    `json:"total_attempts"`
	FixHistory      []FixAttempt           `json:"fix_history"`
	AllAttempts     []FixAttempt           `json:"all_attempts"`
	FinalAttempt    *FixAttempt            `json:"final_attempt"`
	RecommendedNext []string               `json:"recommended_next"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// FixStrategy represents a strategy for fixing issues
type FixStrategy struct {
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	Type          string                             `json:"type"`
	Priority      int                                `json:"priority"`
	EstimatedTime time.Duration                      `json:"estimated_time"`
	Applicable    func(error) bool                   `json:"-"`
	Apply         func(context.Context, error) error `json:"-"`
	FileChanges   []FileChange                       `json:"file_changes,omitempty"`
	Commands      []string                           `json:"commands,omitempty"`
	Metadata      map[string]interface{}             `json:"metadata"`
}

// FileChange represents a file modification in a fix strategy
type FileChange struct {
	FilePath   string `json:"file_path"`
	Operation  string `json:"operation"`
	Content    string `json:"content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	Reason     string `json:"reason"`
}

// FixableOperation represents an operation that can be fixed
type FixableOperation interface {
	// ExecuteOnce runs the operation once
	ExecuteOnce(ctx context.Context) error

	// GetFailureAnalysis analyzes failure and returns rich error
	GetFailureAnalysis(ctx context.Context, err error) (*RichError, error)

	// PrepareForRetry prepares the operation for retry
	PrepareForRetry(ctx context.Context, fixAttempt *FixAttempt) error

	// Execute runs the operation
	Execute(ctx context.Context) error

	// CanRetry determines if the operation can be retried
	CanRetry() bool

	// GetLastError returns the last error encountered
	GetLastError() error
}

// RichError provides detailed error information
type RichError struct {
	Code     string `json:"code"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Error implements the error interface
func (e *RichError) Error() string {
	return e.Message
}

// FixAttempt represents a single fix attempt
type FixAttempt struct {
	AttemptNumber  int                    `json:"attempt_number"`
	Strategy       string                 `json:"strategy"`
	FixStrategy    FixStrategy            `json:"fix_strategy"`
	Error          error                  `json:"error,omitempty"`
	Success        bool                   `json:"success"`
	Duration       time.Duration          `json:"duration"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	AnalysisPrompt string                 `json:"analysis_prompt,omitempty"`
	AnalysisResult string                 `json:"analysis_result,omitempty"`
	Changes        []string               `json:"changes"`
	FixedContent   string                 `json:"fixed_content,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// =============================================================================
// UNIFIED RESULT TYPES
// =============================================================================

// BuildResult represents the result of a Docker build operation
type BuildResult struct {
	ImageID  string      `json:"image_id"`
	ImageRef string      `json:"image_ref"`
	Success  bool        `json:"success"`
	Error    *BuildError `json:"error,omitempty"`
	Logs     string      `json:"logs,omitempty"`
}

// BuildError represents a build error with structured information
type BuildError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// HealthCheckResult represents the result of a health check operation
type HealthCheckResult struct {
	Healthy     bool              `json:"healthy"`
	Status      string            `json:"status"`
	PodStatuses []PodStatus       `json:"pod_statuses"`
	Error       *HealthCheckError `json:"error,omitempty"`
}

// PodStatus represents the status of a Kubernetes pod
type PodStatus struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// HealthCheckError represents a health check error
type HealthCheckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// =============================================================================
// LEGACY INTERFACES (to be refactored)
// =============================================================================

// PipelineOperations provides pipeline-related operations
type PipelineOperations interface {
	// Session management
	GetSessionWorkspace(sessionID string) string
	UpdateSessionFromDockerResults(sessionID string, result interface{}) error

	// Docker operations
	BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*BuildResult, error)
	PullDockerImage(sessionID, imageRef string) error
	PushDockerImage(sessionID, imageRef string) error
	TagDockerImage(sessionID, sourceRef, targetRef string) error
	ConvertToDockerState(sessionID string) (*DockerState, error)

	// Kubernetes operations
	GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*KubernetesManifestResult, error)
	DeployToKubernetes(sessionID string, manifests []string) (*KubernetesDeploymentResult, error)
	CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*HealthCheckResult, error)

	// Resource management
	AcquireResource(sessionID, resourceType string) error
	ReleaseResource(sessionID, resourceType string) error
}

// ToolSessionManager manages tool sessions
type ToolSessionManager interface {
	// Session CRUD operations
	// Note: These return internal session types for now - to be migrated to unified types
	GetSession(sessionID string) (interface{}, error)
	GetSessionInterface(sessionID string) (interface{}, error)
	GetOrCreateSession(sessionID string) (interface{}, error)
	GetOrCreateSessionFromRepo(repoURL string) (interface{}, error)
	UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Session listing and searching
	ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)
	FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error)
}

// Pipeline operation result types
// Note: DockerBuildResult has been replaced by the unified BuildResult type above

type DockerState struct {
	Images     []string `json:"images"`
	Containers []string `json:"containers"`
	Networks   []string `json:"networks"`
	Volumes    []string `json:"volumes"`
}

type KubernetesManifestResult struct {
	Success   bool                `json:"success"`
	Manifests []GeneratedManifest `json:"manifests"`
	Error     *RichError          `json:"error,omitempty"`
}

type GeneratedManifest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type KubernetesDeploymentResult struct {
	Success     bool       `json:"success"`
	Namespace   string     `json:"namespace"`
	Deployments []string   `json:"deployments"`
	Services    []string   `json:"services"`
	Error       *RichError `json:"error,omitempty"`
}

// HealthCheckResult moved to unified types section above
// PodStatus is used by the legacy HealthCheckResult type

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
