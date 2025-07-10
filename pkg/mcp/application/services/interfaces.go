// Package services provides focused service interfaces as part of the EPSILON workstream.
// These interfaces replace the monolithic Manager interfaces with better testability
// and separation of concerns.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// PipelineService defines the interface for pipeline orchestration without importing the concrete type
type PipelineService interface {
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool

	// Job management
	CancelJob(ctx context.Context, jobID string) error
}

// ServiceContainer provides access to all services through dependency injection.
// This is the central interface for wiring services together.
type ServiceContainer interface {
	// Core services
	SessionStore() SessionStore
	SessionState() SessionState
	BuildExecutor() BuildExecutor
	ToolRegistry() ToolRegistry
	WorkflowExecutor() WorkflowExecutor
	Scanner() Scanner
	ConfigValidator() ConfigValidator
	ErrorReporter() ErrorReporter

	// Additional services
	StateManager() StateManager
	KnowledgeBase() KnowledgeBase
	K8sClient() K8sClient
	Analyzer() Analyzer
	Persistence() Persistence
	Logger() *slog.Logger

	// Conversation services
	ConversationService() ConversationService
	PromptService() PromptService

	// Infrastructure services
	DockerService() docker.Service
	ManifestService() kubernetes.ManifestService
	DeploymentService() kubernetes.Service
	PipelineService() PipelineService
	FileAccessService() FileAccessService
}

// ConversationService handles chat-based interactions per ADR-006
type ConversationService interface {
	// ProcessMessage handles a user message in a conversation
	ProcessMessage(ctx context.Context, sessionID, message string) (*ConversationResponse, error)

	// GetConversationState returns the current conversation state
	GetConversationState(ctx context.Context, sessionID string) (*ConversationState, error)

	// UpdateConversationStage updates the conversation stage
	UpdateConversationStage(ctx context.Context, sessionID string, stage domaintypes.ConversationStage) error

	// GetConversationHistory returns conversation history
	GetConversationHistory(ctx context.Context, sessionID string, limit int) ([]ConversationTurn, error)

	// ClearConversationContext clears conversation context
	ClearConversationContext(ctx context.Context, sessionID string) error
}

// PromptService manages AI prompt interactions per ADR-006
type PromptService interface {
	// BuildPrompt creates a prompt for the given stage and context
	BuildPrompt(ctx context.Context, stage domaintypes.ConversationStage, promptContext map[string]interface{}) (string, error)

	// ProcessPromptResponse processes AI response and updates state
	ProcessPromptResponse(ctx context.Context, response string, state *ConversationState) error

	// DetectWorkflowIntent detects if message indicates workflow intent
	DetectWorkflowIntent(ctx context.Context, message string) (*WorkflowIntent, error)

	// ShouldAutoAdvance determines if conversation should auto-advance
	ShouldAutoAdvance(ctx context.Context, state *ConversationState) (bool, *AutoAdvanceConfig)
}

// ConversationResponse represents a response from the conversation service
type ConversationResponse struct {
	SessionID     string                         `json:"session_id"`
	Message       string                         `json:"message"`
	Stage         domaintypes.ConversationStage  `json:"stage"`
	Status        string                         `json:"status"`
	Options       []Option                       `json:"options,omitempty"`
	Artifacts     []ArtifactSummary              `json:"artifacts,omitempty"`
	NextSteps     []string                       `json:"next_steps,omitempty"`
	Progress      *StageProgress                 `json:"progress,omitempty"`
	ToolCalls     []ToolCall                     `json:"tool_calls,omitempty"`
	RequiresInput bool                           `json:"requires_input"`
	NextStage     *domaintypes.ConversationStage `json:"next_stage,omitempty"`
	AutoAdvance   *AutoAdvanceConfig             `json:"auto_advance,omitempty"`
}

// ConversationState represents the state of a conversation
type ConversationState struct {
	SessionID        string                        `json:"session_id"`
	CurrentStage     domaintypes.ConversationStage `json:"current_stage"`
	History          []ConversationTurn            `json:"conversation_history"`
	Preferences      domaintypes.UserPreferences   `json:"user_preferences"`
	PendingDecision  *DecisionPoint                `json:"pending_decision,omitempty"`
	WorkflowSession  *WorkflowSession              `json:"workflow_session,omitempty"`
	LastActivity     time.Time                     `json:"last_activity"`
	RetryState       *RetryState                   `json:"retry_state,omitempty"`
	AutoAdvanceState *AutoAdvanceState             `json:"auto_advance_state,omitempty"`
}

// ConversationTurn represents a single turn in a conversation
type ConversationTurn struct {
	ID        string                        `json:"id"`
	Timestamp time.Time                     `json:"timestamp"`
	Role      string                        `json:"role"`
	Content   string                        `json:"content"`
	Stage     domaintypes.ConversationStage `json:"stage"`
	Metadata  map[string]interface{}        `json:"metadata,omitempty"`
}

// Supporting types for the conversation service
type Option struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ArtifactSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type StageProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type ToolCall struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Args   interface{} `json:"args"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type DecisionPoint struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []Option `json:"options"`
}

type WorkflowSession struct {
	SessionID string                 `json:"session_id"`
	Context   map[string]interface{} `json:"context"`
}

type RetryState struct {
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	LastError   string    `json:"last_error,omitempty"`
}

type AutoAdvanceConfig struct {
	Enabled bool          `json:"enabled"`
	Delay   time.Duration `json:"delay"`
}

type AutoAdvanceState struct {
	Scheduled bool      `json:"scheduled"`
	NextTime  time.Time `json:"next_time"`
}

type WorkflowIntent struct {
	Detected   bool                   `json:"detected"`
	Workflow   string                 `json:"workflow"`
	Parameters map[string]interface{} `json:"parameters"`
}

// SessionStore handles session persistence operations
type SessionStore interface {
	// Create creates a new session
	Create(ctx context.Context, session *api.Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*api.Session, error)

	// Update updates an existing session
	Update(ctx context.Context, session *api.Session) error

	// Delete removes a session
	Delete(ctx context.Context, sessionID string) error

	// List returns all sessions
	List(ctx context.Context) ([]*api.Session, error)
}

// SessionState manages session state and checkpoints
type SessionState interface {
	// SaveState saves the current state for a session
	SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error

	// GetState retrieves the state for a session
	GetState(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// CreateCheckpoint creates a checkpoint for rollback
	CreateCheckpoint(ctx context.Context, sessionID string, name string) error

	// RestoreCheckpoint restores to a previous checkpoint
	RestoreCheckpoint(ctx context.Context, sessionID string, name string) error

	// ListCheckpoints lists available checkpoints
	ListCheckpoints(ctx context.Context, sessionID string) ([]string, error)

	// GetWorkspaceDir gets the workspace directory for a session
	GetWorkspaceDir(ctx context.Context, sessionID string) (string, error)

	// SetWorkspaceDir sets the workspace directory for a session
	SetWorkspaceDir(ctx context.Context, sessionID string, dir string) error

	// GetSessionMetadata gets session metadata
	GetSessionMetadata(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// UpdateSessionData updates session data
	UpdateSessionData(ctx context.Context, sessionID string, data map[string]interface{}) error
}

// BuildExecutor handles container build operations
// This interface wraps the core docker.Service to provide focused build operations
type BuildExecutor interface {
	// QuickBuild performs a quick build without template generation
	QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options docker.BuildOptions) (*docker.BuildResult, error)

	// QuickPush performs a quick push of an already built image
	QuickPush(ctx context.Context, imageRef string, options docker.PushOptions) (*docker.RegistryPushResult, error)

	// QuickPull performs a quick pull of an image
	QuickPull(ctx context.Context, imageRef string) (*docker.PullResult, error)
}

// ToolRegistry is an alias to the canonical api ToolRegistry interface
// This resolves interface validation conflicts while maintaining compatibility
type ToolRegistry = api.ToolRegistry

// WorkflowExecutor handles multi-step workflow execution
type WorkflowExecutor interface {
	// ExecuteWorkflow runs a complete workflow
	ExecuteWorkflow(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error)

	// ExecuteStep executes a single workflow step
	ExecuteStep(ctx context.Context, step *api.WorkflowStep) (*api.StepResult, error)

	// ValidateWorkflow validates a workflow definition
	ValidateWorkflow(ctx context.Context, workflow *api.Workflow) error
}

// Scanner provides security scanning capabilities
// This interface wraps the core security.Service
type Scanner interface {
	// ScanImage scans a container image for vulnerabilities
	ScanImage(ctx context.Context, image string, options security.ScanOptionsService) (*security.ScanResult, error)

	// ScanDirectory scans a directory for secrets
	ScanDirectory(ctx context.Context, path string, options security.ScanOptionsService) (*security.ScanResult, error)

	// GetAvailableScanners returns available scanner types
	GetAvailableScanners(ctx context.Context) []string
}

// ConfigValidator validates configuration using BETA's validation framework
type ConfigValidator interface {
	// ValidateDockerfile validates a Dockerfile
	ValidateDockerfile(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateManifest validates a Kubernetes manifest
	ValidateManifest(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateConfig validates general configuration
	ValidateConfig(ctx context.Context, config map[string]interface{}) (*ValidationResult, error)
}

// ErrorReporter provides unified error reporting and recovery
type ErrorReporter interface {
	// ReportError reports an error with context
	ReportError(ctx context.Context, err error, context map[string]interface{})

	// GetErrorStats returns error statistics
	GetErrorStats(ctx context.Context) ErrorStats

	// SuggestFix suggests fixes for common errors
	SuggestFix(ctx context.Context, err error) []string
}

// StateManager manages application state beyond sessions
type StateManager interface {
	// SaveState saves state with a key
	SaveState(ctx context.Context, key string, state interface{}) error

	// GetState retrieves state by key
	GetState(ctx context.Context, key string, state interface{}) error

	// UpdateState updates existing state
	UpdateState(ctx context.Context, key string, tool string, state interface{}) error

	// DeleteState removes state
	DeleteState(ctx context.Context, key string) error
}

// KnowledgeBase stores and retrieves learned patterns
type KnowledgeBase interface {
	// Store stores a pattern or solution
	Store(ctx context.Context, key string, value interface{}) error

	// Retrieve gets a stored pattern
	Retrieve(ctx context.Context, key string) (interface{}, error)

	// Search searches for patterns
	Search(ctx context.Context, query string) ([]interface{}, error)
}

// K8sClient provides Kubernetes operations
type K8sClient interface {
	// Apply applies manifests to a cluster
	Apply(ctx context.Context, manifests string, namespace string) error

	// Delete removes resources from a cluster
	Delete(ctx context.Context, manifests string, namespace string) error

	// GetStatus gets resource status
	GetStatus(ctx context.Context, resource, name, namespace string) (interface{}, error)
}

// Analyzer provides code and repository analysis
// This interface wraps the core analysis.RepositoryAnalyzer
type Analyzer interface {
	// AnalyzeRepository analyzes a repository (matches core analysis.RepositoryAnalyzer)
	AnalyzeRepository(ctx context.Context, repoPath string) (*analysis.AnalysisResult, error)
}

// AnalysisService provides analysis operations for backward compatibility
type AnalysisService interface {
	// AnalyzeRepository analyzes a repository with progress callback
	AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error)

	// AnalyzeWithAI performs AI-powered analysis
	AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)

	// GetAnalysisProgress gets the progress of an ongoing analysis
	GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error)
}

// Supporting types

// Use core docker types directly
// type BuildOptions = docker.BuildOptions
// type BuildResult = docker.BuildResult
// type PushOptions = docker.PushOptions
// type PullOptions = docker.PullOptions

// Use core security types directly
// type ScanOptions = security.ScanOptionsService
// type ScanResult = security.ScanResult

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
	Score    int
}

// ValidationError represents a validation error
type ValidationError struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// ErrorStats contains error statistics
type ErrorStats struct {
	TotalErrors  int64
	ErrorsByType map[string]int64
	RecentErrors []ErrorEntry
	RecoveryRate float64
}

// ErrorEntry represents a single error occurrence
type ErrorEntry struct {
	Timestamp time.Time
	Error     string
	Context   map[string]interface{}
	Fixed     bool
}

// Use the core analysis.AnalysisResult directly
// type AnalysisResult = analysis.AnalysisResult

// Persistence handles persistent storage operations
type Persistence interface {
	// Put stores a key-value pair
	Put(ctx context.Context, bucket string, key string, value interface{}) error

	// Get retrieves a value by key
	Get(ctx context.Context, bucket string, key string, result interface{}) error

	// Delete removes a key-value pair
	Delete(ctx context.Context, bucket string, key string) error

	// List returns all key-value pairs in a bucket
	List(ctx context.Context, bucket string) (map[string]interface{}, error)

	// Close closes the persistence layer
	Close() error
}

// Analysis-related types for backward compatibility

// ProgressCallback is called during long-running operations to report progress
type ProgressCallback func(progress AnalysisProgress)

// RepositoryAnalysis represents the result of analyzing a repository
type RepositoryAnalysis struct {
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	Dependencies    []string               `json:"dependencies"`
	EntryPoint      string                 `json:"entry_point"`
	Port            int                    `json:"port"`
	BuildCommand    string                 `json:"build_command"`
	RunCommand      string                 `json:"run_command"`
	Issues          []AnalysisIssue        `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// AnalysisIssue represents an issue found during analysis
type AnalysisIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Suggestion string `json:"suggestion"`
}

// AIAnalysis represents the result of AI-powered analysis
type AIAnalysis struct {
	Summary         string                 `json:"summary"`
	Insights        []string               `json:"insights"`
	Recommendations []string               `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// AnalysisProgress represents the progress of an ongoing analysis
type AnalysisProgress struct {
	AnalysisID    string         `json:"analysis_id"`
	Status        string         `json:"status"`
	CurrentStep   string         `json:"current_step"`
	StepNumber    int            `json:"step_number"`
	TotalSteps    int            `json:"total_steps"`
	Percentage    float64        `json:"percentage"`
	ElapsedTime   time.Duration  `json:"elapsed_time"`
	EstimatedTime *time.Duration `json:"estimated_time,omitempty"`
	LastUpdate    time.Time      `json:"last_update"`
}

// FileAccessService provides secure file access operations for MCP tools
type FileAccessService interface {
	// ReadFile reads a file within the session workspace
	ReadFile(ctx context.Context, sessionID, path string) (string, error)

	// ListDirectory lists files and directories within the session workspace
	ListDirectory(ctx context.Context, sessionID, path string) ([]FileInfo, error)

	// FileExists checks if a file exists within the session workspace
	FileExists(ctx context.Context, sessionID, path string) (bool, error)

	// GetFileTree returns a tree representation of the directory structure
	GetFileTree(ctx context.Context, sessionID, rootPath string) (string, error)

	// ReadFileWithMetadata reads a file with additional metadata
	ReadFileWithMetadata(ctx context.Context, sessionID, path string) (*FileContent, error)

	// SearchFiles searches for files matching a pattern within the session workspace
	SearchFiles(ctx context.Context, sessionID, pattern string) ([]FileInfo, error)
}

// FileInfo represents file information
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
	Mode    string    `json:"mode"`
}

// FileContent represents file content with metadata
type FileContent struct {
	Path     string    `json:"path"`
	Content  string    `json:"content"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Encoding string    `json:"encoding"`
	Lines    int       `json:"lines"`
}

// AI Analysis Service interfaces and types

// AIAnalysisService provides AI-powered analysis capabilities
type AIAnalysisService interface {
	// AnalyzeCodePatterns performs AI analysis of code patterns and architecture
	AnalyzeCodePatterns(ctx context.Context, files map[string]string) (*CodeAnalysisResult, error)

	// SuggestDockerfileOptimizations provides AI-powered Dockerfile optimization suggestions
	SuggestDockerfileOptimizations(ctx context.Context, dockerfile string, optContext *OptimizationContext) (*DockerfileOptimizations, error)

	// DetectSecurityIssues uses AI to detect potential security vulnerabilities
	DetectSecurityIssues(ctx context.Context, code string, language string) (*SecurityAnalysisResult, error)

	// AnalyzePerformance provides performance analysis recommendations
	AnalyzePerformance(ctx context.Context, code string, metrics map[string]interface{}) (*PerformanceAnalysisResult, error)

	// SuggestContainerizationApproach recommends containerization strategies
	SuggestContainerizationApproach(ctx context.Context, repoAnalysis *RepositoryAnalysis) (*ContainerizationRecommendations, error)

	// ValidateConfiguration validates configuration files with AI assistance
	ValidateConfiguration(ctx context.Context, configType string, content string) (*ConfigurationResult, error)

	// GetCachedAnalysis retrieves cached analysis results
	GetCachedAnalysis(ctx context.Context, cacheKey string, timeRange *TimeRange) (*CachedAnalysis, error)
}

// CachedAnalysis represents cached analysis data
type CachedAnalysis struct {
	Key       string                 `json:"key"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// OptimizationContext provides context for optimization suggestions
type OptimizationContext struct {
	Language       string                 `json:"language"`
	Framework      string                 `json:"framework"`
	TargetSize     string                 `json:"target_size"`               // "minimal", "balanced", "feature-rich"
	Environment    string                 `json:"environment"`               // "development", "production", "testing"
	TargetPlatform string                 `json:"target_platform,omitempty"` // "linux/amd64", "linux/arm64", etc.
	Dependencies   []string               `json:"dependencies,omitempty"`
	Constraints    map[string]interface{} `json:"constraints,omitempty"`
	Preferences    map[string]interface{} `json:"preferences,omitempty"`
}

// DockerfileOptimizations contains Dockerfile optimization recommendations
type DockerfileOptimizations struct {
	OriginalSize     int64                    `json:"original_size"`
	EstimatedSize    int64                    `json:"estimated_size"`
	SizeReduction    float64                  `json:"size_reduction"`
	BuildTime        time.Duration            `json:"build_time"`
	SecurityScore    float64                  `json:"security_score"`
	OptimizedContent string                   `json:"optimized_content"`
	Optimizations    []DockerfileOptimization `json:"optimizations"`
	RecommendedBase  string                   `json:"recommended_base"`
	SecurityIssues   []SecurityIssue          `json:"security_issues"`
	BestPractices    []BestPractice           `json:"best_practices"`
	Confidence       float64                  `json:"confidence"`
}

// DockerfileOptimization represents a single dockerfile optimization
type DockerfileOptimization struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`
	Priority    string  `json:"priority"`
	Before      string  `json:"before"`
	After       string  `json:"after"`
	Confidence  float64 `json:"confidence"`
}

// OptimizationSuggestion represents a single optimization recommendation
type OptimizationSuggestion struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`
	Priority    string  `json:"priority"`
	Before      string  `json:"before"`
	After       string  `json:"after"`
	Confidence  float64 `json:"confidence"`
}

// SecurityAnalysisResult contains security analysis results
type SecurityAnalysisResult struct {
	OverallRisk     string                   `json:"overall_risk"`
	SecurityScore   float64                  `json:"security_score"`
	Issues          []SecurityIssue          `json:"issues"`
	Recommendations []SecurityRecommendation `json:"recommendations"`
	Compliance      ComplianceReport         `json:"compliance"`
	Confidence      float64                  `json:"confidence"`
}

// SecurityRecommendation represents a security improvement recommendation
type SecurityRecommendation struct {
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
}

// ComplianceReport contains compliance assessment results
type ComplianceReport struct {
	Standards map[string]ComplianceResult `json:"standards"`
}

// ComplianceResult represents compliance with a specific standard
type ComplianceResult struct {
	Score       float64  `json:"score"`
	Violations  []string `json:"violations"`
	Suggestions []string `json:"suggestions"`
}

// SecurityIssue represents a security vulnerability or issue
type SecurityIssue struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Fix         string `json:"fix,omitempty"`
	Reference   string `json:"reference,omitempty"`
}

// PerformanceAnalysisResult contains performance analysis recommendations
type PerformanceAnalysisResult struct {
	OverallScore     float64                   `json:"overall_score"`
	Bottlenecks      []PerformanceBottleneck   `json:"bottlenecks"`
	Optimizations    []PerformanceOptimization `json:"optimizations"`
	ScalabilityScore float64                   `json:"scalability_score"`
	Recommendations  []string                  `json:"recommendations"`
	OptimizedMetrics map[string]interface{}    `json:"optimized_metrics"`
	Confidence       float64                   `json:"confidence"`
}

// PerformanceIssue represents a performance bottleneck or issue
type PerformanceIssue struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      string  `json:"impact"`
	File        string  `json:"file,omitempty"`
	Line        int     `json:"line,omitempty"`
	Fix         string  `json:"fix,omitempty"`
	Priority    string  `json:"priority"`
	Confidence  float64 `json:"confidence"`
}

// ContainerizationRecommendations contains containerization strategy recommendations
type ContainerizationRecommendations struct {
	Strategy        string                 `json:"strategy"`
	BaseImage       string                 `json:"base_image"`
	MultiStage      bool                   `json:"multi_stage"`
	BuildSteps      []BuildStep            `json:"build_steps"`
	RuntimeConfig   map[string]interface{} `json:"runtime_config"`
	SecurityConfig  map[string]interface{} `json:"security_config"`
	Recommendations []string               `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
}

// BuildStep represents a single build step in the containerization process
type BuildStep struct {
	Stage       string   `json:"stage"`
	Commands    []string `json:"commands"`
	Description string   `json:"description"`
	Purpose     string   `json:"purpose"`
}

// ConfigurationResult contains configuration validation results
type ConfigurationResult struct {
	Valid           bool                   `json:"valid"`
	Issues          []ConfigurationIssue   `json:"issues"`
	Suggestions     []string               `json:"suggestions"`
	Score           float64                `json:"score"`
	OptimizedConfig map[string]interface{} `json:"optimized_config,omitempty"`
	Confidence      float64                `json:"confidence"`
}

// ConfigurationIssue represents a configuration problem
type ConfigurationIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`
	Fix         string `json:"fix,omitempty"`
}

// TimeRange represents a time range for cache queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CodeAnalysisResult contains the results of code pattern analysis
type CodeAnalysisResult struct {
	Summary         string                 `json:"summary"`
	Architecture    ArchitectureAnalysis   `json:"architecture"`
	CodeQuality     CodeQualityMetrics     `json:"code_quality"`
	Patterns        []DetectedPattern      `json:"patterns"`
	Dependencies    []DependencyAnalysis   `json:"dependencies"`
	Recommendations []string               `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ArchitectureAnalysis contains architecture-related analysis
type ArchitectureAnalysis struct {
	Style           string   `json:"style"`
	Layers          []string `json:"layers"`
	Patterns        []string `json:"patterns"`
	Violations      []string `json:"violations"`
	Complexity      float64  `json:"complexity"`
	Maintainability float64  `json:"maintainability"`
}

// CodeQualityMetrics contains code quality measurements
type CodeQualityMetrics struct {
	Readability   float64 `json:"readability"`
	Testability   float64 `json:"testability"`
	Modularity    float64 `json:"modularity"`
	Documentation float64 `json:"documentation"`
	ErrorHandling float64 `json:"error_handling"`
	Performance   float64 `json:"performance"`
	Security      float64 `json:"security"`
	OverallScore  float64 `json:"overall_score"`
}

// DetectedPattern represents a detected code pattern
type DetectedPattern struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Confidence  float64  `json:"confidence"`
	Files       []string `json:"files"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`
}

// DependencyAnalysis contains dependency analysis results
type DependencyAnalysis struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Type            string   `json:"type"`
	Risk            string   `json:"risk"`
	Vulnerabilities []string `json:"vulnerabilities"`
	Alternatives    []string `json:"alternatives"`
	Usage           string   `json:"usage"`
}

// BestPractice represents a best practice recommendation
type BestPractice struct {
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Reference   string `json:"reference,omitempty"`
}

// AIUsageMetrics contains AI service usage metrics
type AIUsageMetrics struct {
	TotalRequests      int64                     `json:"total_requests"`
	SuccessfulRequests int64                     `json:"successful_requests"`
	FailedRequests     int64                     `json:"failed_requests"`
	TotalTokens        int64                     `json:"total_tokens"`
	InputTokens        int64                     `json:"input_tokens"`
	OutputTokens       int64                     `json:"output_tokens"`
	TotalCost          float64                   `json:"total_cost"`
	AverageCost        float64                   `json:"average_cost"`
	CostBreakdown      map[string]float64        `json:"cost_breakdown"`
	ResponseTimes      ResponseTimeMetrics       `json:"response_times"`
	Usage              map[string]OperationUsage `json:"usage"`
	SuccessRate        float64                   `json:"success_rate"`
}

// ResponseTimeMetrics contains response time statistics
type ResponseTimeMetrics struct {
	Average    time.Duration `json:"average"`
	Min        time.Duration `json:"min"`
	Max        time.Duration `json:"max"`
	Median     time.Duration `json:"median"`
	P50        time.Duration `json:"p50"`
	P95        time.Duration `json:"p95"`
	P99        time.Duration `json:"p99"`
	SampleSize int           `json:"sample_size"`
}

// Additional types for AI analysis service

// PerformanceBottleneck represents a performance issue
type PerformanceBottleneck struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Solution    string `json:"solution"`
	Improvement string `json:"estimated_improvement"`
}

// PerformanceOptimization represents a performance improvement opportunity
type PerformanceOptimization struct {
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
	Impact      string `json:"implementation"`
	Benefit     string `json:"expected_benefit"`
	Effort      string `json:"effort"`
	Code        string `json:"code,omitempty"`
}

// OperationUsage tracks usage statistics for specific operations
type OperationUsage struct {
	Count         int64               `json:"count"`
	TotalTokens   int64               `json:"total_tokens"`
	TotalCost     float64             `json:"total_cost"`
	AverageTokens float64             `json:"average_tokens"`
	ResponseTime  ResponseTimeMetrics `json:"response_time"`
	Errors        int64               `json:"errors"`
	ErrorRate     float64             `json:"error_rate"`
}
