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
// Deprecated: Use GenericTool[TParams, TResult] for type-safe tool implementations
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

// GenericTool provides type-safe tool interface using BETA's generic types
type GenericTool[TParams ToolParams, TResult ToolResult] interface {
	Execute(ctx context.Context, params TParams) (TResult, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, params TParams) error
}

// ToolParams constraint for tool parameter types
type ToolParams interface {
	Validate() error
}

// ToolResult constraint for tool result types
type ToolResult interface {
	IsSuccess() bool
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
	Input       map[string]interface{} `json:"input"`  // Keeping interface{} for JSON compatibility
	Output      map[string]interface{} `json:"output"` // Keeping interface{} for JSON compatibility
}

// GenericToolExample represents type-safe tool usage example
type GenericToolExample[TParams ToolParams, TResult ToolResult] struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Input       TParams `json:"input"`
	Output      TResult `json:"output"`
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
	Path          string            `json:"path"`
	Type          string            `json:"type"`
	Language      string            `json:"language"`
	Framework     string            `json:"framework"`
	Languages     []string          `json:"languages"`
	Dependencies  map[string]string `json:"dependencies"`
	BuildTools    []string          `json:"build_tools"`
	EntryPoint    string            `json:"entry_point"`
	Port          int               `json:"port"`
	HasDockerfile bool              `json:"has_dockerfile"`
	Metadata      map[string]string `json:"metadata"` // Changed to map[string]string for type safety
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
	CreateSession(userID string) (interface{}, error)
	GetSession(sessionID string) (interface{}, error)
	DeleteSession(sessionID string) error
	ListSessions(userID string) ([]interface{}, error)
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
	Success   bool              `json:"success"`
	Message   string            `json:"message,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"` // Changed to map[string]string for type safety
	Timestamp time.Time         `json:"timestamp"`
}

// IsSuccess implements ToolResult interface
func (b BaseToolResponse) IsSuccess() bool {
	return b.Success
}

// GetSuccess implements legacy interface (deprecated)
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
	GetLogger() interface{} // Returns the logger instance - keeping interface{} for logger compatibility
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
// DEPRECATED: Use TypedPipelineOperations for type-safe operations
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

// TypedPipelineOperations provides type-safe pipeline operation functionality
type TypedPipelineOperations interface {
	// Session operations
	GetSessionWorkspace(sessionID string) string
	UpdateSessionState(sessionID string, updateFunc func(*SessionState)) error

	// Docker operations with type-safe parameters and results
	BuildImageTyped(ctx context.Context, sessionID string, params BuildImageParams) (*BuildImageResult, error)
	PushImageTyped(ctx context.Context, sessionID string, params PushImageParams) (*PushImageResult, error)
	PullImageTyped(ctx context.Context, sessionID string, params PullImageParams) (*PullImageResult, error)
	TagImageTyped(ctx context.Context, sessionID string, params TagImageParams) (*TagImageResult, error)

	// Kubernetes operations with type-safe parameters and results
	GenerateManifestsTyped(ctx context.Context, sessionID string, params GenerateManifestsParams) (*GenerateManifestsResult, error)
	DeployKubernetesTyped(ctx context.Context, sessionID string, params DeployParams) (*DeployResult, error)
	CheckHealthTyped(ctx context.Context, sessionID string, params HealthCheckParams) (*HealthCheckResult, error)

	// Analysis operations with type-safe parameters and results
	AnalyzeRepositoryTyped(ctx context.Context, sessionID string, params AnalyzeParams) (*AnalyzeResult, error)
	ValidateDockerfileTyped(ctx context.Context, sessionID string, params ValidateParams) (*ValidateResult, error)

	// Security operations with type-safe parameters and results
	ScanSecurityTyped(ctx context.Context, sessionID string, params ScanParams) (*ScanResult, error)
	ScanSecretsTyped(ctx context.Context, sessionID string, params ScanSecretsParams) (*ScanSecretsResult, error)
}

// ToolSessionManager provides session management functionality for tools
// DEPRECATED: Use TypedToolSessionManager for type-safe operations
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

// TypedToolSessionManager provides type-safe session management functionality for tools
type TypedToolSessionManager interface {
	// Session retrieval and creation with type safety
	GetSessionTyped(sessionID string) (*SessionState, error)
	GetOrCreateSessionTyped(sessionID string) (*SessionState, error)
	CreateSessionTyped(userID string) (*SessionState, error)

	// Session lifecycle
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessionsTyped(ctx context.Context, filter SessionFilter) ([]*SessionState, error)

	// Statistics
	GetStats() *SessionManagerStats
}

// SessionFilter represents filters for session listing
type SessionFilter struct {
	UserID        string     `json:"user_id,omitempty"`
	Status        string     `json:"status,omitempty"`
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
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
	SessionID     string            `json:"session_id"`
	OperationType string            `json:"operation_type"`
	Phase         string            `json:"phase"`
	ErrorCode     string            `json:"error_code"`
	Metadata      map[string]string `json:"metadata"` // Changed to map[string]string for type safety
	Timestamp     time.Time         `json:"timestamp"`
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
	Type        string            `json:"type"`
	Priority    int               `json:"priority"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Action      string            `json:"action"`
	Metadata    map[string]string `json:"metadata"` // Changed to map[string]string for type safety
}

// ============================================================================
// Type-Safe Parameter and Result Types
// ============================================================================

// BuildImageParams represents parameters for build operations
type BuildImageParams struct {
	SessionID      string            `json:"session_id"`
	DockerfilePath string            `json:"dockerfile_path"`
	ContextPath    string            `json:"context_path"`
	ImageName      string            `json:"image_name"`
	Tags           []string          `json:"tags"`
	BuildArgs      map[string]string `json:"build_args"`
	NoCache        bool              `json:"no_cache"`
	Pull           bool              `json:"pull"`
}

// Validate implements ToolParams interface
func (p BuildImageParams) Validate() error {
	if p.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if p.DockerfilePath == "" {
		return fmt.Errorf("dockerfile_path is required")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p BuildImageParams) GetSessionID() string {
	return p.SessionID
}

// BuildImageResult represents the result of a build operation
type BuildImageResult struct {
	BaseToolResponse
	ImageID    string   `json:"image_id"`
	ImageRef   string   `json:"image_ref"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	BuildTime  float64  `json:"build_time_seconds"`
	LayerCount int      `json:"layer_count"`
}

// PushImageParams represents parameters for push operations
type PushImageParams struct {
	ImageRef   string `json:"image_ref"`
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

// PushImageResult represents the result of a push operation
type PushImageResult struct {
	BaseToolResponse
	ImageRef  string  `json:"image_ref"`
	Registry  string  `json:"registry"`
	PushTime  float64 `json:"push_time_seconds"`
	ImageSize int64   `json:"image_size"`
}

// PullImageParams represents parameters for pull operations
type PullImageParams struct {
	ImageRef string `json:"image_ref"`
	Platform string `json:"platform,omitempty"`
}

// PullImageResult represents the result of a pull operation
type PullImageResult struct {
	BaseToolResponse
	ImageRef  string  `json:"image_ref"`
	ImageID   string  `json:"image_id"`
	PullTime  float64 `json:"pull_time_seconds"`
	ImageSize int64   `json:"image_size"`
}

// TagImageParams represents parameters for tag operations
type TagImageParams struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

// TagImageResult represents the result of a tag operation
type TagImageResult struct {
	BaseToolResponse
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`
}

// GenerateManifestsParams represents parameters for manifest generation
type GenerateManifestsParams struct {
	ImageRef    string            `json:"image_ref"`
	AppName     string            `json:"app_name"`
	Namespace   string            `json:"namespace"`
	Port        int               `json:"port"`
	Replicas    int               `json:"replicas"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Resources   ResourceLimits    `json:"resources"`
	HealthCheck HealthCheckConfig `json:"health_check"`
}

// GenerateManifestsResult represents the result of manifest generation
type GenerateManifestsResult struct {
	BaseToolResponse
	ManifestPaths []string `json:"manifest_paths"`
	ManifestCount int      `json:"manifest_count"`
	Resources     []string `json:"resources"`
	Warnings      []string `json:"warnings"`
}

// DeployParams represents parameters for deployment operations
type DeployParams struct {
	SessionID     string   `json:"session_id"`
	ManifestPaths []string `json:"manifest_paths"`
	Namespace     string   `json:"namespace"`
	DryRun        bool     `json:"dry_run"`
	Wait          bool     `json:"wait"`
	Timeout       int      `json:"timeout_seconds"`
}

// Validate implements ToolParams interface
func (p DeployParams) Validate() error {
	if p.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if len(p.ManifestPaths) == 0 {
		return fmt.Errorf("manifest_paths cannot be empty")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p DeployParams) GetSessionID() string {
	return p.SessionID
}

// DeployResult represents the result of a deployment operation
type DeployResult struct {
	BaseToolResponse
	DeployedResources []string `json:"deployed_resources"`
	Namespace         string   `json:"namespace"`
	Status            string   `json:"status"`
	Warnings          []string `json:"warnings"`
}

// HealthCheckParams represents parameters for health check operations
type HealthCheckParams struct {
	Namespace   string   `json:"namespace"`
	AppName     string   `json:"app_name"`
	Resources   []string `json:"resources"`
	WaitTimeout int      `json:"wait_timeout_seconds"`
}

// HealthCheckResult represents the result of a health check operation
type HealthCheckResult struct {
	BaseToolResponse
	HealthyResources   []string          `json:"healthy_resources"`
	UnhealthyResources []string          `json:"unhealthy_resources"`
	ResourceStatuses   map[string]string `json:"resource_statuses"`
	OverallHealth      string            `json:"overall_health"`
}

// AnalyzeParams represents parameters for analysis operations
type AnalyzeParams struct {
	RepositoryPath string   `json:"repository_path"`
	IncludeFiles   []string `json:"include_files"`
	ExcludeFiles   []string `json:"exclude_files"`
	DeepAnalysis   bool     `json:"deep_analysis"`
}

// AnalyzeResult represents the result of an analysis operation
type AnalyzeResult struct {
	BaseToolResponse
	RepositoryInfo   *RepositoryInfo  `json:"repository_info"`
	Recommendations  []Recommendation `json:"recommendations"`
	SecurityIssues   []string         `json:"security_issues"`
	PerformanceHints []string         `json:"performance_hints"`
}

// ValidateParams represents parameters for validation operations
type ValidateParams struct {
	DockerfilePath string   `json:"dockerfile_path"`
	Rules          []string `json:"rules"`
	StrictMode     bool     `json:"strict_mode"`
}

// ValidateResult represents the result of a validation operation
type ValidateResult struct {
	BaseToolResponse
	Violations  []ValidationIssue `json:"violations"`
	Warnings    []ValidationIssue `json:"warnings"`
	Score       float64           `json:"score"`
	Suggestions []string          `json:"suggestions"`
}

// ScanParams represents parameters for security scan operations
type ScanParams struct {
	SessionID   string   `json:"session_id"`
	ImageRef    string   `json:"image_ref"`
	ScanType    string   `json:"scan_type"` // "vulnerability", "compliance", "both"
	Severity    []string `json:"severity"`  // ["critical", "high", "medium", "low"]
	Format      string   `json:"format"`    // "json", "sarif", "table"
	ExitOnError bool     `json:"exit_on_error"`
}

// Validate implements ToolParams interface
func (p ScanParams) Validate() error {
	if p.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if p.ImageRef == "" {
		return fmt.Errorf("image_ref is required")
	}
	return nil
}

// GetSessionID implements ToolParams interface
func (p ScanParams) GetSessionID() string {
	return p.SessionID
}

// ScanResult represents the result of a security scan operation
type ScanResult struct {
	BaseToolResponse
	ScanReport           *SecurityScanResult `json:"scan_report"`
	VulnerabilityDetails []SecurityFinding   `json:"vulnerability_details"`
	ComplianceIssues     []string            `json:"compliance_issues"`
	ReportPath           string              `json:"report_path"`
}

// ScanSecretsParams represents parameters for secrets scan operations
type ScanSecretsParams struct {
	Path        string   `json:"path"`
	Recursive   bool     `json:"recursive"`
	FileTypes   []string `json:"file_types"`
	ExcludeDirs []string `json:"exclude_dirs"`
}

// ScanSecretsResult represents the result of a secrets scan operation
type ScanSecretsResult struct {
	BaseToolResponse
	SecretsFound []SecretFinding `json:"secrets_found"`
	FilesScanned int             `json:"files_scanned"`
	SecretTypes  []string        `json:"secret_types"`
	RiskLevel    string          `json:"risk_level"`
}

// Supporting types for the above parameters and results

// ResourceLimits represents resource limits and requests
type ResourceLimits struct {
	Requests ResourceSpec `json:"requests"`
	Limits   ResourceSpec `json:"limits"`
}

// ResourceSpec represents CPU and memory specifications
type ResourceSpec struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled             bool   `json:"enabled"`
	Path                string `json:"path"`
	Port                int    `json:"port"`
	InitialDelaySeconds int    `json:"initial_delay_seconds"`
	PeriodSeconds       int    `json:"period_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	FailureThreshold    int    `json:"failure_threshold"`
}

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	Rule       string `json:"rule"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Suggestion string `json:"suggestion"`
}

// SecretFinding represents a detected secret
type SecretFinding struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
	RuleID      string `json:"rule_id"`
}
