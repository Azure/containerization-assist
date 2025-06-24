package types

import (
	"context"
	"time"
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
	CreatedAt       time.Time `json:"created_at"`
	LastAccessedAt  time.Time `json:"last_accessed_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	WorkspaceSize   int64     `json:"workspace_size"`
	OperationCount  int       `json:"operation_count"`
	CurrentStage    string    `json:"current_stage"`
	Labels          []string  `json:"labels"`
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
	Success         bool                `json:"success"`
	ScannedAt       time.Time           `json:"scanned_at"`
	ImageRef        string              `json:"image_ref"`
	Scanner         string              `json:"scanner"`
	Vulnerabilities VulnerabilityCount  `json:"vulnerabilities"`
	FixableCount    int                 `json:"fixable_count"`
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