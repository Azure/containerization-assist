package types

import (
	"context"
	"time"
)

// Unified MCP Interface Types
// This package contains only the interface types to avoid circular imports

// =============================================================================
// CORE INTERFACES (temporarily restored to avoid import cycles)
// =============================================================================

// TODO: Import cycles resolved - interface definitions moved to pkg/mcp/interfaces.go

// NOTE: ToolArgs and ToolResult interfaces are now defined in pkg/mcp/interfaces.go

// Type aliases to avoid breaking existing code during migration
// These will eventually be removed once all references are updated

// NOTE: These interfaces are temporarily restored to avoid import cycles

// NOTE: ToolArgs, ToolResult, and Tool interfaces are now defined in pkg/mcp/interfaces.go
// Type aliases maintained for compatibility during migration

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

// NOTE: ProgressReporter interface is now defined in pkg/mcp/interfaces.go

// ProgressStage represents a stage in a multi-step operation
type ProgressStage struct {
	Name        string  // Human-readable stage name
	Weight      float64 // Relative weight (0.0-1.0) of this stage in overall progress
	Description string  // Optional detailed description
}

// NOTE: Session interface is now defined in pkg/mcp/interfaces.go

// Transport and RequestHandler interfaces - temporarily restored to avoid import cycles
// These will eventually be removed once all references are updated to use pkg/mcp

// Transport represents the unified interface for MCP transport mechanisms
type Transport interface {
	// Serve starts the transport and serves requests
	Serve(ctx context.Context) error

	// Stop gracefully stops the transport
	Stop(ctx context.Context) error

	// Name returns the transport name
	Name() string

	// SetHandler sets the request handler
	SetHandler(handler interface{})
}

// RequestHandler processes MCP requests
type RequestHandler interface {
	HandleRequest(ctx context.Context, req interface{}) (interface{}, error)
}

// Tool represents the unified interface for all MCP tools
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
	GetMetadata() ToolMetadata
	Validate(ctx context.Context, args interface{}) error
}

// NOTE: Transport, RequestHandler, ProgressReporter, Tool, and ToolRegistry interfaces
// are now defined in pkg/mcp/interfaces.go as the canonical source

// NOTE: HealthChecker interface is now defined in pkg/mcp/interfaces.go

// NOTE: These interfaces are now defined in pkg/mcp/interfaces.go
// Keeping type aliases for compatibility during migration

// NOTE: RequestHandler, Transport, and ToolRegistry interfaces are now defined in pkg/mcp/interfaces.go

// ToolRegistry interface is now defined in pkg/mcp/interfaces.go

// ToolOrchestrator interface is for internal use only
type ToolOrchestrator interface {
	ExecuteTool(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error)
	ValidateToolArgs(toolName string, args interface{}) error
	GetToolMetadata(toolName string) (*ToolMetadata, error)
}

// Transport interface is now defined in pkg/mcp/interfaces.go

// RequestHandler interface is now defined in pkg/mcp/interfaces.go
type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// =============================================================================
// SPECIALIZED TOOL TYPES (non-duplicated from main interfaces)
// =============================================================================

// ArgConverter converts generic arguments to tool-specific types
// NOTE: ToolArgs interface is defined in pkg/mcp/interfaces.go
type ArgConverter func(args map[string]interface{}) (interface{}, error)

// ResultConverter converts tool-specific results to generic types
// NOTE: ToolResult interface is defined in pkg/mcp/interfaces.go
type ResultConverter func(result interface{}) (map[string]interface{}, error)

// =============================================================================
// SESSION TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: Session interface is now defined in pkg/mcp/interfaces.go

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
// TRANSPORT TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: Transport interface is now defined above with RequestHandler
// NOTE: MCP types are also defined above with Transport

// =============================================================================
// ORCHESTRATOR TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: Orchestrator interface is now defined in pkg/mcp/interfaces.go

// =============================================================================
// SESSION MANAGER TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: SessionManager interface is now defined in pkg/mcp/interfaces.go

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
// FACTORY AND REGISTRY TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: ToolRegistry interface is now defined in pkg/mcp/interfaces.go

// ToolFactory creates new instances of tools
// NOTE: Tool interface is defined in pkg/mcp/interfaces.go
type ToolFactory func() interface{}

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

	// AttemptFix attempts to fix an issue with a specific attempt number
	AttemptFix(ctx context.Context, issue interface{}, attempt int) (*FixingResult, error)

	// SetMaxAttempts sets the maximum number of fix attempts
	SetMaxAttempts(max int)

	// GetFixHistory returns the history of fix attempts
	GetFixHistory() []FixAttempt

	// GetFailureRouting returns routing rules for different failure types
	GetFailureRouting() map[string]string

	// GetFixStrategies returns available fix strategies
	GetFixStrategies() []string
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
	UpdateSession(sessionID string, updateFunc func(interface{})) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Session listing and searching
	ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)
	FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error)
}

// UpdateSessionHelper is a helper function for updating sessions with type safety
// Usage: UpdateSessionHelper(sessionManager, sessionID, func(s *SessionState) { s.Field = value })
func UpdateSessionHelper[T any](manager ToolSessionManager, sessionID string, updater func(*T)) error {
	return manager.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*T); ok {
			updater(session)
		}
	})
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

// =============================================================================
// AI ANALYSIS INTERFACES
// =============================================================================

// AIAnalyzer provides a unified interface for all AI/LLM analysis operations
// This interface resolves naming conflicts with other Analyzer interfaces
type AIAnalyzer interface {
	// Analyze performs basic text analysis with the LLM
	Analyze(ctx context.Context, prompt string) (string, error)

	// AnalyzeWithFileTools performs analysis with file system access
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)

	// AnalyzeWithFormat performs analysis with formatted prompts
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)

	// GetTokenUsage returns usage statistics (may be empty for non-Azure implementations)
	GetTokenUsage() TokenUsage

	// ResetTokenUsage resets usage statistics
	ResetTokenUsage()
}

// TokenUsage holds the token usage information for LLM operations
type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// =============================================================================
// HEALTH AND MONITORING TYPES (interface defined in main interfaces file)
// =============================================================================

// HealthChecker interface is now defined in pkg/mcp/interfaces.go

// NOTE: HealthChecker interface is now defined above

// SystemResources represents system resource information
type SystemResources struct {
	CPUUsage    float64   `json:"cpu_usage_percent"`
	MemoryUsage float64   `json:"memory_usage_percent"`
	DiskUsage   float64   `json:"disk_usage_percent"`
	OpenFiles   int       `json:"open_files"`
	GoRoutines  int       `json:"goroutines"`
	HeapSize    int64     `json:"heap_size_bytes"`
	LastUpdated time.Time `json:"last_updated"`
}

// SessionHealthStats represents session-related health statistics
type SessionHealthStats struct {
	ActiveSessions    int     `json:"active_sessions"`
	TotalSessions     int     `json:"total_sessions"`
	FailedSessions    int     `json:"failed_sessions"`
	AverageSessionAge float64 `json:"average_session_age_minutes"`
	SessionErrors     int     `json:"session_errors_last_hour"`
}

// CircuitBreakerStatus represents the status of a circuit breaker
type CircuitBreakerStatus struct {
	State         string    `json:"state"` // open, closed, half-open
	FailureCount  int       `json:"failure_count"`
	LastFailure   time.Time `json:"last_failure"`
	NextRetry     time.Time `json:"next_retry"`
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
}

// Circuit breaker states
const (
	CircuitBreakerClosed   = "closed"
	CircuitBreakerOpen     = "open"
	CircuitBreakerHalfOpen = "half-open"
)

// ServiceHealth represents the health of an external service
type ServiceHealth struct {
	Name         string                 `json:"name"`
	Status       string                 `json:"status"` // healthy, degraded, unhealthy
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// JobQueueStats represents job queue statistics
type JobQueueStats struct {
	QueuedJobs      int     `json:"queued_jobs"`
	RunningJobs     int     `json:"running_jobs"`
	CompletedJobs   int64   `json:"completed_jobs"`
	FailedJobs      int64   `json:"failed_jobs"`
	AverageWaitTime float64 `json:"average_wait_time_seconds"`
}

// RecentError represents a recent error for debugging
type RecentError struct {
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Severity  string                 `json:"severity"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// =============================================================================
// PROGRESS TRACKING TYPES (interface defined in main interfaces file)
// =============================================================================

// NOTE: ProgressReporter interface is now defined in pkg/mcp/interfaces.go
// ProgressTracker provides centralized progress reporting for tools
type ProgressTracker interface {
	// RunWithProgress executes an operation with standardized progress reporting
	RunWithProgress(
		ctx context.Context,
		operation string,
		stages []ProgressStage,
		fn func(ctx context.Context, reporter interface{}) error,
	) error
}

// NOTE: ProgressStage is defined above with ProgressReporter

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

// =============================================================================
// BASE TOOL INTERFACES (migrated from tools/base)
// =============================================================================

// NOTE: BaseAnalyzer and BaseValidator interfaces are defined in their respective packages:
// - BaseAnalyzer: pkg/mcp/internal/tools/base/analyzer.go
// - BaseValidator: pkg/mcp/internal/tools/base/validator.go

// BaseAnalysisOptions provides common options for analysis
type BaseAnalysisOptions struct {
	// Depth of analysis (shallow, normal, deep)
	Depth string

	// Specific aspects to analyze
	Aspects []string

	// Enable recommendations
	GenerateRecommendations bool

	// Custom analysis parameters
	CustomParams map[string]interface{}
}

// BaseValidationOptions provides common options for validation
type BaseValidationOptions struct {
	// Severity level for filtering issues
	Severity string

	// Rules to ignore during validation
	IgnoreRules []string

	// Enable strict validation mode
	StrictMode bool

	// Custom validation parameters
	CustomParams map[string]interface{}
}

// BaseAnalysisResult represents the result of analysis
type BaseAnalysisResult struct {
	// Summary of findings
	Summary BaseAnalysisSummary

	// Detailed findings
	Findings []BaseFinding

	// Recommendations based on analysis
	Recommendations []BaseRecommendation

	// Metrics collected during analysis
	Metrics map[string]interface{}

	// Risk assessment
	RiskAssessment BaseRiskAssessment

	// Additional context
	Context  map[string]interface{}
	Metadata BaseAnalysisMetadata
}

// BaseValidationResult represents the result of validation
type BaseValidationResult struct {
	// Overall validation status
	IsValid bool
	Score   int // 0-100

	// Issues found during validation
	Errors   []BaseValidationError
	Warnings []BaseValidationWarning

	// Summary statistics
	TotalIssues    int
	CriticalIssues int

	// Additional context
	Context  map[string]interface{}
	Metadata BaseValidationMetadata
}

// BaseAnalyzerCapabilities describes what an analyzer can do
type BaseAnalyzerCapabilities struct {
	SupportedTypes   []string
	SupportedAspects []string
	RequiresContext  bool
	SupportsDeepScan bool
}

// Support types for base interfaces
type BaseAnalysisSummary struct {
	TotalFindings    int
	CriticalFindings int
	Strengths        []string
	Weaknesses       []string
	OverallScore     int // 0-100
}

type BaseFinding struct {
	ID          string
	Type        string
	Category    string
	Severity    string
	Title       string
	Description string
	Evidence    []string
	Impact      string
	Location    BaseFindingLocation
}

type BaseFindingLocation struct {
	File      string
	Line      int
	Component string
	Context   string
}

type BaseRecommendation struct {
	ID          string
	Priority    string // high, medium, low
	Category    string
	Title       string
	Description string
	Benefits    []string
	Effort      string // low, medium, high
	Impact      string // low, medium, high
}

type BaseRiskAssessment struct {
	OverallRisk string // low, medium, high, critical
	RiskFactors []BaseRiskFactor
	Mitigations []BaseMitigation
}

type BaseRiskFactor struct {
	ID          string
	Category    string
	Description string
	Likelihood  string // low, medium, high
	Impact      string // low, medium, high
	Score       int
}

type BaseMitigation struct {
	RiskID        string
	Description   string
	Effort        string
	Effectiveness string
}

type BaseAnalysisMetadata struct {
	AnalyzerName    string
	AnalyzerVersion string
	Duration        time.Duration
	Timestamp       time.Time
	Parameters      map[string]interface{}
}

type BaseValidationError struct {
	Code          string
	Type          string
	Message       string
	Severity      string // critical, high, medium, low
	Location      BaseErrorLocation
	Fix           string
	Documentation string
}

type BaseValidationWarning struct {
	Code       string
	Type       string
	Message    string
	Suggestion string
	Impact     string // performance, security, maintainability, etc.
	Location   BaseWarningLocation
}

type BaseErrorLocation struct {
	File   string
	Line   int
	Column int
	Path   string // JSON path or similar
}

type BaseWarningLocation struct {
	File string
	Line int
	Path string
}

type BaseValidationMetadata struct {
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}
