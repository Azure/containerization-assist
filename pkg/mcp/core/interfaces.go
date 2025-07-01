package core

import (
	"context"
	"fmt"
	"time"
)

// Package core defines the core interfaces for the Container Kit MCP server.
// This package serves as the single source of truth for all interface definitions,
// breaking import cycles and eliminating the need for adapter patterns.

// ============================================================================
// Core Tool Interface
// ============================================================================

// Tool represents the unified interface for all MCP tools.
// This is the single Tool interface definition used throughout the system.
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata represents tool metadata information
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

// ToolExample represents tool usage example
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// ============================================================================
// Progress Reporting Interface
// ============================================================================

// ProgressReporter provides unified progress reporting across all tools.
// This eliminates the need for multiple progress adapter implementations.
type ProgressReporter interface {
	StartStage(stage string) ProgressToken
	UpdateProgress(token ProgressToken, message string, percent int)
	CompleteStage(token ProgressToken, success bool, message string)
}

// ProgressToken represents a unique identifier for a progress stage
type ProgressToken string

// ProgressStage represents the state of a progress stage
type ProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`   // "pending", "running", "completed", "failed"
	Progress    int     `json:"progress"` // 0-100
	Message     string  `json:"message"`
	Weight      float64 `json:"weight"` // Relative weight (0.0-1.0) of this stage in overall progress
}

// ============================================================================
// Repository Analysis Interface
// ============================================================================

// RepositoryAnalyzer provides repository analysis functionality.
// This interface breaks the import cycle between analyze and build packages.
type RepositoryAnalyzer interface {
	AnalyzeStructure(ctx context.Context, path string) (*RepositoryInfo, error)
	AnalyzeDockerfile(ctx context.Context, path string) (*DockerfileInfo, error)
	GetBuildRecommendations(ctx context.Context, repo *RepositoryInfo) (*BuildRecommendations, error)
}

// RepositoryInfo represents repository analysis results
type RepositoryInfo struct {
	Path          string                 `json:"path"`
	Type          string                 `json:"type"`
	Language      string                 `json:"language"`
	Framework     string                 `json:"framework"`
	Languages     []string               `json:"languages"`
	Dependencies  map[string]string      `json:"dependencies"`
	BuildTools    []string               `json:"build_tools"`
	EntryPoint    string                 `json:"entry_point"`
	Port          int                    `json:"port"`
	HasDockerfile bool                   `json:"has_dockerfile"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// DockerfileInfo represents Dockerfile analysis results
type DockerfileInfo struct {
	Path           string            `json:"path"`
	BaseImage      string            `json:"base_image"`
	ExposedPorts   []int             `json:"exposed_ports"`
	WorkingDir     string            `json:"working_dir"`
	EntryPoint     []string          `json:"entry_point"`
	Cmd            []string          `json:"cmd"`
	HealthCheck    *HealthCheckInfo  `json:"health_check,omitempty"`
	Labels         map[string]string `json:"labels"`
	BuildArgs      map[string]string `json:"build_args"`
	MultiStage     bool              `json:"multi_stage"`
	SecurityIssues []string          `json:"security_issues"`
}

// HealthCheckInfo represents Docker health check configuration
type HealthCheckInfo struct {
	Test     []string      `json:"test"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// BuildRecommendations represents build optimization recommendations
type BuildRecommendations struct {
	OptimizationTips []string          `json:"optimization_tips"`
	SecurityTips     []string          `json:"security_tips"`
	PerformanceTips  []string          `json:"performance_tips"`
	BestPractices    []string          `json:"best_practices"`
	Suggestions      map[string]string `json:"suggestions"`
}

// ============================================================================
// Transport Interface
// ============================================================================

// Transport provides unified transport abstraction.
// This eliminates the need for transport adapter patterns.
type Transport interface {
	Serve(ctx context.Context) error
	Stop(ctx context.Context) error
	SetHandler(handler RequestHandler)
	Name() string
}

// RequestHandler provides unified request handling interface
type RequestHandler interface {
	HandleRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error)
}

// ============================================================================
// Tool Registry Interface
// ============================================================================

// ToolRegistry provides simplified tool registration and retrieval.
// This eliminates the need for complex auto-registration adapters.
type ToolRegistry interface {
	Register(tool Tool)
	Get(name string) (Tool, bool)
	GetTool(name string) (Tool, error) // Legacy compatibility method
	List() []string
	GetMetadata(name string) (ToolMetadata, bool)
}

// ============================================================================
// Tool Orchestration Interface
// ============================================================================

// Orchestrator provides tool orchestration functionality
type Orchestrator interface {
	ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)
	RegisterTool(name string, tool Tool) error
	ValidateToolArgs(toolName string, args interface{}) error
	GetToolMetadata(toolName string) (*ToolMetadata, error)
}

// ============================================================================
// Session Management Interface
// ============================================================================

// SessionManager provides session management functionality
type SessionManager interface {
	CreateSession(userID string) (Session, error)
	GetSession(sessionID string) (Session, error)
	DeleteSession(sessionID string) error
	ListSessions(userID string) ([]Session, error)
}

// Session represents a user session
type Session interface {
	ID() string
	GetWorkspace() string
	UpdateState(func(*SessionState))
	GetState() *SessionState
}

// SessionState represents session state information
type SessionState struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at"`

	WorkspaceDir string `json:"workspace_dir"`

	// Repository state
	RepositoryAnalyzed bool            `json:"repository_analyzed"`
	RepositoryInfo     *RepositoryInfo `json:"repository_info,omitempty"`
	RepoURL            string          `json:"repo_url"`

	// Build state
	DockerfileGenerated bool   `json:"dockerfile_generated"`
	DockerfilePath      string `json:"dockerfile_path"`
	ImageBuilt          bool   `json:"image_built"`
	ImageRef            string `json:"image_ref"`
	ImagePushed         bool   `json:"image_pushed"`

	// Deployment state
	ManifestsGenerated  bool     `json:"manifests_generated"`
	ManifestPaths       []string `json:"manifest_paths"`
	DeploymentValidated bool     `json:"deployment_validated"`

	// Workflow state
	CurrentStage string   `json:"current_stage"`
	Status       string   `json:"status"`
	Stage        string   `json:"stage"`
	Errors       []string `json:"errors"`

	// Security state
	SecurityScan *SecurityScanResult `json:"security_scan,omitempty"`

	// Extensible metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// SecurityScanResult represents security scan results
type SecurityScanResult struct {
	Success            bool               `json:"success"`
	HasVulnerabilities bool               `json:"has_vulnerabilities"`
	ScannedAt          time.Time          `json:"scanned_at"`
	ImageRef           string             `json:"image_ref"`
	Scanner            string             `json:"scanner"`
	Vulnerabilities    VulnerabilityCount `json:"vulnerabilities"`
	CriticalCount      int                `json:"critical_count"`
	HighCount          int                `json:"high_count"`
	MediumCount        int                `json:"medium_count"`
	LowCount           int                `json:"low_count"`
	VulnerabilityList  []string           `json:"vulnerability_list"`
	ScanTime           time.Time          `json:"scan_time"`
}

// VulnerabilityCount represents counts of vulnerabilities by severity
type VulnerabilityCount struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// SecurityFinding represents a security vulnerability finding
type SecurityFinding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixedIn     string `json:"fixed_in,omitempty"`
}

// ============================================================================
// MCP Protocol Types
// ============================================================================

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ============================================================================
// Base Tool Response
// ============================================================================

// BaseToolResponse provides common response structure for all tools
type BaseToolResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// GetSuccess implements ToolResult interface
func (b BaseToolResponse) GetSuccess() bool {
	return b.Success
}

// ============================================================================
// Additional Types for Adapter Elimination
// ============================================================================

// AlternativeStrategy represents an alternative approach or strategy
type AlternativeStrategy struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Commands    []string `json:"commands"`
}

// Server represents the MCP server interface
type Server interface {
	Start(ctx context.Context) error
	Stop() error
	Shutdown(ctx context.Context) error
	EnableConversationMode(config ConversationConfig) error
	GetStats() *ServerStats
	GetSessionManagerStats() *SessionManagerStats
	GetWorkspaceStats() *WorkspaceStats
	GetLogger() interface{} // Returns the logger instance
}

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	// Session management
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64

	// Storage
	StorePath string

	// Transport
	TransportType string // "stdio", "http"
	HTTPAddr      string
	HTTPPort      int
	CORSOrigins   []string // CORS allowed origins
	APIKey        string   // API key for authentication
	RateLimit     int      // Requests per minute per IP

	// Features
	SandboxEnabled bool

	// Logging
	LogLevel       string
	LogHTTPBodies  bool  // Log HTTP request/response bodies
	MaxBodyLogSize int64 // Maximum size of bodies to log

	// Cleanup
	CleanupInterval time.Duration

	// Job Management
	MaxWorkers int
	JobTTL     time.Duration

	// OpenTelemetry configuration
	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ConversationConfig holds configuration for conversation mode
type ConversationConfig struct {
	EnableTelemetry          bool
	TelemetryPort            int
	PreferencesDBPath        string
	PreferencesEncryptionKey string // Optional encryption key for preference store

	// OpenTelemetry configuration
	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ServerStats represents overall server statistics
type ServerStats struct {
	Transport string               `json:"transport"`
	Sessions  *SessionManagerStats `json:"sessions"`
	Workspace *WorkspaceStats      `json:"workspace"`
	Uptime    time.Duration        `json:"uptime"`
	StartTime time.Time            `json:"start_time"`
}

// SessionManagerStats represents session management statistics
type SessionManagerStats struct {
	ActiveSessions    int       `json:"active_sessions"`
	TotalSessions     int       `json:"total_sessions"`
	FailedSessions    int       `json:"failed_sessions"`
	ExpiredSessions   int       `json:"expired_sessions"`
	SessionsWithJobs  int       `json:"sessions_with_jobs"`
	AverageSessionAge float64   `json:"average_session_age_minutes"`
	SessionErrors     int       `json:"session_errors_last_hour"`
	TotalDiskUsage    int64     `json:"total_disk_usage_bytes"`
	ServerStartTime   time.Time `json:"server_start_time"`
}

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalDiskUsage int64 `json:"total_disk_usage"`
	SessionCount   int   `json:"session_count"`
	TotalFiles     int   `json:"total_files"`
	DiskLimit      int64 `json:"disk_limit"`
}

// ConversationStage represents different stages of conversation
type ConversationStage string

const (
	ConversationStagePreFlight  ConversationStage = "preflight"
	ConversationStageAnalyze    ConversationStage = "analyze"
	ConversationStageDockerfile ConversationStage = "dockerfile"
	ConversationStageBuild      ConversationStage = "build"
	ConversationStagePush       ConversationStage = "push"
	ConversationStageManifests  ConversationStage = "manifests"
	ConversationStageDeploy     ConversationStage = "deploy"
	ConversationStageScan       ConversationStage = "scan"
	ConversationStageCompleted  ConversationStage = "completed"
	ConversationStageError      ConversationStage = "error"
)

// ============================================================================
// AI and Fixing Interfaces
// ============================================================================

// AIAnalyzer provides AI analysis functionality
type AIAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() TokenUsage
	ResetTokenUsage()
}

// TokenUsage represents token usage tracking
type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// IterativeFixer provides iterative fixing functionality
type IterativeFixer interface {
	// AttemptFix performs iterative fixing with retry logic
	AttemptFix(ctx context.Context, sessionID string, toolName string, operationType string, err error, maxAttempts int, baseDir string) (*FixingResult, error)
}

// FixingResult represents the result of a fixing operation
type FixingResult struct {
	Success         bool          `json:"success"`
	AttemptsUsed    int           `json:"attempts_used"`
	OriginalError   error         `json:"original_error,omitempty"`
	FinalError      error         `json:"final_error,omitempty"`
	FixApplied      bool          `json:"fix_applied"`
	FixDescription  string        `json:"fix_description,omitempty"`
	AllAttempts     []interface{} `json:"all_attempts"`
	TotalAttempts   int           `json:"total_attempts"`
	Duration        time.Duration `json:"duration"`
	LastAttemptTime time.Time     `json:"last_attempt_time"`
}

// BaseAIContextResult provides AI context result information
type BaseAIContextResult struct {
	AIContextType     string        `json:"ai_context_type"`
	IsSuccessful      bool          `json:"is_successful"`
	Duration          time.Duration `json:"duration"`
	TokensUsed        int           `json:"tokens_used,omitempty"`
	ContextEnhanced   bool          `json:"context_enhanced"`
	EnhancementErrors []string      `json:"enhancement_errors,omitempty"`
}

// NewBaseAIContextResult creates a new BaseAIContextResult
func NewBaseAIContextResult(contextType string, successful bool, duration time.Duration) BaseAIContextResult {
	return BaseAIContextResult{
		AIContextType:     contextType,
		IsSuccessful:      successful,
		Duration:          duration,
		ContextEnhanced:   false,
		EnhancementErrors: []string{},
	}
}

// ============================================================================
// Pipeline and Session Management Interfaces
// ============================================================================

// PipelineOperations provides pipeline operation functionality
type PipelineOperations interface {
	// Session operations
	GetSessionWorkspace(sessionID string) string
	UpdateSessionState(sessionID string, updateFunc func(*SessionState)) error

	// Docker operations
	BuildImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	PushImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	PullImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	TagImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error)

	// Kubernetes operations
	GenerateManifests(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	DeployKubernetes(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	CheckHealth(ctx context.Context, sessionID string, args interface{}) (interface{}, error)

	// Analysis operations
	AnalyzeRepository(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	ValidateDockerfile(ctx context.Context, sessionID string, args interface{}) (interface{}, error)

	// Security operations
	ScanSecurity(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
	ScanSecrets(ctx context.Context, sessionID string, args interface{}) (interface{}, error)
}

// ToolSessionManager provides session management functionality for tools
type ToolSessionManager interface {
	// Session retrieval and creation
	GetSession(sessionID string) (interface{}, error)
	GetOrCreateSession(sessionID string) (interface{}, error)
	CreateSession(userID string) (interface{}, error)

	// Session lifecycle
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)

	// Statistics
	GetStats() *SessionManagerStats
}

// ============================================================================
// Additional Operation Types
// ============================================================================

// FixableOperation represents an operation that can be fixed when it fails
type FixableOperation interface {
	// Execute performs the operation
	Execute(ctx context.Context) error
	// ExecuteOnce runs the operation once
	ExecuteOnce(ctx context.Context) error
	// CanRetry determines if the operation can be retried after failure
	CanRetry(err error) bool
	// GetFailureAnalysis analyzes the failure for potential fixes
	GetFailureAnalysis(ctx context.Context, err error) (*FailureAnalysis, error)
	// PrepareForRetry prepares the operation for retry (e.g., cleanup, state reset)
	PrepareForRetry(ctx context.Context, fixAttempt interface{}) error
}

// FailureAnalysis represents analysis of an operation failure
type FailureAnalysis struct {
	FailureType    string   `json:"failure_type"`
	IsCritical     bool     `json:"is_critical"`
	IsRetryable    bool     `json:"is_retryable"`
	RootCauses     []string `json:"root_causes"`
	SuggestedFixes []string `json:"suggested_fixes"`
	ErrorContext   string   `json:"error_context"`
}

// Error implements the error interface for FailureAnalysis
func (fa *FailureAnalysis) Error() string {
	if fa == nil {
		return "failure analysis: <nil>"
	}
	return fmt.Sprintf("failure analysis: %s (%s)", fa.FailureType, fa.ErrorContext)
}

// ErrorContext provides contextual information about errors
type ErrorContext struct {
	SessionID     string                 `json:"session_id"`
	OperationType string                 `json:"operation_type"`
	Phase         string                 `json:"phase"`
	ErrorCode     string                 `json:"error_code"`
	Metadata      map[string]interface{} `json:"metadata"`
	Timestamp     time.Time              `json:"timestamp"`
}

// LocalProgressStage represents a local progress stage
type LocalProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Progress    int     `json:"progress"`
	Status      string  `json:"status"`
	Weight      float64 `json:"weight"`
}

// Recommendation represents an AI recommendation
type Recommendation struct {
	Type        string                 `json:"type"`
	Priority    int                    `json:"priority"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"`
	Metadata    map[string]interface{} `json:"metadata"`
}
